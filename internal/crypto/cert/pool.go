package cert

import (
	"bytes"
	"crypto/x509"
	"fmt"
	"sync"

	"github.com/pkg/errors"

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
		privateKey, err := ParsePrivateKeyBytes(pri)
		if err != nil {
			return nil, err
		}
		if !Match(certCopy, privateKey) {
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

// DeletePublicRootCACert is used to delete public root CA certificate.
func (p *Pool) DeletePublicRootCACert(i int) error {
	p.rwm.Lock()
	defer p.rwm.Unlock()
	if i < 0 || i > len(p.publicRootCACerts)-1 {
		return errors.Errorf("invalid id: %d", i)
	}
	p.publicRootCACerts = append(p.publicRootCACerts[:i], p.publicRootCACerts[i+1:]...)
	return nil
}

// DeletePublicClientCACert is used to delete public client CA certificate.
func (p *Pool) DeletePublicClientCACert(i int) error {
	p.rwm.Lock()
	defer p.rwm.Unlock()
	if i < 0 || i > len(p.publicClientCACerts)-1 {
		return errors.Errorf("invalid id: %d", i)
	}
	p.publicClientCACerts = append(p.publicClientCACerts[:i], p.publicClientCACerts[i+1:]...)
	return nil
}

// DeletePublicClientCert is used to delete public client certificate.
func (p *Pool) DeletePublicClientCert(i int) error {
	p.rwm.Lock()
	defer p.rwm.Unlock()
	if i < 0 || i > len(p.publicClientCerts)-1 {
		return errors.Errorf("invalid id: %d", i)
	}
	p.publicClientCerts = append(p.publicClientCerts[:i], p.publicClientCerts[i+1:]...)
	return nil
}

// DeletePrivateRootCACert is used to delete private root CA certificate.
func (p *Pool) DeletePrivateRootCACert(i int) error {
	p.rwm.Lock()
	defer p.rwm.Unlock()
	if i < 0 || i > len(p.privateRootCACerts)-1 {
		return errors.Errorf("invalid id: %d", i)
	}
	p.privateRootCACerts = append(p.privateRootCACerts[:i], p.privateRootCACerts[i+1:]...)
	return nil
}

// DeletePrivateClientCACert is used to delete private client CA certificate.
func (p *Pool) DeletePrivateClientCACert(i int) error {
	p.rwm.Lock()
	defer p.rwm.Unlock()
	if i < 0 || i > len(p.privateClientCACerts)-1 {
		return errors.Errorf("invalid id: %d", i)
	}
	p.privateClientCACerts = append(p.privateClientCACerts[:i], p.privateClientCACerts[i+1:]...)
	return nil
}

// DeletePrivateClientCert is used to delete private client certificate.
func (p *Pool) DeletePrivateClientCert(i int) error {
	p.rwm.Lock()
	defer p.rwm.Unlock()
	if i < 0 || i > len(p.privateClientCerts)-1 {
		return errors.Errorf("invalid id: %d", i)
	}
	p.privateClientCerts = append(p.privateClientCerts[:i], p.privateClientCerts[i+1:]...)
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

// AddToRawCertPool is used to add certificates to raw certificate pool.
func (p *Pool) AddToRawCertPool(rcp *RawCertPool) {
	pubRootCACerts := p.GetPublicRootCACerts()
	for i := 0; i < len(pubRootCACerts); i++ {
		rcp.PublicRootCACerts = append(rcp.PublicRootCACerts, pubRootCACerts[i].Raw)
	}
	pubClientCACerts := p.GetPublicClientCACerts()
	for i := 0; i < len(pubClientCACerts); i++ {
		rcp.PublicClientCACerts = append(rcp.PublicClientCACerts, pubClientCACerts[i].Raw)
	}
	pubClientPairs := p.GetPublicClientPairs()
	for i := 0; i < len(pubClientPairs); i++ {
		c, k := pubClientPairs[i].Encode()
		rcp.PublicClientPairs = append(rcp.PublicClientPairs, struct {
			Cert []byte `msgpack:"a"`
			Key  []byte `msgpack:"b"`
		}{Cert: c, Key: k})
	}
	priRootCACerts := p.GetPrivateRootCACerts()
	for i := 0; i < len(priRootCACerts); i++ {
		rcp.PrivateRootCACerts = append(rcp.PrivateRootCACerts, priRootCACerts[i].Raw)
	}
	priClientCACerts := p.GetPrivateClientCACerts()
	for i := 0; i < len(priClientCACerts); i++ {
		rcp.PrivateClientCACerts = append(rcp.PrivateClientCACerts, priClientCACerts[i].Raw)
	}
	priClientPairs := p.GetPrivateClientPairs()
	for i := 0; i < len(priClientPairs); i++ {
		c, k := priClientPairs[i].Encode()
		rcp.PrivateClientPairs = append(rcp.PrivateClientPairs, struct {
			Cert []byte `msgpack:"a"`
			Key  []byte `msgpack:"b"`
		}{Cert: c, Key: k})
	}
}

// RawCertPool contains raw certificates, it used for Node and Beacon Config.
type RawCertPool struct {
	PublicRootCACerts   [][]byte `msgpack:"a"`
	PublicClientCACerts [][]byte `msgpack:"b"`
	PublicClientPairs   []struct {
		Cert []byte `msgpack:"a"`
		Key  []byte `msgpack:"b"`
	} `msgpack:"c"`
	PrivateRootCACerts   [][]byte `msgpack:"d"`
	PrivateClientCACerts [][]byte `msgpack:"e"`
	PrivateClientPairs   []struct {
		Cert []byte `msgpack:"a"`
		Key  []byte `msgpack:"b"`
	} `msgpack:"f"`
}

// NewPoolFromRawCertPool is used to create a certificate pool from raw certificate pool.
func NewPoolFromRawCertPool(rcp *RawCertPool) (*Pool, error) {
	memory := security.NewMemory()
	defer memory.Flush()

	pool := NewPool()
	for i := 0; i < len(rcp.PublicRootCACerts); i++ {
		err := pool.AddPublicRootCACert(rcp.PublicRootCACerts[i])
		if err != nil {
			return nil, err
		}
	}
	for i := 0; i < len(rcp.PublicClientCACerts); i++ {
		err := pool.AddPublicClientCACert(rcp.PublicClientCACerts[i])
		if err != nil {
			return nil, err
		}
	}
	for i := 0; i < len(rcp.PublicClientPairs); i++ {
		memory.Padding()
		pair := rcp.PublicClientPairs[i]
		err := pool.AddPublicClientCert(pair.Cert, pair.Key)
		if err != nil {
			return nil, err
		}
	}
	for i := 0; i < len(rcp.PrivateRootCACerts); i++ {
		err := pool.AddPrivateRootCACert(rcp.PrivateRootCACerts[i], nil)
		if err != nil {
			return nil, err
		}
	}
	for i := 0; i < len(rcp.PrivateClientCACerts); i++ {
		err := pool.AddPrivateClientCACert(rcp.PrivateClientCACerts[i], nil)
		if err != nil {
			return nil, err
		}
	}
	for i := 0; i < len(rcp.PrivateClientPairs); i++ {
		memory.Padding()
		pair := rcp.PrivateClientPairs[i]
		err := pool.AddPrivateClientCert(pair.Cert, pair.Key)
		if err != nil {
			return nil, err
		}
	}
	return pool, nil
}

// NewPoolWithSystemCerts is used to create a certificate pool with system certificate.
func NewPoolWithSystemCerts() (*Pool, error) {
	systemCertPool, err := SystemCertPool()
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
