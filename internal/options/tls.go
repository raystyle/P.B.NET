package options

import (
	"crypto/tls"
	"crypto/x509"

	"project/internal/crypto/cert"
)

type TLS_Config struct {
	Certificates       []TLS_KeyPair // tls.keypair(pem)
	RootCAs            []string      // x509.Cert(pem)
	ClientCAs          []string      // x509.Cert(pem)
	InsecureSkipVerify bool
}

type TLS_KeyPair struct {
	Cert_PEM string
	Key_PEM  string
}

func (this *TLS_Config) Apply() (*tls.Config, error) {
	config := &tls.Config{
		InsecureSkipVerify: this.InsecureSkipVerify,
	}
	l := len(this.Certificates)
	if l != 0 {
		config.Certificates = make([]tls.Certificate, l)
		for i := 0; i < l; i++ {
			c := []byte(this.Certificates[i].Cert_PEM)
			k := []byte(this.Certificates[i].Key_PEM)
			tls_cert, err := tls.X509KeyPair(c, k)
			if err != nil {
				return nil, err
			}
			config.Certificates[i] = tls_cert
		}
	}
	l = len(this.RootCAs)
	if l != 0 {
		config.RootCAs = x509.NewCertPool()
		for i := 0; i < l; i++ {
			c, err := cert.Parse([]byte(this.RootCAs[i]))
			if err != nil {
				return nil, err
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
				return nil, err
			}
			config.ClientCAs.AddCert(c)
		}
	}
	return config, nil
}
