// Package httpclient provides the one tuned *http.Transport every Finora
// component that makes outbound HTTP calls to another Finora service
// should use, instead of inheriting Go's http.DefaultTransport
// (MaxIdleConnsPerHost: http.DefaultMaxIdleConnsPerHost, i.e. 2). Two call
// sites need this today: the gateway's reverse proxies to each backend
// service (services/gateway/internal/proxy), and budget-service's REST
// client to expense-service (services/budget-service/internal/client) —
// the fleet's two hottest internal-network paths.
//
// Why the default is actively dangerous under load, not just slow: beyond
// two concurrent requests to the same backend, every additional request
// opens a fresh TCP connection, and because that connection has nowhere to
// go back to (the idle pool is already full), the initiating side closes
// it and holds the socket in TIME_WAIT for 60 seconds. Sustained load
// against one backend then exhausts the local ephemeral port range
// (~28,000 ports on Linux by default) and every further call fails with
// "cannot assign requested address" — a total outage produced entirely by
// a default, not a real capacity limit.
package httpclient

import (
	"net"
	"net/http"
	"time"
)

// NewTransport returns an *http.Transport tuned for sustained,
// high-concurrency calls to a small, fixed set of internal backend hosts —
// exactly the shape of every internal call in this fleet. It is not a
// general-purpose fan-out client: the pool sizes assume one Finora pod
// talking to a handful of other Finora services on the internal network,
// not an unbounded number of external hosts.
func NewTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   3 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2: true,

		// The actual fix: http.DefaultTransport's MaxIdleConnsPerHost is 2,
		// fine for occasional calls and pathological under sustained
		// concurrent load — see the package doc comment.
		MaxIdleConnsPerHost: 256,
		MaxIdleConns:        512,

		// Bulkhead: bounds how many connections (idle + active) this
		// transport will ever hold open to one host, so a single slow or
		// unreachable backend can't consume every file descriptor this pod
		// has.
		MaxConnsPerHost: 512,

		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   3 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}
