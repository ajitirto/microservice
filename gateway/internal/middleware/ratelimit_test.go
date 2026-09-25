package middleware

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeLimiter struct {
	allow   bool
	err     error
	calls   int
	lastKey string
}

func (f *fakeLimiter) Allow(ctx context.Context, key string) (bool, error) {
	f.calls++
	f.lastKey = key
	return f.allow, f.err
}

func newRateHandler(limiter *fakeLimiter) http.Handler {
	h := RateLimit(limiter, slog.New(slog.NewTextHandler(io.Discard, nil)))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	return h
}

func TestRateLimitAllowsWithinLimit(t *testing.T) {
	limiter := &fakeLimiter{allow: true}
	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	rec := httptest.NewRecorder()

	newRateHandler(limiter).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if limiter.calls != 1 {
		t.Errorf("limiter calls = %d, want 1", limiter.calls)
	}
}

func TestRateLimitRejectsOverLimit(t *testing.T) {
	limiter := &fakeLimiter{allow: false}
	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	rec := httptest.NewRecorder()

	newRateHandler(limiter).ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}

func TestRateLimitSkipsHealth(t *testing.T) {
	limiter := &fakeLimiter{allow: false}
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	newRateHandler(limiter).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (health skipped)", rec.Code, http.StatusOK)
	}
	if limiter.calls != 0 {
		t.Errorf("limiter calls = %d, want 0", limiter.calls)
	}
}

func TestRateLimitFailsOpenOnError(t *testing.T) {
	limiter := &fakeLimiter{err: context.DeadlineExceeded}
	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	rec := httptest.NewRecorder()

	newRateHandler(limiter).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (fail open)", rec.Code, http.StatusOK)
	}
}

func TestRateLimitKeysByClientIP(t *testing.T) {
	limiter := &fakeLimiter{allow: true}
	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	req.RemoteAddr = "203.0.113.7:54321"
	rec := httptest.NewRecorder()

	newRateHandler(limiter).ServeHTTP(rec, req)

	if limiter.lastKey != "rl:203.0.113.7" {
		t.Errorf("key = %q, want rl:203.0.113.7", limiter.lastKey)
	}
}
