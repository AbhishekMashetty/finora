// Package config loads expense-service's environment configuration, wrapping
// shared/config so every value keeps flowing through one env-var seam that
// maps directly onto Kubernetes ConfigMaps/Secrets.
package config

import (
	"strings"
	"time"

	"github.com/finora/shared/config"
)

// Config holds all environment-derived settings for expense-service.
type Config struct {
	Port               string
	MongoURI           string
	LogLevel           string
	ShutdownTimeout    time.Duration
	CORSAllowedOrigins []string
}

// Load reads the process environment and returns a populated Config.
// EXPENSE_SERVICE_MONGO_URI has no sane default, so a missing value panics at
// boot rather than causing a confusing runtime connection failure later.
func Load() Config {
	port := config.GetEnv("EXPENSE_SERVICE_PORT", "8082")
	mongoURI := config.MustGetEnv("EXPENSE_SERVICE_MONGO_URI")
	logLevel := config.GetEnv("LOG_LEVEL", "info")
	shutdownTimeout := config.GetEnvDuration("SHUTDOWN_TIMEOUT", 10*time.Second)
	origins := config.GetEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")

	return Config{
		Port:               port,
		MongoURI:           mongoURI,
		LogLevel:           logLevel,
		ShutdownTimeout:    shutdownTimeout,
		CORSAllowedOrigins: splitCSV(origins),
	}
}

// Addr returns the bind address for the HTTP server.
func (c Config) Addr() string {
	return "0.0.0.0:" + c.Port
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
