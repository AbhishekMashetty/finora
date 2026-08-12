// Package health implements the /live, /ready, /health trio every Finora
// service exposes, following Kubernetes probe conventions: liveness never
// depends on downstream state, readiness does.
package health

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

// Checker is a single dependency readiness check (e.g. a Mongo ping).
type Checker interface {
	Name() string
	Check(ctx context.Context) error
}

// Gate is a Checker that starts healthy and can be flipped to failing
// exactly once. shared/server.Run flips it at the very start of its
// shutdown sequence — before calling http.Server's own Shutdown — so
// /ready starts reporting "not ready" while the server is still accepting
// and completing in-flight requests normally. That ordering is what gives
// a Kubernetes Service (or any load balancer polling /ready) a window to
// stop routing new traffic here before the listener actually stops
// accepting connections. Without it, SIGTERM and this pod's eventual
// removal from Service endpoints race — endpoint propagation across every
// kube-proxy/ingress controller typically takes a few seconds — so
// requests keep landing on a pod that has already begun shutting down,
// which is what turns a rolling deploy into a burst of connection errors.
type Gate struct {
	down atomic.Bool
}

// Name implements Checker.
func (g *Gate) Name() string { return "shutdown" }

// Check implements Checker: fails once MarkNotReady has been called, and
// never recovers — a gate only ever transitions healthy -> not ready.
func (g *Gate) Check(_ context.Context) error {
	if g.down.Load() {
		return errors.New("server is shutting down")
	}
	return nil
}

// MarkNotReady flips the gate so Check starts failing. Safe to call more
// than once, and from any goroutine.
func (g *Gate) MarkNotReady() {
	g.down.Store(true)
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
