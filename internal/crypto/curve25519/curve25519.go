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

func ScalarMult(in, base []byte) ([]byte, error) {
	if len(in) != 32 || len(base) != 32 {
		return nil, errors.New("invalid in or base size")
	}
	var (
		dstArray  [32]byte
		inArray   [32]byte
		baseArray [32]byte
	)
	for i := 0; i < 32; i++ {
		inArray[i] = in[i]
		baseArray[i] = base[i]
	}
	curve25519.ScalarMult(&dstArray, &inArray, &baseArray)
	dst := dstArray[:]
	if subtle.ConstantTimeCompare(dst, zeros) == 1 {
		return nil, errors.New("curve25519 public value has wrong order")
	}
	return dst, nil
}

func ScalarBaseMult(in []byte) ([]byte, error) {
	return ScalarMult(in, basePoint)
}
