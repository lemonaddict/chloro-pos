package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const (
	apiURL      = "https://api.example.com/api/v1/auth/login"
	frontOrigin = "https://app.example.com"
)

// setCookie runs Set for a request that carries the given Origin and returns the Set-Cookie line.
func setCookie(t *testing.T, o CookieOptions, origin string) string {
	t.Helper()
	p, err := NewCookiePolicy(o)
	if err != nil {
		t.Fatalf("NewCookiePolicy: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, apiURL, nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	rec := httptest.NewRecorder()
	p.Set(rec, req, "token-value", time.Now().Add(time.Hour))
	line := rec.Header().Get("Set-Cookie")
	if !strings.HasPrefix(line, CookieName+"=token-value") {
		t.Fatalf("unexpected Set-Cookie: %q", line)
	}
	return line
}

func TestCookieSameSiteChoice(t *testing.T) {
	prod := CookieOptions{Production: true, Secure: true, AllowedOrigins: []string{frontOrigin}}

	cases := []struct {
		name    string
		opts    CookieOptions
		origin  string
		want    string   // attribute that must appear
		wantNot []string // attributes that must not
	}{
		{"auto, production, listed frontend on another origin", prod, frontOrigin,
			"SameSite=None", nil},
		{"auto, same origin (Vite proxy or all-in-one image)", prod, "https://api.example.com",
			"SameSite=Strict", []string{"Partitioned"}},
		{"auto, no Origin header", prod, "",
			"SameSite=Strict", []string{"Partitioned"}},
		{"auto, other origin that is not listed", prod, "https://evil.example.net",
			"SameSite=Strict", []string{"Partitioned"}},
		{"auto, development never relaxes", CookieOptions{Secure: true, AllowedOrigins: []string{frontOrigin}}, frontOrigin,
			"SameSite=Strict", []string{"Partitioned"}},
		{"auto, not Secure never relaxes", CookieOptions{Production: true, AllowedOrigins: []string{frontOrigin}}, frontOrigin,
			"SameSite=Strict", []string{"Partitioned"}},
		{"strict stays strict even cross-origin", CookieOptions{Production: true, Secure: true, SameSite: "strict", AllowedOrigins: []string{frontOrigin}}, frontOrigin,
			"SameSite=Strict", []string{"Partitioned"}},
		{"lax for a shared parent domain", CookieOptions{Production: true, Secure: true, SameSite: "lax", AllowedOrigins: []string{frontOrigin}}, frontOrigin,
			"SameSite=Lax", []string{"Partitioned"}},
		{"none is always None", CookieOptions{Secure: true, SameSite: "none"}, "",
			"SameSite=None", nil},
		{"mode is case-insensitive", CookieOptions{Secure: true, SameSite: " LAX "}, "",
			"SameSite=Lax", nil},
	}
	for _, c := range cases {
		line := setCookie(t, c.opts, c.origin)
		if !strings.Contains(line, c.want) {
			t.Errorf("%s: want %q in %q", c.name, c.want, line)
		}
		for _, bad := range c.wantNot {
			if strings.Contains(line, bad) {
				t.Errorf("%s: %q must not appear in %q", c.name, bad, line)
			}
		}
		if !strings.Contains(line, "HttpOnly") {
			t.Errorf("%s: the session cookie must stay HttpOnly: %q", c.name, line)
		}
	}
}

func TestCookieCrossSiteIsSecureAndPartitioned(t *testing.T) {
	line := setCookie(t, CookieOptions{Production: true, Secure: true, AllowedOrigins: []string{frontOrigin}}, frontOrigin)
	for _, want := range []string{"Secure", "SameSite=None", "Partitioned", "HttpOnly", "Path=/"} {
		if !strings.Contains(line, want) {
			t.Errorf("want %q in %q", want, line)
		}
	}
}

func TestCookieDomainIsOptional(t *testing.T) {
	if line := setCookie(t, CookieOptions{Secure: true}, ""); strings.Contains(line, "Domain=") {
		t.Errorf("no COOKIE_DOMAIN must give a host-only cookie: %q", line)
	}
	if line := setCookie(t, CookieOptions{Secure: true, Domain: ".example.com"}, ""); !strings.Contains(line, "Domain=example.com") {
		t.Errorf("want Domain=example.com in %q", line)
	}
}

// A cookie is only deleted by a Set-Cookie with the same name, path, domain and partitioning, and a
// cross-site response is only honoured if it says SameSite=None. So Clear must match Set per request.
func TestCookieClearMatchesSet(t *testing.T) {
	p, err := NewCookiePolicy(CookieOptions{Production: true, Secure: true, AllowedOrigins: []string{frontOrigin}})
	if err != nil {
		t.Fatal(err)
	}
	for _, origin := range []string{frontOrigin, "https://api.example.com"} {
		req := httptest.NewRequest(http.MethodPost, apiURL, nil)
		req.Header.Set("Origin", origin)

		set := httptest.NewRecorder()
		p.Set(set, req, "x", time.Now().Add(time.Hour))
		cleared := httptest.NewRecorder()
		p.Clear(cleared, req)

		line := cleared.Header().Get("Set-Cookie")
		if !strings.Contains(line, "Max-Age=0") {
			t.Errorf("origin %s: Clear must expire the cookie: %q", origin, line)
		}
		for _, attr := range []string{"SameSite=None", "SameSite=Strict", "Partitioned", "Secure"} {
			if strings.Contains(set.Header().Get("Set-Cookie"), attr) != strings.Contains(line, attr) {
				t.Errorf("origin %s: Set and Clear disagree on %q:\n set:   %q\n clear: %q", origin, attr, set.Header().Get("Set-Cookie"), line)
			}
		}
	}
}

func TestNewCookiePolicyRejectsBadOptions(t *testing.T) {
	cases := []struct {
		name string
		opts CookieOptions
	}{
		{"unknown mode", CookieOptions{Secure: true, SameSite: "sometimes"}},
		{"none without Secure", CookieOptions{SameSite: "none"}},
		{"domain with a scheme", CookieOptions{Secure: true, Domain: "https://example.com"}},
		{"domain with a port", CookieOptions{Secure: true, Domain: "example.com:8080"}},
	}
	for _, c := range cases {
		if _, err := NewCookiePolicy(c.opts); err == nil {
			t.Errorf("%s: want an error", c.name)
		}
	}
}
