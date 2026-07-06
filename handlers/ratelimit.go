package handlers

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	apperrors "github.com/helloamitkr/cvfit-tools/errors"
)

// RateLimiter is a simple in-memory, per-IP fixed-window rate limiter.
// It has no external dependencies and is safe for concurrent use.
//
// Note: state is per-process. On a single server this is exact; on Lambda each
// warm container keeps its own counters, so the effective limit scales with
// concurrency — still enough to stop a single client from hammering the AI routes.
type RateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	limit    int
	window   time.Duration
}

type visitor struct {
	count       int
	windowStart time.Time
}

// NewRateLimiter creates a limiter allowing `limit` requests per `window` per IP
// and starts a background goroutine that evicts stale entries.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		limit:    limit,
		window:   window,
	}
	go rl.cleanupLoop()
	return rl
}

// allow reports whether the given key may make another request right now.
func (rl *RateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	v, ok := rl.visitors[key]
	if !ok || now.Sub(v.windowStart) > rl.window {
		rl.visitors[key] = &visitor{count: 1, windowStart: now}
		return true
	}
	if v.count >= rl.limit {
		return false
	}
	v.count++
	return true
}

// cleanupLoop periodically removes visitors whose window has expired.
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		rl.mu.Lock()
		for key, v := range rl.visitors {
			if now.Sub(v.windowStart) > rl.window {
				delete(rl.visitors, key)
			}
		}
		rl.mu.Unlock()
	}
}

// Middleware wraps a handler, rejecting requests from IPs that exceed the limit.
func (rl *RateLimiter) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return rl.MiddlewareBy(clientIP)(next)
}

// MiddlewareBy is like Middleware but derives the bucket key from `key` instead
// of always using the client IP — e.g. per-user when authenticated. Returns a
// wrapper so it composes with the auth middleware: OptionalAuth(limiter(handler)).
func (rl *RateLimiter) MiddlewareBy(key func(*http.Request) string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if !rl.allow(key(r)) {
				writeError(w, apperrors.TooManyRequests("too many requests — please slow down and try again shortly"))
				return
			}
			next(w, r)
		}
	}
}

// AIRateKey buckets AI requests per authenticated user, falling back to client
// IP for anonymous callers. A signed-in user gets their own allowance regardless
// of shared/NAT'd IPs, while abuse from one account is still capped.
func AIRateKey(r *http.Request) string {
	if uid := UserIDFromCtx(r.Context()); uid != "" {
		return "user:" + uid
	}
	return clientIP(r)
}

// clientIP extracts the best-guess client IP, honouring proxy/API-Gateway headers.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// First entry is the original client.
		if first := strings.TrimSpace(strings.Split(xff, ",")[0]); first != "" {
			return first
		}
	}
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return strings.TrimSpace(xrip)
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
