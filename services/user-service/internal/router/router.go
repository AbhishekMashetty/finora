// Package router wires user-service's gin.Engine: middleware order, public
// health routes, and the /api/v1 route groups.
package router

import (
	"log/slog"

	"github.com/finora/shared/health"
	"github.com/finora/shared/middleware"
	"github.com/finora/user-service/internal/handler"
	"github.com/gin-gonic/gin"
)

// New builds the fully-wired gin.Engine. Middleware order matches every
// other Finora backend service: RequestID -> Logging -> Recovery (no CORS —
// that's the gateway's job alone, see the comment below), then public
// health routes, then the /api/v1 groups (auth is public, users requires
// identity).
func New(
	log *slog.Logger,
	corsAllowedOrigins []string,
	authHandler *handler.AuthHandler,
	userHandler *handler.UserHandler,
	checkers ...health.Checker,
) *gin.Engine {
	r := gin.New()

	r.Use(middleware.RequestID())
	r.Use(middleware.Logging(log))
	r.Use(middleware.Recovery(log))
	// No CORS middleware here: this service is only ever called by the
	// gateway (server-to-server), never directly by a browser. The gateway
	// is the sole CORS boundary — see architecture/api-contracts.md. Adding
	// CORS here too caused a real bug: httputil.ReverseProxy copies backend
	// response headers with Header().Add, so the gateway's and this
	// service's Access-Control-Allow-Origin values stacked into one
	// comma-joined header, which browsers reject outright.
	// (corsAllowedOrigins is accepted for signature/config-load compatibility
	// but intentionally unused.)

	health.Register(r, "user-service", checkers...)

	v1 := r.Group("/api/v1")

	auth := v1.Group("/auth")
	{
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)
		auth.POST("/refresh", authHandler.Refresh)
		auth.POST("/logout", authHandler.Logout)
		auth.POST("/password-reset", authHandler.RequestPasswordReset)
	}

	users := v1.Group("/users")
	users.Use(middleware.RequireIdentity())
	{
		users.GET("/me", userHandler.Me)
		users.PUT("/me", userHandler.UpdateMe)
		users.GET("/me/settings", userHandler.GetSettings)
		users.PUT("/me/settings", userHandler.UpdateSettings)
	}

	return r
}
