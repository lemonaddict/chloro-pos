package identity

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"chloropos/internal/platform/apperr"
	"chloropos/internal/platform/auth"
	"chloropos/internal/platform/database"
)

const (
	sessionCacheTTL  = 10 * time.Second
	maxTenantsListed = 500
)

var (
	errBadCredentials = apperr.Unauthorized("invalid_credentials", "Username atau kata sandi salah.")
	errBadCashierCode = apperr.Unauthorized("invalid_credentials", "Username toko atau kode kasir salah.")
	errSessionEnded   = apperr.Unauthorized("unauthenticated", "Sesi sudah berakhir. Silakan masuk lagi.")
	errBadLimit       = apperr.Invalid("invalid_limit", "Jatah kasir harus antara 0 dan 100.")
	errBadSeatName    = apperr.Invalid("invalid_name", "Nama kasir harus 1 sampai 40 karakter.")
	usernameRe        = regexp.MustCompile(`^[a-z0-9._-]{3,40}$`)
)

// principal is what a session is checked against: who the database says this account is right now.
type principal struct {
	role       string
	tenantID   uuid.UUID // uuid.Nil for the admin
	active     bool
	validAfter time.Time
}

type cachedPrincipal struct {
	p  *principal
	at time.Time
}

type Service struct {
	repo      Repository
	tx        database.Transactor
	tokens    *auth.TokenManager
	dummyHash string // verified against when the account doesn't exist, so timing doesn't reveal it

	mu    sync.Mutex
	cache map[uuid.UUID]cachedPrincipal
}

func NewService(repo Repository, tx database.Transactor, tokens *auth.TokenManager) *Service {
	dummy, err := auth.HashPassword("placeholder-for-unknown-users")
	if err != nil {
		panic(err) // the OS random source is broken; nothing safe can run
	}
	return &Service{repo: repo, tx: tx, tokens: tokens, dummyHash: dummy, cache: make(map[uuid.UUID]cachedPrincipal)}
}

type Session struct {
	Identity  Identity
	Token     string
	CSRF      string
	ExpiresAt time.Time
}

func identityOf(u *User) Identity {
	return Identity{ID: u.ID, Username: u.Username, DisplayName: u.DisplayName, Role: u.Role, IsActive: u.IsActive}
}

// burn does the hashing work of a real check, so a missing account takes as long as a wrong password.
func (s *Service) burn() { auth.VerifyPassword("placeholder", s.dummyHash) }

// Login signs in an owner or the admin with username and password. Every failure gets the same
// message and does the same hashing work.
func (s *Service) Login(ctx context.Context, username, password string) (*Session, error) {
	username = strings.TrimSpace(username)
	if username == "" || len(username) > 40 || password == "" || len(password) > 128 {
		return nil, errBadCredentials
	}
	u, err := s.repo.FindByUsername(ctx, username)
	if errors.Is(err, ErrNotFound) {
		s.burn()
		return nil, errBadCredentials
	}
	if err != nil {
		return nil, err
	}
	if !auth.VerifyPassword(password, u.PasswordHash) || !u.IsActive {
		return nil, errBadCredentials
	}
	tenantID := uuid.Nil
	if u.Role == RoleOwner {
		tenantID = u.ID
	}
	token, csrf, exp, err := s.tokens.Issue(u.ID, u.Role, tenantID)
	if err != nil {
		return nil, err
	}
	return &Session{Identity: identityOf(u), Token: token, CSRF: csrf, ExpiresAt: exp}, nil
}

