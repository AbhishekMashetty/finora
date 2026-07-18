// Package config loads user-service's environment configuration, wrapping
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
	Port               string
	MongoURI           string
	JWTAccessSecret    string
	JWTRefreshSecret   string
	JWTAccessTTL       time.Duration
	JWTRefreshTTL      time.Duration
	LogLevel           string
	ShutdownTimeout    time.Duration
	CORSAllowedOrigins []string
}

// Load reads every env var user-service consumes. Secrets and the Mongo URI
// have no sane default and use MustGetEnv, so misconfiguration fails fast at
// boot rather than surfacing as a confusing runtime error later.
func Load() Config {
	return Config{
		Port:               sharedconfig.GetEnv("USER_SERVICE_PORT", "8081"),
		MongoURI:           sharedconfig.MustGetEnv("USER_SERVICE_MONGO_URI"),
		JWTAccessSecret:    sharedconfig.MustGetEnv("JWT_ACCESS_SECRET"),
		JWTRefreshSecret:   sharedconfig.MustGetEnv("JWT_REFRESH_SECRET"),
		JWTAccessTTL:       sharedconfig.GetEnvDuration("JWT_ACCESS_TTL", 15*time.Minute),
		JWTRefreshTTL:      sharedconfig.GetEnvDuration("JWT_REFRESH_TTL", 168*time.Hour),
		LogLevel:           sharedconfig.GetEnv("LOG_LEVEL", "info"),
		ShutdownTimeout:    sharedconfig.GetEnvDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
		CORSAllowedOrigins: splitCSV(sharedconfig.GetEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")),
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
// (e.g. "mongodb://mongo-user:27017/finora_users" -> "finora_users"), since
// shared/mongox.Connect only returns a *mongo.Client, not a bound database.
// Falls back to "finora_users" if the URI carries no path segment.
func DBNameFromURI(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return "finora_users"
	}
	name := strings.TrimPrefix(u.Path, "/")
	if name == "" {
		return "finora_users"
	}
	return name
}
