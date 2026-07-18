// Package router assembles the gin.Engine for notification-service: shared
// cross-cutting middleware in the mandated order, public health routes, and
// the owner-scoped /api/v1 notification routes behind RequireIdentity.
package router

import (
	"log/slog"

	"github.com/finora/notification-service/internal/handler"
	"github.com/finora/shared/health"
	"github.com/finora/shared/middleware"
	"github.com/gin-gonic/gin"
)

// New builds the fully-wired gin.Engine.
func New(log *slog.Logger, corsOrigins []string, notificationHandler *handler.NotificationHandler, checkers ...health.Checker) *gin.Engine {
	r := gin.New()

	// No CORS middleware here: this service is only ever called by the
	// gateway (server-to-server), never directly by a browser — the gateway
	// is the sole CORS boundary (architecture/api-contracts.md). Applying it
	// here too caused a real bug: httputil.ReverseProxy copies backend
	// response headers via Header().Add, so the gateway's and this
	// service's Access-Control-Allow-Origin values stacked into one
	// comma-joined header, which browsers reject outright. corsOrigins is
	// kept as a parameter for call-site/config compatibility but unused.
	r.Use(middleware.RequestID())
	r.Use(middleware.Logging(log))
	r.Use(middleware.Recovery(log))

	health.Register(r, "notification-service", checkers...)

	v1 := r.Group("/api/v1")
	v1.Use(middleware.RequireIdentity())
	{
		notifications := v1.Group("/notifications")
		notifications.GET("", notificationHandler.List)
		notifications.POST("", notificationHandler.Create)
		notifications.PATCH("/:id/read", notificationHandler.MarkRead)
	}

	return r
}
