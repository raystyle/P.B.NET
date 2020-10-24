package security

import (
	"errors"
	"io"
	"io/ioutil"
)

// LimitReadAll is used to read all with limited size.
// if out of size it will not return an error.
func LimitReadAll(r io.Reader, size int64) ([]byte, error) {
	lr := io.LimitReader(r, size)
	return ioutil.ReadAll(lr)
}

type limitedReader struct {
	r io.Reader // underlying reader
	n int64     // max bytes remaining
}

func (l *limitedReader) Read(p []byte) (n int, err error) {
	if l.n <= 0 {
		// try to read again for make sure
		// it can read new data
		n, err = l.r.Read(p)
		if err == io.EOF && n == 0 {
			return 0, io.EOF
		}
		return 0, errors.New("limit read all is not finished")
	}
	if int64(len(p)) > l.n {
		p = p[0:l.n]
	}
	n, err = l.r.Read(p)
	l.n -= int64(n)
	return
}

// LimitReadAllWithError is used to read all with limited size.
// if out of size it will not return an error.
func LimitReadAllWithError(r io.Reader, size int64) ([]byte, error) {
	lr := limitedReader{r: r, n: size}
	return ioutil.ReadAll(&lr)
}
