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
	ERR_INVALID_PEM_BLOCK  = errors.New("invalid PEM block")
	ERR_INVALID_PUBLIC_KEY = errors.New("invalid public key")
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

func Import_PublicKey(c elliptic.Curve, publickey []byte) (*PublicKey, error) {
	x, y := elliptic.Unmarshal(c, publickey)
	if x == nil || y == nil {
		return nil, ERR_INVALID_PUBLIC_KEY
	}
	return &ecdsa.PublicKey{Curve: c, X: x, Y: y}, nil
}

func Export_PublicKey(c elliptic.Curve, p *PublicKey) []byte {
	return elliptic.Marshal(c, p.X, p.Y)
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
