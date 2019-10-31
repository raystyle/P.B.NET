package testsuite

import (
	"testing"
)

func TestNopCloser(t *testing.T) {
	closer := NopCloser()
	closer.Get()
	_ = closer.Close()
}
