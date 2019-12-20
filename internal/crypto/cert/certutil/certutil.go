package certutil

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
)

// error
var (
	ErrInvalidPEMBlock = errors.New("invalid PEM block")
)

// ParseCertificate is used to parse certificate from PEM
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

// ParsePrivateKey is used to parse private key from PEM
// it support RSA ECDSA and ED25519
func ParsePrivateKey(pemBlock []byte) (interface{}, error) {
	block, _ := pem.Decode(pemBlock)
	if block == nil {
		return nil, ErrInvalidPEMBlock
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	return nil, errors.New("failed to parse private key")
}

// ParseCertificates is used to parse certificates from PEM
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
