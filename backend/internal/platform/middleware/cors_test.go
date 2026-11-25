package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

const (
	frontend = "https://app.example.com"
	stranger = "https://evil.example.net"
)

// api mirrors how app.New mounts things: CORS in front of the router, OriginCheck inside /api/v1,
// and a POST-only route, so a preflight that reached the router would be answered 405.
func api() http.Handler {
	origins := []string{frontend}
	r := chi.NewRouter()
	r.Use(CORS(origins))
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(OriginCheck(origins))
		r.Post("/ping", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	})
	return r
}

func preflight(origin string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/ping", nil)
	req.Header.Set("Origin", origin)
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "content-type, x-csrf-token")
	rec := httptest.NewRecorder()
	api().ServeHTTP(rec, req)
	return rec
}

func TestPreflightFromTheFrontend(t *testing.T) {
	rec := preflight(frontend)

	if rec.Code < 200 || rec.Code > 299 {
		t.Fatalf("preflight must succeed, got %d", rec.Code)
	}
	h := rec.Header()
	if got := h.Get("Access-Control-Allow-Origin"); got != frontend {
		t.Errorf("Allow-Origin must echo the exact origin, got %q", got)
	}
	if got := h.Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Allow-Credentials must be true, got %q", got)
	}
	// Header names are compared case-insensitively by browsers.
	if got := strings.ToLower(h.Get("Access-Control-Allow-Headers")); !strings.Contains(got, "x-csrf-token") || !strings.Contains(got, "content-type") {
		t.Errorf("Allow-Headers must cover Content-Type and X-CSRF-Token, got %q", got)
	}
	if got := h.Get("Access-Control-Allow-Methods"); !strings.EqualFold(got, http.MethodPost) {
		t.Errorf("Allow-Methods must allow POST, got %q", got)
	}
}

func TestPreflightFromAStrangerGetsNoCORSHeaders(t *testing.T) {
	rec := preflight(stranger)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("an unlisted origin must not be allowed, got %q", got)
	}
}

func TestActualRequestFromTheFrontend(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ping", nil)
	req.Header.Set("Origin", frontend)
	rec := httptest.NewRecorder()
	api().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("the listed frontend must get through OriginCheck, got %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != frontend {
		t.Errorf("Allow-Origin must echo the exact origin, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Allow-Credentials must be true, got %q", got)
	}
}

func TestActualRequestFromAStrangerIsRefused(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ping", nil)
	req.Header.Set("Origin", stranger)
	rec := httptest.NewRecorder()
	api().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("OriginCheck must refuse an unlisted origin, got %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("a refused origin must not be allowed by CORS, got %q", got)
	}
}

func TestNeverAWildcard(t *testing.T) {
	for _, rec := range []*httptest.ResponseRecorder{preflight(frontend), preflight(stranger)} {
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got == "*" {
			t.Errorf("credentials are allowed, so Allow-Origin must never be *")
		}
	}
}
