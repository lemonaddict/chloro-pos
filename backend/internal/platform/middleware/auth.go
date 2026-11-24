package middleware

import (
	"context"
	"crypto/subtle"
	"net/http"

	"github.com/google/uuid"

	"chloropos/internal/platform/auth"
	"chloropos/internal/platform/httpx"
	"chloropos/internal/platform/tenancy"
)

// SessionValidator re-checks a signed session against the database (account still active,
// role unchanged, no password reset or deactivation since it was issued).
type SessionValidator func(ctx context.Context, c *auth.Claims) bool

// Authenticate verifies the session cookie, then asks the validator whether the account is
// still allowed in. Handlers take the acting user from the context, never from the request body.
func Authenticate(tokens *auth.TokenManager, valid SessionValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(auth.CookieName)
			if err != nil {
				httpx.Error(w, http.StatusUnauthorized, "unauthenticated", "Silakan masuk untuk melanjutkan.")
				return
			}
			claims, err := tokens.Parse(cookie.Value)
			if err != nil || !valid(r.Context(), claims) {
				httpx.Error(w, http.StatusUnauthorized, "unauthenticated", "Sesi sudah berakhir. Silakan masuk lagi.")
				return
			}
			next.ServeHTTP(w, r.WithContext(auth.WithClaims(r.Context(), claims)))
		})
	}
}

// RequireRole allows only the listed roles.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		allowed[role] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := auth.FromContext(r.Context())
			if !ok {
				httpx.Error(w, http.StatusUnauthorized, "unauthenticated", "Silakan masuk untuk melanjutkan.")
				return
			}
			if _, ok := allowed[claims.Role]; !ok {
				httpx.Error(w, http.StatusForbidden, "forbidden", "Akunmu tidak punya izin untuk ini. Hubungi admin.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// TenantScope admits only members of a shop (its owner and its kasir seats) and puts the shop's
// tenant ID in the context. The platform admin is turned away here: it manages customers, and
// has no business reading what they sell. Must run after Authenticate.
func TenantScope(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.FromContext(r.Context())
		if !ok {
			httpx.Error(w, http.StatusUnauthorized, "unauthenticated", "Silakan masuk untuk melanjutkan.")
			return
		}
		tid, err := uuid.Parse(claims.TID)
		if (claims.Role != auth.RoleOwner && claims.Role != auth.RoleCashier) || err != nil || tid == uuid.Nil {
			httpx.Error(w, http.StatusForbidden, "forbidden", "Akunmu tidak punya izin untuk ini.")
			return
		}
		next.ServeHTTP(w, r.WithContext(tenancy.With(r.Context(), tid)))
	})
}

// CSRF requires the X-CSRF-Token header to match the token inside the signed session on every
// state-changing request. Must run after Authenticate.
func CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		claims, ok := auth.FromContext(r.Context())
		got := r.Header.Get("X-CSRF-Token")
		if !ok || claims.CSRF == "" || subtle.ConstantTimeCompare([]byte(got), []byte(claims.CSRF)) != 1 {
			httpx.Error(w, http.StatusForbidden, "bad_csrf", "Sesi perlu disegarkan. Muat ulang halaman lalu coba lagi.")
			return
		}
		next.ServeHTTP(w, r)
	})
}
