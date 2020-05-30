package cert

import (
	"bytes"
	"crypto/x509"
	"fmt"
	"sync"

	"github.com/pkg/errors"

	"project/internal/crypto/cert/certpool"
	"project/internal/security"
)

// ErrMismatchedKey is the error about the key.
var ErrMismatchedKey = fmt.Errorf("private key does not match public key in certificate")

// pair is used to protect private key about certificate.
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
	pubRootCACerts   []*x509.Certificate
	pubClientCACerts []*x509.Certificate
	pubClientCerts   []*pair

	// private means these certificates are from the Controller or self.
	priRootCACerts   []*pair // only Controller contain the Private Key
	priClientCACerts []*pair // only Controller contain the Private Key
	priClientCerts   []*pair

	rwm sync.RWMutex
}

// NewPool is used to create a new certificate pool.
func NewPool() *Pool {
	security.PaddingMemory()
	defer security.FlushMemory()
	memory := security.NewMemory()
	defer memory.Flush()
	return new(Pool)
}

func certIsExist(certs []*x509.Certificate, cert []byte) bool {
	for i := 0; i < len(certs); i++ {
		if bytes.Equal(certs[i].Raw, cert) {
			return true
		}
	}
	return false
}

func pairIsExist(pairs []*pair, cert []byte) bool {
	for i := 0; i < len(pairs); i++ {
		if bytes.Equal(pairs[i].Certificate.Raw, cert) {
			return true
		}
	}
	return false
}

func loadPair(cert, pri []byte) (*pair, error) {
	if len(cert) == 0 {
		return nil, errors.New("no certificate")
	}
	if len(pri) == 0 {
		return nil, errors.New("no private key")
	}
	raw := make([]byte, len(cert))
	copy(raw, cert)
	certCp, err := x509.ParseCertificate(raw)
	if err != nil {
		return nil, err
	}
	privateKey, err := ParsePrivateKeyBytes(pri)
	if err != nil {
		return nil, err
	}
	if !Match(certCp, privateKey) {
		return nil, ErrMismatchedKey
	}
	priBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, err
	}
	return &pair{
		Certificate: certCp,
		PrivateKey:  security.NewBytes(priBytes),
	}, nil
}

func loadCertToPair(cert []byte) (*pair, error) {
	if len(cert) == 0 {
		return nil, errors.New("no certificate")
	}
	raw := make([]byte, len(cert))
	copy(raw, cert)
	certCopy, err := x509.ParseCertificate(raw)
	if err != nil {
		return nil, err
	}
	return &pair{Certificate: certCopy}, nil
}

// AddPublicRootCACert is used to add public root CA certificate.
func (p *Pool) AddPublicRootCACert(cert []byte) error {
	p.rwm.Lock()
	defer p.rwm.Unlock()
	if certIsExist(p.pubRootCACerts, cert) {
		return errors.New("this public root ca certificate already exists")
	}
	// must copy
	raw := make([]byte, len(cert))
	copy(raw, cert)
	certCopy, err := x509.ParseCertificate(raw)
	if err != nil {
		return errors.Wrap(err, "failed to add public root ca certificate")
	}
	p.pubRootCACerts = append(p.pubRootCACerts, certCopy)
	return nil
}

// AddPublicClientCACert is used to add public client CA certificate.
func (p *Pool) AddPublicClientCACert(cert []byte) error {
	p.rwm.Lock()
	defer p.rwm.Unlock()
	if certIsExist(p.pubClientCACerts, cert) {
		return errors.New("this public client ca certificate already exists")
	}
	// must copy
	raw := make([]byte, len(cert))
	copy(raw, cert)
	certCopy, err := x509.ParseCertificate(raw)
	if err != nil {
		return errors.Wrap(err, "failed to add public client ca certificate")
	}
	p.pubClientCACerts = append(p.pubClientCACerts, certCopy)
	return nil
}

// AddPublicClientPair is used to add public client certificate.
func (p *Pool) AddPublicClientPair(cert, pri []byte) error {
	p.rwm.Lock()
	defer p.rwm.Unlock()
	if pairIsExist(p.pubClientCerts, cert) {
		return errors.New("this public client certificate already exists")
	}
	pair, err := loadPair(cert, pri)
	if err != nil {
		return errors.Wrap(err, "failed to add public client certificate")
	}
	p.pubClientCerts = append(p.pubClientCerts, pair)
	return nil
}

