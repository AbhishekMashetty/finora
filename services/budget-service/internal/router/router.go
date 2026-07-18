// Package router assembles the gin.Engine for budget-service: shared
// cross-cutting middleware in the mandated order, public health routes, and
// the owner-scoped /api/v1 routes behind RequireIdentity.
package router

import (
	"log/slog"

	"github.com/finora/budget-service/internal/handler"
	"github.com/finora/shared/health"
	"github.com/finora/shared/middleware"
	"github.com/gin-gonic/gin"
)

// New builds the fully-wired gin.Engine.
//
// Middleware order matters: RequestID -> Logging -> Recovery, then health
// routes are registered public (kubelet probes don't carry X-User-Id), and
// finally the /api/v1 group has RequireIdentity applied to the whole group
// since every route under it is owner-scoped. No CORS here — see the
// comment in New() for why.
func New(
	log *slog.Logger,
	corsOrigins []string,
	budgetHandler *handler.BudgetHandler,
	goalHandler *handler.GoalHandler,
	reportHandler *handler.ReportHandler,
	checkers ...health.Checker,
) *gin.Engine {
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

	health.Register(r, "budget-service", checkers...)

	v1 := r.Group("/api/v1")
	v1.Use(middleware.RequireIdentity())
	{
		budgets := v1.Group("/budgets")
		budgets.GET("", budgetHandler.List)
		budgets.POST("", budgetHandler.Create)
		budgets.GET("/:id", budgetHandler.Get)
		budgets.PUT("/:id", budgetHandler.Update)
		budgets.DELETE("/:id", budgetHandler.Delete)

		goals := v1.Group("/goals")
		goals.GET("", goalHandler.List)
		goals.POST("", goalHandler.Create)
		goals.GET("/:id", goalHandler.Get)
		goals.PUT("/:id", goalHandler.Update)
		goals.DELETE("/:id", goalHandler.Delete)

		reports := v1.Group("/reports")
		reports.GET("/summary", reportHandler.Summary)
	}

	return r
}
