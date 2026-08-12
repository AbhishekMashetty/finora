// Package server wraps http.Server with the graceful-shutdown behavior every
// Finora service needs to exit cleanly on SIGTERM (a pod eviction/rollout in
// Kubernetes), instead of dropping in-flight requests.
package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/finora/shared/health"
)

// newHTTPServer builds the *http.Server every Finora service runs, with
// every timeout explicit rather than left at Go's zero-value defaults —
// which mean "no limit", not "sensible default". Without
// ReadHeaderTimeout in particular, a client that opens a connection and
// sends headers one byte at a time (a Slowloris attack) holds a goroutine
// and a file descriptor open forever: the request never finishes
// arriving, so neither the rate limiter nor the body-size limiter ever
// sees it to reject it. WriteTimeout is set more generously (60s) than
// the others because budget-service's reports endpoint currently makes a
// serial, unbounded-in-practice number of cross-service calls to compute
// a summary; once that endpoint is backed by a single aggregation query
// instead of a per-budget pagination loop, this should shrink to match
// its real p99.
func newHTTPServer(addr string, handler http.Handler, log *slog.Logger) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MiB, matching shared/middleware.BodyLimit's own default ceiling
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelError),
	}
}

// Run starts handler on addr and blocks until SIGINT/SIGTERM.
//
// On a shutdown signal, if gate is non-nil, Run first calls
// gate.MarkNotReady() and waits drainDelay before calling http.Server's own
// Shutdown to drain in-flight requests for up to shutdownTimeout. That
// ordering exists because SIGTERM and this pod's removal from Kubernetes
// Service endpoints are dispatched concurrently, not sequentially —
// endpoint propagation across every kube-proxy/ingress controller
// typically takes a few seconds. Calling Shutdown immediately on SIGTERM
// means the pod keeps receiving real traffic for that whole window while
// it has already stopped wanting it, which is what turns every rolling
// deploy into a burst of connection errors. Flipping /ready first (via
// gate, which health.Register wires in as just another Checker) gives load
// balancers a chance to stop routing here before the listener actually
// stops accepting connections; only after drainDelay has passed does
// Shutdown begin draining what's still in flight. drainDelay should be set
// to at least readinessProbe.periodSeconds * failureThreshold, plus a
// margin. gate may be nil, in which case Shutdown is called immediately —
// the previous, pre-drain behavior — which is fine for any caller with no
// readiness probe watching it.
func Run(addr string, handler http.Handler, log *slog.Logger, shutdownTimeout, drainDelay time.Duration, gate *health.Gate) error {
	srv := newHTTPServer(addr, handler, log)

	errCh := make(chan error, 1)
	go func() {
		log.Info("server starting", slog.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case sig := <-stop:
		log.Info("shutdown signal received", slog.String("signal", sig.String()))
	}

	if gate != nil {
		gate.MarkNotReady()
		log.Info("marked not-ready, draining before shutdown", slog.Duration("drain", drainDelay))
		time.Sleep(drainDelay)
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("graceful shutdown failed", slog.String("error", err.Error()))
		return err
	}
	log.Info("server shut down cleanly")
	return nil
}
