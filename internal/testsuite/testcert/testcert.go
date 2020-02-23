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
	addPrivateRootCACerts(t, pool)
	addPrivateClientCACerts(t, pool)
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
	add := func() {
		caPair, err := cert.GenerateCA(nil)
		require.NoError(t, err)
		cPair1, err := cert.Generate(caPair.Certificate, caPair.PrivateKey, nil)
		require.NoError(t, err)
		cPair2, err := cert.Generate(caPair.Certificate, caPair.PrivateKey, nil)
		require.NoError(t, err)

		err = pool.AddPublicClientCACert(caPair.Certificate)
		require.NoError(t, err)
		err = pool.AddPublicClientCert(cPair1.Certificate, cPair1.PrivateKey)
		require.NoError(t, err)
		err = pool.AddPublicClientCert(cPair2.Certificate, cPair2.PrivateKey)
		require.NoError(t, err)
	}
	add()
	add()
}

func addPrivateRootCACerts(t *testing.T, pool *cert.Pool) {
	add := func() {
		caPair, err := cert.GenerateCA(nil)
		require.NoError(t, err)
		err = pool.AddPrivateRootCACert(caPair.Certificate, caPair.PrivateKey)
		require.NoError(t, err)
	}
	add()
	add()
}

func addPrivateClientCACerts(t *testing.T, pool *cert.Pool) {
	add := func() {
		caPair, err := cert.GenerateCA(nil)
		require.NoError(t, err)
		cPair1, err := cert.Generate(caPair.Certificate, caPair.PrivateKey, nil)
		require.NoError(t, err)
		cPair2, err := cert.Generate(caPair.Certificate, caPair.PrivateKey, nil)
		require.NoError(t, err)

		err = pool.AddPrivateClientCACert(caPair.Certificate, caPair.PrivateKey)
		require.NoError(t, err)
		err = pool.AddPrivateClientCert(cPair1.Certificate, cPair1.PrivateKey)
		require.NoError(t, err)
		err = pool.AddPrivateClientCert(cPair2.Certificate, cPair2.PrivateKey)
		require.NoError(t, err)
	}
	add()
	add()
}
