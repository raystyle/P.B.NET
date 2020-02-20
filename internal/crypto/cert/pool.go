package cert

import (
	"crypto/x509"
	"errors"
	"sync"

	"project/internal/security"
)

type pair struct {
	Certificate *security.Bytes // ANS1 data
	PrivateKey  *security.Bytes // PKCS8
}

// Pool include all certificates from public and private.
type Pool struct {
	// public means these certificates are from the common organization,
	// like Let's Encrypt, GlobalSign ...
	PublicRootCACerts   []*x509.Certificate
	PublicClientCACerts []*x509.Certificate
	PublicClientCerts   []*pair

	// private means these certificates are from the Controller or self.
	PrivateRootCACerts   []*pair // only Controller contain the Private Key
	PrivateClientCACerts []*pair // only Controller contain the Private Key
	PrivateClientCerts   []*pair

	m sync.RWMutex
}

// NewPool is used to create a new pool.
func NewPool() *Pool {
	security.PaddingMemory()
	defer security.FlushMemory()
	memory := security.NewMemory()
	defer memory.Flush()
	return new(Pool)
}

func certIsExist(certs []*x509.Certificate, cert *x509.Certificate) bool {
	for i := 0; i < len(certs); i++ {
		if certs[i].Equal(cert) {
			return true
		}
	}
	return false
}

func pairIsExist(pairs []*pair, p *x509.Certificate) bool {
	for i := 0; i < len(pairs); i++ {

	}
	return false
}

// AddPublicRootCACert is used to add public root CA certificate.
func (p *Pool) AddPublicRootCACert(cert *x509.Certificate) error {
	p.m.Lock()
	defer p.m.Unlock()
	if certIsExist(p.PublicRootCACerts, cert) {
		return errors.New("this public root ca certificate already exists")
	}
	p.PublicRootCACerts = append(p.PublicRootCACerts, cert)
	return nil
}
