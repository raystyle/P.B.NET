package certutil

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
)

// ErrInvalidPEMBlock is the error about the PEM block.
var ErrInvalidPEMBlock = errors.New("invalid PEM block")

// ParseCertificate is used to parse certificate from PEM.
func ParseCertificate(pemBlock []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(pemBlock)
	if block == nil {
		return nil, ErrInvalidPEMBlock
	}
	if block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("invalid PEM block type: %s", block.Type)
	}
	return x509.ParseCertificate(block.Bytes)
}

// ParseCertificates is used to parse certificates from PEM.
func ParseCertificates(pemBlock []byte) ([]*x509.Certificate, error) {
	var (
		certs []*x509.Certificate
		block *pem.Block
	)
	for {
		block, pemBlock = pem.Decode(pemBlock)
		if block == nil {
			return nil, ErrInvalidPEMBlock
		}
		if block.Type != "CERTIFICATE" {
			return nil, fmt.Errorf("invalid PEM block type: %s", block.Type)
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		certs = append(certs, cert)
		if len(pemBlock) == 0 {
			break
		}
	}
	return certs, nil
}

// ParsePrivateKey is used to parse private key from PEM.
// It support RSA ECDSA and ED25519.
func ParsePrivateKey(pemBlock []byte) (interface{}, error) {
	block, _ := pem.Decode(pemBlock)
	if block == nil {
		return nil, ErrInvalidPEMBlock
	}
	return ParsePrivateKeyBytes(block.Bytes)
}

// ParsePrivateKeys is used to parse private keys from PEM.
// It support RSA ECDSA and ED25519.
func ParsePrivateKeys(pemBlock []byte) ([]interface{}, error) {
	var (
		keys  []interface{}
		block *pem.Block
	)
	for {
		block, pemBlock = pem.Decode(pemBlock)
		if block == nil {
			return nil, ErrInvalidPEMBlock
		}
		key, err := ParsePrivateKeyBytes(block.Bytes)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
		if len(pemBlock) == 0 {
			break
		}
	}
	return keys, nil
}

// ParsePrivateKeyBytes is used to parse private key from bytes.
// It support RSA ECDSA and ED25519.
func ParsePrivateKeyBytes(bytes []byte) (interface{}, error) {
	if key, err := x509.ParsePKCS1PrivateKey(bytes); err == nil {
		return key, nil
	}
	if key, err := x509.ParsePKCS8PrivateKey(bytes); err == nil {
		return key, nil
	}
	if key, err := x509.ParseECPrivateKey(bytes); err == nil {
		return key, nil
	}
	return nil, errors.New("failed to parse private key")
}

// Match is used to check the private key is match the public key in the certificate.
func Match(cert *x509.Certificate, pri interface{}) bool {
	switch pub := cert.PublicKey.(type) {
	case *rsa.PublicKey:
		pri, ok := pri.(*rsa.PrivateKey)
		if !ok {
			return false
		}
		if pub.N.Cmp(pri.N) != 0 {
			return false
		}
	case *ecdsa.PublicKey:
		pri, ok := pri.(*ecdsa.PrivateKey)
		if !ok {
			return false
		}
		if pub.X.Cmp(pri.X) != 0 || pub.Y.Cmp(pri.Y) != 0 {
			return false
		}
	case ed25519.PublicKey:
		pri, ok := pri.(ed25519.PrivateKey)
		if !ok {
			return false
		}
		if !bytes.Equal(pri.Public().(ed25519.PublicKey), pub) {
			return false
		}
	default:
		return false
	}
	return true
}
