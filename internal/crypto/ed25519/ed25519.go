package ed25519

import (
	"crypto/ed25519"
	"errors"
	"io"

	"project/internal/crypto/rand"
)

const (
	PublicKeySize  = 32
	PrivateKeySize = 64
	SignatureSize  = 64
	SeedSize       = 32
)

var (
	ErrInvalidPrivateKey = errors.New("invalid private key size")
	ErrInvalidPublicKey  = errors.New("invalid public key size")
)

type PrivateKey []byte

func (p PrivateKey) PublicKey() PublicKey {
	publicKey := make([]byte, PublicKeySize)
	copy(publicKey, p[32:])
	return publicKey
}

type PublicKey []byte

func GenerateKey() (PrivateKey, error) {
	seed := make([]byte, SeedSize)
	_, err := io.ReadFull(rand.Reader, seed)
	if err != nil {
		return nil, err
	}
	return NewKeyFromSeed(seed), nil
}

func NewKeyFromSeed(seed []byte) PrivateKey {
	privateKey := ed25519.NewKeyFromSeed(seed)
	p := make([]byte, PrivateKeySize)
	copy(p, privateKey)
	return p
}

func ImportPrivateKey(key []byte) (PrivateKey, error) {
	if len(key) != PrivateKeySize {
		return nil, ErrInvalidPrivateKey
	}
	pri := make([]byte, PrivateKeySize)
	copy(pri, key)
	return pri, nil
}

func ImportPublicKey(key []byte) (PublicKey, error) {
	if len(key) != PublicKeySize {
		return nil, ErrInvalidPublicKey
	}
	pub := make([]byte, PublicKeySize)
	copy(pub, key)
	return pub, nil
}

func Sign(p PrivateKey, message []byte) []byte {
	return ed25519.Sign(ed25519.PrivateKey(p), message)
}

func Verify(p PublicKey, message, signature []byte) bool {
	return ed25519.Verify(ed25519.PublicKey(p), message, signature)
}
