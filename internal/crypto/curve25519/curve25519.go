package curve25519

import (
	"crypto/subtle"
	"errors"

	"golang.org/x/crypto/curve25519"
)

// basePoint is the x coordinate of the generator of the curve.
var basePoint = []byte{9, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

var zeros = make([]byte, 32)

func Scalar_Mult(in, base []byte) ([]byte, error) {
	if len(in) != 32 || len(base) != 32 {
		return nil, errors.New("invalid in or base size")
	}
	var (
		dst_array  [32]byte
		in_array   [32]byte
		base_array [32]byte
	)
	for i := 0; i < 32; i++ {
		in_array[i] = in[i]
		base_array[i] = base[i]
	}
	curve25519.ScalarMult(&dst_array, &in_array, &base_array)
	dst := dst_array[:]
	if subtle.ConstantTimeCompare(dst, zeros) == 1 {
		return nil, errors.New("curve25519 public value has wrong order")
	}
	return dst, nil
}

func Scalar_Base_Mult(in []byte) ([]byte, error) {
	return Scalar_Mult(in, basePoint)
}
