package middleware

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"s3-service/internal/httpapi"
)

const (
	DefaultRateLimitPerWindow = 60
	DefaultRateLimitWindow    = time.Minute
)

type rateLimitCounter struct {
	windowStart time.Time
	count       int
}

type identityIPRateLimiter struct {
	limit   int
	window  time.Duration
	nowFunc func() time.Time

	mu       sync.Mutex
	counters map[string]rateLimitCounter
}

func NewIdentityIPRateLimitMiddleware(limit int, window time.Duration) func(http.Handler) http.Handler {
	if limit <= 0 {
		limit = DefaultRateLimitPerWindow
	}
	if window <= 0 {
		window = DefaultRateLimitWindow
	}

	r := &identityIPRateLimiter{
		limit:    limit,
		window:   window,
		nowFunc:  time.Now,
		counters: make(map[string]rateLimitCounter),
	}

	return r.middleware
}

func (r *identityIPRateLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		identity, ok := callerIdentity(req)
		if !ok {
			httpapi.WriteError(w, req, http.StatusUnauthorized, "auth_failed", "authentication required", httpapi.AuthDetails{Reason: "missing"})
			return
		}
		ip := clientIP(req)
		key := identity + "|" + ip

		now := r.nowFunc()
		allowed, remaining, retryAfter := r.consume(key, now)
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(r.limit))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			httpapi.WriteError(
				w,
				req,
				http.StatusTooManyRequests,
				"throttle",
				"rate limit exceeded",
				httpapi.RateLimitDetails{RetryAfter: retryAfter, Limit: r.limit, Remaining: 0},
			)
			return
		}

		next.ServeHTTP(w, req)
	})
}

func (r *identityIPRateLimiter) consume(key string, now time.Time) (allowed bool, remaining int, retryAfter int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	counter, ok := r.counters[key]
	if !ok || now.Sub(counter.windowStart) >= r.window {
		counter = rateLimitCounter{windowStart: now, count: 0}
	}

	if counter.count >= r.limit {
		retry := counter.windowStart.Add(r.window).Sub(now)
		if retry < 0 {
			retry = 0
		}
		retryAfter = int((retry + time.Second - 1) / time.Second)
		if retryAfter < 1 {
			retryAfter = 1
		}
		remaining = 0
		r.counters[key] = counter
		return false, remaining, retryAfter
	}

	counter.count++
	r.counters[key] = counter
	remaining = r.limit - counter.count
	if remaining < 0 {
		remaining = 0
	}
	return true, remaining, 0
}

func callerIdentity(req *http.Request) (string, bool) {
	claims, ok := ClaimsFromContext(req.Context())
	if !ok {
		return "", false
	}

	principalType := string(claims.PrincipalType)
	subject := claims.Subject
	if principalType == "" || subject == "" {
		return "", false
	}

	return principalType + ":" + subject, true
}

func clientIP(req *http.Request) string {
	remoteAddr := strings.TrimSpace(req.RemoteAddr)
	if remoteAddr == "" {
		return "unknown"
	}

	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil && host != "" {
		return host
	}

	return remoteAddr
}
