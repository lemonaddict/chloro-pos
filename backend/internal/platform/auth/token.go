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
	RoleAdmin     = "admin"   // the company running the service: manages customers, never sees their data
	RoleOwner     = "owner"   // a SOHO customer, and the tenant of everything in their shop
	RoleCashier   = "cashier" // one kasir seat inside an owner's shop
	tokenIssuer   = "chloropos"
	tokenAudience = "chloropos-web"
)

// Claims is the signed session. Subject is the account (an admin, an owner, or a kasir seat),
// TID is the tenant it acts for (empty for the admin). CSRF is a random per-session value the
// browser must echo back in the X-CSRF-Token header on every state-changing request.
type Claims struct {
	Role string `json:"role"`
	TID  string `json:"tid,omitempty"`
	CSRF string `json:"csrf"`
	jwt.RegisteredClaims
}

// UserID is the signed-in account: the admin's or owner's user ID, or the kasir seat's ID.
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

// Issue signs a session for subject. Pass uuid.Nil as tenantID for the platform admin.
func (m *TokenManager) Issue(subject uuid.UUID, role string, tenantID uuid.UUID) (token, csrf string, expires time.Time, err error) {
	csrf, err = RandomToken(32)
	if err != nil {
		return "", "", time.Time{}, err
	}
	now := time.Now()
	expires = now.Add(m.ttl)
	var tid string
	if tenantID != uuid.Nil {
		tid = tenantID.String()
	}
	claims := Claims{
		Role: role,
		TID:  tid,
		CSRF: csrf,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject.String(),
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
