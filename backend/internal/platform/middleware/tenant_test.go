package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"chloropos/internal/platform/auth"
	"chloropos/internal/platform/tenancy"
)

func TestTenantScope(t *testing.T) {
	shop := uuid.New()
	var seen uuid.UUID
	h := TenantScope(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen, _ = tenancy.From(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))

	cases := []struct {
		name   string
		claims *auth.Claims
		want   int
	}{
		{"owner", &auth.Claims{Role: auth.RoleOwner, TID: shop.String()}, http.StatusNoContent},
		{"kasir", &auth.Claims{Role: auth.RoleCashier, TID: shop.String()}, http.StatusNoContent},
		{"admin is kept out of shop data", &auth.Claims{Role: auth.RoleAdmin}, http.StatusForbidden},
		{"admin even with a shop in the token", &auth.Claims{Role: auth.RoleAdmin, TID: shop.String()}, http.StatusForbidden},
		{"owner without a shop", &auth.Claims{Role: auth.RoleOwner}, http.StatusForbidden},
		{"no session", nil, http.StatusUnauthorized},
	}
	for _, c := range cases {
		seen = uuid.Nil
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if c.claims != nil {
			req = req.WithContext(auth.WithClaims(req.Context(), c.claims))
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != c.want {
			t.Errorf("%s: want status %d, got %d", c.name, c.want, rec.Code)
		}
		if c.want == http.StatusNoContent && seen != shop {
			t.Errorf("%s: the handler should see the shop in its context, got %v", c.name, seen)
		}
		if c.want != http.StatusNoContent && seen != uuid.Nil {
			t.Errorf("%s: the handler must not run", c.name)
		}
	}
}
