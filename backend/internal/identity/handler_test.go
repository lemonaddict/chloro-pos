package identity

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"golang.org/x/time/rate"

	"chloropos/internal/platform/auth"
	"chloropos/internal/platform/middleware"
)

func newTestRouter(t *testing.T, f *fixture, secureCookie, isProd bool, allowedOrigins []string) http.Handler {
	t.Helper()
	h := NewHandler(f.svc, secureCookie, isProd, middleware.NewRateLimiter(t.Context(), rate.Every(time.Second), 20))
	ipLimit := middleware.NewRateLimiter(t.Context(), rate.Every(time.Second), 20)

	r := chi.NewRouter()
	if len(allowedOrigins) > 0 {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   allowedOrigins,
			AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodOptions},
			AllowedHeaders:   []string{"Content-Type", "X-CSRF-Token", "Accept"},
			AllowCredentials: true,
			MaxAge:           600,
		}))
	}
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.OriginCheck(allowedOrigins))
		h.PublicRoutes(r, ipLimit.Middleware)
		r.Group(func(r chi.Router) {
			r.Use(middleware.Authenticate(f.tokens, f.svc.SessionValid), middleware.CSRF)
			h.ProtectedRoutes(r)
		})
	})
	return r
}

func findSessionCookie(t *testing.T, resp *http.Response) *http.Cookie {
	t.Helper()
	for _, c := range resp.Cookies() {
		if c.Name == auth.CookieName {
			return c
		}
	}
	t.Fatalf("expected %q cookie in Set-Cookie headers", auth.CookieName)
	return nil
}

