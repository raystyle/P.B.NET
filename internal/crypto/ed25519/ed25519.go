package ed25519

import (
	"errors"
	"io"

	"golang.org/x/crypto/ed25519"

	"project/internal/crypto/rand"
)

// size
const (
	PublicKeySize  = 32
	PrivateKeySize = 64
	SignatureSize  = 64
	SeedSize       = 32
)

// errors
var (
	ErrInvalidPrivateKey = errors.New("invalid private key size")
	ErrInvalidPublicKey  = errors.New("invalid public key size")
)

// PublicKey is the ed25519 public key
type PublicKey []byte

// PrivateKey is the ed25519 private key
type PrivateKey []byte

// PublicKey is used to get the public key of the private key
func (p PrivateKey) PublicKey() PublicKey {
	publicKey := make([]byte, PublicKeySize)
	copy(publicKey, p[32:])
	return publicKey
}

// GenerateKey is used to generate private key
func GenerateKey() (PrivateKey, error) {
	seed := make([]byte, SeedSize)
	_, err := io.ReadFull(rand.Reader, seed)
	if err != nil {
		return nil, err
	}
	return NewKeyFromSeed(seed), nil
}

// NewKeyFromSeed calculates a private key from a seed. It will panic if
// len(seed) is not SeedSize. This function is provided for interoperability
// with RFC 8032. RFC 8032's private keys correspond to seeds in this package
func NewKeyFromSeed(seed []byte) PrivateKey {
	privateKey := ed25519.NewKeyFromSeed(seed)
	p := make([]byte, PrivateKeySize)
	copy(p, privateKey)
	return p
}

// ImportPrivateKey is used to import private key from bytes
func ImportPrivateKey(key []byte) (PrivateKey, error) {
	if len(key) != PrivateKeySize {
		return nil, ErrInvalidPrivateKey
	}
	pri := make([]byte, PrivateKeySize)
	copy(pri, key)
	return pri, nil
}

// ImportPublicKey is used to import public key from bytes
func ImportPublicKey(key []byte) (PublicKey, error) {
	if len(key) != PublicKeySize {
		return nil, ErrInvalidPublicKey
	}
	pub := make([]byte, PublicKeySize)
	copy(pub, key)
	return pub, nil
}

// Sign signs the message with privateKey and returns a signature
// It will panic if len(privateKey) is not PrivateKeySize
func Sign(p PrivateKey, message []byte) []byte {
	return ed25519.Sign(ed25519.PrivateKey(p), message)
}

// Verify reports whether sig is a valid signature of message by publicKey
// It will panic if len(publicKey) is not PublicKeySize.
func Verify(p PublicKey, message, signature []byte) bool {
	return ed25519.Verify(ed25519.PublicKey(p), message, signature)
}
