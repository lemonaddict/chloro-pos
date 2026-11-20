// Package identity owns users, sign-in and sessions.
package identity

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"sohopos/internal/platform/auth"
)

const (
	RoleOwner   = auth.RoleOwner
	RoleCashier = auth.RoleCashier
)

var ErrNotFound = errors.New("user not found")

type User struct {
	ID           uuid.UUID
	Username     string
	DisplayName  string
	Role         string
	PasswordHash string
	IsActive     bool
}

// Repository is the persistence port. The Postgres adapter lives in repository_pg.go.
type Repository interface {
	FindByUsername(ctx context.Context, username string) (*User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*User, error)
	CountUsers(ctx context.Context) (int64, error)
	Create(ctx context.Context, u *User) error
}
