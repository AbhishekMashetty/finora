// Package router wires the gin.Engine: middleware chain, health endpoints,
// public auth passthrough routes, and the JWT-protected catch-all that
// resolves each request to its owning backend proxy.
package router

import (
	"log/slog"
	"net/http"
	"net/http/httputil"

	"github.com/finora/shared/health"
	"github.com/finora/shared/httpx"
	"github.com/finora/shared/middleware"
	"github.com/finora/shared/openapidoc"
	"github.com/gin-gonic/gin"

	"github.com/finora/gateway/internal/authmw"
	gwconfig "github.com/finora/gateway/internal/config"
	"github.com/finora/gateway/internal/proxy"
)

// Backends holds one reverse proxy per downstream service, already built by
// the caller (see internal/proxy.New) from USER_SERVICE_URL,
// EXPENSE_SERVICE_URL, BUDGET_SERVICE_URL and NOTIFICATION_SERVICE_URL.
type Backends struct {
	User         *httputil.ReverseProxy
	Expense      *httputil.ReverseProxy
	Budget       *httputil.ReverseProxy
	Notification *httputil.ReverseProxy
}

// New builds the fully wired gin.Engine for the gateway.
func New(cfg gwconfig.Config, log *slog.Logger, b Backends, openapiSpec []byte) *gin.Engine {
	r := gin.New()

	// MIDDLEWARE ORDER: RequestID -> Logging -> Recovery -> CORS -> RateLimit.
	// RateLimit sits last in the chain (Phase 6) and, like CORS, is
	// deliberately gateway-only — see shared/middleware.RateLimit's doc
	// comment for why backend services must never re-apply it themselves.
	r.Use(middleware.RequestID())
	r.Use(middleware.Logging(log))
	r.Use(middleware.Recovery(log))
	r.Use(middleware.CORS(cfg.CORSAllowedOrigins))
	r.Use(middleware.RateLimit(float64(cfg.RateLimitRequestsPerSecond), cfg.RateLimitBurst))

	// Health routes are public and mirror /live — the gateway owns no data,
	// so it registers no checkers (its /ready is never more meaningful than
	// /live, per architecture/api-contracts.md).
	health.Register(r, "gateway")
	r.GET("/openapi.yaml", openapidoc.Handler(openapiSpec))

	// ---- Public routes (no JWT check): straight through to user-service ----
	r.POST("/api/v1/auth/register", proxy.Handler(b.User))
	r.POST("/api/v1/auth/login", proxy.Handler(b.User))
	r.POST("/api/v1/auth/refresh", proxy.Handler(b.User))
	r.POST("/api/v1/auth/password-reset", proxy.Handler(b.User))

	// ---- Protected routes: JWT-validate, then resolve to owning backend ----
	table := proxy.Table{
		{Prefix: "/api/v1/auth/logout", Proxy: b.User},
		{Prefix: "/api/v1/users/me", Proxy: b.User}, // covers /users/me and /users/me/settings
		{Prefix: "/api/v1/accounts", Proxy: b.Expense},
		{Prefix: "/api/v1/transactions", Proxy: b.Expense},
		{Prefix: "/api/v1/categories", Proxy: b.Expense},
		{Prefix: "/api/v1/budgets", Proxy: b.Budget},
		{Prefix: "/api/v1/goals", Proxy: b.Budget},
		{Prefix: "/api/v1/reports", Proxy: b.Budget},
		{Prefix: "/api/v1/notifications", Proxy: b.Notification},
	}

	// NOTE on implementation: the spec's suggested
	// `group.Any("/*proxyPath", ...)` mount conflicts with gin's radix tree
	// once static routes (e.g. /api/v1/auth/register) already occupy a
	// segment of the same prefix — gin panics at startup with "catch-all
	// wildcard conflicts with existing path segment". NoRoute sits outside
	// the trie entirely, so it composes cleanly with the public static
	// routes above while still running the same auth-then-resolve chain for
	// every other /api/v1/* path, with the full original path/query
	// untouched (Table.Resolve only inspects c.Request.URL.Path to pick a
	// backend; it never rewrites the request).
	r.NoRoute(authmw.RequireAccessToken(cfg.JWTAccessSecret), func(c *gin.Context) {
		rp, ok := table.Resolve(c.Request.URL.Path)
		if !ok {
			httpx.Fail(c, http.StatusNotFound, httpx.CodeNotFound, "no route matches this path", nil)
			return
		}
		proxy.Handler(rp)(c)
	})

	return r
}
