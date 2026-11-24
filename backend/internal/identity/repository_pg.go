package identity

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"chloropos/internal/platform/apperr"
	"chloropos/internal/platform/database"
)

// The identity tables (users, cashier_seats) are the platform's own and carry no row-level security:
// signing in has to look an account up before any tenant is known. Every seat query below filters by
// tenant_id itself, and the tenant ID always comes from the verified session.

type PGRepository struct{ db *database.Store }

func NewPGRepository(db *database.Store) *PGRepository { return &PGRepository{db: db} }

const userCols = `id, username, display_name, password_hash, role, is_active, sessions_valid_after, max_cashiers`

type scanner interface{ Scan(dest ...any) error }

func scanUser(row scanner) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.DisplayName, &u.PasswordHash, &u.Role, &u.IsActive, &u.SessionsValidAfter, &u.MaxCashiers)
	return u, err
}

func oneUser(row pgx.Row) (*User, error) {
	u, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *PGRepository) FindByUsername(ctx context.Context, username string) (*User, error) {
	return oneUser(r.db.DB(ctx).QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE lower(username) = lower($1)`, username))
}

func (r *PGRepository) FindByID(ctx context.Context, id uuid.UUID) (*User, error) {
	return oneUser(r.db.DB(ctx).QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE id = $1`, id))
}

func (r *PGRepository) CountAdmins(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.DB(ctx).QueryRow(ctx, `SELECT count(*) FROM users WHERE role = 'admin'`).Scan(&n)
	return n, err
}

