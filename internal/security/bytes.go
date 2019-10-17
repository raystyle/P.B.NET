package security

import (
	"time"

	"project/internal/random"
)

func FlushBytes(b []byte) {
	mem := NewMemory()
	mem.Padding()
	rand := random.New(time.Now().UnixNano())
	randBytes := rand.Bytes(len(b))
	copy(b, randBytes)
	mem.Flush()
}
