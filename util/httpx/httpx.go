package httpx

import (
	"net"
	"net/http"
	"time"
)

// NewDefaultClient returns a new http client
// configured with sane timeout because globals are bad
func NewDefaultClient() *http.Client {
	// Start from the default transport so proxy settings (HTTPS_PROXY, NO_PROXY),
	// TLS handshake timeouts and connection pooling keep their defaults.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{
		Timeout:   2 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext

	return &http.Client{
		Timeout:   10 * time.Second,
		Transport: transport,
	}
}