// LoginCashier signs in a kasir seat: the shop's username plus the seat's own code. The owner's
// password is never part of it, so handing a kasir their code does not hand them the shop.
func (s *Service) LoginCashier(ctx context.Context, username, rawCode string) (*Session, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	code, ok := parseCashierCode(rawCode)
	if !ok || username == "" || len(username) > 40 {
		s.burn()
		return nil, errBadCashierCode
	}
	tenant, err := s.repo.FindByUsername(ctx, username)
	if errors.Is(err, ErrNotFound) {
		s.burn()
		return nil, errBadCashierCode
	}
	if err != nil {
		return nil, err
	}
	if tenant.Role != RoleOwner || !tenant.IsActive {
		s.burn()
		return nil, errBadCashierCode
	}
	seat, err := s.repo.FindSeatBySelector(ctx, tenant.ID, code.Selector)
	if errors.Is(err, ErrNotFound) {
		s.burn()
		return nil, errBadCashierCode
	}
	if err != nil {
		return nil, err
	}
	if !auth.VerifyPassword(code.Secret, seat.CodeHash) || !seat.IsActive {
		return nil, errBadCashierCode
	}
	token, csrf, exp, err := s.tokens.Issue(seat.ID, RoleCashier, tenant.ID)
	if err != nil {
		return nil, err
	}
	id := Identity{ID: seat.ID, Username: tenant.Username, DisplayName: seat.Name, Role: RoleCashier, IsActive: true}
	return &Session{Identity: id, Token: token, CSRF: csrf, ExpiresAt: exp}, nil
}

// SessionValid is called on every authenticated request. Accounts are cached for a few seconds,
// so deactivating a kasir seat (or a whole shop) takes effect almost immediately without a query
// per request.
func (s *Service) SessionValid(ctx context.Context, c *auth.Claims) bool {
	id, err := c.UserID()
	if err != nil {
		return false
	}
	p, err := s.principal(ctx, c.Role, id)
	if err != nil {
		return false // unknown account or database trouble: fail closed
	}
	if !p.active || p.role != c.Role {
		return false
	}
	// The tenant in the token must be the one the database says this account belongs to.
	switch p.role {
	case RoleAdmin:
		if c.TID != "" {
			return false
		}
	default:
		if p.tenantID == uuid.Nil || c.TID != p.tenantID.String() {
			return false
		}
	}
	if c.IssuedAt != nil && c.IssuedAt.Time.Before(p.validAfter.Truncate(time.Second)) {
		return false
	}
	return true
}

func (s *Service) principal(ctx context.Context, role string, id uuid.UUID) (*principal, error) {
	s.mu.Lock()
	if e, ok := s.cache[id]; ok && e.p.role == role && time.Since(e.at) < sessionCacheTTL {
		s.mu.Unlock()
		return e.p, nil
	}
	s.mu.Unlock()

	var p *principal
	switch role {
	case RoleAdmin, RoleOwner:
		u, err := s.repo.FindByID(ctx, id)
		if err != nil {
			return nil, err
		}
		p = &principal{role: u.Role, active: u.IsActive, validAfter: u.SessionsValidAfter}
		if u.Role == RoleOwner {
			p.tenantID = u.ID
		}
	case RoleCashier:
		seat, err := s.repo.FindSeatForSession(ctx, id)
		if err != nil {
			return nil, err
		}
		p = &principal{role: RoleCashier, tenantID: seat.TenantID, active: seat.IsActive, validAfter: seat.SessionsValidAfter}
	default:
		return nil, ErrNotFound
	}
	s.mu.Lock()
	s.cache[id] = cachedPrincipal{p: p, at: time.Now()}
	s.mu.Unlock()
	return p, nil
}

func (s *Service) forget(id uuid.UUID) {
	s.mu.Lock()
	delete(s.cache, id)
	s.mu.Unlock()
}

// Me re-checks the database so a deactivated account stops working at the next page load.
func (s *Service) Me(ctx context.Context, c *auth.Claims) (*Identity, error) {
	id, err := c.UserID()
	if err != nil {
		return nil, errSessionEnded
	}
	if c.Role == RoleCashier {
		seat, err := s.repo.FindSeatForSession(ctx, id)
		if errors.Is(err, ErrNotFound) || (err == nil && !seat.IsActive) {
			return nil, errSessionEnded
		}
		if err != nil {
			return nil, err
		}
		return &Identity{ID: seat.ID, Username: seat.TenantUsername, DisplayName: seat.Name, Role: RoleCashier, IsActive: true}, nil
	}
	u, err := s.repo.FindByID(ctx, id)
	if errors.Is(err, ErrNotFound) || (err == nil && !u.IsActive) {
		return nil, errSessionEnded
	}
	if err != nil {
		return nil, err
	}
	out := identityOf(u)
	return &out, nil
}

