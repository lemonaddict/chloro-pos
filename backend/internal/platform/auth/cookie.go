package auth

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// SameSite modes for the session cookie (COOKIE_SAMESITE).
const (
	modeAuto   = "auto"
	modeStrict = "strict"
	modeLax    = "lax"
	modeNone   = "none"
)

// CookieOptions configures the session cookie.
type CookieOptions struct {
	// Production lets "auto" mode hand out a cross-site cookie. Anywhere else "auto" means Strict.
	Production bool
	// Secure sets the Secure flag. Browsers drop SameSite=None cookies that lack it.
	Secure bool
	// SameSite is "auto" (the default when empty), "strict", "lax" or "none".
	SameSite string
	// Domain is optional. Leave it empty for a host-only cookie: that is the narrower choice, and it
	// is still sent from app.example.com to api.example.com because both are the same site.
	Domain string
	// AllowedOrigins are the frontends (ALLOWED_ORIGINS) that may receive a cross-site cookie in "auto" mode.
	AllowedOrigins []string
}

// CookiePolicy decides the attributes of the session cookie.
//
// Strict is right while the page and the API share an origin (the Vite proxy in development, or the
// all-in-one image). A static site on another *site* needs SameSite=None, because browsers withhold
// Strict and Lax cookies from cross-site fetches, and *.onrender.com subdomains are different sites.
// In "auto" mode that choice is made per request from the Origin header, so one binary serves both
// setups. The same attributes are used to clear the cookie: a browser only replaces or deletes a
// cookie that the new Set-Cookie matches, and it ignores a Strict one that arrives cross-site.
//
// A SameSite=None cookie is also marked Partitioned (CHIPS): it is then stored per top-level site,
// which keeps it working where Chrome blocks ordinary third-party cookies (Incognito, or the
// setting turned on). Safari ignores the attribute and still blocks the cookie, so a shared parent
// domain (COOKIE_SAMESITE=lax) is the only setup that works in every browser.
type CookiePolicy struct {
	mode       string
	production bool
	secure     bool
	domain     string
	origins    map[string]struct{}
}

func NewCookiePolicy(o CookieOptions) (*CookiePolicy, error) {
	mode := strings.ToLower(strings.TrimSpace(o.SameSite))
	switch mode {
	case "":
		mode = modeAuto
	case modeAuto, modeStrict, modeLax, modeNone:
	default:
		return nil, fmt.Errorf("COOKIE_SAMESITE %q is not one of auto, strict, lax, none", o.SameSite)
	}
	if mode == modeNone && !o.Secure {
		return nil, errors.New("COOKIE_SAMESITE=none needs COOKIE_SECURE=true: browsers drop SameSite=None cookies that are not Secure")
	}
	domain := strings.TrimSpace(o.Domain)
	if strings.ContainsAny(domain, "/: \t") {
		return nil, fmt.Errorf("COOKIE_DOMAIN %q must be a bare domain such as .example.com", o.Domain)
	}
	origins := make(map[string]struct{}, len(o.AllowedOrigins))
	for _, origin := range o.AllowedOrigins {
		origins[origin] = struct{}{}
	}
	return &CookiePolicy{mode: mode, production: o.Production, secure: o.Secure, domain: domain, origins: origins}, nil
}

// Set starts the session cookie. Call it before writing the response body.
func (p *CookiePolicy) Set(w http.ResponseWriter, r *http.Request, token string, expires time.Time) {
	c := p.attributes(r)
	c.Value = token
	c.Expires = expires
	c.MaxAge = int(time.Until(expires).Seconds())
	http.SetCookie(w, &c)
}

// Clear removes the session cookie.
func (p *CookiePolicy) Clear(w http.ResponseWriter, r *http.Request) {
	c := p.attributes(r)
	c.MaxAge = -1
	http.SetCookie(w, &c)
}

func (p *CookiePolicy) attributes(r *http.Request) http.Cookie {
	c := http.Cookie{
		Name:     CookieName,
		Path:     "/",
		Domain:   p.domain,
		HttpOnly: true,
		Secure:   p.secure,
		SameSite: http.SameSiteStrictMode,
	}
	switch p.mode {
	case modeLax:
		c.SameSite = http.SameSiteLaxMode
	case modeNone:
		c.SameSite = http.SameSiteNoneMode
		c.Partitioned = true
	case modeAuto:
		if p.production && p.secure && p.fromOtherOrigin(r) {
			c.SameSite = http.SameSiteNoneMode
			c.Partitioned = true
		}
	}
	return c
}

// fromOtherOrigin reports whether r comes from a configured frontend that lives on a different
// origin than this API. A request with no Origin header, or from an unlisted origin, never counts.
func (p *CookiePolicy) fromOtherOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	if _, ok := p.origins[origin]; !ok {
		return false
	}
	u, err := url.Parse(origin)
	return err == nil && u.Host != r.Host
}
