// Package httputil provides shared HTTP middleware for all backend services.
package httputil

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// maxVisitors caps the number of tracked client keys so a flood of distinct
// source IPs cannot grow the map without bound between cleanup ticks
// (memory-exhaustion DoS). When the cap is reached, new keys are refused with a
// rate-limited response rather than allowed, which is the safe (fail-closed)
// choice for an abusive-traffic condition.
const maxVisitors = 100_000

// RateLimiter is a simple per-IP token-bucket rate limiter.
type RateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rate     int           // tokens per window
	window   time.Duration // refill window

	// trustForwardedFor controls whether the X-Forwarded-For / X-Real-Ip
	// headers are used to identify the client. It defaults to false because
	// those headers are attacker-controlled when the service is reachable
	// directly: trusting them unconditionally lets a client rotate the header
	// to evade the limit. Only enable it when a trusted reverse proxy is known
	// to set the header (see TrustForwardedFor).
	trustForwardedFor bool

	stopOnce sync.Once
	stop     chan struct{}
}

type visitor struct {
	tokens    int
	lastReset time.Time
}

// NewRateLimiter creates a rate limiter that allows rate requests per window
// per client IP. The client IP is taken from the connection's RemoteAddr; see
// TrustForwardedFor to additionally honour proxy headers.
func NewRateLimiter(rate int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		rate:     rate,
		window:   window,
		stop:     make(chan struct{}),
	}
	go rl.cleanup()
	return rl
}

// TrustForwardedFor enables (or disables) using the X-Forwarded-For / X-Real-Ip
// header as the client identity. Only enable this behind a trusted proxy that
// overwrites the header, otherwise the limit can be trivially bypassed.
func (rl *RateLimiter) TrustForwardedFor(trust bool) *RateLimiter {
	rl.trustForwardedFor = trust
	return rl
}

// Stop terminates the background cleanup goroutine. Safe to call more than once.
func (rl *RateLimiter) Stop() {
	rl.stopOnce.Do(func() { close(rl.stop) })
}

func (rl *RateLimiter) cleanup() {
	t := time.NewTicker(5 * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-rl.stop:
			return
		case <-t.C:
			rl.mu.Lock()
			for ip, v := range rl.visitors {
				if time.Since(v.lastReset) > 2*rl.window {
					delete(rl.visitors, ip)
				}
			}
			rl.mu.Unlock()
		}
	}
}

func (rl *RateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[ip]
	now := time.Now()

	if !exists {
		// Bounded map: refuse new keys once the cap is hit (fail-closed under flood).
		if len(rl.visitors) >= maxVisitors {
			return false
		}
		rl.visitors[ip] = &visitor{tokens: rl.rate - 1, lastReset: now}
		return true
	}

	if now.Sub(v.lastReset) >= rl.window {
		v.tokens = rl.rate - 1
		v.lastReset = now
		return true
	}

	if v.tokens > 0 {
		v.tokens--
		return true
	}
	return false
}

// clientIP derives the rate-limit key for a request. By default it uses the
// connection RemoteAddr (host portion); when proxy headers are trusted it
// prefers X-Real-Ip then the first X-Forwarded-For hop.
func (rl *RateLimiter) clientIP(r *http.Request) string {
	if rl.trustForwardedFor {
		if v := r.Header.Get("X-Real-Ip"); v != "" {
			return v
		}
		if v := r.Header.Get("X-Forwarded-For"); v != "" {
			first, _, _ := strings.Cut(v, ",")
			return strings.TrimSpace(first)
		}
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// Middleware returns an http.Handler middleware that rate-limits by client IP.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.allow(rl.clientIP(r)) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate_limit_exceeded","message":"too many requests, please try again later"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}
