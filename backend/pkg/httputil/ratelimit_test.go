package httputil

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiterAllowsThenBlocks(t *testing.T) {
	rl := NewRateLimiter(3, time.Minute)
	defer rl.Stop()

	const ip = "1.2.3.4"
	for i := 0; i < 3; i++ {
		if !rl.allow(ip) {
			t.Fatalf("request %d should have been allowed", i+1)
		}
	}
	if rl.allow(ip) {
		t.Fatal("4th request should have been blocked")
	}
	// A different IP has its own bucket.
	if !rl.allow("5.6.7.8") {
		t.Fatal("different IP should be allowed")
	}
}

func TestRateLimiterWindowRefill(t *testing.T) {
	rl := NewRateLimiter(1, 10*time.Millisecond)
	defer rl.Stop()

	if !rl.allow("ip") {
		t.Fatal("first should be allowed")
	}
	if rl.allow("ip") {
		t.Fatal("second within window should be blocked")
	}
	time.Sleep(15 * time.Millisecond)
	if !rl.allow("ip") {
		t.Fatal("should be allowed after window")
	}
}

func TestClientIPIgnoresSpoofedHeaderByDefault(t *testing.T) {
	rl := NewRateLimiter(10, time.Minute)
	defer rl.Stop()

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "203.0.113.9:5555"
	r.Header.Set("X-Real-Ip", "10.0.0.1")
	r.Header.Set("X-Forwarded-For", "10.0.0.2")

	if got := rl.clientIP(r); got != "203.0.113.9" {
		t.Fatalf("clientIP=%q, want connection IP 203.0.113.9 (header must not be trusted by default)", got)
	}
}

func TestClientIPTrustsHeaderWhenEnabled(t *testing.T) {
	rl := NewRateLimiter(10, time.Minute).TrustForwardedFor(true)
	defer rl.Stop()

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "203.0.113.9:5555"
	r.Header.Set("X-Forwarded-For", "10.0.0.2, 70.0.0.1")

	if got := rl.clientIP(r); got != "10.0.0.2" {
		t.Fatalf("clientIP=%q, want first forwarded hop 10.0.0.2", got)
	}
}

func TestMiddlewareReturns429(t *testing.T) {
	rl := NewRateLimiter(1, time.Minute)
	defer rl.Stop()

	h := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	newReq := func() *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "9.9.9.9:1234"
		return r
	}

	w1 := httptest.NewRecorder()
	h.ServeHTTP(w1, newReq())
	if w1.Code != http.StatusOK {
		t.Fatalf("first request code=%d want 200", w1.Code)
	}

	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, newReq())
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request code=%d want 429", w2.Code)
	}
}

func TestStopIsIdempotent(t *testing.T) {
	rl := NewRateLimiter(1, time.Minute)
	rl.Stop()
	rl.Stop() // must not panic on double close
}
