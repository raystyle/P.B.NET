package ecdsa

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"math/big"

	"project/internal/convert"
)

var (
	ERR_INVALID_PEM_BLOCK = errors.New("invalid PEM block")
	ERR_NOT_PUBLIC_KEY    = errors.New("not ecdsa public key")
)

type PublicKey = ecdsa.PublicKey
type PrivateKey = ecdsa.PrivateKey

func Generate_Key(c elliptic.Curve) (*PrivateKey, error) {
	return ecdsa.GenerateKey(c, rand.Reader)
}

func Import_PrivateKey_PEM(pemdata []byte) (*PrivateKey, error) {
	block, _ := pem.Decode(pemdata)
	if block == nil {
		return nil, ERR_INVALID_PEM_BLOCK
	}
	return x509.ParseECPrivateKey(block.Bytes)
}

func Import_PrivateKey(privatekey []byte) (*PrivateKey, error) {
	return x509.ParseECPrivateKey(privatekey)
}

func Export_PrivateKey(p *PrivateKey) ([]byte, error) {
	return x509.MarshalECPrivateKey(p)
}

func Import_PublicKey(publickey []byte) (*PublicKey, error) {
	pub, err := x509.ParsePKIXPublicKey(publickey)
	if err != nil {
		return nil, err
	}
	p, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, ERR_NOT_PUBLIC_KEY
	}
	return p, nil
}

func Export_PublicKey(p *PublicKey) []byte {
	data, _ := x509.MarshalPKIXPublicKey(p)
	return data
}

//len r(2 byte) + r.bytes + len s(2 byte) + s.bytes
func Sign(p *PrivateKey, data []byte) ([]byte, error) {
	h := sha256.New()
	h.Write(data)
	r, s, err := ecdsa.Sign(rand.Reader, p, h.Sum(nil))
	if err != nil {
		return nil, err
	}
	r_b := r.Bytes()
	r_b_l := len(r_b)
	s_b := s.Bytes()
	s_b_l := len(s_b)
	//2 + r_b + 2 +s_b
	signature := make([]byte, 4+r_b_l+s_b_l)
	copy(signature, convert.Uint16_Bytes(uint16(r_b_l)))
	copy(signature[2:], r_b)
	copy(signature[2+r_b_l:], convert.Uint16_Bytes(uint16(s_b_l)))
	copy(signature[4+r_b_l:], s_b)
	return signature, nil
}

func Verify(p *PublicKey, data, signature []byte) bool {
	if len(signature) < 2 {
		return false
	}
	r_l := int(convert.Bytes_Uint16(signature[:2]))
	if len(signature[2:]) < r_l {
		return false
	}
	r := &big.Int{}
	r.SetBytes(signature[2 : 2+r_l])
	if len(signature[2+r_l:]) < 2 {
		return false
	}
	s_l := int(convert.Bytes_Uint16(signature[2+r_l : 2+r_l+2]))
	if len(signature[2+r_l+2:]) != s_l {
		return false
	}
	s := &big.Int{}
	s.SetBytes(signature[2+r_l+2:])
	h := sha256.New()
	h.Write(data)
	return ecdsa.Verify(p, h.Sum(nil), r, s)
}
