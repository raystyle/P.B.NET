package ed25519

import (
	"errors"
	"io"

	"golang.org/x/crypto/ed25519"

	"project/internal/crypto/rand"
)

const (
	PublicKey_Size  = 32
	PrivateKey_Size = 64
	Signature_Size  = 64
	Seed_Size       = 32
)

var (
	ERR_INVALID_PRIVATEKEY = errors.New("invalid private key size")
	ERR_INVALID_PUBLICKEY  = errors.New("invalid public key size")
)

type PrivateKey []byte

func (this PrivateKey) PublicKey() PublicKey {
	publicKey := make([]byte, PublicKey_Size)
	copy(publicKey, this[32:])
	return PublicKey(publicKey)
}

type PublicKey []byte

func Generate_Key() (PrivateKey, error) {
	seed := make([]byte, Seed_Size)
	_, err := io.ReadFull(rand.Reader, seed)
	if err != nil {
		return nil, err
	}
	return New_Key_From_Seed(seed), nil
}

func New_Key_From_Seed(seed []byte) PrivateKey {
	privatekey := ed25519.NewKeyFromSeed(seed)
	p := make([]byte, PrivateKey_Size)
	copy(p, privatekey)
	return p
}

func Import_PrivateKey(key []byte) (PrivateKey, error) {
	if len(key) != PrivateKey_Size {
		return nil, ERR_INVALID_PRIVATEKEY
	}
	pri := make([]byte, PrivateKey_Size)
	copy(pri, key)
	return PrivateKey(pri), nil
}

func Import_PublicKey(key []byte) (PublicKey, error) {
	if len(key) != PublicKey_Size {
		return nil, ERR_INVALID_PUBLICKEY
	}
	pub := make([]byte, PublicKey_Size)
	copy(pub, key)
	return PublicKey(pub), nil
}

func Sign(p PrivateKey, message []byte) []byte {
	return ed25519.Sign(ed25519.PrivateKey(p), message)
}

func Verify(p PublicKey, message, signature []byte) bool {
	return ed25519.Verify(ed25519.PublicKey(p), message, signature)
}
