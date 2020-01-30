package curve25519

import (
	"golang.org/x/crypto/curve25519"
)

// ScalarSize is the size of the scalar input to X25519.
const ScalarSize = 32

// ScalarMult is a link to curve25519.X25519
func ScalarMult(in, base []byte) ([]byte, error) {
	return curve25519.X25519(in, base)
}

// ScalarBaseMult use curve25519.Basepoint as base
func ScalarBaseMult(in []byte) ([]byte, error) {
	return ScalarMult(in, curve25519.Basepoint)
}
