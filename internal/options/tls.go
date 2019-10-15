package options

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"project/internal/crypto/cert"
	"project/internal/security"
)

type TLSConfig struct {
	Certificates       []X509KeyPair `toml:"certificates"` // tls.X509KeyPair
	RootCAs            []string      `toml:"root_ca"`      // pem
	ClientCAs          []string      `toml:"client_ca"`    // pem
	NextProtos         []string      `toml:"next_protos"`
	InsecureSkipVerify bool          `toml:"insecure_skip_verify"`
}

type X509KeyPair struct {
	Cert string `toml:"cert"`
	Key  string `toml:"key"`
}

func (t *TLSConfig) failed(err error) error {
	return fmt.Errorf("tls config apply failed: %s", err)
}

func (t *TLSConfig) Apply() (*tls.Config, error) {
	config := &tls.Config{
		NextProtos:         t.NextProtos,
		InsecureSkipVerify: t.InsecureSkipVerify,
	}
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
	l = len(t.RootCAs)
	if l != 0 {
		config.RootCAs = x509.NewCertPool()
		for i := 0; i < l; i++ {
			c, err := cert.Parse([]byte(t.RootCAs[i]))
			if err != nil {
				return nil, t.failed(err)
			}
			config.RootCAs.AddCert(c)
		}
	}
	l = len(t.ClientCAs)
	if l != 0 {
		config.ClientCAs = x509.NewCertPool()
		for i := 0; i < l; i++ {
			c, err := cert.Parse([]byte(t.ClientCAs[i]))
			if err != nil {
				return nil, t.failed(err)
			}
			config.ClientCAs.AddCert(c)
		}
	}
	return config, nil
}
