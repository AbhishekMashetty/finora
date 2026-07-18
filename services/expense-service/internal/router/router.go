// Package router builds expense-service's gin.Engine, wiring middleware in
// the order every Finora service follows: RequestID -> Logging -> Recovery ->
// public health routes -> an /api/v1 group gated by RequireIdentity. No CORS
// here — see the comment in New() for why.
// (every route this service owns is owner-scoped; there are no public
// routes here).
package router

import (
	"log/slog"

	"github.com/finora/expense-service/internal/handler"
	"github.com/finora/shared/health"
	"github.com/finora/shared/middleware"
	"github.com/finora/shared/openapidoc"
	"github.com/gin-gonic/gin"
)

// Deps bundles everything the router needs to wire routes.
type Deps struct {
	Logger             *slog.Logger
	ServiceName        string
	CORSAllowedOrigins []string
	HealthCheckers     []health.Checker

	AccountHandler     *handler.AccountHandler
	TransactionHandler *handler.TransactionHandler
	CategoryHandler    *handler.CategoryHandler

	// OpenAPISpec is this service's openapi.yaml content (see
	// shared/openapidoc), served publicly at GET /openapi.yaml — Phase 6's
	// "OpenAPI specs served as docs". Nil is fine (openapidoc.Handler 404s
	// gracefully); not proxied through the gateway, since it's a developer/
	// operator convenience on the internal network, not end-user-facing
	// API surface the way /api/v1/* is.
	OpenAPISpec []byte
}

// New builds the fully-wired gin.Engine.
func New(d Deps) *gin.Engine {
	r := gin.New()

	// No CORS middleware here: this service is only ever called by the
	// gateway (server-to-server), never directly by a browser — the gateway
	// is the sole CORS boundary (architecture/api-contracts.md). Applying it
	// here too caused a real bug: httputil.ReverseProxy copies backend
	// response headers via Header().Add, so the gateway's and this
	// service's Access-Control-Allow-Origin values stacked into one
	// comma-joined header, which browsers reject outright. d.CORSAllowedOrigins
	// is kept on Deps for config-load compatibility but intentionally unused.
	r.Use(
		middleware.RequestID(),
		middleware.Logging(d.Logger),
		middleware.Recovery(d.Logger),
	)

	// Health routes are public — no identity required, mirroring Kubernetes
	// liveness/readiness probe conventions.
	health.Register(r, d.ServiceName, d.HealthCheckers...)

	// OpenAPI spec is also public — developer/operator docs, not user data.
	r.GET("/openapi.yaml", openapidoc.Handler(d.OpenAPISpec))

	api := r.Group("/api/v1")
	api.Use(middleware.RequireIdentity())
	{
		accounts := api.Group("/accounts")
		accounts.GET("", d.AccountHandler.List)
		accounts.POST("", d.AccountHandler.Create)
		accounts.GET("/:id", d.AccountHandler.Get)
		accounts.PUT("/:id", d.AccountHandler.Update)
		accounts.DELETE("/:id", d.AccountHandler.Delete)

		transactions := api.Group("/transactions")
		transactions.GET("", d.TransactionHandler.List)
		transactions.POST("", d.TransactionHandler.Create)
		transactions.GET("/:id", d.TransactionHandler.Get)
		transactions.PUT("/:id", d.TransactionHandler.Update)
		transactions.DELETE("/:id", d.TransactionHandler.Delete)

		categories := api.Group("/categories")
		categories.GET("", d.CategoryHandler.List)
		categories.POST("", d.CategoryHandler.Create)
	}

	return r
}
