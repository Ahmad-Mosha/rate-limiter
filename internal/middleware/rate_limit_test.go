package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ahmad-mosha/go-rate-limiter/internal/limiter"
)

// mockLimiter is a simple test double that implements limiter.RateLimiter.
// It returns the pre-configured result and err values on every call,
// and records the key it was called with.
type mockLimiter struct {
	result  limiter.Result
	err     error
	lastKey string
}

// compile-time check: mockLimiter must satisfy the RateLimiter interface.
var _ limiter.RateLimiter = (*mockLimiter)(nil)

func (m *mockLimiter) Allow(ctx context.Context, key string) (limiter.Result, error) {
	m.lastKey = key
	return m.result, m.err
}

// dummyHandler is the "next" handler behind the middleware.
// It writes 200 OK with a body so tests can verify it was reached.
func dummyHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("passed"))
}

func TestRateLimiterMiddleware_AllowedRequest(t *testing.T) {
	mock := &mockLimiter{result: limiter.Result{Allowed: true, Remaining: 4}}
	handler := RateLimiterMiddleware(mock, http.HandlerFunc(dummyHandler))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if rr.Body.String() != "passed" {
		t.Errorf("expected body %q, got %q", "passed", rr.Body.String())
	}
	if rr.Header().Get("X-RateLimit-Remaining") != "4" {
		t.Errorf("expected X-RateLimit-Remaining %q, got %q", "4", rr.Header().Get("X-RateLimit-Remaining"))
	}
	if rr.Header().Get("Retry-After") != "" {
		t.Errorf("expected no Retry-After header on allowed request, got %q", rr.Header().Get("Retry-After"))
	}
}

func TestRateLimiterMiddleware_DeniedRequest(t *testing.T) {
	mock := &mockLimiter{result: limiter.Result{Allowed: false, Remaining: 0, RetryAfter: 3 * time.Second}}
	handler := RateLimiterMiddleware(mock, http.HandlerFunc(dummyHandler))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("expected status %d, got %d", http.StatusTooManyRequests, rr.Code)
	}
	if rr.Header().Get("X-RateLimit-Remaining") != "0" {
		t.Errorf("expected X-RateLimit-Remaining %q, got %q", "0", rr.Header().Get("X-RateLimit-Remaining"))
	}
	if rr.Header().Get("Retry-After") != "3" {
		t.Errorf("expected Retry-After %q, got %q", "3", rr.Header().Get("Retry-After"))
	}
}

func TestRateLimiterMiddleware_LimiterError(t *testing.T) {
	mock := &mockLimiter{err: errors.New("storage failure")}
	handler := RateLimiterMiddleware(mock, http.HandlerFunc(dummyHandler))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}
}

func TestRateLimiterMiddleware_IPExtraction(t *testing.T) {
	mock := &mockLimiter{result: limiter.Result{Allowed: true}}
	handler := RateLimiterMiddleware(mock, http.HandlerFunc(dummyHandler))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.5:9999"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if mock.lastKey != "10.0.0.5" {
		t.Errorf("expected key %q, got %q", "10.0.0.5", mock.lastKey)
	}

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestRateLimiterMiddleware_IPExtractionNoPort(t *testing.T) {
	mock := &mockLimiter{result: limiter.Result{Allowed: true}}
	handler := RateLimiterMiddleware(mock, http.HandlerFunc(dummyHandler))

	// RemoteAddr without a port — net.SplitHostPort will error,
	// so the middleware should fall back to using the raw value.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.5"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if mock.lastKey != "10.0.0.5" {
		t.Errorf("expected key %q, got %q", "10.0.0.5", mock.lastKey)
	}

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}
