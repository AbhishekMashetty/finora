// Package openapidoc serves each service's openapi.yaml live over HTTP
// (Phase 6 — "OpenAPI specs served as docs", not just correct static files
// nobody but a developer with a local checkout can browse).
//
// Deliberately reads the file from disk at startup rather than Go's
// //go:embed: embed patterns cannot reference a parent directory ("..")
// under Go's own build restrictions, and openapi.yaml lives at each
// service's root (services/<name>/openapi.yaml, per CLAUDE.md's documented
// per-service layout) while the code that would embed it lives under
// internal/ or cmd/ — moving the spec file to make it embeddable would
// violate that documented layout instead. Each service's Dockerfile copies
// openapi.yaml into the final image and sets WORKDIR so the same relative
// path ("openapi.yaml") resolves correctly in both a Docker container and
// a native `go run`/`make run` from the service's own directory — no
// environment-specific path logic needed here.
package openapidoc

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/finora/shared/httpx"
	"github.com/gin-gonic/gin"
)

// Load reads path (typically just "openapi.yaml") and returns its bytes, or
// nil if the file can't be read. A missing spec is logged but never fatal —
// GET /openapi.yaml is a nice-to-have docs endpoint, not correctness-
// critical the way GET /ready's Mongo check is, so a bad path degrades to a
// 404 on that one route rather than crashing the whole service at boot.
func Load(path string, log *slog.Logger) []byte {
	b, err := os.ReadFile(path)
	if err != nil {
		log.Warn("could not read openapi.yaml — GET /openapi.yaml will 404 until this is fixed",
			slog.String("path", path), slog.String("error", err.Error()))
		return nil
	}
	return b
}

// Handler returns a gin.HandlerFunc serving spec (as loaded by Load) as
// GET /openapi.yaml. A nil/empty spec (Load failed) responds with the
// standard 404 envelope, never a bare empty body.
func Handler(spec []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		if len(spec) == 0 {
			httpx.Fail(c, http.StatusNotFound, httpx.CodeNotFound, "openapi spec not available", nil)
			return
		}
		c.Data(http.StatusOK, "application/yaml; charset=utf-8", spec)
	}
}
