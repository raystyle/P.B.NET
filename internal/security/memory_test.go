package security

import (
	"testing"
)

func Test_Memory(t *testing.T) {
	Padding_Memory()
	Flush_Memory()
	Padding_Memory()
	Flush_Memory()
}
