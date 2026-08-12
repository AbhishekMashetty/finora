package server

import (
	"log/slog"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestNewHTTPServer_SetsExplicitTimeouts(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	srv := newHTTPServer("127.0.0.1:0", http.NewServeMux(), log)

	tests := []struct {
		name string
		got  time.Duration
	}{
		{"ReadHeaderTimeout", srv.ReadHeaderTimeout},
		{"ReadTimeout", srv.ReadTimeout},
		{"WriteTimeout", srv.WriteTimeout},
		{"IdleTimeout", srv.IdleTimeout},
	}
	for _, tt := range tests {
		if tt.got <= 0 {
			t.Errorf("%s = %v, want a positive timeout (Go's zero value means unbounded, which is the Slowloris vector this fixes)", tt.name, tt.got)
		}
	}
}

func TestNewHTTPServer_SetsMaxHeaderBytes(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	srv := newHTTPServer("127.0.0.1:0", http.NewServeMux(), log)

	if srv.MaxHeaderBytes <= 0 {
		t.Errorf("MaxHeaderBytes = %d, want a positive bound", srv.MaxHeaderBytes)
	}
}
