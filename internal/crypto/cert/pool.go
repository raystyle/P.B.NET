package cert

import (
	"bytes"
	"crypto/x509"
	"fmt"
	"sync"

	"github.com/pkg/errors"

	"project/internal/crypto/cert/certutil"
	"project/internal/security"
)

// ErrMismatchedKey is the error about the key.
var ErrMismatchedKey = fmt.Errorf("private key does not match public key in certificate")

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

func loadCertAndPrivateKey(cert, pri []byte) (*pair, error) {
	if len(pri) == 0 {
		return nil, errors.New("need private key")
	}
	return loadCertWithPrivateKey(cert, pri)
}

func loadCertWithPrivateKey(cert, pri []byte) (*pair, error) {
	raw := make([]byte, len(cert))
	copy(raw, cert)
	certCopy, err := x509.ParseCertificate(raw)
	if err != nil {
		return nil, err
	}
	pair := pair{Certificate: certCopy}
	if len(pri) != 0 {
		privateKey, err := certutil.ParsePrivateKeyBytes(pri)
		if err != nil {
			return nil, err
		}
		if !certutil.Match(certCopy, privateKey) {
			return nil, ErrMismatchedKey
		}
		priBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
		if err != nil {
			return nil, err
		}
		pair.PrivateKey = security.NewBytes(priBytes)
	}
	return &pair, nil
}

// AddPublicRootCACert is used to add public root CA certificate.
func (p *Pool) AddPublicRootCACert(cert []byte) error {
	p.rwm.Lock()
	defer p.rwm.Unlock()
	if certIsExist(p.publicRootCACerts, cert) {
		return errors.New("this public root ca certificate already exists")
	}
	// must copy
	raw := make([]byte, len(cert))
	copy(raw, cert)
	certCopy, err := x509.ParseCertificate(raw)
	if err != nil {
		return errors.Wrap(err, "failed to add public root ca certificate")
	}
	p.publicRootCACerts = append(p.publicRootCACerts, certCopy)
	return nil
}

// AddPublicClientCACert is used to add public client CA certificate.
func (p *Pool) AddPublicClientCACert(cert []byte) error {
	p.rwm.Lock()
	defer p.rwm.Unlock()
	if certIsExist(p.publicClientCACerts, cert) {
		return errors.New("this public client ca certificate already exists")
	}
	// must copy
	raw := make([]byte, len(cert))
	copy(raw, cert)
	certCopy, err := x509.ParseCertificate(raw)
	if err != nil {
		return errors.Wrap(err, "failed to add public client ca certificate")
	}
	p.publicClientCACerts = append(p.publicClientCACerts, certCopy)
	return nil
}

// AddPublicClientCert is used to add public client certificate.
func (p *Pool) AddPublicClientCert(cert, pri []byte) error {
	p.rwm.Lock()
	defer p.rwm.Unlock()
	if pairIsExist(p.publicClientCerts, cert) {
		return errors.New("this public client certificate already exists")
	}
	pair, err := loadCertAndPrivateKey(cert, pri)
	if err != nil {
		return errors.Wrap(err, "failed to add public client certificate")
	}
	p.publicClientCerts = append(p.publicClientCerts, pair)
	return nil
}

// AddPrivateRootCACert is used to add private root CA certificate.
func (p *Pool) AddPrivateRootCACert(cert, pri []byte) error {
	p.rwm.Lock()
	defer p.rwm.Unlock()
	if pairIsExist(p.privateRootCACerts, cert) {
		return errors.New("this private root ca certificate already exists")
	}
	pair, err := loadCertWithPrivateKey(cert, pri)
	if err != nil {
		return errors.Wrap(err, "failed to add private root ca certificate")
	}
	p.privateRootCACerts = append(p.privateRootCACerts, pair)
	return nil
}

// AddPrivateClientCACert is used to add private client CA certificate.
func (p *Pool) AddPrivateClientCACert(cert, pri []byte) error {
	p.rwm.Lock()
	defer p.rwm.Unlock()
	if pairIsExist(p.privateClientCACerts, cert) {
		return errors.New("this private client ca certificate already exists")
	}
	pair, err := loadCertWithPrivateKey(cert, pri)
	if err != nil {
		return errors.Wrap(err, "failed to add private client ca certificate")
	}
	p.privateClientCACerts = append(p.privateClientCACerts, pair)
	return nil
}

// AddPrivateClientCert is used to add private client certificate.
func (p *Pool) AddPrivateClientCert(cert, pri []byte) error {
	p.rwm.Lock()
	defer p.rwm.Unlock()
	if pairIsExist(p.privateClientCerts, cert) {
		return errors.New("this private client certificate already exists")
	}
	pair, err := loadCertAndPrivateKey(cert, pri)
	if err != nil {
		return errors.Wrap(err, "failed to add private client certificate")
	}
	p.privateClientCerts = append(p.privateClientCerts, pair)
	return nil
}

// GetPublicRootCACerts is used to get all public root CA certificates.
func (p *Pool) GetPublicRootCACerts() []*x509.Certificate {
	p.rwm.RLock()
	defer p.rwm.RUnlock()
	certs := make([]*x509.Certificate, len(p.publicRootCACerts))
	copy(certs, p.publicRootCACerts)
	return certs
}

// GetPublicClientCACerts is used to get all public client CA certificates.
func (p *Pool) GetPublicClientCACerts() []*x509.Certificate {
	p.rwm.RLock()
	defer p.rwm.RUnlock()
	certs := make([]*x509.Certificate, len(p.publicClientCACerts))
	copy(certs, p.publicClientCACerts)
	return certs
}

