package middleware

import (
	"context"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"sohopos/internal/platform/httpx"
)

// RateLimiter is an in-memory token bucket per key. Fine for a single node; swap for a
// shared store (Redis) if the app ever runs as several instances.
type RateLimiter struct {
	mu      sync.Mutex
	limit   rate.Limit
	burst   int
	clients map[string]*visitor
}

type visitor struct {
	lim      *rate.Limiter
	lastSeen time.Time
}

func NewRateLimiter(ctx context.Context, limit rate.Limit, burst int) *RateLimiter {
	l := &RateLimiter{limit: limit, burst: burst, clients: make(map[string]*visitor)}
	go l.sweep(ctx)
	return l
}

func (l *RateLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	v, ok := l.clients[key]
	if !ok {
		v = &visitor{lim: rate.NewLimiter(l.limit, l.burst)}
		l.clients[key] = v
	}
	v.lastSeen = time.Now()
	return v.lim.Allow()
}

// Middleware limits by client IP.
func (l *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.Allow(httpx.ClientIP(r)) {
			TooManyRequests(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func TooManyRequests(w http.ResponseWriter) {
	w.Header().Set("Retry-After", "60")
	httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "Too many attempts. Wait a minute and try again.")
}

func (l *RateLimiter) sweep(ctx context.Context) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			l.mu.Lock()
			for k, v := range l.clients {
				if time.Since(v.lastSeen) > 10*time.Minute {
					delete(l.clients, k)
				}
			}
			l.mu.Unlock()
		}
	}
}
