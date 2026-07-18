// Package config loads gateway configuration from environment variables,
// wrapping shared/config so every value name matches .env.example exactly.
package config

import (
	"strings"
	"time"

	"github.com/finora/shared/config"
)

// Config holds every environment-derived setting the gateway needs to boot.
type Config struct {
	// GatewayPort is the local bind port (e.g. "8080"); the server binds to
	// "0.0.0.0:"+GatewayPort.
	GatewayPort string

	UserServiceURL         string
	ExpenseServiceURL      string
	BudgetServiceURL       string
	NotificationServiceURL string

	JWTAccessSecret string

	LogLevel           string
	ShutdownTimeout    time.Duration
	CORSAllowedOrigins []string

	// RateLimitRequestsPerSecond/RateLimitBurst configure
	// shared/middleware.RateLimit — see that file's doc comment for why
	// this lives only in the gateway. Defaults (10 req/s sustained, burst
	// 20) are generous enough for legitimate parallel dashboard fetches
	// (the frontend's overview/search pages fire up to 5 concurrent
	// requests on load) while still meaningfully throttling abuse.
	RateLimitRequestsPerSecond int
	RateLimitBurst             int
}

// Load reads all gateway env vars, applying the exact names from
// architecture/api-contracts.md and .env.example. Required downstream URLs
// and the JWT secret fail fast via MustGetEnv since a misconfigured gateway
// with no route targets or no way to verify tokens is not safe to serve.
func Load() Config {
	origins := config.GetEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")

	return Config{
		GatewayPort: config.GetEnv("GATEWAY_PORT", "8080"),

		UserServiceURL:         config.MustGetEnv("USER_SERVICE_URL"),
		ExpenseServiceURL:      config.MustGetEnv("EXPENSE_SERVICE_URL"),
		BudgetServiceURL:       config.MustGetEnv("BUDGET_SERVICE_URL"),
		NotificationServiceURL: config.MustGetEnv("NOTIFICATION_SERVICE_URL"),

		JWTAccessSecret: config.MustGetEnv("JWT_ACCESS_SECRET"),

		LogLevel:           config.GetEnv("LOG_LEVEL", "info"),
		ShutdownTimeout:    config.GetEnvDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
		CORSAllowedOrigins: splitAndTrim(origins),

		RateLimitRequestsPerSecond: config.GetEnvInt("RATE_LIMIT_REQUESTS_PER_SECOND", 10),
		RateLimitBurst:             config.GetEnvInt("RATE_LIMIT_BURST", 20),
	}
}

func splitAndTrim(csv string) []string {
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
