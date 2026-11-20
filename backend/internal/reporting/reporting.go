// Package reporting is the read side: daily revenue and fast-moving items.
//
// It reads the orders tables directly, which is fine inside the monolith. If this module is
// ever extracted, feed it from order events instead and drop the cross-module reads.
package reporting

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"sohopos/internal/platform/auth"
	"sohopos/internal/platform/database"
	"sohopos/internal/platform/httpx"
	"sohopos/internal/platform/middleware"
)

type Totals struct {
	RevenueCents int64 `json:"revenue_cents"`
	Orders       int64 `json:"orders"`
}

type DayTotal struct {
	Date         string `json:"date"`
	RevenueCents int64  `json:"revenue_cents"`
	Orders       int64  `json:"orders"`
}

type TopItem struct {
	ProductID    uuid.UUID `json:"product_id"`
	Name         string    `json:"name"`
	Quantity     int64     `json:"quantity"`
	RevenueCents int64     `json:"revenue_cents"`
}

type Summary struct {
	Today    Totals     `json:"today"`
	Daily    []DayTotal `json:"daily"`
	TopItems []TopItem  `json:"top_items"`
}

type Repository interface {
	Summary(ctx context.Context, tz string, days int) (*Summary, error)
	CheckTimezone(ctx context.Context, tz string) error
}

type Service struct {
	repo Repository
	tz   string
}

func NewService(repo Repository, tz string) *Service { return &Service{repo: repo, tz: tz} }

// CheckTimezone fails fast at startup if REPORT_TZ isn't a name PostgreSQL knows.
func (s *Service) CheckTimezone(ctx context.Context) error { return s.repo.CheckTimezone(ctx, s.tz) }

func (s *Service) Summary(ctx context.Context, days int) (*Summary, error) {
	days = min(max(days, 1), 31)
	sum, err := s.repo.Summary(ctx, s.tz, days)
	if err != nil {
		return nil, err
	}
	if n := len(sum.Daily); n > 0 {
		last := sum.Daily[n-1] // days are ordered oldest to newest, so the last one is today
		sum.Today = Totals{RevenueCents: last.RevenueCents, Orders: last.Orders}
	}
	return sum, nil
}

// ---- Postgres adapter ----

type PGRepository struct{ db *database.Store }

func NewPGRepository(db *database.Store) *PGRepository { return &PGRepository{db: db} }

func (r *PGRepository) CheckTimezone(ctx context.Context, tz string) error {
	_, err := r.db.DB(ctx).Exec(ctx, `SELECT now() AT TIME ZONE $1::text`, tz)
	return err
}

// Days are the shop's local days (REPORT_TZ), including days with no sales, so the chart has no gaps.
// The join uses a created_at range, which lets PostgreSQL use the orders_completed_created_idx index.
func (r *PGRepository) Summary(ctx context.Context, tz string, days int) (*Summary, error) {
	db := r.db.DB(ctx)
	sum := &Summary{Daily: make([]DayTotal, 0, days), TopItems: make([]TopItem, 0, 5)}

	rows, err := db.Query(ctx, `
		WITH local_today AS (SELECT date_trunc('day', now() AT TIME ZONE $1::text) AS d),
		     days AS (
		       SELECT (t.d - make_interval(days => g))::date AS day
		         FROM local_today t, generate_series(0, $2::int - 1) AS g
		     )
		SELECT to_char(d.day, 'YYYY-MM-DD'), COALESCE(SUM(o.total_cents), 0)::bigint, COUNT(o.id)::bigint
		  FROM days d
		  LEFT JOIN orders o
		    ON o.status = 'completed'
		   AND o.created_at >= (d.day::timestamp AT TIME ZONE $1::text)
		   AND o.created_at <  ((d.day + 1)::timestamp AT TIME ZONE $1::text)
		 GROUP BY d.day
		 ORDER BY d.day`, tz, days)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var d DayTotal
		if err := rows.Scan(&d.Date, &d.RevenueCents, &d.Orders); err != nil {
			rows.Close()
			return nil, err
		}
		sum.Daily = append(sum.Daily, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	top, err := db.Query(ctx, `
		SELECT oi.product_id, MAX(oi.product_name), SUM(oi.quantity)::bigint, SUM(oi.line_total_cents)::bigint
		  FROM order_items oi
		  JOIN orders o ON o.id = oi.order_id
		 WHERE o.status = 'completed'
		   AND o.created_at >= ((date_trunc('day', now() AT TIME ZONE $1::text)
		                         - make_interval(days => $2::int - 1)) AT TIME ZONE $1::text)
		 GROUP BY oi.product_id
		 ORDER BY 3 DESC, 2
		 LIMIT 5`, tz, days)
	if err != nil {
		return nil, err
	}
	defer top.Close()
	for top.Next() {
		var t TopItem
		if err := top.Scan(&t.ProductID, &t.Name, &t.Quantity, &t.RevenueCents); err != nil {
			return nil, err
		}
		sum.TopItems = append(sum.TopItems, t)
	}
	return sum, top.Err()
}

// ---- HTTP ----

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Routes are mounted behind Authenticate + CSRF. Reports are owner-only.
func (h *Handler) Routes(r chi.Router) {
	r.With(middleware.RequireRole(auth.RoleOwner)).Get("/reports/summary", h.summary)
}

func (h *Handler) summary(w http.ResponseWriter, r *http.Request) {
	sum, err := h.svc.Summary(r.Context(), httpx.IntQuery(r, "days", 7, 1, 31))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, sum)
}
