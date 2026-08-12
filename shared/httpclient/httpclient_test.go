package httpclient

import (
	"net/http"
	"testing"
)

func TestNewTransport_RaisesIdleConnsPerHostAboveTheDefault(t *testing.T) {
	tr := NewTransport()

	if tr.MaxIdleConnsPerHost <= http.DefaultMaxIdleConnsPerHost {
		t.Errorf("MaxIdleConnsPerHost = %d, want greater than http.DefaultMaxIdleConnsPerHost (%d) — "+
			"the default is what causes ephemeral-port exhaustion under sustained load",
			tr.MaxIdleConnsPerHost, http.DefaultMaxIdleConnsPerHost)
	}
}

func TestNewTransport_SetsConnectionBulkheadsAndTimeouts(t *testing.T) {
	tr := NewTransport()

	if tr.MaxConnsPerHost <= 0 {
		t.Error("MaxConnsPerHost is unset — a slow or unreachable backend could consume unlimited connections")
	}
	if tr.IdleConnTimeout <= 0 {
		t.Error("IdleConnTimeout is unset")
	}
	if tr.ResponseHeaderTimeout <= 0 {
		t.Error("ResponseHeaderTimeout is unset — a backend that accepts but never responds would hang forever")
	}
	if tr.DialContext == nil {
		t.Error("DialContext is unset — dialing would have no connect timeout")
	}
}
