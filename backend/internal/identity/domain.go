// Package identity owns accounts, sign-in and sessions for the three kinds of people who use the app:
//
//	admin   the company running the service. Manages customers, never sees their shop data.
//	owner   a SOHO customer. One owner account is one tenant (one shop).
//	cashier a kasir seat of an owner: a limited "view" of the shop that signs in with its own code.
package identity

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"chloropos/internal/platform/auth"
)

const (
	RoleAdmin   = auth.RoleAdmin
	RoleOwner   = auth.RoleOwner
	RoleCashier = auth.RoleCashier

	// DefaultMaxCashiers is how many kasir seats a new owner gets. The admin raises it per owner.
	DefaultMaxCashiers = 2
	maxCashiersCap     = 100
)

var (
	ErrNotFound = errors.New("account not found")

	// errSelectorTaken means two seats of one shop drew the same code prefix. The caller draws again.
	errSelectorTaken = errors.New("kasir code prefix already used in this shop")
)

// SeatLimitError means the owner already uses every kasir seat they have.
type SeatLimitError struct{ Limit int }

func (e *SeatLimitError) Error() string { return "kasir seat limit reached" }

// User is an admin or an owner. An owner's ID is also the tenant ID of their shop.
type User struct {
	ID                 uuid.UUID
	Username           string
	DisplayName        string
	Role               string
	PasswordHash       string
	IsActive           bool
	SessionsValidAfter time.Time
	MaxCashiers        int
}

// Tenant is an owner as the admin sees them: the account and how many kasir seats are in use.
type Tenant struct {
	User
	SeatsUsed int
}

// Seat is one kasir "instance" of an owner's shop.
type Seat struct {
	ID                 uuid.UUID
	TenantID           uuid.UUID
	Name               string
	Selector           string // first part of the kasir code; not secret
	CodeHash           string // Argon2id hash of the secret part of the kasir code
	IsActive           bool
	SessionsValidAfter time.Time
	CreatedAt          time.Time

	// Filled only by FindSeatForSession: the owner's username (what the kasir types as "username toko"),
	// and IsActive then means the seat AND its owner are both active.
	TenantUsername string
}

// Identity is who is signed in, as shown to the browser. For a kasir it is the seat.
type Identity struct {
	ID          uuid.UUID
	Username    string // for a kasir: the shop's username
	DisplayName string // for a kasir: the seat's name
	Role        string
	IsActive    bool
}

// Repository is the persistence port. The Postgres adapter lives in repository_pg.go.
//
// Admin-side methods act on owners only (role = 'owner'), so an admin account can never be edited
// through them. Seat methods that take a tenantID always filter by it.
type Repository interface {
	FindByUsername(ctx context.Context, username string) (*User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*User, error)
	CountAdmins(ctx context.Context) (int64, error)
	Create(ctx context.Context, u *User) error

	// ListTenants returns at most limit owners, with their seat usage.
	ListTenants(ctx context.Context, limit int) ([]Tenant, error)
	// UpdateTenant changes the name, the active flag and (when non-nil) the seat limit.
	// Deactivating also ends the owner's open sessions.
	UpdateTenant(ctx context.Context, id uuid.UUID, displayName string, active bool, maxCashiers *int) error
	// SetTenantPassword stores a new hash and ends the owner's open sessions.
	SetTenantPassword(ctx context.Context, id uuid.UUID, hash string) error

	FindSeatBySelector(ctx context.Context, tenantID uuid.UUID, selector string) (*Seat, error)
	FindSeatForSession(ctx context.Context, id uuid.UUID) (*Seat, error)
	FindSeat(ctx context.Context, tenantID, id uuid.UUID) (*Seat, error)
	ListSeats(ctx context.Context, tenantID uuid.UUID) ([]Seat, error)
	// CreateSeat and UpdateSeat lock the owner's row and enforce the seat limit; call them in a transaction.
	CreateSeat(ctx context.Context, s *Seat) error
	UpdateSeat(ctx context.Context, tenantID, id uuid.UUID, name string, active bool) error
	// SetSeatCode swaps the code and ends the seat's open sessions.
	SetSeatCode(ctx context.Context, tenantID, id uuid.UUID, selector, codeHash string) error
}
