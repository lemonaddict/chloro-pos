// Package app is the composition root: it builds each module and mounts its routes.
// Modules never import each other. Only this package knows how they fit together.
package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/time/rate"

	"sohopos/internal/catalog"
	"sohopos/internal/identity"
	"sohopos/internal/platform/auth"
	"sohopos/internal/platform/config"
	"sohopos/internal/platform/database"
	"sohopos/internal/platform/httpx"
	"sohopos/internal/platform/middleware"
	"sohopos/internal/reporting"
	"sohopos/internal/sales"
)

func New(ctx context.Context, cfg config.Config, pool *pgxpool.Pool) (http.Handler, error) {
	store := database.NewStore(pool)
	tokens := auth.NewTokenManager(cfg.JWTSecret, cfg.SessionTTL)

	// Rate limits: a loose per-IP cap on the whole API, and two tight brakes on sign-in.
	apiLimit := middleware.NewRateLimiter(ctx, 30, 90)
	loginIPLimit := middleware.NewRateLimiter(ctx, rate.Every(12*time.Second), 5)
	loginUserLimit := middleware.NewRateLimiter(ctx, rate.Every(6*time.Second), 10)

	// Modules: repository -> service (use cases) -> handler.
	identitySvc := identity.NewService(identity.NewPGRepository(store), tokens)
	identityH := identity.NewHandler(identitySvc, cfg.CookieSecure, loginUserLimit)
	catalogH := catalog.NewHandler(catalog.NewService(catalog.NewPGRepository(store), store))
	salesH := sales.NewHandler(sales.NewService(sales.NewPGRepository(store), store))

	reportSvc := reporting.NewService(reporting.NewPGRepository(store), cfg.ReportTZ)
	if err := reportSvc.CheckTimezone(ctx); err != nil {
		return nil, fmt.Errorf("REPORT_TZ %q isn’t a valid time zone name: %w", cfg.ReportTZ, err)
	}
	reportH := reporting.NewHandler(reportSvc)

	if cfg.BootstrapOwnerUsername != "" {
		if err := identitySvc.BootstrapOwner(ctx, cfg.BootstrapOwnerUsername, cfg.BootstrapOwnerPassword); err != nil {
			return nil, fmt.Errorf("bootstrap owner: %w", err)
		}
	}

	r := chi.NewRouter()
	r.Use(chimw.RequestID, middleware.RequestLogger, chimw.Recoverer, middleware.SecurityHeaders(cfg.CookieSecure))
	if len(cfg.AllowedOrigins) > 0 { // same-origin deployments (the default) need no CORS at all
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   cfg.AllowedOrigins,
			AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodOptions},
			AllowedHeaders:   []string{"Content-Type", "X-CSRF-Token"},
			AllowCredentials: true,
			MaxAge:           600,
		}))
	}

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := store.Ping(ctx); err != nil {
			httpx.Error(w, http.StatusServiceUnavailable, "unhealthy", "Database unavailable.")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(chimw.Timeout(30*time.Second), middleware.NoStore, middleware.OriginCheck(cfg.AllowedOrigins), apiLimit.Middleware)
		r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
			httpx.Error(w, http.StatusNotFound, "not_found", "Not found.")
		})

		identityH.PublicRoutes(r, loginIPLimit.Middleware)

		r.Group(func(r chi.Router) {
			r.Use(middleware.Authenticate(tokens), middleware.CSRF)
			identityH.ProtectedRoutes(r)
			catalogH.Routes(r)
			salesH.Routes(r)
			reportH.Routes(r)
		})
	})

	if cfg.StaticDir != "" { // production: Go serves the built Svelte app too
		r.NotFound(spaHandler(cfg.StaticDir))
	}
	return r, nil
}