// AddPrivateRootCAPair is used to add private root CA certificate with private key.
func (p *Pool) AddPrivateRootCAPair(cert, pri []byte) error {
	p.rwm.Lock()
	defer p.rwm.Unlock()
	if pairIsExist(p.priRootCACerts, cert) {
		return errors.New("this private root ca certificate already exists")
	}
	pair, err := loadPair(cert, pri)
	if err != nil {
		return errors.Wrap(err, "failed to add private root ca certificate")
	}
	p.priRootCACerts = append(p.priRootCACerts, pair)
	return nil
}

// AddPrivateRootCACert is used to add private root CA certificate.
func (p *Pool) AddPrivateRootCACert(cert []byte) error {
	p.rwm.Lock()
	defer p.rwm.Unlock()
	if pairIsExist(p.priRootCACerts, cert) {
		return errors.New("this private root ca certificate already exists")
	}
	pair, err := loadCertToPair(cert)
	if err != nil {
		return errors.Wrap(err, "failed to add private root ca certificate")
	}
	p.priRootCACerts = append(p.priRootCACerts, pair)
	return nil
}

// AddPrivateClientCAPair is used to add private client CA certificate with private key.
func (p *Pool) AddPrivateClientCAPair(cert, pri []byte) error {
	p.rwm.Lock()
	defer p.rwm.Unlock()
	if pairIsExist(p.priClientCACerts, cert) {
		return errors.New("this private client ca certificate already exists")
	}
	pair, err := loadPair(cert, pri)
	if err != nil {
		return errors.Wrap(err, "failed to add private client ca certificate")
	}
	p.priClientCACerts = append(p.priClientCACerts, pair)
	return nil
}

// AddPrivateClientCACert is used to add private client CA certificate with private key.
func (p *Pool) AddPrivateClientCACert(cert []byte) error {
	p.rwm.Lock()
	defer p.rwm.Unlock()
	if pairIsExist(p.priClientCACerts, cert) {
		return errors.New("this private client ca certificate already exists")
	}
	pair, err := loadCertToPair(cert)
	if err != nil {
		return errors.Wrap(err, "failed to add private client ca certificate")
	}
	p.priClientCACerts = append(p.priClientCACerts, pair)
	return nil
}

// AddPrivateClientPair is used to add private client certificate.
func (p *Pool) AddPrivateClientPair(cert, pri []byte) error {
	p.rwm.Lock()
	defer p.rwm.Unlock()
	if pairIsExist(p.priClientCerts, cert) {
		return errors.New("this private client certificate already exists")
	}
	pair, err := loadPair(cert, pri)
	if err != nil {
		return errors.Wrap(err, "failed to add private client certificate")
	}
	p.priClientCerts = append(p.priClientCerts, pair)
	return nil
}

// DeletePublicRootCACert is used to delete public root CA certificate.
func (p *Pool) DeletePublicRootCACert(i int) error {
	p.rwm.Lock()
	defer p.rwm.Unlock()
	if i < 0 || i > len(p.pubRootCACerts)-1 {
		return errors.Errorf("invalid id: %d", i)
	}
	p.pubRootCACerts = append(p.pubRootCACerts[:i], p.pubRootCACerts[i+1:]...)
	return nil
}

// DeletePublicClientCACert is used to delete public client CA certificate.
func (p *Pool) DeletePublicClientCACert(i int) error {
	p.rwm.Lock()
	defer p.rwm.Unlock()
	if i < 0 || i > len(p.pubClientCACerts)-1 {
		return errors.Errorf("invalid id: %d", i)
	}
	p.pubClientCACerts = append(p.pubClientCACerts[:i], p.pubClientCACerts[i+1:]...)
	return nil
}

// DeletePublicClientCert is used to delete public client certificate.
func (p *Pool) DeletePublicClientCert(i int) error {
	p.rwm.Lock()
	defer p.rwm.Unlock()
	if i < 0 || i > len(p.pubClientCerts)-1 {
		return errors.Errorf("invalid id: %d", i)
	}
	p.pubClientCerts = append(p.pubClientCerts[:i], p.pubClientCerts[i+1:]...)
	return nil
}

// DeletePrivateRootCACert is used to delete private root CA certificate.
func (p *Pool) DeletePrivateRootCACert(i int) error {
	p.rwm.Lock()
	defer p.rwm.Unlock()
	if i < 0 || i > len(p.priRootCACerts)-1 {
		return errors.Errorf("invalid id: %d", i)
	}
	p.priRootCACerts = append(p.priRootCACerts[:i], p.priRootCACerts[i+1:]...)
	return nil
}

