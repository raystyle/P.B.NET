package cert

import (
	"crypto/x509"
	"errors"
	"sync"

	"project/internal/crypto/cert/certutil"
	"project/internal/security"
)

// ErrMismatchedKey is the error about the key
var ErrMismatchedKey = errors.New("private key does not match public key in certificate")

type pair struct {
	Certificate *x509.Certificate
	PrivateKey  *security.Bytes // PKCS8
}

// ToPair is used to convert *pair to *Pair.
func (p *pair) ToPair() *Pair {
	pkcs8 := p.PrivateKey.Get()
	defer p.PrivateKey.Put(pkcs8)
	pri, err := x509.ParsePKCS8PrivateKey(pkcs8)
	if err != nil {
		panic(err)
	}
	return &Pair{
		Certificate: p.Certificate,
		PrivateKey:  pri,
	}
}

// Pool include all certificates from public and private place.
type Pool struct {
	// public means these certificates are from the common organization,
	// like Let's Encrypt, GlobalSign ...
	publicRootCACerts   []*x509.Certificate
	publicClientCACerts []*x509.Certificate
	publicClientCerts   []*pair

	// private means these certificates are from the Controller or self.
	privateRootCACerts   []*pair // only Controller contain the Private Key
	privateClientCACerts []*pair // only Controller contain the Private Key
	privateClientCerts   []*pair

	m sync.RWMutex
}

// NewPool is used to create a new certificate pool.
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

func pairIsExist(pairs []*pair, cert *x509.Certificate) bool {
	for i := 0; i < len(pairs); i++ {
		if pairs[i].Certificate.Equal(cert) {
			return true
		}
	}
	return false
}

// AddPublicRootCACert is used to add public root CA certificate.
func (p *Pool) AddPublicRootCACert(cert *x509.Certificate) error {
	p.m.Lock()
	defer p.m.Unlock()
	if certIsExist(p.publicRootCACerts, cert) {
		return errors.New("this public root ca certificate already exists")
	}
	// must copy
	raw := make([]byte, len(cert.Raw))
	copy(raw, cert.Raw)
	certCopy, err := x509.ParseCertificate(raw)
	if err != nil {
		return err
	}
	p.publicRootCACerts = append(p.publicRootCACerts, certCopy)
	return nil
}

// AddPublicClientCACert is used to add public client CA certificate.
func (p *Pool) AddPublicClientCACert(cert *x509.Certificate) error {
	p.m.Lock()
	defer p.m.Unlock()
	if certIsExist(p.publicClientCACerts, cert) {
		return errors.New("this public client ca certificate already exists")
	}
	// must copy
	raw := make([]byte, len(cert.Raw))
	copy(raw, cert.Raw)
	certCopy, err := x509.ParseCertificate(raw)
	if err != nil {
		return err
	}
	p.publicClientCACerts = append(p.publicClientCACerts, certCopy)
	return nil
}

// AddPublicClientCert is used to add public client certificate.
func (p *Pool) AddPublicClientCert(cert *x509.Certificate, pri interface{}) error {
	if !certutil.Match(cert, pri) {
		return ErrMismatchedKey
	}
	p.m.Lock()
	defer p.m.Unlock()
	if pairIsExist(p.publicClientCerts, cert) {
		return errors.New("this public client certificate already exists")
	}
	// must copy
	raw := make([]byte, len(cert.Raw))
	copy(raw, cert.Raw)
	certCopy, err := x509.ParseCertificate(raw)
	if err != nil {
		return err
	}
	priBytes, err := x509.MarshalPKCS8PrivateKey(pri)
	if err != nil {
		return err
	}
	pair := pair{
		Certificate: certCopy,
		PrivateKey:  security.NewBytes(priBytes),
	}
	p.publicClientCerts = append(p.publicClientCerts, &pair)
	return nil
}

// AddPrivateRootCACert is used to add private root CA certificate.
func (p *Pool) AddPrivateRootCACert(cert *x509.Certificate, pri interface{}) error {
	if pri != nil {
		if !certutil.Match(cert, pri) {
			return ErrMismatchedKey
		}
	}
	p.m.Lock()
	defer p.m.Unlock()
	if pairIsExist(p.privateRootCACerts, cert) {
		return errors.New("this private root ca certificate already exists")
	}
	// must copy
	raw := make([]byte, len(cert.Raw))
	copy(raw, cert.Raw)
	certCopy, err := x509.ParseCertificate(raw)
	if err != nil {
		return err
	}
	pair := pair{Certificate: certCopy}
	if pri != nil {
		priBytes, err := x509.MarshalPKCS8PrivateKey(pri)
		if err != nil {
			return err
		}
		pair.PrivateKey = security.NewBytes(priBytes)
	}
	p.privateRootCACerts = append(p.privateRootCACerts, &pair)
	return nil
}

