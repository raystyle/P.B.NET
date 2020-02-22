package testcert

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/cert"
	"project/internal/crypto/cert/certutil"
)

// CertPool is used to create a certificate pool for test.
func CertPool(t *testing.T) *cert.Pool {
	pool := cert.NewPool()
	addPublicRootCACerts(t, pool)
	addPublicClientCACerts(t, pool)
	addPublicClientCerts(t, pool)
	addPrivateRootCACerts(t, pool)
	addPrivateClientCACerts(t, pool)
	addPrivateClientCerts(t, pool)
	return pool
}

func addPublicRootCACerts(t *testing.T, pool *cert.Pool) {
	systemCertPool, err := certutil.SystemCertPool()
	require.NoError(t, err)
	certs := systemCertPool.Certs()
	for i := 0; i < len(certs); i++ {
		err = pool.AddPublicRootCACert(certs[i])
		require.NoError(t, err)
	}
}

func addPublicClientCACerts(t *testing.T, pool *cert.Pool) {

	err := pool.AddPublicClientCACert(nil)
	require.NoError(t, err)
}

func addPublicClientCerts(t *testing.T, pool *cert.Pool) {
	err := pool.AddPublicClientCert(nil, nil)
	require.NoError(t, err)
}

func addPrivateRootCACerts(t *testing.T, pool *cert.Pool) {
	err := pool.AddPrivateRootCACert(nil, nil)
	require.NoError(t, err)
}

func addPrivateClientCACerts(t *testing.T, pool *cert.Pool) {
	err := pool.AddPrivateClientCACert(nil, nil)
	require.NoError(t, err)
}

func addPrivateClientCerts(t *testing.T, pool *cert.Pool) {
	err := pool.AddPrivateClientCert(nil, nil)
	require.NoError(t, err)
}
