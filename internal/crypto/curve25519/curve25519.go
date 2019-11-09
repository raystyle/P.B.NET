package curve25519

import (
	"golang.org/x/crypto/curve25519"
)

// basePoint is the x coordinate of the generator of the curve.
var basePoint = []byte{9, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

func ScalarMult(in, base []byte) ([]byte, error) {
	return curve25519.X25519(in, base)
}

func ScalarBaseMult(in []byte) ([]byte, error) {
	return ScalarMult(in, basePoint)
}
