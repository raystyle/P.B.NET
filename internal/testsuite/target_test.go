package testsuite

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGetIPv4Address(t *testing.T) {
	for i := 0; i < 20; i++ {
		t.Log("get IPv4 address:", GetIPv4Address())
	}
}

func TestGetIPv6Address(t *testing.T) {
	for i := 0; i < 20; i++ {
		t.Log("get IPv6 address:", GetIPv6Address())
	}
}

func TestGetHTTP(t *testing.T) {
	for i := 0; i < 20; i++ {
		t.Log("get http:", GetHTTP())
	}
}

func TestGetHTTPS(t *testing.T) {
	for i := 0; i < 20; i++ {
		t.Log("get https:", GetHTTPS())
	}
}

func TestHTTPResponse(t *testing.T) {
	tr := http.Transport{}
	client := http.Client{
		Transport: &tr,
		Timeout:   time.Minute,
	}
	defer client.CloseIdleConnections()

	resp, err := client.Get(GetHTTP())
	require.NoError(t, err)
	HTTPResponse(t, resp)

	resp, err = client.Get(GetHTTPS())
	require.NoError(t, err)
	HTTPResponse(t, resp)
}