func TestCookiePolicySameOriginVsProdCrossOrigin(t *testing.T) {
	f := newFixture(t, 2)
	body := `{"username":"tmb","password":"` + ownerPassword + `"}`

	// 1. Production cross-origin: SameSite=None; Secure; HttpOnly.
	prodRouter := newTestRouter(t, f, true, true, []string{"https://app.example.com"})
	req := httptest.NewRequest(http.MethodPost, "https://api.example.com/api/v1/auth/login", bytes.NewBufferString(body))
	req.Host = "api.example.com"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://app.example.com")
	rec := httptest.NewRecorder()
	prodRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("cross-origin login want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("want exact ACAO header, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("want ACAC=true, got %q", got)
	}

	res := rec.Result()
	defer res.Body.Close()
	cookie := findSessionCookie(t, res)
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteNoneMode {
		t.Fatalf("prod cross-origin cookie want HttpOnly+Secure+SameSite=None, got HttpOnly=%v Secure=%v SameSite=%v",
			cookie.HttpOnly, cookie.Secure, cookie.SameSite)
	}

	var loginPayload struct {
		CSRFToken string `json:"csrf_token"`
	}
	if err := json.NewDecoder(res.Body).Decode(&loginPayload); err != nil {
		t.Fatalf("decode login payload: %v", err)
	}

	// Preflight OPTIONS on protected route must succeed with CORS headers (not 401/403).
	preReq := httptest.NewRequest(http.MethodOptions, "https://api.example.com/api/v1/auth/logout", nil)
	preReq.Host = "api.example.com"
	preReq.Header.Set("Origin", "https://app.example.com")
	preReq.Header.Set("Access-Control-Request-Method", http.MethodPost)
	preReq.Header.Set("Access-Control-Request-Headers", "Content-Type, X-CSRF-Token, Accept")
	preRec := httptest.NewRecorder()
	prodRouter.ServeHTTP(preRec, preReq)
	if preRec.Code != http.StatusOK && preRec.Code != http.StatusNoContent {
		t.Fatalf("preflight want 200/204, got %d", preRec.Code)
	}
	if got := preRec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("preflight ACAO want exact origin, got %q", got)
	}
	if got := preRec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("preflight ACAC want true, got %q", got)
	}

	// Logout without CSRF must fail with 403.
	badLogout := httptest.NewRequest(http.MethodPost, "https://api.example.com/api/v1/auth/logout", nil)
	badLogout.Host = "api.example.com"
	badLogout.Header.Set("Origin", "https://app.example.com")
	badLogout.AddCookie(cookie)
	badLogoutRec := httptest.NewRecorder()
	prodRouter.ServeHTTP(badLogoutRec, badLogout)
	if badLogoutRec.Code != http.StatusForbidden {
		t.Fatalf("logout without CSRF want 403, got %d", badLogoutRec.Code)
	}

	// Logout with CSRF must clear cookie using matching SameSite=None; Secure.
	logoutReq := httptest.NewRequest(http.MethodPost, "https://api.example.com/api/v1/auth/logout", nil)
	logoutReq.Host = "api.example.com"
	logoutReq.Header.Set("Origin", "https://app.example.com")
	logoutReq.Header.Set("X-CSRF-Token", loginPayload.CSRFToken)
	logoutReq.AddCookie(cookie)
	logoutRec := httptest.NewRecorder()
	prodRouter.ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusNoContent {
		t.Fatalf("logout want 204, got %d", logoutRec.Code)
	}
	logoutRes := logoutRec.Result()
	defer logoutRes.Body.Close()
	cleared := findSessionCookie(t, logoutRes)
	if cleared.MaxAge != -1 || !cleared.Secure || cleared.SameSite != http.SameSiteNoneMode {
		t.Fatalf("cleared cross-origin cookie want MaxAge=-1 Secure=true SameSite=None, got MaxAge=%d Secure=%v SameSite=%v",
			cleared.MaxAge, cleared.Secure, cleared.SameSite)
	}

	// 2. Production same-origin keeps SameSite=Strict.
	sameOriginReq := httptest.NewRequest(http.MethodPost, "https://api.example.com/api/v1/auth/login", bytes.NewBufferString(body))
	sameOriginReq.Host = "api.example.com"
	sameOriginReq.Header.Set("Content-Type", "application/json")
	sameOriginReq.Header.Set("Origin", "https://api.example.com")
	sameOriginRec := httptest.NewRecorder()
	prodRouter.ServeHTTP(sameOriginRec, sameOriginReq)
	if sameOriginRec.Code != http.StatusOK {
		t.Fatalf("same-origin login want 200, got %d", sameOriginRec.Code)
	}
	sameRes := sameOriginRec.Result()
	defer sameRes.Body.Close()
	sameCookie := findSessionCookie(t, sameRes)
	if sameCookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("same-origin prod cookie want SameSite=Strict, got %v", sameCookie.SameSite)
	}

	// 3. Development (isProd=false) keeps SameSite=Strict even when Origin != Host.
	devRouter := newTestRouter(t, f, false, false, []string{"http://localhost:5173"})
	devReq := httptest.NewRequest(http.MethodPost, "http://localhost:8080/api/v1/auth/login", bytes.NewBufferString(body))
	devReq.Host = "localhost:8080"
	devReq.Header.Set("Content-Type", "application/json")
	devReq.Header.Set("Origin", "http://localhost:5173")
	devRec := httptest.NewRecorder()
	devRouter.ServeHTTP(devRec, devReq)
	if devRec.Code != http.StatusOK {
		t.Fatalf("dev login want 200, got %d", devRec.Code)
	}
	devRes := devRec.Result()
	defer devRes.Body.Close()
	devCookie := findSessionCookie(t, devRes)
	if devCookie.SameSite != http.SameSiteStrictMode || devCookie.Secure {
		t.Fatalf("dev cookie want SameSite=Strict Secure=false, got SameSite=%v Secure=%v", devCookie.SameSite, devCookie.Secure)
	}

	// 4. Disallowed origin is rejected by OriginCheck.
	evilReq := httptest.NewRequest(http.MethodPost, "https://api.example.com/api/v1/auth/login", bytes.NewBufferString(body))
	evilReq.Host = "api.example.com"
	evilReq.Header.Set("Content-Type", "application/json")
	evilReq.Header.Set("Origin", "https://evil.example.com")
	evilRec := httptest.NewRecorder()
	prodRouter.ServeHTTP(evilRec, evilReq)
	if evilRec.Code != http.StatusForbidden {
		t.Fatalf("disallowed origin want 403, got %d", evilRec.Code)
	}
}
