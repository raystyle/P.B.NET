package security

import (
	"bytes"
	"testing"
)

func Test_Flush_Slice(t *testing.T) {
	b := []byte{1, 2, 3, 4}
	Flush_Bytes(b)
	if !bytes.Equal(b, bytes.Repeat([]byte{0}, 4)) {
		t.Fatal("flush slice failed")
	}
}
