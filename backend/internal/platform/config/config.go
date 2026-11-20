// Package config reads settings from the environment (plus an optional .env for local dev).
package config

import (
	"bufio"
	"errors"
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
	AllowedOrigins []string
	StaticDir      string
	ReportTZ       string

	// Optional: creates the first owner when the users table is empty.
	BootstrapOwnerUsername string
	BootstrapOwnerPassword string
}

func Load() (Config, error) {
	loadDotEnv(".env")
	loadDotEnv("../.env")

	c := Config{
		Env:                    get("APP_ENV", "development"),
		HTTPAddr:               get("HTTP_ADDR", ":8080"),
		DatabaseURL:            os.Getenv("DATABASE_URL"),
		DBMaxConns:             int32(getInt("DB_MAX_CONNS", 10)),
		DBMinConns:             int32(getInt("DB_MIN_CONNS", 2)),
		JWTSecret:              []byte(os.Getenv("JWT_SECRET")),
		SessionTTL:             time.Duration(getInt("SESSION_HOURS", 10)) * time.Hour,
		CookieSecure:           getBool("COOKIE_SECURE", true),
		AllowedOrigins:         splitCSV(os.Getenv("ALLOWED_ORIGINS")),
		StaticDir:              os.Getenv("STATIC_DIR"),
		ReportTZ:               get("REPORT_TZ", "UTC"),
		BootstrapOwnerUsername: os.Getenv("BOOTSTRAP_OWNER_USERNAME"),
		BootstrapOwnerPassword: os.Getenv("BOOTSTRAP_OWNER_PASSWORD"),
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
