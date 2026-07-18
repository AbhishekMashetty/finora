package middleware

import (
	"log/slog"
	"net/http"

	"github.com/finora/shared/httpx"
	"github.com/gin-gonic/gin"
)

// Recovery converts a panic into a structured log line and a clean 500
// envelope instead of a bare stack trace leaking to the client.
func Recovery(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				requestID, _ := c.Get("request_id")
				log.Error("panic recovered",
					slog.String("request_id", toString(requestID)),
					slog.Any("panic", r),
				)
				httpx.Fail(c, http.StatusInternalServerError, httpx.CodeInternal, "internal server error", nil)
				c.Abort()
			}
		}()
		c.Next()
	}
}
