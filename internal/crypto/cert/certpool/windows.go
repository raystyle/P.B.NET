// +build windows

package certpool

import (
	"crypto/x509"
	"errors"
	"sync"
	"syscall"
	"unsafe"
)

var (
	errSystemCert = errors.New("no system certificates")

	systemCerts   []*x509.Certificate
	systemCertsMu sync.Mutex
)

// System is used to return certificate pool from system.
// On windows, the number of the CA and ROOT Certificate is
// incorrect, because the CA "Root Agency" is used to test.
func System() (*x509.CertPool, error) {
	var certs []*x509.Certificate
	systemCertsMu.Lock()
	defer systemCertsMu.Unlock()
	if errSystemCert == nil {
		certs = make([]*x509.Certificate, len(systemCerts))
		copy(certs, systemCerts)
	} else {
		c, err := loadSystemCert()
		if err != nil {
			return nil, err
		}
		systemCerts = c
		errSystemCert = nil
		certs = make([]*x509.Certificate, len(systemCerts))
		copy(certs, systemCerts)
	}
	// must new pool
	pool := x509.NewCertPool()
	for i := 0; i < len(certs); i++ {
		pool.AddCert(certs[i])
	}
	return pool, nil
}

func loadSystemCert() ([]*x509.Certificate, error) {
	var certs [][]byte
	names := []string{"ROOT", "CA"}
	for i := 0; i < len(names); i++ {
		raw, err := LoadSystemCertWithName(names[i])
		if err != nil {
			return nil, err
		}
		certs = append(certs, raw...)
	}
	var pool []*x509.Certificate
	for i := 0; i < len(certs); i++ {
		cert, err := x509.ParseCertificate(certs[i])
		if err == nil {
			pool = append(pool, cert)
		}
	}
	return pool, nil
}

// LoadSystemCertWithName is used to load system certificate pool.
// Usually name is "ROOT" or "CA".
func LoadSystemCertWithName(name string) ([][]byte, error) {
	n, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}
	store, err := syscall.CertOpenSystemStore(0, n)
	if err != nil {
		return nil, err
	}
	defer func() { _ = syscall.CertCloseStore(store, 0) }()
	var certs [][]byte
	var cert *syscall.CertContext
	for {
		cert, err = syscall.CertEnumCertificatesInStore(store, cert)
		if err != nil {
			if errno, ok := err.(syscall.Errno); ok {
				if errno == 0x80092004 { // 0x80092004 is CRYPT_E_NOT_FOUND
					break
				}
			}
			return nil, err
		}
		if cert == nil {
			break
		}
		// copy the buf, since ParseCertificate does not create its own copy.
		buf := (*[1 << 20]byte)(unsafe.Pointer(cert.EncodedCert))[:] // #nosec
		buf2 := make([]byte, cert.Length)
		copy(buf2, buf)
		certs = append(certs, buf2)
	}
	return certs, nil
}
