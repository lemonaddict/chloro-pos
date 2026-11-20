package middleware

import (
	"crypto/subtle"
	"net/http"

	"sohopos/internal/platform/auth"
	"sohopos/internal/platform/httpx"
)

// Authenticate verifies the session cookie and puts the claims on the request context.
// Handlers take the acting user from here, never from the request body.
func Authenticate(tokens *auth.TokenManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(auth.CookieName)
			if err != nil {
				httpx.Error(w, http.StatusUnauthorized, "unauthenticated", "Sign in to continue.")
				return
			}
			claims, err := tokens.Parse(cookie.Value)
			if err != nil {
				httpx.Error(w, http.StatusUnauthorized, "unauthenticated", "Your session has ended. Sign in again.")
				return
			}
			next.ServeHTTP(w, r.WithContext(auth.WithClaims(r.Context(), claims)))
		})
	}
}

// RequireRole allows only the listed roles.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := auth.FromContext(r.Context())
			if !ok {
				httpx.Error(w, http.StatusUnauthorized, "unauthenticated", "Sign in to continue.")
				return
			}
			if _, ok := allowed[claims.Role]; !ok {
				httpx.Error(w, http.StatusForbidden, "forbidden", "Your account can’t do this. Ask the shop owner.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
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
			httpx.Error(w, http.StatusForbidden, "bad_csrf", "Your session needs a refresh. Reload the page and try again.")
			return
		}
		next.ServeHTTP(w, r)
	})
}