// GetPublicClientPairs is used to get all public client certificates.
func (p *Pool) GetPublicClientPairs() []*Pair {
	p.rwm.RLock()
	defer p.rwm.RUnlock()
	l := len(p.publicClientCerts)
	pairs := make([]*Pair, l)
	for i := 0; i < l; i++ {
		pairs[i] = p.publicClientCerts[i].ToPair()
	}
	return pairs
}

// GetPrivateRootCACerts is used to get all private root CA certificates.
func (p *Pool) GetPrivateRootCACerts() []*x509.Certificate {
	p.rwm.RLock()
	defer p.rwm.RUnlock()
	l := len(p.privateRootCACerts)
	certs := make([]*x509.Certificate, l)
	for i := 0; i < l; i++ {
		certs[i] = p.privateRootCACerts[i].Certificate
	}
	return certs
}

// GetPrivateClientCACerts is used to get all private client CA certificates.
func (p *Pool) GetPrivateClientCACerts() []*x509.Certificate {
	p.rwm.RLock()
	defer p.rwm.RUnlock()
	l := len(p.privateClientCACerts)
	certs := make([]*x509.Certificate, l)
	for i := 0; i < l; i++ {
		certs[i] = p.privateClientCACerts[i].Certificate
	}
	return certs
}

// GetPrivateRootCAPairs is used to get all private root CA certificates.
func (p *Pool) GetPrivateRootCAPairs() []*Pair {
	p.rwm.RLock()
	defer p.rwm.RUnlock()
	l := len(p.privateRootCACerts)
	pairs := make([]*Pair, l)
	for i := 0; i < l; i++ {
		pairs[i] = p.privateRootCACerts[i].ToPair()
	}
	return pairs
}

// GetPrivateClientCAPairs is used to get all private client CA certificates.
func (p *Pool) GetPrivateClientCAPairs() []*Pair {
	p.rwm.RLock()
	defer p.rwm.RUnlock()
	l := len(p.privateClientCACerts)
	pairs := make([]*Pair, l)
	for i := 0; i < l; i++ {
		pairs[i] = p.privateClientCACerts[i].ToPair()
	}
	return pairs
}

// GetPrivateClientPairs is used to get all private client certificates.
func (p *Pool) GetPrivateClientPairs() []*Pair {
	p.rwm.RLock()
	defer p.rwm.RUnlock()
	l := len(p.privateClientCerts)
	pairs := make([]*Pair, l)
	for i := 0; i < l; i++ {
		pairs[i] = p.privateClientCerts[i].ToPair()
	}
	return pairs
}

// RawCertPool contains raw certificates, it used for Node and Beacon Config.
type RawCertPool struct {
	PublicRootCACerts   [][]byte `msgpack:"a"`
	PublicClientCACerts [][]byte `msgpack:"b"`
	PublicClientCerts   []struct {
		Cert []byte `msgpack:"a"`
		Key  []byte `msgpack:"b"`
	} `msgpack:"c"`
	PrivateRootCACerts   [][]byte `msgpack:"d"`
	PrivateClientCACerts [][]byte `msgpack:"e"`
	PrivateClientCerts   []struct {
		Cert []byte `msgpack:"a"`
		Key  []byte `msgpack:"b"`
	} `msgpack:"f"`
}

// NewPoolFromRawCertPool is used to create a certificate pool from raw certificate pool.
func NewPoolFromRawCertPool(pool *RawCertPool) (*Pool, error) {
	memory := security.NewMemory()
	defer memory.Flush()

	certPool := NewPool()
	for i := 0; i < len(pool.PublicRootCACerts); i++ {
		err := certPool.AddPublicRootCACert(pool.PublicRootCACerts[i])
		if err != nil {
			return nil, err
		}
	}
	for i := 0; i < len(pool.PublicClientCACerts); i++ {
		err := certPool.AddPublicClientCACert(pool.PublicClientCACerts[i])
		if err != nil {
			return nil, err
		}
	}
	for i := 0; i < len(pool.PublicClientCerts); i++ {
		memory.Padding()
		pair := pool.PublicClientCerts[i]
		err := certPool.AddPublicClientCert(pair.Cert, pair.Key)
		if err != nil {
			return nil, err
		}
	}
	for i := 0; i < len(pool.PrivateRootCACerts); i++ {
		err := certPool.AddPrivateRootCACert(pool.PrivateRootCACerts[i], nil)
		if err != nil {
			return nil, err
		}
	}
	for i := 0; i < len(pool.PrivateClientCACerts); i++ {
		err := certPool.AddPrivateClientCACert(pool.PrivateClientCACerts[i], nil)
		if err != nil {
			return nil, err
		}
	}
	for i := 0; i < len(pool.PrivateClientCerts); i++ {
		memory.Padding()
		pair := pool.PrivateClientCerts[i]
		err := certPool.AddPrivateClientCert(pair.Cert, pair.Key)
		if err != nil {
			return nil, err
		}
	}
	return certPool, nil
}

// NewPoolWithSystemCerts is used to create a certificate pool with system certificate.
func NewPoolWithSystemCerts() (*Pool, error) {
	systemCertPool, err := certutil.SystemCertPool()
	if err != nil {
		return nil, err
	}
	certPool := NewPool()
	certs := systemCertPool.Certs()
	for i := 0; i < len(certs); i++ {
		err = certPool.AddPublicRootCACert(certs[i].Raw)
		if err != nil {
			return nil, err
		}
	}
	return certPool, nil
}
