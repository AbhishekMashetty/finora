// Package config loads budget-service's environment configuration, wrapping
// shared/config so every env var name matches .env.example exactly.
package config

import (
	"net/url"
	"strings"
	"time"

	sharedconfig "github.com/finora/shared/config"
)

// Config is the fully-resolved configuration for one process lifetime.
type Config struct {
	Port                   string
	MongoURI               string
	LogLevel               string
	ShutdownTimeout        time.Duration
	CORSAllowedOrigins     []string
	ExpenseServiceURL      string
	NotificationServiceURL string
}

// Load reads every env var budget-service consumes. The Mongo URI has no
// sane default and uses MustGetEnv, so misconfiguration fails fast at boot
// rather than surfacing as a confusing runtime error later.
func Load() Config {
	return Config{
		Port:               sharedconfig.GetEnv("BUDGET_SERVICE_PORT", "8083"),
		MongoURI:           sharedconfig.MustGetEnv("BUDGET_SERVICE_MONGO_URI"),
		LogLevel:           sharedconfig.GetEnv("LOG_LEVEL", "info"),
		ShutdownTimeout:    sharedconfig.GetEnvDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
		CORSAllowedOrigins: splitCSV(sharedconfig.GetEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")),
		// ExpenseServiceURL is the Phase-3 cross-service call target for
		// reports (budget-vs-actual). Same docker-compose network address
		// the gateway uses to reach expense-service (see .env.example).
		ExpenseServiceURL: sharedconfig.GetEnv("EXPENSE_SERVICE_URL", "http://expense-service:8082"),
		// NotificationServiceURL is the Phase-4 cross-service call target
		// for the reports-triggered overspend notification (see
		// internal/service/report_service.go's Summary). Same
		// docker-compose network address the gateway uses to reach
		// notification-service (see .env.example) — the var itself already
		// existed as a forward-looking seam before this phase wired it up.
		NotificationServiceURL: sharedconfig.GetEnv("NOTIFICATION_SERVICE_URL", "http://notification-service:8084"),
	}
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// DBNameFromURI extracts the database name from a Mongo connection string
// (e.g. "mongodb://mongo-budget:27017/finora_budgets" -> "finora_budgets"),
// since shared/mongox.Connect only returns a *mongo.Client, not a bound
// database. Falls back to "finora_budgets" if the URI carries no path
// segment.
func DBNameFromURI(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return "finora_budgets"
	}
	name := strings.TrimPrefix(u.Path, "/")
	if name == "" {
		return "finora_budgets"
	}
	return name
}
