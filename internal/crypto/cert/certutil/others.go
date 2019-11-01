// +build !windows

package certutil

import (
	"crypto/x509"
)

func SystemCertPool() (*x509.CertPool, error) {
	return x509.SystemCertPool()
}
