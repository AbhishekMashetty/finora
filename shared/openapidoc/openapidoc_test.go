package openapidoc_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/finora/shared/openapidoc"
	"github.com/gin-gonic/gin"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestLoad_ExistingFileReturnsItsBytes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "openapi.yaml")
	want := []byte("openapi: 3.0.3\ninfo:\n  title: test\n")
	if err := os.WriteFile(path, want, 0o644); err != nil {
		t.Fatalf("failed writing fixture file: %v", err)
	}

	got := openapidoc.Load(path, discardLogger())

	if string(got) != string(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestLoad_MissingFileReturnsNilAndLogsRatherThanPanicking(t *testing.T) {
	got := openapidoc.Load(filepath.Join(t.TempDir(), "does-not-exist.yaml"), discardLogger())

	if got != nil {
		t.Fatalf("expected nil for a missing file, got %q", got)
	}
}

func TestHandler_ServesTheSpecWithTheRightContentType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	spec := []byte("openapi: 3.0.3\n")
	r := gin.New()
	r.GET("/openapi.yaml", openapidoc.Handler(spec))

	req := httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/yaml; charset=utf-8" {
		t.Errorf("got Content-Type %q, want %q", ct, "application/yaml; charset=utf-8")
	}
	if w.Body.String() != string(spec) {
		t.Errorf("got body %q, want %q", w.Body.String(), spec)
	}
}

func TestHandler_EmptySpecReturns404NotFoundEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/openapi.yaml", openapidoc.Handler(nil))

	req := httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404 for a nil/empty spec; body: %s", w.Code, w.Body.String())
	}
}
