package identity

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"chloropos/internal/platform/auth"
	"chloropos/internal/platform/httpx"
	"chloropos/internal/platform/middleware"
	"chloropos/internal/platform/tenancy"
)

type Handler struct {
	svc         *Service
	cookies     *auth.CookiePolicy      // SameSite, Secure and Domain of the session cookie
	userLimiter *middleware.RateLimiter // per-username brake, on top of the per-IP one on the route
}

func NewHandler(svc *Service, cookies *auth.CookiePolicy, userLimiter *middleware.RateLimiter) *Handler {
	return &Handler{svc: svc, cookies: cookies, userLimiter: userLimiter}
}

// PublicRoutes need no session. ipLimit is the strict per-IP limiter for sign-in.
func (h *Handler) PublicRoutes(r chi.Router, ipLimit func(http.Handler) http.Handler) {
	r.With(ipLimit).Post("/auth/login", h.login)
	r.With(ipLimit).Post("/auth/login/cashier", h.loginCashier)
}

// ProtectedRoutes run behind Authenticate and CSRF, for any signed-in account.
// Managing owners (the SOHO customers) is the platform admin's job and nobody else's.
func (h *Handler) ProtectedRoutes(r chi.Router) {
	admin := middleware.RequireRole(RoleAdmin)
	r.Post("/auth/logout", h.logout)
	r.Get("/auth/me", h.me)
	r.With(admin).Get("/users", h.listUsers)
	r.With(admin).Post("/users", h.createUser)
	r.With(admin).Put("/users/{id}", h.updateUser)
	r.With(admin).Post("/users/{id}/password", h.resetPassword)
}

// OwnerRoutes run behind Authenticate, CSRF and TenantScope. An owner manages the kasir seats of
// their own shop: the shop is the tenant in the context, never something the request names.
func (h *Handler) OwnerRoutes(r chi.Router) {
	owner := middleware.RequireRole(RoleOwner)
	r.With(owner).Get("/cashiers", h.listSeats)
	r.With(owner).Post("/cashiers", h.createSeat)
	r.With(owner).Put("/cashiers/{id}", h.updateSeat)
	r.With(owner).Post("/cashiers/{id}/code", h.resetSeatCode)
}

// ---- what leaves the server: explicit allowlists, so password and code hashes never do ----

type identityView struct {
	ID          uuid.UUID `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"`
	IsActive    bool      `json:"is_active"`
}

type sessionView struct {
	User      identityView `json:"user"`
	CSRFToken string       `json:"csrf_token"`
}

type accountView struct {
	identityView
	MaxCashiers int `json:"max_cashiers"`
}

type tenantView struct {
	accountView
	SeatsUsed int `json:"seats_used"`
}

type seatView struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

type seatListView struct {
	Seats  []seatView `json:"seats"`
	Limit  int        `json:"limit"`
	Active int        `json:"active"`
}

// seatCodeView carries a seat's code. It exists only in the response that creates or resets the
// code: the server keeps a hash, so the code cannot be shown again.
type seatCodeView struct {
	seatView
	Code string `json:"code"`
}

func toIdentityView(i Identity) identityView {
	return identityView{ID: i.ID, Username: i.Username, DisplayName: i.DisplayName, Role: i.Role, IsActive: i.IsActive}
}

func toAccountView(u *User) accountView {
	return accountView{identityView: toIdentityView(identityOf(u)), MaxCashiers: u.MaxCashiers}
}

func toSeatView(s *Seat) seatView {
	return seatView{ID: s.ID, Name: s.Name, IsActive: s.IsActive, CreatedAt: s.CreatedAt}
}

// audit leaves a trail of who changed which account. It records IDs only, never passwords or codes.
func audit(r *http.Request, action string, target uuid.UUID) {
	actor := ""
	if c, ok := auth.FromContext(r.Context()); ok {
		actor = c.Subject
	}
	slog.InfoContext(r.Context(), "audit", "action", action, "actor", actor, "target", target.String())
}

// ---- sign-in ----

func (h *Handler) startSession(w http.ResponseWriter, r *http.Request, sess *Session) {
	h.cookies.Set(w, r, sess.Token, sess.ExpiresAt)
	httpx.JSON(w, http.StatusOK, sessionView{User: toIdentityView(sess.Identity), CSRFToken: sess.CSRF})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !h.userLimiter.Allow("password:" + strings.ToLower(strings.TrimSpace(in.Username))) {
		middleware.TooManyRequests(w)
		return
	}
	sess, err := h.svc.Login(r.Context(), in.Username, in.Password)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	h.startSession(w, r, sess)
}

