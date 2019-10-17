package testdata

import (
	"project/internal/crypto/aes"
	"project/internal/crypto/curve25519"
	"project/internal/crypto/ed25519"
	"project/internal/random"
)

var (
	CtrlED25519      ed25519.PrivateKey
	CtrlCurve25519   []byte
	CtrlBroadcastKey []byte // controller broadcast use it
)

func init() {
	// ed25519
	pri, err := ed25519.GenerateKey()
	if err != nil {
		panic(err)
	}
	CtrlED25519 = pri
	// curve25519
	pub, err := curve25519.ScalarBaseMult(pri[:32])
	if err != nil {
		panic(err)
	}
	CtrlCurve25519 = pub
	// broadcast
	CtrlBroadcastKey = random.Bytes(aes.Bit256 + aes.IVSize)
}
