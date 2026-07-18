package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/finora/shared/middleware"
	"github.com/gin-gonic/gin"
)

func newRateLimitTestRouter(requestsPerSecond float64, burst int) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.RateLimit(requestsPerSecond, burst))
	r.GET("/ping", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func TestRateLimit_AllowsUpToBurstThenRejects(t *testing.T) {
	// A tiny sustained rate (effectively 0 refill within the test's
	// duration) with burst=2, so exactly the first 2 requests succeed and
	// the 3rd is rejected — deterministic, no timing flakiness.
	r := newRateLimitTestRouter(0.0001, 2)

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.RemoteAddr = "203.0.113.5:1234"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: got %d, want 200 (within burst)", i+1, w.Code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.RemoteAddr = "203.0.113.5:1234"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("request 3 (beyond burst): got %d, want 429", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"RATE_LIMITED"`) {
		t.Errorf("429 response body doesn't carry the RATE_LIMITED code: %s", w.Body.String())
	}
}

func TestRateLimit_DifferentClientIPsHaveIndependentBudgets(t *testing.T) {
	r := newRateLimitTestRouter(0.0001, 1)

	// Client A uses its one token.
	reqA := httptest.NewRequest(http.MethodGet, "/ping", nil)
	reqA.RemoteAddr = "203.0.113.10:1111"
	wA := httptest.NewRecorder()
	r.ServeHTTP(wA, reqA)
	if wA.Code != http.StatusOK {
		t.Fatalf("client A first request: got %d, want 200", wA.Code)
	}

	// Client B must not be affected by client A's usage.
	reqB := httptest.NewRequest(http.MethodGet, "/ping", nil)
	reqB.RemoteAddr = "203.0.113.20:2222"
	wB := httptest.NewRecorder()
	r.ServeHTTP(wB, reqB)
	if wB.Code != http.StatusOK {
		t.Fatalf("client B first request: got %d, want 200 (independent budget from client A)", wB.Code)
	}

	// Client A's second request, though, should now be rejected.
	reqA2 := httptest.NewRequest(http.MethodGet, "/ping", nil)
	reqA2.RemoteAddr = "203.0.113.10:1111"
	wA2 := httptest.NewRecorder()
	r.ServeHTTP(wA2, reqA2)
	if wA2.Code != http.StatusTooManyRequests {
		t.Fatalf("client A second request: got %d, want 429", wA2.Code)
	}
}

func TestRateLimit_ZeroConfigDisablesLimiting(t *testing.T) {
	// The critical fail-open guarantee: a Config that never set these
	// fields (every pre-existing router test in this fleet, before this
	// middleware existed) must not have every request rejected.
	r := newRateLimitTestRouter(0, 0)

	for i := 0; i < 50; i++ {
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.RemoteAddr = "203.0.113.99:9999"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d with zero-value rate limit config: got %d, want 200 (should be fully disabled)", i+1, w.Code)
		}
	}
}
