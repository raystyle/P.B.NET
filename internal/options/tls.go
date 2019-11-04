package options

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"

	"project/internal/crypto/cert/certutil"
	"project/internal/security"
)

var (
	ErrInvalidPEMBlock     = errors.New("invalid PEM block")
	ErrInvalidPEMBlockType = errors.New("invalid PEM block type")
)

type TLSConfig struct {
	ServerName   string        `toml:"server_name"`
	Certificates []X509KeyPair `toml:"certificates"`
	RootCAs      []string      `toml:"root_ca"`   // PEM
	ClientCAs    []string      `toml:"client_ca"` // PEM
	NextProtos   []string      `toml:"next_protos"`
	MinVersion   uint16        `toml:"min_version"`
	MaxVersion   uint16        `toml:"max_version"`

	InsecureLoadFromSystem bool `toml:"insecure_load_from_system"`
}

type X509KeyPair struct {
	Cert string `toml:"cert"` // PEM
	Key  string `toml:"key"`  // PEM
}

func (t *TLSConfig) failed(err error) error {
	return fmt.Errorf("failed to apply tls config: %s", err)
}

func (t *TLSConfig) RootCA() ([]*x509.Certificate, error) {
	var certs []*x509.Certificate
	for i := 0; i < len(t.RootCAs); i++ {
		cert, err := parseCertificates([]byte(t.RootCAs[i]))
		if err != nil {
			return nil, t.failed(err)
		}
		certs = append(certs, cert...)
	}
	return certs, nil
}

func (t *TLSConfig) Apply() (*tls.Config, error) {
	// set next protocols
	nextProtos := make([]string, len(t.NextProtos))
	copy(nextProtos, t.NextProtos)
	config := &tls.Config{
		ServerName: t.ServerName,
		NextProtos: nextProtos,
		MinVersion: t.MinVersion,
		MaxVersion: t.MaxVersion,
	}

	// set certificates
	l := len(t.Certificates)
	if l != 0 {
		config.Certificates = make([]tls.Certificate, l)
		for i := 0; i < l; i++ {
			c := []byte(t.Certificates[i].Cert)
			k := []byte(t.Certificates[i].Key)
			tlsCert, err := tls.X509KeyPair(c, k)
			if err != nil {
				return nil, t.failed(err)
			}
			security.FlushBytes(c)
			security.FlushBytes(k)
			config.Certificates[i] = tlsCert
		}
	}

	var err error
	// <security> warning: load certificates pool from system
	if t.InsecureLoadFromSystem {
		config.RootCAs, err = certutil.SystemCertPool()
		if err != nil {
			return nil, err
		}
	}

	// <security> force new certificate pool
	// not use system certificates
	if config.RootCAs == nil {
		config.RootCAs = x509.NewCertPool()
	}

	// set Root CA
	rootCAs, err := t.RootCA()
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(rootCAs); i++ {
		config.RootCAs.AddCert(rootCAs[i])
	}

	// set Client CA
	config.ClientCAs = x509.NewCertPool()
	for i := 0; i < len(t.ClientCAs); i++ {
		cert, err := parseCertificates([]byte(t.ClientCAs[i]))
		if err != nil {
			return nil, t.failed(err)
		}
		for i := 0; i < len(cert); i++ {
			config.ClientCAs.AddCert(cert[i])
		}
	}

	// version
	if config.MinVersion == 0 {
		config.MinVersion = tls.VersionTLS12
	}
	return config, nil
}

func parseCertificates(certPEMBlock []byte) ([]*x509.Certificate, error) {
	var (
		certs []*x509.Certificate
		block *pem.Block
	)
	for {
		block, certPEMBlock = pem.Decode(certPEMBlock)
		if block == nil {
			return nil, ErrInvalidPEMBlock
		}
		if block.Type != "CERTIFICATE" {
			return nil, ErrInvalidPEMBlockType
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		certs = append(certs, cert)
		if len(certPEMBlock) == 0 {
			break
		}
	}
	return certs, nil
}
