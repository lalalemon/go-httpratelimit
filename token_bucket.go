package httpratelimit

import (
	"sync"
	"time"
)

// TokenBucketLimiter implements a token bucket rate limiter.
// Tokens are added at a fixed rate, and each request consumes one token.
type TokenBucketLimiter struct {
	mu      sync.RWMutex
	entries map[string]*tokenBucketEntry
	rate    float64 // tokens added per second
	burst   float64 // maximum bucket size
}

// NewTokenBucket creates a new token bucket rate limiter.
// rate: tokens added per second.
// burst: maximum number of tokens (bucket capacity).
func NewTokenBucket(rate float64, burst int) *TokenBucketLimiter {
	l := &TokenBucketLimiter{
		entries: make(map[string]*tokenBucketEntry),
		rate:    rate,
		burst:   float64(burst),
	}

	go l.cleanup()
	return l
}

func (l *TokenBucketLimiter) getEntry(key string) *tokenBucketEntry {
	l.mu.RLock()
	entry, exists := l.entries[key]
	l.mu.RUnlock()

	if exists {
		return entry
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if entry, exists = l.entries[key]; exists {
		return entry
	}

	entry = &tokenBucketEntry{
		tokens:     l.burst,
		lastRefill: time.Now(),
	}
	l.entries[key] = entry
	return entry
}

// Allow checks if a request is allowed and consumes a token if so.
func (l *TokenBucketLimiter) Allow(key string) bool {
	entry := l.getEntry(key)
	entry.mu.Lock()
	defer entry.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(entry.lastRefill).Seconds()

	// Refill tokens based on elapsed time
	entry.tokens += elapsed * l.rate
	if entry.tokens > l.burst {
		entry.tokens = l.burst
	}
	entry.lastRefill = now

	// Check if a token is available
	if entry.tokens < 1.0 {
		return false
	}

	entry.tokens -= 1.0
	return true
}

// Reset clears the bucket state for a given key.
func (l *TokenBucketLimiter) Reset(key string) {
	l.mu.Lock()
	delete(l.entries, key)
	l.mu.Unlock()
}

// cleanup periodically removes stale entries.
func (l *TokenBucketLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		l.mu.Lock()
		for key, entry := range l.entries {
			entry.mu.Lock()
			if time.Since(entry.lastRefill) > 10*time.Minute {
				delete(l.entries, key)
			}
			entry.mu.Unlock()
		}
		l.mu.Unlock()
	}
}
