package httpratelimit

import (
	"net/http"
	"sync"
	"time"
)

// Limiter defines the interface for rate limiting strategies.
type Limiter interface {
	// Allow checks if a request with the given key should be allowed.
	Allow(key string) bool
	// Reset clears the state for the given key.
	Reset(key string)
}

// KeyFunc extracts a rate limit key from the request.
// Common implementations: IP-based, user-based, path-based.
type KeyFunc func(r *http.Request) string

// Config holds rate limiter middleware configuration.
type Config struct {
	// Limiter is the rate limiting strategy to use.
	Limiter Limiter
	// KeyFunc extracts the rate limit key from requests.
	// Defaults to client IP if nil.
	KeyFunc KeyFunc
	// StatusCode is the HTTP status for rate-limited requests (default 429).
	StatusCode int
	// Message is the body for rate-limited responses.
	Message string
	// OnLimit is an optional callback when a request is rate-limited.
	OnLimit func(key string, r *http.Request)
}

// DefaultKeyFunc returns the client IP address from the request.
func DefaultKeyFunc(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = r.Header.Get("X-Real-IP")
	}
	if ip == "" {
		ip = r.RemoteAddr
	}
	return ip
}

// Middleware returns an HTTP middleware that enforces rate limiting.
func Middleware(cfg Config) func(http.Handler) http.Handler {
	if cfg.KeyFunc == nil {
		cfg.KeyFunc = DefaultKeyFunc
	}
	if cfg.StatusCode == 0 {
		cfg.StatusCode = http.StatusTooManyRequests
	}
	if cfg.Message == "" {
		cfg.Message = "rate limit exceeded\n"
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := cfg.KeyFunc(r)

			if !cfg.Limiter.Allow(key) {
				if cfg.OnLimit != nil {
					cfg.OnLimit(key, r)
				}
				w.Header().Set("Retry-After", "1")
				w.Header().Set("X-RateLimit-Limit", "exceeded")
				http.Error(w, cfg.Message, cfg.StatusCode)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// NewKeyFuncByPath returns a KeyFunc that rate-limits by request path + IP.
func NewKeyFuncByPath() KeyFunc {
	return func(r *http.Request) string {
		return DefaultKeyFunc(r) + ":" + r.URL.Path
	}
}

// NewKeyFuncByHeader returns a KeyFunc that rate-limits by a specific header value.
func NewKeyFuncByHeader(header string) KeyFunc {
	return func(r *http.Request) string {
		val := r.Header.Get(header)
		if val == "" {
			return DefaultKeyFunc(r)
		}
		return val
	}
}

// slidingWindowEntry tracks requests within a time window.
type slidingWindowEntry struct {
	mu       sync.Mutex
	requests []time.Time
}

// tokenBucketEntry tracks token bucket state.
type tokenBucketEntry struct {
	mu         sync.Mutex
	tokens     float64
	lastRefill time.Time
}
