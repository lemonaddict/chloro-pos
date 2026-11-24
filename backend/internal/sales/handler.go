package sales

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"chloropos/internal/platform/apperr"
	"chloropos/internal/platform/auth"
	"chloropos/internal/platform/httpx"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Routes are mounted behind Authenticate, CSRF and TenantScope: owners and kasir seats of one shop.
func (h *Handler) Routes(r chi.Router) {
	r.Post("/orders", h.checkout)
	r.Get("/orders/{id}", h.get)
}

// sellerSeat returns the kasir seat behind a session, or nil for the owner.
func sellerSeat(claims *auth.Claims) (*uuid.UUID, error) {
	if claims.Role != auth.RoleCashier {
		return nil, nil
	}
	id, err := claims.UserID()
	if err != nil {
		return nil, apperr.Unauthorized("unauthenticated", "Silakan masuk lagi.")
	}
	return &id, nil
}

func (h *Handler) checkout(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, apperr.Unauthorized("unauthenticated", "Silakan masuk untuk melanjutkan."))
		return
	}
	seatID, err := sellerSeat(claims) // the seller always comes from the session
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var in CheckoutInput
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	order, err := h.svc.Checkout(r.Context(), seatID, in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, order)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, apperr.Unauthorized("unauthenticated", "Silakan masuk untuk melanjutkan."))
		return
	}
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	onlySeat, err := sellerSeat(claims) // an owner sees every order of the shop, a seat only its own
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	order, err := h.svc.GetOrder(r.Context(), id, onlySeat)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, order)
}
