package rand

import (
	"crypto/rand"
	"io"
	"time"

	"project/internal/random"
)

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
	g := random.New(time.Now().Unix())
	for i := 0; i < l; i++ {
		b[i] = buffer[g.Int(size)]
	}
	return l, nil
}
