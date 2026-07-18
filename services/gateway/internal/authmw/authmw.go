// Package authmw is the gateway's JWT validation middleware. The gateway is
// the only Finora component that ever validates a JWT (see
// architecture/api-contracts.md, "Auth Header Contract") — every downstream
// service instead trusts the X-User-Id header this middleware sets.
package authmw

import (
	"net/http"
	"strings"

	"github.com/finora/shared/httpx"
	"github.com/finora/shared/jwtx"
	"github.com/finora/shared/middleware"
	"github.com/gin-gonic/gin"
)

const bearerPrefix = "Bearer "

// RequireAccessToken validates the Authorization: Bearer <token> header
// against secret using jwtx.Parse(..., jwtx.TypeAccess). On success it sets
// X-User-Id (from the token's sub claim) and X-Request-Id on the outgoing
// request before handing off to the next handler (the proxy). It never
// forwards any X-User-Id the original client may have sent — the header is
// always overwritten with the validated value.
func RequireAccessToken(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, bearerPrefix) {
			httpx.Fail(c, http.StatusUnauthorized, httpx.CodeUnauthorized, "missing or malformed authorization header", nil)
			c.Abort()
			return
		}

		token := strings.TrimPrefix(header, bearerPrefix)
		if token == "" {
			httpx.Fail(c, http.StatusUnauthorized, httpx.CodeUnauthorized, "missing or malformed authorization header", nil)
			c.Abort()
			return
		}

		claims, err := jwtx.Parse(secret, token, jwtx.TypeAccess)
		if err != nil {
			httpx.Fail(c, http.StatusUnauthorized, httpx.CodeUnauthorized, "invalid or expired token", nil)
			c.Abort()
			return
		}

		// Never trust a client-supplied X-User-Id — always overwrite with the
		// value we just validated.
		c.Request.Header.Set(middleware.UserIDHeader, claims.UserID)

		if id, ok := c.Get("request_id"); ok {
			if s, ok := id.(string); ok && s != "" {
				c.Request.Header.Set(middleware.RequestIDHeader, s)
			}
		}

		c.Set("user_id", claims.UserID)
		c.Next()
	}
}
