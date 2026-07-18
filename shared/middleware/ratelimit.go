package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/finora/shared/httpx"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// visitor pairs a per-client token-bucket limiter with when it was last
// used, so RateLimit's background sweep can evict clients that have gone
// quiet instead of growing the visitors map forever.
type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimit is gateway-only, by the same reasoning CLAUDE.md §5 already
// gives for CORS: the gateway is the single public entrypoint every
// external request passes through, so this is the one place a cross-
// cutting request-shaping concern belongs — backend services trust the
// internal network already and must never re-apply it themselves (the
// exact "backends must not duplicate a gateway-only concern" rule that
// shipped a real duplicate-CORS-header bug once when it wasn't followed).
//
// Deliberately in-memory (golang.org/x/time/rate, the standard Go rate-
// limiting primitive — not a third-party library), not Redis-backed:
// Redis is explicitly on plan.md's "Future Evolution, don't introduce
// before there's a legitimate reason" list, and today's deployment is a
// single gateway instance (Kubernetes HPA / multi-replica scaling is
// Phase 9, not built yet). An in-memory limiter is exactly right for one
// instance and gets revisited if/when the gateway is ever horizontally
// scaled — at which point per-instance limits would under-enforce (each
// replica has its own independent bucket), and that's the legitimate
// reason to introduce Redis for this, not before.
//
// Keyed by client IP (gin's c.ClientIP(), which already respects
// configured trusted-proxy headers) rather than authenticated user ID,
// because rate limiting has to cover the public, unauthenticated routes
// too (register/login/refresh/password-reset) — precisely the endpoints
// most worth protecting from brute-force, and JWT validation for
// protected routes happens later in the chain (authmw, only in the
// catch-all route resolver), so a user ID isn't available this early for
// every request uniformly.
//
// RateLimit returns gin middleware enforcing a token-bucket limit of
// requestsPerSecond sustained per client IP, allowing short bursts up to
// burst above that. A request beyond the limit gets 429 RATE_LIMITED via
// the standard envelope, never a bare status code.
//
// requestsPerSecond <= 0 or burst <= 0 disables rate limiting entirely
// (every request passes through) rather than the literal token-bucket
// interpretation of "zero capacity" — which would silently block 100% of
// traffic. This matters because it makes the zero-value gin.HandlerFunc
// (e.g. a Config struct that never set these fields, as every pre-existing
// router test in this fleet does) fail open, not closed: forgetting to
// configure a rate limit must never look like a full outage. Caught live —
// the gateway's own pre-existing router_test.go, which never set these
// fields, started failing every single request with 429 the moment this
// middleware was wired in, before this guard was added.
func RateLimit(requestsPerSecond float64, burst int) gin.HandlerFunc {
	if requestsPerSecond <= 0 || burst <= 0 {
		return func(c *gin.Context) { c.Next() }
	}

	var (
		mu       sync.Mutex
		visitors = make(map[string]*visitor)
	)

	// Sweep stale entries periodically so a client that stops sending
	// requests doesn't keep its limiter (and map slot) alive forever —
	// unbounded per-IP map growth would itself be a resource-exhaustion
	// vector, the exact thing this middleware exists to prevent in the
	// first place.
	go func() {
		for {
			time.Sleep(time.Minute)
			mu.Lock()
			for ip, v := range visitors {
				if time.Since(v.lastSeen) > 3*time.Minute {
					delete(visitors, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		ip := c.ClientIP()

		mu.Lock()
		v, ok := visitors[ip]
		if !ok {
			v = &visitor{limiter: rate.NewLimiter(rate.Limit(requestsPerSecond), burst)}
			visitors[ip] = v
		}
		v.lastSeen = time.Now()
		allowed := v.limiter.Allow()
		mu.Unlock()

		if !allowed {
			httpx.Fail(c, http.StatusTooManyRequests, httpx.CodeRateLimited, "too many requests, slow down and try again", nil)
			c.Abort()
			return
		}
		c.Next()
	}
}