// AddPrivateClientCACert is used to add private client CA certificate.
func (p *Pool) AddPrivateClientCACert(cert *x509.Certificate, pri interface{}) error {
	if pri != nil {
		if !certutil.Match(cert, pri) {
			return ErrMismatchedKey
		}
	}
	p.m.Lock()
	defer p.m.Unlock()
	if pairIsExist(p.privateClientCACerts, cert) {
		return errors.New("this private client ca certificate already exists")
	}
	// must copy
	raw := make([]byte, len(cert.Raw))
	copy(raw, cert.Raw)
	certCopy, err := x509.ParseCertificate(raw)
	if err != nil {
		return err
	}
	pair := pair{Certificate: certCopy}
	if pri != nil {
		priBytes, err := x509.MarshalPKCS8PrivateKey(pri)
		if err != nil {
			return err
		}
		pair.PrivateKey = security.NewBytes(priBytes)
	}
	p.privateClientCACerts = append(p.privateClientCACerts, &pair)
	return nil
}

// AddPrivateClientCert is used to add private client certificate.
func (p *Pool) AddPrivateClientCert(cert *x509.Certificate, pri interface{}) error {
	if !certutil.Match(cert, pri) {
		return ErrMismatchedKey
	}
	p.m.Lock()
	defer p.m.Unlock()
	if pairIsExist(p.privateClientCerts, cert) {
		return errors.New("this private client certificate already exists")
	}
	// must copy
	raw := make([]byte, len(cert.Raw))
	copy(raw, cert.Raw)
	certCopy, err := x509.ParseCertificate(raw)
	if err != nil {
		return err
	}
	priBytes, err := x509.MarshalPKCS8PrivateKey(pri)
	if err != nil {
		return err
	}
	pair := pair{
		Certificate: certCopy,
		PrivateKey:  security.NewBytes(priBytes),
	}
	p.privateClientCerts = append(p.privateClientCerts, &pair)
	return nil
}

// GetPublicRootCACerts is used to get all public root CA certificates.
func (p *Pool) GetPublicRootCACerts() []*x509.Certificate {
	p.m.RLock()
	defer p.m.RUnlock()
	certs := make([]*x509.Certificate, len(p.publicRootCACerts))
	copy(certs, p.publicRootCACerts)
	return certs
}

// GetPublicClientCACerts is used to get all public client CA certificates.
func (p *Pool) GetPublicClientCACerts() []*x509.Certificate {
	p.m.RLock()
	defer p.m.RUnlock()
	certs := make([]*x509.Certificate, len(p.publicClientCACerts))
	copy(certs, p.publicClientCACerts)
	return certs
}

// GetPublicClientPairs is used to get all public client certificates.
func (p *Pool) GetPublicClientPairs() []*Pair {
	p.m.RLock()
	defer p.m.RUnlock()
	l := len(p.publicClientCerts)
	pairs := make([]*Pair, l)
	for i := 0; i < l; i++ {
		pairs[i] = p.publicClientCerts[i].ToPair()
	}
	return pairs
}

// GetPrivateRootCACerts is used to get all private root CA certificates.
func (p *Pool) GetPrivateRootCACerts() []*x509.Certificate {
	p.m.RLock()
	defer p.m.RUnlock()
	l := len(p.privateRootCACerts)
	certs := make([]*x509.Certificate, l)
	for i := 0; i < l; i++ {
		certs[i] = p.privateRootCACerts[i].Certificate
	}
	return certs
}

// GetPrivateClientCACerts is used to get all private client CA certificates.
func (p *Pool) GetPrivateClientCACerts() []*x509.Certificate {
	p.m.RLock()
	defer p.m.RUnlock()
	l := len(p.privateClientCACerts)
	certs := make([]*x509.Certificate, l)
	for i := 0; i < l; i++ {
		certs[i] = p.privateClientCACerts[i].Certificate
	}
	return certs
}

// GetPrivateRootCAPairs is used to get all private root CA certificates.
func (p *Pool) GetPrivateRootCAPairs() []*Pair {
	p.m.RLock()
	defer p.m.RUnlock()
	l := len(p.privateRootCACerts)
	pairs := make([]*Pair, l)
	for i := 0; i < l; i++ {
		pairs[i] = p.privateRootCACerts[i].ToPair()
	}
	return pairs
}

// GetPrivateClientCAPairs is used to get all private client CA certificates.
func (p *Pool) GetPrivateClientCAPairs() []*Pair {
	p.m.RLock()
	defer p.m.RUnlock()
	l := len(p.privateClientCACerts)
	pairs := make([]*Pair, l)
	for i := 0; i < l; i++ {
		pairs[i] = p.privateClientCACerts[i].ToPair()
	}
	return pairs
}

// GetPrivateClientPairs is used to get all private client certificates.
func (p *Pool) GetPrivateClientPairs() []*Pair {
	p.m.RLock()
	defer p.m.RUnlock()
	l := len(p.privateClientCerts)
	pairs := make([]*Pair, l)
	for i := 0; i < l; i++ {
		pairs[i] = p.privateClientCerts[i].ToPair()
	}
	return pairs
}
