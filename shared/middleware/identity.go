package middleware

import (
	"net/http"

	"github.com/finora/shared/httpx"
	"github.com/gin-gonic/gin"
)

const UserIDHeader = "X-User-Id"

// RequireIdentity enforces that the gateway has already authenticated the
// caller and forwarded their user id. Downstream services never validate
// JWTs themselves (see architecture/api-contracts.md, Auth Header Contract) —
// they trust this header on the internal network and reject its absence.
func RequireIdentity() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetHeader(UserIDHeader)
		if userID == "" {
			httpx.Fail(c, http.StatusUnauthorized, httpx.CodeUnauthorized, "missing user identity", nil)
			c.Abort()
			return
		}
		c.Set("user_id", userID)
		c.Next()
	}
}

// UserID reads the authenticated user id set by RequireIdentity.
func UserID(c *gin.Context) string {
	if v, ok := c.Get("user_id"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
