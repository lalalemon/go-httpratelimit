package httpratelimit

import (
	"sync"
	"time"
)

// SlidingWindowLimiter implements a sliding window rate limiter.
// It tracks the number of requests within a rolling time window.
type SlidingWindowLimiter struct {
	mu      sync.RWMutex
	entries map[string]*slidingWindowEntry
	limit   int
	window  time.Duration
}

// NewSlidingWindow creates a new sliding window rate limiter.
// limit: maximum number of requests allowed in the window.
// window: the time window duration.
func NewSlidingWindow(limit int, window time.Duration) *SlidingWindowLimiter {
	l := &SlidingWindowLimiter{
		entries: make(map[string]*slidingWindowEntry),
		limit:   limit,
		window:  window,
	}

	// Periodic cleanup of stale entries
	go l.cleanup()

	return l
}

func (l *SlidingWindowLimiter) getEntry(key string) *slidingWindowEntry {
	l.mu.RLock()
	entry, exists := l.entries[key]
	l.mu.RUnlock()

	if exists {
		return entry
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Double-check after acquiring write lock
	if entry, exists = l.entries[key]; exists {
		return entry
	}

	entry = &slidingWindowEntry{}
	l.entries[key] = entry
	return entry
}

// Allow checks if a request is allowed under the sliding window limit.
func (l *SlidingWindowLimiter) Allow(key string) bool {
	entry := l.getEntry(key)
	entry.mu.Lock()
	defer entry.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-l.window)

	// Remove requests outside the current window
	validStart := 0
	for i, t := range entry.requests {
		if t.After(windowStart) {
			validStart = i
			break
		}
		if i == len(entry.requests)-1 {
			validStart = len(entry.requests)
		}
	}
	entry.requests = entry.requests[validStart:]

	// Check if under limit
	if len(entry.requests) >= l.limit {
		return false
	}

	entry.requests = append(entry.requests, now)
	return true
}

// Reset clears the request history for a given key.
func (l *SlidingWindowLimiter) Reset(key string) {
	l.mu.Lock()
	delete(l.entries, key)
	l.mu.Unlock()
}

// cleanup periodically removes stale entries to prevent memory leaks.
func (l *SlidingWindowLimiter) cleanup() {
	ticker := time.NewTicker(l.window * 2)
	defer ticker.Stop()

	for range ticker.C {
		l.mu.Lock()
		for key, entry := range l.entries {
			entry.mu.Lock()
			if len(entry.requests) == 0 || time.Since(entry.requests[len(entry.requests)-1]) > l.window*2 {
				delete(l.entries, key)
			}
			entry.mu.Unlock()
		}
		l.mu.Unlock()
	}
}
