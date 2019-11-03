package testsuite

import (
	"testing"
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

func TestGetIPv4Domain(t *testing.T) {
	for i := 0; i < 20; i++ {
		t.Log("get IPv4 domain:", GetIPv4Domain())
	}
}

func TestGetIPv6Domain(t *testing.T) {
	for i := 0; i < 20; i++ {
		t.Log("get IPv6 domain:", GetIPv6Domain())
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
