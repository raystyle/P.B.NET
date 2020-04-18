package rand

import (
	cr "crypto/rand"
	"io"
	"math/rand"
	"time"
)

// Reader is a global, shared instance of a
// cryptographically secure random number generator.
var Reader io.Reader

func init() {
	Reader = new(reader)
}

type reader struct{}

func (r reader) Read(b []byte) (int, error) {
	l := len(b)
	size := 4 * l
	buffer := make([]byte, size)
	_, err := io.ReadFull(cr.Reader, buffer)
	if err != nil {
		return 0, err
	}
	rd := rand.New(rand.NewSource(time.Now().UnixNano())) // #nosec
	for i := 0; i < l; i++ {
		b[i] = buffer[rd.Intn(size)]
	}
	return l, nil
}
