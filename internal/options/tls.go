package options

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"project/internal/crypto/cert"
	"project/internal/security"
)

type TLS_Config struct {
	Certificates       []TLS_KeyPair `toml:"certificates"` // tls.keypair(pem)
	RootCAs            []string      `toml:"root_ca"`      // x509.Cert(pem)
	ClientCAs          []string      `toml:"client_ca"`    // x509.Cert(pem)
	InsecureSkipVerify bool          `toml:"insecure_skip_verify"`
}

type TLS_KeyPair struct {
	Cert string `toml:"cert"`
	Key  string `toml:"key"`
}

func (this *TLS_Config) failed(err error) error {
	return fmt.Errorf("tls config apply failed: %s", err)
}

func (this *TLS_Config) Apply() (*tls.Config, error) {
	config := &tls.Config{
		InsecureSkipVerify: this.InsecureSkipVerify,
	}
	l := len(this.Certificates)
	if l != 0 {
		config.Certificates = make([]tls.Certificate, l)
		for i := 0; i < l; i++ {
			c := []byte(this.Certificates[i].Cert)
			k := []byte(this.Certificates[i].Key)
			tls_cert, err := tls.X509KeyPair(c, k)
			if err != nil {
				return nil, this.failed(err)
			}
			security.Flush_Bytes(c)
			security.Flush_Bytes(k)
			config.Certificates[i] = tls_cert
		}
	}
	l = len(this.RootCAs)
	if l != 0 {
		config.RootCAs = x509.NewCertPool()
		for i := 0; i < l; i++ {
			c, err := cert.Parse([]byte(this.RootCAs[i]))
			if err != nil {
				return nil, this.failed(err)
			}
			config.RootCAs.AddCert(c)
		}
	}
	l = len(this.ClientCAs)
	if l != 0 {
		config.ClientCAs = x509.NewCertPool()
		for i := 0; i < l; i++ {
			c, err := cert.Parse([]byte(this.ClientCAs[i]))
			if err != nil {
				return nil, this.failed(err)
			}
			config.ClientCAs.AddCert(c)
		}
	}
	return config, nil
}