func (r *PGRepository) Create(ctx context.Context, u *User) error {
	err := r.db.DB(ctx).QueryRow(ctx,
		`INSERT INTO users (username, display_name, password_hash, role, max_cashiers)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		u.Username, u.DisplayName, u.PasswordHash, u.Role, u.MaxCashiers).Scan(&u.ID)
	if name, ok := database.UniqueViolation(err); ok && name == "users_username_key" {
		return apperr.Conflict("username_taken", "Username itu sudah dipakai.")
	}
	return err
}

func (r *PGRepository) ListTenants(ctx context.Context, limit int) ([]Tenant, error) {
	rows, err := r.db.DB(ctx).Query(ctx,
		`SELECT u.id, u.username, u.display_name, u.password_hash, u.role, u.is_active, u.sessions_valid_after, u.max_cashiers,
		        (SELECT count(*) FROM cashier_seats s WHERE s.tenant_id = u.id AND s.is_active)
		   FROM users u
		  WHERE u.role = 'owner'
		  ORDER BY lower(u.display_name), u.id
		  LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Tenant, 0, 16)
	for rows.Next() {
		var t Tenant
		if err := rows.Scan(&t.ID, &t.Username, &t.DisplayName, &t.PasswordHash, &t.Role, &t.IsActive,
			&t.SessionsValidAfter, &t.MaxCashiers, &t.SeatsUsed); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *PGRepository) UpdateTenant(ctx context.Context, id uuid.UUID, displayName string, active bool, maxCashiers *int) error {
	tag, err := r.db.DB(ctx).Exec(ctx,
		`UPDATE users
		    SET display_name = $2,
		        is_active = $3::bool,
		        max_cashiers = COALESCE($4::int, max_cashiers),
		        sessions_valid_after = CASE WHEN $3::bool THEN sessions_valid_after ELSE now() END,
		        updated_at = now()
		  WHERE id = $1 AND role = 'owner'`, id, displayName, active, maxCashiers)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PGRepository) SetTenantPassword(ctx context.Context, id uuid.UUID, hash string) error {
	tag, err := r.db.DB(ctx).Exec(ctx,
		`UPDATE users SET password_hash = $2, sessions_valid_after = now(), updated_at = now()
		  WHERE id = $1 AND role = 'owner'`, id, hash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- kasir seats ----

const seatCols = `id, tenant_id, name, code_selector, code_hash, is_active, sessions_valid_after, created_at`

func scanSeat(row scanner) (Seat, error) {
	var s Seat
	err := row.Scan(&s.ID, &s.TenantID, &s.Name, &s.Selector, &s.CodeHash, &s.IsActive, &s.SessionsValidAfter, &s.CreatedAt)
	return s, err
}

func oneSeat(row pgx.Row) (*Seat, error) {
	s, err := scanSeat(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *PGRepository) FindSeatBySelector(ctx context.Context, tenantID uuid.UUID, selector string) (*Seat, error) {
	return oneSeat(r.db.DB(ctx).QueryRow(ctx,
		`SELECT `+seatCols+` FROM cashier_seats WHERE tenant_id = $1 AND code_selector = $2`, tenantID, selector))
}

func (r *PGRepository) FindSeat(ctx context.Context, tenantID, id uuid.UUID) (*Seat, error) {
	return oneSeat(r.db.DB(ctx).QueryRow(ctx,
		`SELECT `+seatCols+` FROM cashier_seats WHERE tenant_id = $1 AND id = $2`, tenantID, id))
}

// FindSeatForSession looks a seat up by ID alone, as a session check must. The seat counts as
// active only while its owner is active too.
func (r *PGRepository) FindSeatForSession(ctx context.Context, id uuid.UUID) (*Seat, error) {
	var s Seat
	err := r.db.DB(ctx).QueryRow(ctx,
		`SELECT s.id, s.tenant_id, s.name, s.code_selector, s.code_hash, (s.is_active AND u.is_active),
		        s.sessions_valid_after, s.created_at, u.username
		   FROM cashier_seats s JOIN users u ON u.id = s.tenant_id
		  WHERE s.id = $1`, id).
		Scan(&s.ID, &s.TenantID, &s.Name, &s.Selector, &s.CodeHash, &s.IsActive, &s.SessionsValidAfter, &s.CreatedAt, &s.TenantUsername)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *PGRepository) ListSeats(ctx context.Context, tenantID uuid.UUID) ([]Seat, error) {
	rows, err := r.db.DB(ctx).Query(ctx,
		`SELECT `+seatCols+` FROM cashier_seats WHERE tenant_id = $1 ORDER BY created_at, id LIMIT 200`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Seat, 0, 4)
	for rows.Next() {
		s, err := scanSeat(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// lockTenant locks the owner's row and returns their seat limit. Seat changes that could exceed
// the limit queue behind this lock, so two requests can never both take the last free seat.
func lockTenant(ctx context.Context, db database.DBTX, tenantID uuid.UUID) (int, error) {
	var limit int
	err := db.QueryRow(ctx,
		`SELECT max_cashiers FROM users WHERE id = $1 AND role = 'owner' AND is_active FOR UPDATE`, tenantID).Scan(&limit)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	return limit, err
}

func (r *PGRepository) CreateSeat(ctx context.Context, s *Seat) error {
	db := r.db.DB(ctx)
	limit, err := lockTenant(ctx, db, s.TenantID)
	if err != nil {
		return err
	}
	var used int
	if err := db.QueryRow(ctx,
		`SELECT count(*) FROM cashier_seats WHERE tenant_id = $1 AND is_active`, s.TenantID).Scan(&used); err != nil {
		return err
	}
	if used >= limit {
		return &SeatLimitError{Limit: limit}
	}
	err = db.QueryRow(ctx,
		`INSERT INTO cashier_seats (tenant_id, name, code_selector, code_hash)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, is_active, sessions_valid_after, created_at`,
		s.TenantID, s.Name, s.Selector, s.CodeHash).Scan(&s.ID, &s.IsActive, &s.SessionsValidAfter, &s.CreatedAt)
	return mapSeatWriteErr(err)
}

func (r *PGRepository) UpdateSeat(ctx context.Context, tenantID, id uuid.UUID, name string, active bool) error {
	db := r.db.DB(ctx)
	limit, err := lockTenant(ctx, db, tenantID)
	if err != nil {
		return err
	}
	var wasActive bool
	err = db.QueryRow(ctx,
		`SELECT is_active FROM cashier_seats WHERE id = $1 AND tenant_id = $2 FOR UPDATE`, id, tenantID).Scan(&wasActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if active && !wasActive { // switching a seat back on takes a seat, so the limit applies again
		var used int
		if err := db.QueryRow(ctx,
			`SELECT count(*) FROM cashier_seats WHERE tenant_id = $1 AND is_active`, tenantID).Scan(&used); err != nil {
			return err
		}
		if used >= limit {
			return &SeatLimitError{Limit: limit}
		}
	}
	_, err = db.Exec(ctx,
		`UPDATE cashier_seats
		    SET name = $3,
		        is_active = $4::bool,
		        sessions_valid_after = CASE WHEN $4::bool THEN sessions_valid_after ELSE now() END,
		        updated_at = now()
		  WHERE id = $1 AND tenant_id = $2`, id, tenantID, name, active)
	return mapSeatWriteErr(err)
}

func (r *PGRepository) SetSeatCode(ctx context.Context, tenantID, id uuid.UUID, selector, codeHash string) error {
	tag, err := r.db.DB(ctx).Exec(ctx,
		`UPDATE cashier_seats
		    SET code_selector = $3, code_hash = $4, sessions_valid_after = now(), updated_at = now()
		  WHERE id = $1 AND tenant_id = $2`, id, tenantID, selector, codeHash)
	if err != nil {
		return mapSeatWriteErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func mapSeatWriteErr(err error) error {
	if name, ok := database.UniqueViolation(err); ok {
		switch name {
		case "cashier_seats_selector_key":
			return errSelectorTaken
		case "cashier_seats_name_key":
			return apperr.Conflict("seat_name_taken", "Sudah ada kasir dengan nama itu.")
		}
	}
	return err
}
