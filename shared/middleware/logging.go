package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// healthCheckPaths are hit every few seconds by Docker/Kubernetes probes
// regardless of real traffic; logging them at Info would drown out actual
// request activity, so they're skipped here rather than downgraded to Debug —
// there's no per-service Debug consumer today, and a skipped health-check log
// line is never something an operator needs to go back and find.
var healthCheckPaths = map[string]bool{
	"/live":   true,
	"/ready":  true,
	"/health": true,
}

// Logging emits one structured JSON line per request with request id, method,
// path, status and latency, so log aggregation needs no app-specific parsing.
// Health-check probe paths (/live, /ready, /health) are not logged.
func Logging(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		if healthCheckPaths[path] {
			c.Next()
			return
		}

		c.Next()

		requestID, _ := c.Get("request_id")
		attrs := []any{
			slog.String("request_id", toString(requestID)),
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.Int("status", c.Writer.Status()),
			slog.Duration("latency", time.Since(start)),
		}

		if len(c.Errors) > 0 {
			attrs = append(attrs, slog.String("error", c.Errors.String()))
			log.Error("request completed with errors", attrs...)
			return
		}
		log.Info("request completed", attrs...)
	}
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
