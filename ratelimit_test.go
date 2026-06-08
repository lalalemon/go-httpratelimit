package httpratelimit

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestSlidingWindow_Allow(t *testing.T) {
	l := NewSlidingWindow(3, 1*time.Second)

	// First 3 requests should be allowed
	for i := 0; i < 3; i++ {
		if !l.Allow("key1") {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}

	// 4th request should be denied
	if l.Allow("key1") {
		t.Fatal("4th request should be denied")
	}

	// Different key should be allowed
	if !l.Allow("key2") {
		t.Fatal("different key should be allowed")
	}
}

func TestSlidingWindow_WindowExpiry(t *testing.T) {
	l := NewSlidingWindow(2, 100*time.Millisecond)

	l.Allow("key1")
	l.Allow("key1")

	// Should be denied immediately
	if l.Allow("key1") {
		t.Fatal("should be denied within window")
	}

	// Wait for window to expire
	time.Sleep(150 * time.Millisecond)

	// Should be allowed after window expires
	if !l.Allow("key1") {
		t.Fatal("should be allowed after window expires")
	}
}

func TestSlidingWindow_Reset(t *testing.T) {
	l := NewSlidingWindow(1, 1*time.Second)

	l.Allow("key1")
	if l.Allow("key1") {
		t.Fatal("should be denied")
	}

	l.Reset("key1")
	if !l.Allow("key1") {
		t.Fatal("should be allowed after reset")
	}
}

func TestTokenBucket_Allow(t *testing.T) {
	l := NewTokenBucket(10, 5) // 10 tokens/sec, burst of 5

	// Should allow up to burst
	for i := 0; i < 5; i++ {
		if !l.Allow("key1") {
			t.Fatalf("request %d should be allowed (within burst)", i+1)
		}
	}

	// 6th should be denied (bucket empty)
	if l.Allow("key1") {
		t.Fatal("should be denied after burst exhausted")
	}
}

func TestTokenBucket_Refill(t *testing.T) {
	l := NewTokenBucket(100, 1) // 100 tokens/sec, burst of 1

	l.Allow("key1") // consume the 1 token

	// Should be denied immediately
	if l.Allow("key1") {
		t.Fatal("should be denied immediately")
	}

	// Wait for refill
	time.Sleep(15 * time.Millisecond)

	// Should be allowed after refill
	if !l.Allow("key1") {
		t.Fatal("should be allowed after refill")
	}
}

func TestTokenBucket_DifferentKeys(t *testing.T) {
	l := NewTokenBucket(1, 1)

	l.Allow("key1")
	if !l.Allow("key2") {
		t.Fatal("different key should have independent bucket")
	}
}

func TestMiddleware(t *testing.T) {
	l := NewSlidingWindow(2, 1*time.Second)
	cfg := Config{
		Limiter:    l,
		StatusCode: 429,
		Message:    "too many requests\n",
	}

	handler := Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	// First 2 requests should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// 3rd should be rate limited
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 429 {
		t.Fatalf("expected 429, got %d", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Fatal("missing Retry-After header")
	}
}

func TestMiddleware_DifferentIPs(t *testing.T) {
	l := NewSlidingWindow(1, 1*time.Second)
	handler := Middleware(Config{Limiter: l})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// IP1 uses up quota
	req1 := httptest.NewRequest("GET", "/", nil)
	req1.RemoteAddr = "10.0.0.1:1234"
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatal("IP1 first request should succeed")
	}

	// IP2 should still be allowed
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.RemoteAddr = "10.0.0.2:1234"
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatal("IP2 should be allowed (independent limit)")
	}
}

func TestKeyFuncByPath(t *testing.T) {
	keyFunc := NewKeyFuncByPath()

	req1 := httptest.NewRequest("GET", "/api/users", nil)
	req1.RemoteAddr = "1.2.3.4:80"
	k1 := keyFunc(req1)

	req2 := httptest.NewRequest("GET", "/api/posts", nil)
	req2.RemoteAddr = "1.2.3.4:80"
	k2 := keyFunc(req2)

	if k1 == k2 {
		t.Fatal("different paths should produce different keys")
	}
}

func TestKeyFuncByHeader(t *testing.T) {
	keyFunc := NewKeyFuncByHeader("X-API-Key")

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-API-Key", "my-secret-key")
	k := keyFunc(req)

	if k != "my-secret-key" {
		t.Fatalf("expected 'my-secret-key', got '%s'", k)
	}
}

func TestSlidingWindow_Concurrent(t *testing.T) {
	l := NewSlidingWindow(100, 1*time.Second)
	var wg sync.WaitGroup

	allowed := 0
	var mu sync.Mutex

	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if l.Allow("concurrent-key") {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	if allowed > 100 {
		t.Fatalf("expected at most 100 allowed, got %d", allowed)
	}
}

func TestTokenBucket_Concurrent(t *testing.T) {
	l := NewTokenBucket(1000, 50)
	var wg sync.WaitGroup

	allowed := 0
	var mu sync.Mutex

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if l.Allow("concurrent-key") {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	if allowed > 50 {
		t.Fatalf("expected at most 50 allowed, got %d", allowed)
	}
}
