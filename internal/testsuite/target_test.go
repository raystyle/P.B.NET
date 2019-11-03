package testsuite

import (
	"testing"
)

func TestGetIPv4Address(t *testing.T) {
	for i := 0; i < 20; i++ {
		t.Log("get IPv4 address:", getIPv4Address())
	}
}

func TestGetIPv6Address(t *testing.T) {
	for i := 0; i < 20; i++ {
		t.Log("get IPv6 address:", getIPv6Address())
	}
}
