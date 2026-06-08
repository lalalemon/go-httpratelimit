package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	ratelimit "github.com/lalalemon/go-httpratelimit"
)

func main() {
	// Create a sliding window limiter: 100 requests per minute
	limiter := ratelimit.NewSlidingWindow(100, 1*time.Minute)

	// Configure the middleware
	middleware := ratelimit.Middleware(ratelimit.Config{
		Limiter:    limiter,
		KeyFunc:    ratelimit.DefaultKeyFunc,
		StatusCode: http.StatusTooManyRequests,
		Message:    `{"error": "rate limit exceeded", "retry_after": "60s"}`,
		OnLimit: func(key string, r *http.Request) {
			log.Printf("rate limited: %s %s from %s", r.Method, r.URL.Path, key)
		},
	})

	// Set up routes
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, World!")
	})

	mux.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"data": "some useful data"}`)
	})

	// Apply middleware
	handler := middleware(mux)

	fmt.Println("Server starting on :8080")
	fmt.Println("Rate limit: 100 requests per minute per IP")
	log.Fatal(http.ListenAndServe(":8080", handler))
}
