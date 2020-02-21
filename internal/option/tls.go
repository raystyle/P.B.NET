package option

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"project/internal/crypto/cert"
	"project/internal/crypto/cert/certutil"
	"project/internal/security"
)

// TLSConfig contains options about tls.Config.
type TLSConfig struct {
	// add certs manually for this config
	Certificates []X509KeyPair `toml:"certificates"`
	RootCAs      []string      `toml:"root_ca"`   // PEM
	ClientCAs    []string      `toml:"client_ca"` // PEM

	ClientAuth   tls.ClientAuthType `toml:"client_auth"`
	ServerName   string             `toml:"server_name"`
	NextProtos   []string           `toml:"next_protos"`
	MinVersion   uint16             `toml:"min_version"`
	MaxVersion   uint16             `toml:"max_version"`
	CipherSuites []uint16           `toml:"cipher_suites"`

	InsecureLoadFromSystem bool `toml:"insecure_load_from_system"`

	// add certs manually from certificate pool
	CertPool         *cert.Pool `toml:"-"`
	LoadFromCertPool struct {
		// public will be load automatically
		SkipPublicRootCACerts   bool `toml:"skip_public_root_ca_certs"`
		SkipPublicClientCACerts bool `toml:"skip_public_client_ca_certs"`
		SkipPublicClientCerts   bool `toml:"skip_public_client_certs"`

		// private need be load manually
		LoadPrivateRootCACerts   bool `toml:"load_private_root_ca_certs"`
		LoadPrivateClientCACerts bool `toml:"load_private_client_ca_certs"`
		LoadPrivateClientCerts   bool `toml:"load_private_client_certs"`
	} `toml:"cert_pool"`
}

// X509KeyPair include certificate and private key.
type X509KeyPair struct {
	Cert string `toml:"cert"` // PEM
	Key  string `toml:"key"`  // PEM
}

func (t *TLSConfig) error(err error) error {
	return fmt.Errorf("failed to apply tls config: %s", err)
}

func (t *TLSConfig) parseCertificates(cert []string) ([]*x509.Certificate, error) {
	var certs []*x509.Certificate
	for _, c := range cert {
		c, err := certutil.ParseCertificates([]byte(c))
		if err != nil {
			return nil, err
		}
		certs = append(certs, c...)
	}
	return certs, nil
}

// GetRootCAs is used to parse TLSConfig.RootCAs.
func (t *TLSConfig) GetRootCAs() ([]*x509.Certificate, error) {
	rootCAs, err := t.parseCertificates(t.RootCAs)
	if err != nil {
		return nil, fmt.Errorf("failed to parse root ca: %s", err)
	}
	if t.CertPool == nil {
		return rootCAs, nil
	}
	if !t.LoadFromCertPool.SkipPublicRootCACerts {
		rootCAs = append(rootCAs, t.CertPool.GetPublicRootCACerts()...)
	}
	if t.LoadFromCertPool.LoadPrivateRootCACerts {
		rootCAs = append(rootCAs, t.CertPool.GetPrivateRootCACerts()...)
	}
	return rootCAs, nil
}

// GetClientCAs is used to parse TLSConfig.ClientCAs.
func (t *TLSConfig) GetClientCAs() ([]*x509.Certificate, error) {
	clientCAs, err := t.parseCertificates(t.ClientCAs)
	if err != nil {
		return nil, fmt.Errorf("failed to parse client ca: %s", err)
	}
	if t.CertPool == nil {
		return clientCAs, nil
	}
	if !t.LoadFromCertPool.SkipPublicClientCACerts {
		clientCAs = append(clientCAs, t.CertPool.GetPublicClientCACerts()...)
	}
	if t.LoadFromCertPool.LoadPrivateRootCACerts {
		clientCAs = append(clientCAs, t.CertPool.GetPrivateClientCACerts()...)
	}
	return clientCAs, nil
}

// Apply is used to create *tls.Config.
func (t *TLSConfig) Apply() (*tls.Config, error) {
	config := new(tls.Config)
	// set certificates
	l := len(t.Certificates)
	if l != 0 {
		config.Certificates = make([]tls.Certificate, l)
		for i := 0; i < l; i++ {
			c := []byte(t.Certificates[i].Cert)
			k := []byte(t.Certificates[i].Key)
			tlsCert, err := tls.X509KeyPair(c, k)
			if err != nil {
				return nil, t.error(err)
			}
			security.CoverBytes(c)
			security.CoverBytes(k)
			config.Certificates[i] = tlsCert
		}
	}
	// set Root CAs
	if t.InsecureLoadFromSystem {
		var err error
		config.RootCAs, err = certutil.SystemCertPool()
		if err != nil {
			return nil, t.error(err)
		}
	}
	// <security> force create new certificate pool
	// that not use system certificates
	if config.RootCAs == nil {
		config.RootCAs = x509.NewCertPool()
	}
	rootCAs, err := t.GetRootCAs()
	if err != nil {
		return nil, t.error(err)
	}
	for i := 0; i < len(rootCAs); i++ {
		config.RootCAs.AddCert(rootCAs[i])
	}
	// set Client CAs
	clientCAs, err := t.GetClientCAs()
	if err != nil {
		return nil, t.error(err)
	}
	l = len(clientCAs)
	if l > 0 {
		config.ClientCAs = x509.NewCertPool()
		for i := 0; i < l; i++ {
			config.ClientCAs.AddCert(clientCAs[i])
		}
	}
	// set next protocols
	l = len(t.NextProtos)
	if l > 0 {
		config.NextProtos = make([]string, len(t.NextProtos))
		copy(config.NextProtos, t.NextProtos)
	}
	// set cipher suites
	l = len(t.CipherSuites)
	if l > 0 {
		config.CipherSuites = make([]uint16, len(t.CipherSuites))
		copy(config.CipherSuites, t.CipherSuites)
	}
	config.ServerName = t.ServerName
	config.MinVersion = t.MinVersion
	config.MaxVersion = t.MaxVersion
	config.ClientAuth = t.ClientAuth
	return config, nil
}