// ---- admin: manage owners (the SOHO customers) ----

type CreateTenantInput struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
	MaxCashiers *int   `json:"max_cashiers"` // nil: DefaultMaxCashiers
}

// CreateTenant makes a SOHO customer. The role is fixed here, never taken from the request.
func (s *Service) CreateTenant(ctx context.Context, in CreateTenantInput) (*User, error) {
	limit := DefaultMaxCashiers
	if in.MaxCashiers != nil {
		limit = *in.MaxCashiers
	}
	if !validLimit(limit) {
		return nil, errBadLimit
	}
	return s.createAccount(ctx, RoleOwner, in.Username, in.DisplayName, in.Password, limit)
}

func (s *Service) createAccount(ctx context.Context, role, username, displayName, password string, maxCashiers int) (*User, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	displayName = strings.TrimSpace(displayName)
	switch {
	case !usernameRe.MatchString(username):
		return nil, apperr.Invalid("invalid_username", "Username harus 3 sampai 40 karakter: huruf kecil, angka, titik, strip, atau garis bawah.")
	case !validName(displayName):
		return nil, apperr.Invalid("invalid_name", "Nama harus 1 sampai 80 karakter.")
	case !validPassword(password):
		return nil, apperr.Invalid("weak_password", "Kata sandi harus 12 sampai 128 karakter.")
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, err
	}
	u := &User{Username: username, DisplayName: displayName, Role: role, PasswordHash: hash, IsActive: true, MaxCashiers: maxCashiers}
	if err := s.repo.Create(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

func (s *Service) ListTenants(ctx context.Context) ([]Tenant, error) {
	return s.repo.ListTenants(ctx, maxTenantsListed)
}

// UpdateTenant renames an owner, switches them on and off, and sets how many kasir seats they may use.
func (s *Service) UpdateTenant(ctx context.Context, id uuid.UUID, displayName string, active bool, maxCashiers *int) (*User, error) {
	displayName = strings.TrimSpace(displayName)
	if !validName(displayName) {
		return nil, apperr.Invalid("invalid_name", "Nama harus 1 sampai 80 karakter.")
	}
	if maxCashiers != nil && !validLimit(*maxCashiers) {
		return nil, errBadLimit
	}
	if err := s.repo.UpdateTenant(ctx, id, displayName, active, maxCashiers); err != nil {
		return nil, accountErr(err)
	}
	s.forget(id)
	u, err := s.repo.FindByID(ctx, id)
	return u, accountErr(err)
}

// ResetTenantPassword sets a new password and signs the owner out everywhere.
func (s *Service) ResetTenantPassword(ctx context.Context, id uuid.UUID, password string) error {
	if !validPassword(password) {
		return apperr.Invalid("weak_password", "Kata sandi harus 12 sampai 128 karakter.")
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	if err := s.repo.SetTenantPassword(ctx, id, hash); err != nil {
		return accountErr(err)
	}
	s.forget(id)
	return nil
}

// BootstrapAdmin creates the platform admin from environment settings, only while no admin exists.
func (s *Service) BootstrapAdmin(ctx context.Context, username, password string) error {
	n, err := s.repo.CountAdmins(ctx)
	if err != nil || n > 0 {
		return err
	}
	_, err = s.createAccount(ctx, RoleAdmin, username, "Admin", password, DefaultMaxCashiers)
	return err
}

// ---- owner: manage the kasir seats of their own shop ----

type SeatList struct {
	Seats  []Seat
	Limit  int // how many seats may be active at once
	Active int
}

func (s *Service) ListSeats(ctx context.Context, tenantID uuid.UUID) (*SeatList, error) {
	owner, err := s.repo.FindByID(ctx, tenantID)
	if err != nil {
		return nil, accountErr(err)
	}
	seats, err := s.repo.ListSeats(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := &SeatList{Seats: seats, Limit: owner.MaxCashiers}
	for _, seat := range seats {
		if seat.IsActive {
			out.Active++
		}
	}
	return out, nil
}

// CreateSeat makes a kasir seat and returns its code. The code is shown once and cannot be read
// back later: only its hash is stored. Seats beyond the owner's limit are refused.
func (s *Service) CreateSeat(ctx context.Context, tenantID uuid.UUID, name string) (*Seat, string, error) {
	name = strings.TrimSpace(name)
	if !validSeatName(name) {
		return nil, "", errBadSeatName
	}
	code, hash, err := newCodeWithHash()
	if err != nil {
		return nil, "", err
	}
	seat := &Seat{TenantID: tenantID, Name: name, Selector: code.Selector, CodeHash: hash}
	// Two seats of one shop can, rarely, draw the same prefix. The database refuses the second and we draw again.
	for attempt := 0; attempt < 5; attempt++ {
		err = s.tx.WithTx(ctx, func(ctx context.Context) error { return s.repo.CreateSeat(ctx, seat) })
		if !errors.Is(err, errSelectorTaken) {
			break
		}
		if code.Selector, err = randomChars(selectorLen); err != nil {
			return nil, "", err
		}
		seat.Selector = code.Selector
	}
	if err != nil {
		return nil, "", seatErr(err)
	}
	return seat, code.String(), nil
}

// UpdateSeat renames a seat or switches it on and off. Switching one off ends its sessions at once.
func (s *Service) UpdateSeat(ctx context.Context, tenantID, id uuid.UUID, name string, active bool) (*Seat, error) {
	name = strings.TrimSpace(name)
	if !validSeatName(name) {
		return nil, errBadSeatName
	}
	err := s.tx.WithTx(ctx, func(ctx context.Context) error {
		return s.repo.UpdateSeat(ctx, tenantID, id, name, active)
	})
	if err != nil {
		return nil, seatErr(err)
	}
	s.forget(id)
	seat, err := s.repo.FindSeat(ctx, tenantID, id)
	return seat, seatErr(err)
}

// ResetSeatCode makes a new code for a seat and signs the old one out everywhere. Use it when a kasir leaves.
func (s *Service) ResetSeatCode(ctx context.Context, tenantID, id uuid.UUID) (*Seat, string, error) {
	code, hash, err := newCodeWithHash()
	if err != nil {
		return nil, "", err
	}
	for attempt := 0; attempt < 5; attempt++ {
		err = s.repo.SetSeatCode(ctx, tenantID, id, code.Selector, hash)
		if !errors.Is(err, errSelectorTaken) {
			break
		}
		if code.Selector, err = randomChars(selectorLen); err != nil {
			return nil, "", err
		}
	}
	if err != nil {
		return nil, "", seatErr(err)
	}
	s.forget(id)
	seat, err := s.repo.FindSeat(ctx, tenantID, id)
	if err != nil {
		return nil, "", seatErr(err)
	}
	return seat, code.String(), nil
}

func newCodeWithHash() (cashierCode, string, error) {
	code, err := newCashierCode()
	if err != nil {
		return cashierCode{}, "", err
	}
	hash, err := auth.HashPassword(code.Secret)
	if err != nil {
		return cashierCode{}, "", err
	}
	return code, hash, nil
}

func validName(s string) bool {
	n := utf8.RuneCountInString(s)
	return n >= 1 && n <= 80
}

func validSeatName(s string) bool {
	n := utf8.RuneCountInString(s)
	if n < 1 || n > 40 {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func validPassword(p string) bool { return len(p) >= 12 && len(p) <= 128 }

func validLimit(n int) bool { return n >= 0 && n <= maxCashiersCap }

func accountErr(err error) error {
	if errors.Is(err, ErrNotFound) {
		return apperr.NotFound("user_not_found", "Akun tidak ditemukan.")
	}
	return err
}

func seatErr(err error) error {
	var limit *SeatLimitError
	switch {
	case errors.Is(err, ErrNotFound):
		return apperr.NotFound("seat_not_found", "Kasir tidak ditemukan.")
	case errors.As(err, &limit):
		return apperr.Conflict("seat_limit", fmt.Sprintf("Jatah %d kasir sudah terpakai semua. Nonaktifkan salah satu, atau hubungi kami untuk menambah jatah.", limit.Limit))
	}
	return err
}
