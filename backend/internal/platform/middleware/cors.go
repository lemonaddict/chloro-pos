package middleware

import (
	"net/http"

	"github.com/go-chi/cors"
)

// CORS lets the configured frontends call the API from another origin and send the session cookie.
//
// Credentials are allowed, so the browser rejects a wildcard Access-Control-Allow-Origin. Each
// origin is therefore matched exactly and echoed back; config.Load refuses "*" and paths. Mount it
// before the router so preflight (OPTIONS) requests are answered here and never reach a route.
func CORS(origins []string) func(http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins:   origins,
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodOptions},
		AllowedHeaders:   []string{"Accept", "Content-Type", "X-CSRF-Token"},
		AllowCredentials: true,
		MaxAge:           600,
	})
}
