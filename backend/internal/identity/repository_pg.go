package identity

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"sohopos/internal/platform/apperr"
	"sohopos/internal/platform/database"
)

type PGRepository struct{ db *database.Store }

func NewPGRepository(db *database.Store) *PGRepository { return &PGRepository{db: db} }

func scanUser(row pgx.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.DisplayName, &u.PasswordHash, &u.Role, &u.IsActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *PGRepository) FindByUsername(ctx context.Context, username string) (*User, error) {
	return scanUser(r.db.DB(ctx).QueryRow(ctx,
		`SELECT id, username, display_name, password_hash, role, is_active
		   FROM users WHERE lower(username) = lower($1)`, username))
}

func (r *PGRepository) FindByID(ctx context.Context, id uuid.UUID) (*User, error) {
	return scanUser(r.db.DB(ctx).QueryRow(ctx,
		`SELECT id, username, display_name, password_hash, role, is_active
		   FROM users WHERE id = $1`, id))
}

func (r *PGRepository) CountUsers(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.DB(ctx).QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n)
	return n, err
}

func (r *PGRepository) Create(ctx context.Context, u *User) error {
	err := r.db.DB(ctx).QueryRow(ctx,
		`INSERT INTO users (username, display_name, password_hash, role)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		u.Username, u.DisplayName, u.PasswordHash, u.Role).Scan(&u.ID)
	if name, ok := database.UniqueViolation(err); ok && name == "users_username_key" {
		return apperr.Conflict("username_taken", "That username is already in use.")
	}
	return err
}
