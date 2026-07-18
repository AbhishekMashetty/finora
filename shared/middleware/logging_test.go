package middleware_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/finora/shared/middleware"
	"github.com/gin-gonic/gin"
)

func TestLogging(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantLogged bool
	}{
		{name: "health check /live is not logged", path: "/live", wantLogged: false},
		{name: "health check /ready is not logged", path: "/ready", wantLogged: false},
		{name: "health check /health is not logged", path: "/health", wantLogged: false},
		{name: "a normal API path is logged", path: "/api/v1/accounts", wantLogged: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			log := slog.New(slog.NewJSONHandler(&buf, nil))

			gin.SetMode(gin.TestMode)
			r := gin.New()
			r.Use(middleware.Logging(log))
			r.GET(tt.path, func(c *gin.Context) { c.Status(http.StatusOK) })

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", w.Code)
			}

			gotLogged := strings.Contains(buf.String(), "request completed")
			if gotLogged != tt.wantLogged {
				t.Fatalf("path %q: expected logged=%v, got logged=%v (output=%s)", tt.path, tt.wantLogged, gotLogged, buf.String())
			}
		})
	}
}
