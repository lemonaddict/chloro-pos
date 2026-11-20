package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	CookieName    = "pos_session"
	RoleOwner     = "owner"
	RoleCashier   = "cashier"
	tokenIssuer   = "sohopos"
	tokenAudience = "sohopos-web"
)

// Claims is the signed session. CSRF is a random per-session value the browser must echo
// back in the X-CSRF-Token header on every state-changing request.
type Claims struct {
	Role string `json:"role"`
	CSRF string `json:"csrf"`
	jwt.RegisteredClaims
}

func (c *Claims) UserID() (uuid.UUID, error) { return uuid.Parse(c.Subject) }

type TokenManager struct {
	secret []byte
	ttl    time.Duration
	parser *jwt.Parser
}

func NewTokenManager(secret []byte, ttl time.Duration) *TokenManager {
	return &TokenManager{
		secret: secret,
		ttl:    ttl,
		parser: jwt.NewParser(
			jwt.WithValidMethods([]string{"HS256"}), // pin the algorithm
			jwt.WithIssuer(tokenIssuer),
			jwt.WithAudience(tokenAudience),
			jwt.WithExpirationRequired(),
			jwt.WithLeeway(5*time.Second),
		),
	}
}

func (m *TokenManager) Issue(userID uuid.UUID, role string) (token, csrf string, expires time.Time, err error) {
	csrf, err = RandomToken(32)
	if err != nil {
		return "", "", time.Time{}, err
	}
	now := time.Now()
	expires = now.Add(m.ttl)
	claims := Claims{
		Role: role,
		CSRF: csrf,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			Issuer:    tokenIssuer,
			Audience:  jwt.ClaimStrings{tokenAudience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expires),
		},
	}
	token, err = jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	return token, csrf, expires, err
}

func (m *TokenManager) Parse(raw string) (*Claims, error) {
	claims := &Claims{}
	if _, err := m.parser.ParseWithClaims(raw, claims, func(*jwt.Token) (any, error) { return m.secret, nil }); err != nil {
		return nil, err
	}
	if claims.Subject == "" || claims.CSRF == "" {
		return nil, errors.New("incomplete claims")
	}
	return claims, nil
}

// RandomToken returns n random bytes from the OS CSPRNG, URL-safe base64 encoded.
func RandomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

type ctxKey struct{}

func WithClaims(ctx context.Context, c *Claims) context.Context {
	return context.WithValue(ctx, ctxKey{}, c)
}

func FromContext(ctx context.Context) (*Claims, bool) {
	c, ok := ctx.Value(ctxKey{}).(*Claims)
	return c, ok
}
