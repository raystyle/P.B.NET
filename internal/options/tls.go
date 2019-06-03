package options

import (
	"crypto/tls"
	"crypto/x509"
)

type TLS_Config struct {
	Certificates       []tls.Certificate
	RootCAs            *x509.CertPool
	ClientCAs          *x509.CertPool
	InsecureSkipVerify bool
}

func (this *TLS_Config) Apply() (*tls.Config, error) {
	c := &tls.Config{
		Certificates:       this.Certificates,
		RootCAs:            this.RootCAs,
		ClientCAs:          this.ClientCAs,
		InsecureSkipVerify: this.InsecureSkipVerify,
	}
	return c, nil
}
