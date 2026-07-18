// Package health implements the /live, /ready, /health trio every Finora
// service exposes, following Kubernetes probe conventions: liveness never
// depends on downstream state, readiness does.
package health

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Checker is a single dependency readiness check (e.g. a Mongo ping).
type Checker interface {
	Name() string
	Check(ctx context.Context) error
}

type checkResult struct {
	Name  string `json:"name"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func runChecks(checkers []Checker) ([]checkResult, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	allOK := true
	results := make([]checkResult, 0, len(checkers))
	for _, ch := range checkers {
		if err := ch.Check(ctx); err != nil {
			allOK = false
			results = append(results, checkResult{Name: ch.Name(), OK: false, Error: err.Error()})
			continue
		}
		results = append(results, checkResult{Name: ch.Name(), OK: true})
	}
	return results, allOK
}

// Register wires /live, /ready and /health onto the given router group.
func Register(r gin.IRouter, serviceName string, checkers ...Checker) {
	r.GET("/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": serviceName})
	})

	r.GET("/ready", func(c *gin.Context) {
		results, ok := runChecks(checkers)
		if !ok {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "service": serviceName, "checks": results})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": serviceName, "checks": results})
	})

	r.GET("/health", func(c *gin.Context) {
		results, ok := runChecks(checkers)
		status := http.StatusOK
		state := "ok"
		if !ok {
			status = http.StatusServiceUnavailable
			state = "degraded"
		}
		c.JSON(status, gin.H{"status": state, "service": serviceName, "checks": results})
	})
}
