package httpx

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewDefaultHTTPClient(t *testing.T) {
	c := NewDefaultClient()
	assert.Equal(t, 10*time.Second, c.Timeout)

	// A bare http.Transport ignores HTTPS_PROXY; make sure we kept the default proxy lookup
	transport, ok := c.Transport.(*http.Transport)
	if assert.True(t, ok, "expected an *http.Transport") {
		assert.NotNil(t, transport.Proxy, "transport should honour proxy environment variables")
	}
}
