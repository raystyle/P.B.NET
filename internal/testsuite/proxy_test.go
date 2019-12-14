package testsuite

import (
	"testing"
)

func TestNopCloser(t *testing.T) {
	closer := NewNopCloser()
	closer.Get()
	_ = closer.Close()
}
