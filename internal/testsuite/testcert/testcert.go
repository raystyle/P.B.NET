package testcert

import (
	"crypto/x509"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/cert"
	"project/internal/crypto/cert/certpool"
)

// certificates from system.
var systemCerts []*x509.Certificate

// PublicRootCANum is the number of the public Root CA certificates.
var PublicRootCANum int

// the number of the generated certificates.
const (
	PublicClientCANum    = 2
	PublicClientCertNum  = 4
	PrivateRootCANum     = 3
	PrivateClientCANum   = 6
	PrivateClientCertNum = 5
)

func init() { loadSystemCertPool() }

func loadSystemCertPool() {
	systemCertPool, err := certpool.System()
	if err != nil {
		const format = "[init] failed to load system certificate pool: %s"
		panic(fmt.Sprintf(format, err))
	}
	systemCerts = systemCertPool.Certs()
	PublicRootCANum = len(systemCerts)
}

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
	for i := 0; i < PublicRootCANum; i++ {
		err := pool.AddPublicRootCACert(systemCerts[i].Raw)
		require.NoError(t, err)
	}
}

var opts = &cert.Options{Algorithm: "rsa|1024"}

func addPublicClientCACerts(t *testing.T, pool *cert.Pool) {
	for i := 0; i < PublicClientCANum; i++ {
		caPair, err := cert.GenerateCA(opts)
		require.NoError(t, err)
		cPair1, err := cert.Generate(caPair.Certificate, caPair.PrivateKey, opts)
		require.NoError(t, err)
		cPair2, err := cert.Generate(caPair.Certificate, caPair.PrivateKey, opts)
		require.NoError(t, err)

		err = pool.AddPublicClientCACert(caPair.Certificate.Raw)
		require.NoError(t, err)
		err = pool.AddPublicClientCert(cPair1.Encode())
		require.NoError(t, err)
		err = pool.AddPublicClientCert(cPair2.Encode())
		require.NoError(t, err)
	}
}

func addPrivateRootCACerts(t *testing.T, pool *cert.Pool) {
	for i := 0; i < PrivateRootCANum; i++ {
		caPair, err := cert.GenerateCA(opts)
		require.NoError(t, err)
		err = pool.AddPrivateRootCACert(caPair.Encode())
		require.NoError(t, err)
	}
}

func addPrivateClientCACerts(t *testing.T, pool *cert.Pool) {
	caPair, err := cert.GenerateCA(opts)
	require.NoError(t, err)
	err = pool.AddPrivateClientCACert(caPair.Encode())
	require.NoError(t, err)

	for i := 0; i < PrivateClientCertNum; i++ {
		caPair, err := cert.GenerateCA(opts)
		require.NoError(t, err)
		cPair, err := cert.Generate(caPair.Certificate, caPair.PrivateKey, opts)
		require.NoError(t, err)

		err = pool.AddPrivateClientCACert(caPair.Encode())
		require.NoError(t, err)
		err = pool.AddPrivateClientCert(cPair.Encode())
		require.NoError(t, err)
	}
}
