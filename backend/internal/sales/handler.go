package sales

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"sohopos/internal/platform/apperr"
	"sohopos/internal/platform/auth"
	"sohopos/internal/platform/httpx"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Routes are mounted behind Authenticate + CSRF.
func (h *Handler) Routes(r chi.Router) {
	r.Post("/orders", h.checkout)
	r.Get("/orders/{id}", h.get)
}

func (h *Handler) checkout(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, apperr.Unauthorized("unauthenticated", "Sign in to continue."))
		return
	}
	cashierID, err := claims.UserID() // the seller always comes from the session
	if err != nil {
		httpx.WriteError(w, r, apperr.Unauthorized("unauthenticated", "Sign in again."))
		return
	}
	var in CheckoutInput
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	order, err := h.svc.Checkout(r.Context(), cashierID, in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, order)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, apperr.Unauthorized("unauthenticated", "Sign in to continue."))
		return
	}
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var onlyCashier *uuid.UUID
	if claims.Role != auth.RoleOwner {
		uid, err := claims.UserID()
		if err != nil {
			httpx.WriteError(w, r, apperr.Unauthorized("unauthenticated", "Sign in again."))
			return
		}
		onlyCashier = &uid
	}
	order, err := h.svc.GetOrder(r.Context(), id, onlyCashier)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, order)
}
