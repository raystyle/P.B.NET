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
	ERR_INVALID_PEM_BLOCK = errors.New("invalid PEM block")
)

type PublicKey = rsa.PublicKey
type PrivateKey = rsa.PrivateKey

func Generate_Key(bits int) (*PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, bits)
}

func Import_PrivateKey_PEM(pemdata []byte) (*PrivateKey, error) {
	block, _ := pem.Decode(pemdata)
	if block == nil {
		return nil, ERR_INVALID_PEM_BLOCK
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

func Import_PrivateKey(privatekey []byte) (*PrivateKey, error) {
	return x509.ParsePKCS1PrivateKey(privatekey)
}

func Export_PrivateKey(p *rsa.PrivateKey) []byte {
	return x509.MarshalPKCS1PrivateKey(p)
}

func Import_PublicKey(publickey []byte) (*PublicKey, error) {
	return x509.ParsePKCS1PublicKey(publickey)
}

func Export_PublicKey(p *PublicKey) []byte {
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

func Encrypt(p *PublicKey, plaindata []byte) ([]byte, error) {
	return rsa.EncryptPKCS1v15(rand.Reader, p, plaindata)
}

func Decrypt(p *PrivateKey, cipherdata []byte) ([]byte, error) {
	return rsa.DecryptPKCS1v15(rand.Reader, p, cipherdata)
}
