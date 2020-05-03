package testsuite

import (
	"testing"

	"project/internal/nettool"
)

func TestInitHTTPServers(t *testing.T) {
	IPv4Enabled = false
	IPv6Enabled = true
	defer func() { IPv4Enabled, IPv6Enabled = nettool.IPEnabled() }()

	initHTTPServers(t)
}

func TestNopCloser(t *testing.T) {
	closer := NewNopCloser()
	closer.Get()
	_ = closer.Close()
}
