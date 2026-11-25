// Package config reads settings from the environment (plus an optional .env for local dev).
package config

import (
	"bufio"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env            string
	HTTPAddr       string
	DatabaseURL    string
	DBMaxConns     int32
	DBMinConns     int32
	JWTSecret      []byte
	SessionTTL     time.Duration
	CookieSecure   bool
	CookieSameSite string // auto | strict | lax | none, checked by auth.NewCookiePolicy
	CookieDomain   string // empty = host-only cookie
	AllowedOrigins []string
	StaticDir      string // empty = API-only: no frontend is served
	ReportTZ       string

	// Optional: creates the platform admin (the company) when no admin exists yet.
	BootstrapAdminUsername string
	BootstrapAdminPassword string
}

func Load() (Config, error) {
	loadDotEnv(".env")
	loadDotEnv("../.env")

	env := get("APP_ENV", "development")
	origins, err := parseOrigins(os.Getenv("ALLOWED_ORIGINS"))
	if err != nil {
		return Config{}, err
	}

	c := Config{
		Env:                    env,
		HTTPAddr:               httpAddr(env),
		DatabaseURL:            os.Getenv("DATABASE_URL"),
		DBMaxConns:             int32(getInt("DB_MAX_CONNS", 10)),
		DBMinConns:             int32(getInt("DB_MIN_CONNS", 2)),
		JWTSecret:              []byte(os.Getenv("JWT_SECRET")),
		SessionTTL:             time.Duration(getInt("SESSION_HOURS", 10)) * time.Hour,
		CookieSecure:           getBool("COOKIE_SECURE", true),
		CookieSameSite:         get("COOKIE_SAMESITE", "auto"),
		CookieDomain:           strings.TrimSpace(os.Getenv("COOKIE_DOMAIN")),
		AllowedOrigins:         origins,
		StaticDir:              os.Getenv("STATIC_DIR"),
		ReportTZ:               get("REPORT_TZ", "Asia/Jakarta"),
		BootstrapAdminUsername: first("BOOTSTRAP_ADMIN_USERNAME", "BOOTSTRAP_OWNER_USERNAME"),
		BootstrapAdminPassword: first("BOOTSTRAP_ADMIN_PASSWORD", "BOOTSTRAP_OWNER_PASSWORD"),
	}
	if c.DatabaseURL == "" {
		return c, errors.New("DATABASE_URL is required")
	}
	if len(c.JWTSecret) < 32 {
		return c, errors.New("JWT_SECRET must be at least 32 characters")
	}
	if c.DBMinConns > c.DBMaxConns {
		c.DBMinConns = c.DBMaxConns
	}
	return c, nil
}

// first returns the first variable that is set. The BOOTSTRAP_OWNER_* names still work, so an
// existing .env keeps starting the app.
func first(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

func get(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getInt(key string, def int) int {
	n, err := strconv.Atoi(os.Getenv(key))
	if err != nil || n <= 0 {
		return def
	}
	return n
}

func getBool(key string, def bool) bool {
	b, err := strconv.ParseBool(os.Getenv(key))
	if err != nil {
		return def
	}
	return b
}

// httpAddr: an explicit HTTP_ADDR wins. In production, hosts such as Render say which port to use
// through PORT. Outside production PORT is ignored, because it is often set for other tools.
func httpAddr(env string) string {
	if v := os.Getenv("HTTP_ADDR"); v != "" {
		return v
	}
	if p := os.Getenv("PORT"); p != "" && env == "production" {
		return ":" + p
	}
	return ":8080"
}

// parseOrigins turns "https://app.example.com/, http://localhost:5173" into exact origins
// (scheme://host[:port]). The list is matched literally against the browser's Origin header and
// every entry may send the session cookie, so a wildcard or anything with a path is an error.
func parseOrigins(csv string) ([]string, error) {
	var out []string
	for _, raw := range splitCSV(csv) {
		u, err := url.Parse(strings.TrimRight(raw, "/"))
		if err != nil || strings.Contains(raw, "*") ||
			(u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
			u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
			return nil, fmt.Errorf("ALLOWED_ORIGINS entry %q must look like https://app.example.com (scheme, host, optional port; no path, no wildcard)", raw)
		}
		out = append(out, u.Scheme+"://"+strings.ToLower(u.Host))
	}
	return out, nil
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// loadDotEnv fills in missing variables from KEY=VALUE lines. Real environment variables win.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		if _, exists := os.LookupEnv(k); !exists {
			_ = os.Setenv(k, v)
		}
	}
}
