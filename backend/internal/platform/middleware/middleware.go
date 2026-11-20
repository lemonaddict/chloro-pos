// Package middleware holds the cross-cutting HTTP guards: headers, origin checks, logging.
package middleware

import (
	"log/slog"
	"net/http"
	"net/url"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"

	"sohopos/internal/platform/httpx"
)

// SecurityHeaders sets the baseline response headers. The CSP has no 'unsafe-inline', so the
// frontend must not use inline <script>, <style> blocks or style="" attributes.
func SecurityHeaders(hsts bool) func(http.Handler) http.Handler {
	const csp = "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; " +
		"object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'"
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("Content-Security-Policy", csp)
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "no-referrer")
			h.Set("Cross-Origin-Opener-Policy", "same-origin")
			h.Set("Permissions-Policy", "camera=(self), microphone=(), geolocation=()")
			if hsts {
				h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}

// NoStore keeps API responses (receipts, sessions, reports) out of browser and proxy caches.
func NoStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// OriginCheck rejects state-changing browser requests that come from another site.
// Requests with no Origin header (curl, scripts) pass; they still need the CSRF token.
func OriginCheck(allowed []string) func(http.Handler) http.Handler {
	set := make(map[string]struct{}, len(allowed))
	for _, o := range allowed {
		set[o] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}
			origin := r.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}
			if _, ok := set[origin]; ok {
				next.ServeHTTP(w, r)
				return
			}
			if u, err := url.Parse(origin); err == nil && u.Host == r.Host {
				next.ServeHTTP(w, r)
				return
			}
			slog.WarnContext(r.Context(), "origin rejected", "origin", origin, "host", r.Host,
				"hint", "Origin and Host differ. Behind a proxy, keep the Host header or list the origin in ALLOWED_ORIGINS.")
			httpx.Error(w, http.StatusForbidden, "bad_origin", "This request came from an unexpected site.")
		})
	}
}

// RequestLogger writes one structured line per request. It logs the path only (no query
// string, headers or bodies) so credentials and customer data stay out of the logs.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		status := ww.Status()
		if status == 0 {
			status = http.StatusOK
		}
		slog.InfoContext(r.Context(), "http",
			"method", r.Method, "path", r.URL.Path, "status", status,
			"ms", time.Since(start).Milliseconds(), "req_id", chimw.GetReqID(r.Context()))
	})
}