// loginCashier has its own per-username bucket, so someone hammering a shop's kasir sign-in
// cannot lock the owner out of the same shop's password sign-in.
func (h *Handler) loginCashier(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username string `json:"username"`
		Code     string `json:"code"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !h.userLimiter.Allow("cashier:" + strings.ToLower(strings.TrimSpace(in.Username))) {
		middleware.TooManyRequests(w)
		return
	}
	sess, err := h.svc.LoginCashier(r.Context(), in.Username, in.Code)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	h.startSession(w, r, sess)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	h.cookies.Clear(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		h.cookies.Clear(w, r)
		httpx.Error(w, http.StatusUnauthorized, "unauthenticated", "Sesi sudah berakhir. Silakan masuk lagi.")
		return
	}
	who, err := h.svc.Me(r.Context(), claims)
	if err != nil {
		h.cookies.Clear(w, r)
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, sessionView{User: toIdentityView(*who), CSRFToken: claims.CSRF})
}

// ---- admin: owners (SOHO customers) ----

func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request) {
	tenants, err := h.svc.ListTenants(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]tenantView, 0, len(tenants))
	for i := range tenants {
		out = append(out, tenantView{accountView: toAccountView(&tenants[i].User), SeatsUsed: tenants[i].SeatsUsed})
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) createUser(w http.ResponseWriter, r *http.Request) {
	var in CreateTenantInput
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	u, err := h.svc.CreateTenant(r.Context(), in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	audit(r, "tenant.create", u.ID)
	httpx.JSON(w, http.StatusCreated, tenantView{accountView: toAccountView(u)})
}

func (h *Handler) updateUser(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var in struct {
		DisplayName string `json:"display_name"`
		IsActive    bool   `json:"is_active"`
		MaxCashiers *int   `json:"max_cashiers"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	u, err := h.svc.UpdateTenant(r.Context(), id, in.DisplayName, in.IsActive, in.MaxCashiers)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	audit(r, "tenant.update", id)
	httpx.JSON(w, http.StatusOK, toAccountView(u))
}

func (h *Handler) resetPassword(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var in struct {
		Password string `json:"password"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := h.svc.ResetTenantPassword(r.Context(), id, in.Password); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	audit(r, "tenant.password_reset", id)
	w.WriteHeader(http.StatusNoContent)
}

// ---- owner: kasir seats of their own shop ----

func (h *Handler) listSeats(w http.ResponseWriter, r *http.Request) {
	tenantID, err := tenancy.Require(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	list, err := h.svc.ListSeats(r.Context(), tenantID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := seatListView{Seats: make([]seatView, 0, len(list.Seats)), Limit: list.Limit, Active: list.Active}
	for i := range list.Seats {
		out.Seats = append(out.Seats, toSeatView(&list.Seats[i]))
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) createSeat(w http.ResponseWriter, r *http.Request) {
	tenantID, err := tenancy.Require(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var in struct {
		Name string `json:"name"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	seat, code, err := h.svc.CreateSeat(r.Context(), tenantID, in.Name)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	audit(r, "cashier.create", seat.ID)
	httpx.JSON(w, http.StatusCreated, seatCodeView{seatView: toSeatView(seat), Code: code})
}

func (h *Handler) updateSeat(w http.ResponseWriter, r *http.Request) {
	tenantID, err := tenancy.Require(r.Context())
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
		Name     string `json:"name"`
		IsActive bool   `json:"is_active"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	seat, err := h.svc.UpdateSeat(r.Context(), tenantID, id, in.Name, in.IsActive)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	audit(r, "cashier.update", id)
	httpx.JSON(w, http.StatusOK, toSeatView(seat))
}

func (h *Handler) resetSeatCode(w http.ResponseWriter, r *http.Request) {
	tenantID, err := tenancy.Require(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	id, err := httpx.UUIDParam(r, "id")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	seat, code, err := h.svc.ResetSeatCode(r.Context(), tenantID, id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	audit(r, "cashier.code_reset", id)
	httpx.JSON(w, http.StatusOK, seatCodeView{seatView: toSeatView(seat), Code: code})
}
