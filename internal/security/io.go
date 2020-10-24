package security

import (
	"io"
	"io/ioutil"
)

// LimitReadAll is used to read all with limited size.
func LimitReadAll(r io.Reader, size int64) ([]byte, error) {
	lr := io.LimitReader(r, size)
	return ioutil.ReadAll(lr)
}
