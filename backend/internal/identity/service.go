package identity

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"sohopos/internal/platform/apperr"
	"sohopos/internal/platform/auth"
)

var (
	errBadCredentials = apperr.Unauthorized("invalid_credentials", "Wrong username or password.")
	usernameRe        = regexp.MustCompile(`^[a-z0-9._-]{3,40}$`)
)

type Service struct {
	repo      Repository
	tokens    *auth.TokenManager
	dummyHash string // verified against when the username doesn't exist, so timing doesn't reveal it
}

func NewService(repo Repository, tokens *auth.TokenManager) *Service {
	dummy, err := auth.HashPassword("placeholder-for-unknown-users")
	if err != nil {
		panic(err) // the OS random source is broken; nothing safe can run
	}
	return &Service{repo: repo, tokens: tokens, dummyHash: dummy}
}

type Session struct {
	User      *User
	Token     string
	CSRF      string
	ExpiresAt time.Time
}

// Login answers every failure with the same message and does the same hashing work.
func (s *Service) Login(ctx context.Context, username, password string) (*Session, error) {
	username = strings.TrimSpace(username)
	if username == "" || len(username) > 40 || password == "" || len(password) > 128 {
		return nil, errBadCredentials
	}
	u, err := s.repo.FindByUsername(ctx, username)
	if errors.Is(err, ErrNotFound) {
		auth.VerifyPassword(password, s.dummyHash)
		return nil, errBadCredentials
	}
	if err != nil {
		return nil, err
	}
	if !auth.VerifyPassword(password, u.PasswordHash) || !u.IsActive {
		return nil, errBadCredentials
	}
	token, csrf, exp, err := s.tokens.Issue(u.ID, u.Role)
	if err != nil {
		return nil, err
	}
	return &Session{User: u, Token: token, CSRF: csrf, ExpiresAt: exp}, nil
}

// Me re-checks the database so a deactivated account stops working at the next page load.
func (s *Service) Me(ctx context.Context, id uuid.UUID) (*User, error) {
	u, err := s.repo.FindByID(ctx, id)
	if errors.Is(err, ErrNotFound) || (err == nil && !u.IsActive) {
		return nil, apperr.Unauthorized("unauthenticated", "Your session has ended. Sign in again.")
	}
	return u, err
}

type CreateUserInput struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
	Role        string `json:"role"`
}

func (s *Service) CreateUser(ctx context.Context, in CreateUserInput) (*User, error) {
	in.Username = strings.ToLower(strings.TrimSpace(in.Username))
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	switch {
	case !usernameRe.MatchString(in.Username):
		return nil, apperr.Invalid("invalid_username", "Username must be 3 to 40 letters, digits, dots, dashes or underscores.")
	case utf8.RuneCountInString(in.DisplayName) < 1 || utf8.RuneCountInString(in.DisplayName) > 80:
		return nil, apperr.Invalid("invalid_name", "Display name must be 1 to 80 characters.")
	case len(in.Password) < 12 || len(in.Password) > 128:
		return nil, apperr.Invalid("weak_password", "Password must be 12 to 128 characters.")
	case in.Role != RoleOwner && in.Role != RoleCashier:
		return nil, apperr.Invalid("invalid_role", "Role must be owner or cashier.")
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return nil, err
	}
	u := &User{Username: in.Username, DisplayName: in.DisplayName, Role: in.Role, PasswordHash: hash, IsActive: true}
	if err := s.repo.Create(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// BootstrapOwner creates the first owner from environment settings, only while no users exist.
func (s *Service) BootstrapOwner(ctx context.Context, username, password string) error {
	n, err := s.repo.CountUsers(ctx)
	if err != nil || n > 0 {
		return err
	}
	_, err = s.CreateUser(ctx, CreateUserInput{Username: username, DisplayName: "Owner", Password: password, Role: RoleOwner})
	return err
}
