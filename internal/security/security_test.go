package security

import (
	"bytes"
	"testing"
)

func TestFlushBytes(t *testing.T) {
	b1 := []byte{1, 2, 3, 4}
	b2 := []byte{1, 2, 3, 4}
	FlushBytes(b2)
	if bytes.Equal(b1, b2) {
		t.Fatal("flush slice failed")
	}
}
