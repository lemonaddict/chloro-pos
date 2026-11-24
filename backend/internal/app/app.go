// Package app is the composition root: it builds each module and mounts its routes.
// Modules never import each other. Only this package knows how they fit together.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/time/rate"

	"chloropos/internal/catalog"
	"chloropos/internal/identity"
	"chloropos/internal/platform/auth"
	"chloropos/internal/platform/config"
	"chloropos/internal/platform/database"
	"chloropos/internal/platform/httpx"
	"chloropos/internal/platform/middleware"
	"chloropos/internal/reporting"
	"chloropos/internal/sales"
	"chloropos/internal/settings"
)

// taxSource lets sales read the tax rules without importing the settings module.
type taxSource struct{ svc *settings.Service }

func (t taxSource) Tax(ctx context.Context) (sales.TaxConfig, error) {
	s, err := t.svc.Get(ctx)
	if err != nil {
		return sales.TaxConfig{}, err
	}
	return sales.TaxConfig{Enabled: s.TaxEnabled, Label: s.TaxLabel, RateBPS: s.TaxRateBPS, Inclusive: s.TaxInclusive}, nil
}

func New(ctx context.Context, cfg config.Config, pool *pgxpool.Pool) (http.Handler, error) {
	store := database.NewStore(pool)
	tokens := auth.NewTokenManager(cfg.JWTSecret, cfg.SessionTTL)

	// Rate limits: a loose per-IP cap on the whole API, and two tight brakes on sign-in.
	apiLimit := middleware.NewRateLimiter(ctx, 30, 90)
	loginIPLimit := middleware.NewRateLimiter(ctx, rate.Every(12*time.Second), 5)
	loginUserLimit := middleware.NewRateLimiter(ctx, rate.Every(6*time.Second), 10)

	// Modules: repository -> service (use cases) -> handler.
	identitySvc := identity.NewService(identity.NewPGRepository(store), store, tokens)
	identityH := identity.NewHandler(identitySvc, cfg.CookieSecure, loginUserLimit)
	catalogH := catalog.NewHandler(catalog.NewService(catalog.NewPGRepository(store), store))

	settingsSvc := settings.NewService(settings.NewPGRepository(store), store)
	settingsH := settings.NewHandler(settingsSvc)

	salesH := sales.NewHandler(sales.NewService(sales.NewPGRepository(store), store, taxSource{svc: settingsSvc}))

	reportSvc := reporting.NewService(reporting.NewPGRepository(store), store, cfg.ReportTZ)
	if err := reportSvc.CheckTimezone(ctx); err != nil {
		return nil, fmt.Errorf("REPORT_TZ %q bukan nama zona waktu yang dikenal PostgreSQL: %w", cfg.ReportTZ, err)
	}
	reportH := reporting.NewHandler(reportSvc)

	if cfg.BootstrapAdminUsername != "" {
		if err := identitySvc.BootstrapAdmin(ctx, cfg.BootstrapAdminUsername, cfg.BootstrapAdminPassword); err != nil {
			return nil, fmt.Errorf("bootstrap admin: %w", err)
		}
	}

	// Row-level security is the backstop that keeps one shop out of another's rows. It does nothing
	// for a superuser or a BYPASSRLS role, and then only the tenant filter in each query is left.
	if bypass, err := store.BypassesRLS(ctx); err != nil {
		slog.Warn("could not check whether the database role bypasses row-level security", "err", err)
	} else if bypass {
		slog.Warn("the database role bypasses row-level security: tenant isolation rests on application queries alone. Connect as an ordinary role (see README)")
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
			httpx.Error(w, http.StatusServiceUnavailable, "unhealthy", "Basis data tidak tersedia.")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(chimw.Timeout(30*time.Second), middleware.NoStore, middleware.OriginCheck(cfg.AllowedOrigins), apiLimit.Middleware)
		r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
			httpx.Error(w, http.StatusNotFound, "not_found", "Tidak ditemukan.")
		})

		identityH.PublicRoutes(r, loginIPLimit.Middleware)

		r.Group(func(r chi.Router) {
			// SessionValid re-checks the account on each request, so a deactivated kasir or shop is out within seconds.
			r.Use(middleware.Authenticate(tokens, identitySvc.SessionValid), middleware.CSRF)
			identityH.ProtectedRoutes(r) // who am I, sign out, and the admin's customer list

			// Everything below is a shop's own data. TenantScope lets in the shop's owner and its
			// kasir seats only, and pins every query to their shop. The platform admin is turned away.
			r.Group(func(r chi.Router) {
				r.Use(middleware.TenantScope)
				identityH.OwnerRoutes(r)
				settingsH.Routes(r)
				catalogH.Routes(r)
				salesH.Routes(r)
				reportH.Routes(r)
			})
		})
	})

	if cfg.StaticDir != "" { // production: Go serves the built Svelte app too
		r.NotFound(spaHandler(cfg.StaticDir))
	}
	return r, nil
}
