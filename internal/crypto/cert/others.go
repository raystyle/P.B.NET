// +build !windows

package cert

import (
	"crypto/x509"
)

// SystemCertPool is used to return system certificate pool.
func SystemCertPool() (*x509.CertPool, error) {
	return x509.SystemCertPool()
}
