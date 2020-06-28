package option

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"project/internal/cert"
	"project/internal/security"
)

// TLSConfig contains options about tls.Config.
type TLSConfig struct {
	// add certificates manually for this TLSConfig
	Certificates []X509KeyPair `toml:"certificates"`
	RootCAs      []string      `toml:"root_ca"`   // PEM
	ClientCAs    []string      `toml:"client_ca"` // PEM

	ClientAuth   tls.ClientAuthType `toml:"client_auth"`
	ServerName   string             `toml:"server_name"`
	NextProtos   []string           `toml:"next_protos"`
	MinVersion   uint16             `toml:"min_version"`
	MaxVersion   uint16             `toml:"max_version"`
	CipherSuites []uint16           `toml:"cipher_suites"`

	// add certificates from certificate pool manually
	CertPool         *cert.Pool `toml:"-" msgpack:"-" check:"-"`
	LoadFromCertPool struct {
		// public will be loaded automatically
		SkipPublicRootCA   bool `toml:"skip_public_root_ca"`
		SkipPublicClientCA bool `toml:"skip_public_client_ca"`
		SkipPublicClient   bool `toml:"skip_public_client"`

		// private need be loaded manually
		LoadPrivateRootCA   bool `toml:"load_private_root_ca"`
		LoadPrivateClientCA bool `toml:"load_private_client_ca"`
		LoadPrivateClient   bool `toml:"load_private_client"`
	} `toml:"cert_pool"`

	// listener need set true
	ServerSide bool `toml:"-" msgpack:"-" check:"-"`
}

// X509KeyPair include certificate and private key.
type X509KeyPair struct {
	Cert string `toml:"cert"` // PEM
	Key  string `toml:"key"`  // PEM
}

func (t *TLSConfig) error(err error) error {
	return fmt.Errorf("failed to apply tls config: %s", err)
}

func (t *TLSConfig) parseCertificates(pem []string) ([]*x509.Certificate, error) {
	var certs []*x509.Certificate
	for _, p := range pem {
		c, err := cert.ParseCertificates([]byte(p))
		if err != nil {
			return nil, err
		}
		certs = append(certs, c...)
	}
	return certs, nil
}

// GetCertificates is used to make tls certificates.
func (t *TLSConfig) GetCertificates() ([]tls.Certificate, error) {
	var certs []tls.Certificate
	for i := 0; i < len(t.Certificates); i++ {
		c := []byte(t.Certificates[i].Cert)
		k := []byte(t.Certificates[i].Key)
		tlsCert, err := tls.X509KeyPair(c, k)
		if err != nil {
			return nil, err
		}
		security.CoverBytes(c)
		security.CoverBytes(k)
		certs = append(certs, tlsCert)
	}
	if t.CertPool == nil {
		return certs, nil
	}
	if !t.LoadFromCertPool.SkipPublicClient && !t.ServerSide {
		pairs := t.CertPool.GetPublicClientPairs()
		certs = append(certs, makeTLSCertificates(pairs)...)
	}
	if t.LoadFromCertPool.LoadPrivateClient && !t.ServerSide {
		pairs := t.CertPool.GetPrivateClientPairs()
		certs = append(certs, makeTLSCertificates(pairs)...)
	}
	return certs, nil
}

func makeTLSCertificates(pairs []*cert.Pair) []tls.Certificate {
	l := len(pairs)
	clientCerts := make([]tls.Certificate, l)
	for i := 0; i < l; i++ {
		clientCerts[i] = pairs[i].TLSCertificate()
	}
	return clientCerts
}

// GetRootCAs is used to parse TLSConfig.RootCAs.
func (t *TLSConfig) GetRootCAs() ([]*x509.Certificate, error) {
	if t.ServerSide {
		return nil, nil
	}
	rootCAs, err := t.parseCertificates(t.RootCAs)
	if err != nil {
		return nil, fmt.Errorf("failed to parse root ca: %s", err)
	}
	if t.CertPool == nil {
		return rootCAs, nil
	}
	if !t.LoadFromCertPool.SkipPublicRootCA {
		rootCAs = append(rootCAs, t.CertPool.GetPublicRootCACerts()...)
	}
	if t.LoadFromCertPool.LoadPrivateRootCA {
		rootCAs = append(rootCAs, t.CertPool.GetPrivateRootCACerts()...)
	}
	return rootCAs, nil
}

// GetClientCAs is used to parse TLSConfig.ClientCAs.
func (t *TLSConfig) GetClientCAs() ([]*x509.Certificate, error) {
	if !t.ServerSide {
		return nil, nil
	}
	clientCAs, err := t.parseCertificates(t.ClientCAs)
	if err != nil {
		return nil, fmt.Errorf("failed to parse client ca: %s", err)
	}
	if t.CertPool == nil {
		return clientCAs, nil
	}
	if !t.LoadFromCertPool.SkipPublicClientCA {
		clientCAs = append(clientCAs, t.CertPool.GetPublicClientCACerts()...)
	}
	if t.LoadFromCertPool.LoadPrivateClientCA {
		clientCAs = append(clientCAs, t.CertPool.GetPrivateClientCACerts()...)
	}
	return clientCAs, nil
}

// Apply is used to create *tls.Config.
func (t *TLSConfig) Apply() (*tls.Config, error) {
	config := new(tls.Config)
	// set certificates
	certs, err := t.GetCertificates()
	if err != nil {
		return nil, t.error(err)
	}
	config.Certificates = certs
	// set Root CAs
	rootCAs, err := t.GetRootCAs()
	if err != nil {
		return nil, t.error(err)
	}
	config.RootCAs = x509.NewCertPool()
	for i := 0; i < len(rootCAs); i++ {
		config.RootCAs.AddCert(rootCAs[i])
	}
	// set Client CAs
	clientCAs, err := t.GetClientCAs()
	if err != nil {
		return nil, t.error(err)
	}
	config.ClientCAs = x509.NewCertPool()
	for i := 0; i < len(clientCAs); i++ {
		config.ClientCAs.AddCert(clientCAs[i])
	}
	// set next protocols
	l := len(t.NextProtos)
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
	if config.MinVersion == 0 {
		config.MinVersion = tls.VersionTLS12
	}
	config.MaxVersion = t.MaxVersion
	config.ClientAuth = t.ClientAuth
	return config, nil
}
