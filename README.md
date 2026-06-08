# go-httpratelimit

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/lalalemon/go-httpratelimit.svg)](https://pkg.go.dev/github.com/lalalemon/go-httpratelimit)

Lightweight HTTP rate limiting middleware for Go with **sliding window** and **token bucket** algorithms. Zero external dependencies.

## Features

- 🪟 **Sliding Window** — tracks requests within a rolling time window
- 🪣 **Token Bucket** — allows bursts up to a configurable capacity with steady refill
- 🔑 **Flexible Key Functions** — rate limit by IP, path, header, or custom logic
- 🧵 **Concurrent Safe** — all limiters use proper locking for goroutine safety
- 🧹 **Auto Cleanup** — stale entries are periodically purged to prevent memory leaks
- 📡 **Standard Middleware** — works with any `http.Handler` or framework (chi, mux, echo, etc.)

## Installation

```bash
go get github.com/lalalemon/go-httpratelimit
```

## Quick Start

```go
package main

import (
    "net/http"
    "time"

    ratelimit "github.com/lalalemon/go-httpratelimit"
)

func main() {
    // 100 requests per minute, sliding window
    limiter := ratelimit.NewSlidingWindow(100, 1*time.Minute)

    handler := ratelimit.Middleware(ratelimit.Config{
        Limiter: limiter,
    })(yourMux)

    http.ListenAndServe(":8080", handler)
}
```

## Usage

### Sliding Window

```go
// 10 requests per 30 seconds
limiter := ratelimit.NewSlidingWindow(10, 30*time.Second)
```

### Token Bucket

```go
// 5 requests/sec average, burst up to 20
limiter := ratelimit.NewTokenBucket(5, 20)
```

### Custom Key Functions

```go
// Rate limit by API key header
cfg := ratelimit.Config{
    Limiter: limiter,
    KeyFunc: ratelimit.NewKeyFuncByHeader("X-API-Key"),
}

// Rate limit by path + IP
cfg := ratelimit.Config{
    Limiter: limiter,
    KeyFunc: ratelimit.NewKeyFuncByPath(),
}
```

### Custom Limit Handler

```go
cfg := ratelimit.Config{
    Limiter:    limiter,
    StatusCode: 429,
    Message:    `{"error": "too many requests"}`,
    OnLimit: func(key string, r *http.Request) {
        log.Printf("rate limited: %s", key)
    },
}
```

## API Reference

### `NewSlidingWindow(limit int, window time.Duration) *SlidingWindowLimiter`

Creates a sliding window rate limiter that allows `limit` requests per `window` duration.

### `NewTokenBucket(rate float64, burst int) *TokenBucketLimiter`

Creates a token bucket rate limiter. `rate` is tokens added per second, `burst` is the maximum bucket size.

### `Middleware(cfg Config) func(http.Handler) http.Handler`

Returns standard HTTP middleware. Configure with `Config` struct.

### Key Functions

| Function | Description |
|----------|-------------|
| `DefaultKeyFunc` | Rate limits by client IP (respects X-Forwarded-For, X-Real-IP) |
| `NewKeyFuncByPath()` | Rate limits by IP + request path |
| `NewKeyFuncByHeader(h)` | Rate limits by a specific header value |

## Contributing

Contributions welcome! Please open an issue or PR.

## License

[MIT](LICENSE)
