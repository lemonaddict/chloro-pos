package catalog

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"chloropos/internal/platform/apperr"
	"chloropos/internal/platform/auth"
	"chloropos/internal/platform/httpx"
	"chloropos/internal/platform/middleware"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Routes are mounted behind Authenticate + CSRF. Reads are open to any signed-in user;
// everything that changes the catalog is owner-only.
func (h *Handler) Routes(r chi.Router) {
	owner := middleware.RequireRole(auth.RoleOwner)
	r.Get("/products", h.listProducts)
	r.With(owner).Post("/products", h.createProduct)
	r.With(owner).Put("/products/{id}", h.updateProduct)
	r.With(owner).Post("/products/{id}/stock-adjustments", h.adjustStock)
	r.Get("/categories", h.listCategories)
	r.With(owner).Post("/categories", h.createCategory)
}

func actor(r *http.Request) (uuid.UUID, string, error) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		return uuid.Nil, "", apperr.Unauthorized("unauthenticated", "Silakan masuk untuk melanjutkan.")
	}
	id, err := claims.UserID()
	return id, claims.Role, err
}

func (h *Handler) listProducts(w http.ResponseWriter, r *http.Request) {
	_, role, err := actor(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	q := r.URL.Query()
	f := ProductFilter{
		Search:          q.Get("search"),
		IncludeInactive: role == auth.RoleOwner && q.Get("include_inactive") == "true",
		LowStock:        q.Get("low_stock") == "true",
		Limit:           httpx.IntQuery(r, "limit", 50, 1, 100),
		Offset:          httpx.IntQuery(r, "offset", 0, 0, 1_000_000),
	}
	if raw := q.Get("category_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			httpx.WriteError(w, r, apperr.Invalid("invalid_category", "Kategori tidak valid."))
			return
		}
		f.CategoryID = &id
	}
	list, err := h.svc.ListProducts(r.Context(), f)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if role != auth.RoleOwner { // cashiers get prices and availability only, never stock counts
		view := make([]CashierProduct, 0, len(list))
		for _, p := range list {
			view = append(view, p.ForCashier())
		}
		httpx.JSON(w, http.StatusOK, view)
		return
	}
	httpx.JSON(w, http.StatusOK, list)
}

func (h *Handler) createProduct(w http.ResponseWriter, r *http.Request) {
	uid, _, err := actor(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var in CreateProductInput
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	p, err := h.svc.CreateProduct(r.Context(), uid, in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, p)
}

func (h *Handler) updateProduct(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var in ProductInput
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	p, err := h.svc.UpdateProduct(r.Context(), id, in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, p)
}

func (h *Handler) adjustStock(w http.ResponseWriter, r *http.Request) {
	uid, _, err := actor(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var in struct {
		Delta  int    `json:"delta"`
		Reason string `json:"reason"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	p, err := h.svc.AdjustStock(r.Context(), uid, id, in.Delta, in.Reason)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, p)
}

func (h *Handler) listCategories(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.ListCategories(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, list)
}

func (h *Handler) createCategory(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	c, err := h.svc.CreateCategory(r.Context(), in.Name)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, c)
}
