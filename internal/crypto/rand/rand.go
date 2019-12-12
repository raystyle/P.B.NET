package rand

import (
	"crypto/rand"
	"io"

	"project/internal/random"
)

// Reader is a global, shared instance of a cryptographically
// secure random number generator
var Reader io.Reader

func init() {
	Reader = new(reader)
}

type reader struct{}

func (r reader) Read(b []byte) (int, error) {
	l := len(b)
	size := 4 * l
	buffer := make([]byte, size)
	_, err := io.ReadFull(rand.Reader, buffer)
	if err != nil {
		return 0, err
	}
	g := random.New()
	for i := 0; i < l; i++ {
		b[i] = buffer[g.Int(size)]
	}
	return l, nil
}
