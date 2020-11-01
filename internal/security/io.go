package security

import (
	"errors"
	"io"
	"io/ioutil"
)

// ErrHasRemainingData is an error that reader is not read finish.
var ErrHasRemainingData = errors.New("has remaining data in reader")

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
		return 0, ErrHasRemainingData
	}
	if int64(len(p)) > l.n {
		p = p[0:l.n]
	}
	n, err = l.r.Read(p)
	l.n -= int64(n)
	return
}

// LimitReader is used to return a limit reader.
func LimitReader(r io.Reader, size int64) io.Reader {
	return &limitedReader{r: r, n: size}
}

// ReadAll is used to read all with limited size.
// if read out of size, it will return an ErrHasRemainingData.
func ReadAll(r io.Reader, size int64) ([]byte, error) {
	return ioutil.ReadAll(LimitReader(r, size))
}

// LimitReadAll is used to read all with limited size.
// if read out of size, it will not return an error.
func LimitReadAll(r io.Reader, size int64) ([]byte, error) {
	lr := io.LimitReader(r, size)
	return ioutil.ReadAll(lr)
}