// DeletePrivateClientCACert is used to delete private client CA certificate.
func (p *Pool) DeletePrivateClientCACert(i int) error {
	p.rwm.Lock()
	defer p.rwm.Unlock()
	if i < 0 || i > len(p.priClientCACerts)-1 {
		return errors.Errorf("invalid id: %d", i)
	}
	p.priClientCACerts = append(p.priClientCACerts[:i], p.priClientCACerts[i+1:]...)
	return nil
}

// DeletePrivateClientCert is used to delete private client certificate.
func (p *Pool) DeletePrivateClientCert(i int) error {
	p.rwm.Lock()
	defer p.rwm.Unlock()
	if i < 0 || i > len(p.priClientCerts)-1 {
		return errors.Errorf("invalid id: %d", i)
	}
	p.priClientCerts = append(p.priClientCerts[:i], p.priClientCerts[i+1:]...)
	return nil
}

// GetPublicRootCACerts is used to get all public root CA certificates.
func (p *Pool) GetPublicRootCACerts() []*x509.Certificate {
	p.rwm.RLock()
	defer p.rwm.RUnlock()
	certs := make([]*x509.Certificate, len(p.pubRootCACerts))
	copy(certs, p.pubRootCACerts)
	return certs
}

// GetPublicClientCACerts is used to get all public client CA certificates.
func (p *Pool) GetPublicClientCACerts() []*x509.Certificate {
	p.rwm.RLock()
	defer p.rwm.RUnlock()
	certs := make([]*x509.Certificate, len(p.pubClientCACerts))
	copy(certs, p.pubClientCACerts)
	return certs
}

// GetPublicClientPairs is used to get all public client certificates.
func (p *Pool) GetPublicClientPairs() []*Pair {
	p.rwm.RLock()
	defer p.rwm.RUnlock()
	l := len(p.pubClientCerts)
	pairs := make([]*Pair, l)
	for i := 0; i < l; i++ {
		pairs[i] = p.pubClientCerts[i].ToPair()
	}
	return pairs
}

// GetPrivateRootCAPairs is used to get all private root CA certificates.
func (p *Pool) GetPrivateRootCAPairs() []*Pair {
	p.rwm.RLock()
	defer p.rwm.RUnlock()
	l := len(p.priRootCACerts)
	pairs := make([]*Pair, l)
	for i := 0; i < l; i++ {
		pairs[i] = p.priRootCACerts[i].ToPair()
	}
	return pairs
}

// GetPrivateRootCACerts is used to get all private root CA certificates.
func (p *Pool) GetPrivateRootCACerts() []*x509.Certificate {
	p.rwm.RLock()
	defer p.rwm.RUnlock()
	l := len(p.priRootCACerts)
	certs := make([]*x509.Certificate, l)
	for i := 0; i < l; i++ {
		certs[i] = p.priRootCACerts[i].Certificate
	}
	return certs
}

// GetPrivateClientCAPairs is used to get all private client CA certificates.
func (p *Pool) GetPrivateClientCAPairs() []*Pair {
	p.rwm.RLock()
	defer p.rwm.RUnlock()
	l := len(p.priClientCACerts)
	pairs := make([]*Pair, l)
	for i := 0; i < l; i++ {
		pairs[i] = p.priClientCACerts[i].ToPair()
	}
	return pairs
}

// GetPrivateClientCACerts is used to get all private client CA certificates.
func (p *Pool) GetPrivateClientCACerts() []*x509.Certificate {
	p.rwm.RLock()
	defer p.rwm.RUnlock()
	l := len(p.priClientCACerts)
	certs := make([]*x509.Certificate, l)
	for i := 0; i < l; i++ {
		certs[i] = p.priClientCACerts[i].Certificate
	}
	return certs
}

// GetPrivateClientPairs is used to get all private client certificates.
func (p *Pool) GetPrivateClientPairs() []*Pair {
	p.rwm.RLock()
	defer p.rwm.RUnlock()
	l := len(p.priClientCerts)
	pairs := make([]*Pair, l)
	for i := 0; i < l; i++ {
		pairs[i] = p.priClientCerts[i].ToPair()
	}
	return pairs
}

// NewPoolWithSystemCerts is used to create a certificate pool with system certificate.
func NewPoolWithSystemCerts() (*Pool, error) {
	systemCertPool, err := certpool.System()
	if err != nil {
		return nil, err
	}
	pool := NewPool()
	certs := systemCertPool.Certs()
	for i := 0; i < len(certs); i++ {
		err = pool.AddPublicRootCACert(certs[i].Raw)
		if err != nil {
			return nil, err
		}
	}
	return pool, nil
}
