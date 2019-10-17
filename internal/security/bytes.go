package security

import (
	"project/internal/random"
)

func FlushBytes(b []byte) {
	mem := NewMemory()
	mem.Padding()
	rand := random.New(0)
	randBytes := rand.Bytes(len(b))
	copy(b, randBytes)
	mem.Flush()
}
