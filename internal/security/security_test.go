package security

import (
	"bytes"
	"strings"
	"testing"
)

func TestFlushBytes(t *testing.T) {
	b1 := []byte{1, 2, 3, 4}
	b2 := []byte{1, 2, 3, 4}
	FlushBytes(b2)
	if bytes.Equal(b1, b2) {
		t.Fatal("failed to flush bytes")
	}
}

func TestFlushString(t *testing.T) {
	// must use strings.Repeat to generate testdata
	// if you use this
	// s1 := "aaa"
	// s2 := "aaa"
	// FlushString(&s1) will panic, because it change const
	s1 := strings.Repeat("a", 10)
	s2 := strings.Repeat("a", 10)
	FlushString(&s1)
	if s1 == s2 {
		t.Fatal("failed to flush string")
	}
}
