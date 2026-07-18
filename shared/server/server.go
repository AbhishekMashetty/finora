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
)

// Run starts handler on addr and blocks until SIGINT/SIGTERM, then drains
// in-flight requests for up to shutdownTimeout before returning.
func Run(addr string, handler http.Handler, log *slog.Logger, shutdownTimeout time.Duration) error {
	srv := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

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

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("graceful shutdown failed", slog.String("error", err.Error()))
		return err
	}
	log.Info("server shut down cleanly")
	return nil
}
