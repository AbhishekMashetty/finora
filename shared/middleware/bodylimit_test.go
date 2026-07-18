package middleware_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/finora/shared/middleware"
	"github.com/gin-gonic/gin"
)

// newBodyLimitTestRouter wires BodyLimit in front of a handler that reads
// the full request body itself (the same shape every real handler in this
// fleet takes via c.ShouldBindJSON: read the body, and if that errors,
// respond accordingly). BodyLimit never rejects a request itself — it only
// makes the body reader fail once a handler tries to read past the limit —
// so the test has to actually read the body to observe the effect, exactly
// like c.ShouldBindJSON would.
func newBodyLimitTestRouter(maxBytes int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.BodyLimit(maxBytes))
	r.POST("/echo", func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.String(http.StatusBadRequest, "read error: %v", err)
			return
		}
		c.String(http.StatusOK, "read %d bytes", len(body))
	})
	return r
}

func TestBodyLimit_UnderLimitPassesThroughUntouched(t *testing.T) {
	r := newBodyLimitTestRouter(1024)

	payload := bytes.Repeat([]byte("a"), 100)
	req := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader(payload))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200 for a body under the limit; body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "read 100 bytes") {
		t.Errorf("expected the handler to see the full 100-byte body untouched, got: %s", w.Body.String())
	}
}

func TestBodyLimit_OverLimitFailsWhenHandlerReadsIt(t *testing.T) {
	r := newBodyLimitTestRouter(100)

	payload := bytes.Repeat([]byte("a"), 1000)
	req := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader(payload))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400 (read error) for a body over the limit; body: %s", w.Code, w.Body.String())
	}
}

func TestBodyLimit_ZeroOrNegativeDisablesLimitEntirely(t *testing.T) {
	for _, maxBytes := range []int64{0, -1} {
		r := newBodyLimitTestRouter(maxBytes)

		payload := bytes.Repeat([]byte("a"), 10_000)
		req := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader(payload))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("maxBytes=%d: got %d, want 200 (limit should be fully disabled); body: %s", maxBytes, w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "read 10000 bytes") {
			t.Errorf("maxBytes=%d: expected the large body to reach the handler untouched, got: %s", maxBytes, w.Body.String())
		}
	}
}
