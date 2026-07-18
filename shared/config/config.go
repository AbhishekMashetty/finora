// Package config provides small env-var helpers so no service ever
// hardcodes a value that Kubernetes should own via ConfigMap/Secret.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// MustGetEnv returns the env var or panics — use only for values with no
// sane default (secrets, connection strings) so misconfiguration fails fast
// at boot rather than causing confusing runtime errors.
func MustGetEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("required environment variable %s is not set", key))
	}
	return v
}

// GetEnv returns the env var or a fallback default.
func GetEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// GetEnvDuration parses the env var as a Go duration (e.g. "15m"), or falls back.
func GetEnvDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

// GetEnvInt parses the env var as an int, or falls back.
func GetEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return i
}
