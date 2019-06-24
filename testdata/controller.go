package testdata

import (
	"bytes"

	"project/internal/crypto/ed25519"
)

var CTRL_ED25519 ed25519.PrivateKey

func init() {
	seed := bytes.Repeat([]byte{0}, ed25519.Seed_Size)
	CTRL_ED25519 = ed25519.New_Key_From_Seed(seed)
}
