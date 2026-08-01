// Package config loads notification-service's environment configuration,
// wrapping shared/config so every value still flows through the same
// panic-fast-on-missing-secret / sane-fallback helpers used fleet-wide.
package config

import (
	"strings"
	"time"

	"github.com/finora/shared/config"
)

// Config holds everything main.go needs to wire the service together.
type Config struct {
	Port               string
	MongoURI           string
	LogLevel           string
	ShutdownTimeout    time.Duration
	CORSAllowedOrigins []string
	NATSURL            string
}

// Load reads the service's env vars. Exact names come from the repo-root
// .env.example: NOTIFICATION_SERVICE_PORT, NOTIFICATION_SERVICE_MONGO_URI,
// LOG_LEVEL, SHUTDOWN_TIMEOUT, CORS_ALLOWED_ORIGINS, NATS_URL. NATS_URL
// (Phase 7) defaults to the docker-compose service name, same reasoning as
// every other cross-service URL defaulting to its compose service name.
func Load() Config {
	raw := strings.Split(config.GetEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000"), ",")
	origins := make([]string, 0, len(raw))
	for _, o := range raw {
		if trimmed := strings.TrimSpace(o); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}

	return Config{
		Port:               config.GetEnv("NOTIFICATION_SERVICE_PORT", "8084"),
		MongoURI:           config.MustGetEnv("NOTIFICATION_SERVICE_MONGO_URI"),
		LogLevel:           config.GetEnv("LOG_LEVEL", "info"),
		ShutdownTimeout:    config.GetEnvDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
		CORSAllowedOrigins: origins,
		NATSURL:            config.GetEnv("NATS_URL", "nats://nats:4222"),
	}
}
