package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// BodyLimit is gateway-only, by the same reasoning CLAUDE.md §5 already
// gives for CORS and shared/middleware.RateLimit's doc comment gives for
// rate limiting: the gateway is the single public entrypoint every external
// request passes through, so this is the one place a cross-cutting
// request-shaping concern belongs — backend services trust the internal
// network already and must never re-apply it themselves.
//
// BodyLimit returns gin middleware that caps the size of an incoming
// request body at maxBytes by wrapping c.Request.Body in an
// http.MaxBytesReader. It does not itself reject anything: the wrapped
// reader returns an *http.MaxBytesError the moment something tries to read
// past the limit. Today (gateway-only, per the doc comment above) that
// "something" is always httputil.ReverseProxy copying the request body
// onward to a backend — never a handler's own c.ShouldBindJSON, since the
// gateway has no request bodies of its own to bind. The proxy's
// ErrorHandler (services/gateway/internal/proxy/proxy.go) special-cases
// *http.MaxBytesError via errors.As to report VALIDATION_ERROR/400, rather
// than letting it fall through to the generic "upstream unavailable"
// INTERNAL_ERROR/502 branch — a first draft of that ErrorHandler didn't do
// this and silently misreported every oversized-body rejection as a
// backend outage, confirmed live via a real oversized request through the
// gateway before this comment was written. If BodyLimit is ever applied to
// a backend service directly (it currently is not — see the doc comment
// above for why), c.ShouldBindJSON's existing bind-error handling would
// need the same *http.MaxBytesError check for the envelope shape to stay
// correct there too.
//
// maxBytes <= 0 disables the limit entirely (a no-op passthrough) rather
// than the literal http.MaxBytesReader interpretation of "cap at zero
// bytes", which would reject every request with a body. This mirrors
// RateLimit's fail-open guard for the same reason: a Config that never set
// this field (a zero-value gwconfig.Config, as pre-existing router tests
// do) must not look like every request suddenly has an empty body allowed
// — forgetting to configure a limit must fail open, not closed.
func BodyLimit(maxBytes int64) gin.HandlerFunc {
	if maxBytes <= 0 {
		return func(c *gin.Context) { c.Next() }
	}

	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}
