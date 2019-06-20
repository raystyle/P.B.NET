package rand

import (
	"crypto/rand"
	"io"

	"project/internal/random"
)

var Reader io.Reader

func init() {
	Reader = new(reader)
}

type reader struct{}

func (this reader) Read(b []byte) (int, error) {
	l := len(b)
	size := 4 * l
	buffer := make([]byte, size)
	_, err := io.ReadFull(rand.Reader, buffer)
	if err != nil {
		return 0, err
	}
	generator := random.New()
	for i := 0; i < l; i++ {
		b[i] = buffer[generator.Int(size)]
	}
	return l, nil
}
