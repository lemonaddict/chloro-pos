package identity

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"sohopos/internal/platform/auth"
	"sohopos/internal/platform/httpx"
	"sohopos/internal/platform/middleware"
)

type Handler struct {
	svc          *Service
	secureCookie bool
	userLimiter  *middleware.RateLimiter // per-username brake, on top of the per-IP one on the route
}

func NewHandler(svc *Service, secureCookie bool, userLimiter *middleware.RateLimiter) *Handler {
	return &Handler{svc: svc, secureCookie: secureCookie, userLimiter: userLimiter}
}

// PublicRoutes need no session. ipLimit is the strict per-IP limiter for sign-in.
func (h *Handler) PublicRoutes(r chi.Router, ipLimit func(http.Handler) http.Handler) {
	r.With(ipLimit).Post("/auth/login", h.login)
}

// ProtectedRoutes run behind Authenticate and CSRF.
func (h *Handler) ProtectedRoutes(r chi.Router) {
	r.Post("/auth/logout", h.logout)
	r.Get("/auth/me", h.me)
	r.With(middleware.RequireRole(RoleOwner)).Post("/users", h.createUser)
}

// userView is an explicit allowlist of what leaves the server. PasswordHash never does.
type userView struct {
	ID          uuid.UUID `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"`
}

type sessionView struct {
	User      userView `json:"user"`
	CSRFToken string   `json:"csrf_token"`
}

func toView(u *User) userView {
	return userView{ID: u.ID, Username: u.Username, DisplayName: u.DisplayName, Role: u.Role}
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
	if !h.userLimiter.Allow(strings.ToLower(strings.TrimSpace(in.Username))) {
		middleware.TooManyRequests(w)
		return
	}
	sess, err := h.svc.Login(r.Context(), in.Username, in.Password)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     auth.CookieName,
		Value:    sess.Token,
		Path:     "/",
		Expires:  sess.ExpiresAt,
		MaxAge:   int(time.Until(sess.ExpiresAt).Seconds()),
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteStrictMode,
	})
	httpx.JSON(w, http.StatusOK, sessionView{User: toView(sess.User), CSRFToken: sess.CSRF})
}

func (h *Handler) logout(w http.ResponseWriter, _ *http.Request) {
	h.clearCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.FromContext(r.Context())
	id, err := claims.UserID()
	if err != nil {
		h.clearCookie(w)
		httpx.Error(w, http.StatusUnauthorized, "unauthenticated", "Your session has ended. Sign in again.")
		return
	}
	u, err := h.svc.Me(r.Context(), id)
	if err != nil {
		h.clearCookie(w)
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, sessionView{User: toView(u), CSRFToken: claims.CSRF})
}

func (h *Handler) createUser(w http.ResponseWriter, r *http.Request) {
	var in CreateUserInput
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	u, err := h.svc.CreateUser(r.Context(), in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, toView(u))
}

func (h *Handler) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: auth.CookieName, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: h.secureCookie, SameSite: http.SameSiteStrictMode,
	})
}
