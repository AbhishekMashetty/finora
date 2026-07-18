// Package proxy builds one reverse proxy per backend service and provides a
// small routing table mapping URL prefixes to the proxy that should handle
// them. The gateway owns no business logic of its own — every matched
// request is forwarded byte-for-byte (path, query, method, body) to the
// owning backend.
package proxy

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/finora/shared/httpx"
)

// New builds a reverse proxy targeting rawTarget (e.g.
// "http://user-service:8081"). httputil.NewSingleHostReverseProxy's default
// Director already forwards the incoming request's full path and query
// string unchanged onto the target's scheme+host, which is exactly what
// backends expect (they route on the complete /api/v1/... path themselves),
// so no path rewriting is added here.
func New(rawTarget string, log *slog.Logger) (*httputil.ReverseProxy, error) {
	target, err := url.Parse(rawTarget)
	if err != nil {
		return nil, err
	}

	rp := httputil.NewSingleHostReverseProxy(target)

	// A downstream service being unreachable is not a gateway bug — but the
	// client still needs the standard envelope shape rather than a raw
	// connection-refused error leaking through.
	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		requestID := r.Header.Get("X-Request-Id")
		log.Error("proxy request failed",
			slog.String("target", rawTarget),
			slog.String("path", r.URL.Path),
			slog.String("request_id", requestID),
			slog.String("error", err.Error()),
		)

		env := httpx.Envelope{
			Success: false,
			Data:    nil,
			Error: &httpx.ErrorBody{
				Code:    httpx.CodeInternal,
				Message: "upstream service unavailable",
			},
			RequestID: requestID,
		}
		body, _ := json.Marshal(env)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write(body)
	}

	return rp, nil
}

// Route pairs a URL path prefix with the reverse proxy that owns it.
type Route struct {
	Prefix string
	Proxy  *httputil.ReverseProxy
}

// Table is an ordered set of prefix -> proxy mappings used to resolve which
// backend a given request path belongs to.
type Table []Route

// Resolve returns the proxy for the longest matching prefix of path, so
// that more specific prefixes (if any overlap) win over shorter ones.
func (t Table) Resolve(path string) (*httputil.ReverseProxy, bool) {
	var best *Route
	for i := range t {
		r := &t[i]
		if strings.HasPrefix(path, r.Prefix) {
			if best == nil || len(r.Prefix) > len(best.Prefix) {
				best = r
			}
		}
	}
	if best == nil {
		return nil, false
	}
	return best.Proxy, true
}
