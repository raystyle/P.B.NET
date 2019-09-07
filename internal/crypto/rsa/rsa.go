package rsa

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"

	"project/internal/crypto/rand"
)

var (
	ErrInvalidPEMBlock = errors.New("invalid PEM block")
)

type PublicKey = rsa.PublicKey
type PrivateKey = rsa.PrivateKey

func GenerateKey(bits int) (*PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, bits)
}

func ImportPrivateKeyPEM(data []byte) (*PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, ErrInvalidPEMBlock
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

func ImportPrivateKey(privateKey []byte) (*PrivateKey, error) {
	return x509.ParsePKCS1PrivateKey(privateKey)
}

func ExportPrivateKey(p *rsa.PrivateKey) []byte {
	return x509.MarshalPKCS1PrivateKey(p)
}

func ImportPublicKey(publicKey []byte) (*PublicKey, error) {
	return x509.ParsePKCS1PublicKey(publicKey)
}

func ExportPublicKey(p *PublicKey) []byte {
	return x509.MarshalPKCS1PublicKey(p)
}

func Sign(p *PrivateKey, data []byte) ([]byte, error) {
	h := sha256.New()
	h.Write(data)
	return rsa.SignPKCS1v15(rand.Reader, p, crypto.SHA256, h.Sum(nil))
}

func Verify(p *PublicKey, data, signature []byte) bool {
	h := sha256.New()
	h.Write(data)
	if rsa.VerifyPKCS1v15(p, crypto.SHA256, h.Sum(nil), signature) == nil {
		return true
	}
	return false
}

func Encrypt(p *PublicKey, data []byte) ([]byte, error) {
	return rsa.EncryptPKCS1v15(rand.Reader, p, data)
}

func Decrypt(p *PrivateKey, data []byte) ([]byte, error) {
	return rsa.DecryptPKCS1v15(rand.Reader, p, data)
}
