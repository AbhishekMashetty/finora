package proxy

import (
	"net/http/httputil"

	"github.com/finora/shared/middleware"
	"github.com/gin-gonic/gin"
)

// Handler returns a gin.HandlerFunc that forwards the request to rp
// unchanged, first stamping the X-Request-Id header (generated/echoed by
// middleware.RequestID()) onto the outgoing request so it propagates to the
// backend regardless of whether this route is public or JWT-protected.
//
// X-User-Id, when required, is set separately by internal/authmw before this
// handler runs (for protected routes only) — this handler never touches it.
func Handler(rp *httputil.ReverseProxy) gin.HandlerFunc {
	return func(c *gin.Context) {
		if id, ok := c.Get("request_id"); ok {
			if s, ok := id.(string); ok && s != "" {
				c.Request.Header.Set(middleware.RequestIDHeader, s)
			}
		}
		rp.ServeHTTP(c.Writer, c.Request)
	}
}
