package testcert

import (
	"crypto/x509"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/cert/certpool"
	"project/internal/patch/monkey"
	"project/internal/testsuite"
)

func TestLoadSystemCertPool(t *testing.T) {
	defer func() {
		r := recover()
		require.NotNil(t, r)
		t.Log(r)
	}()
	patch := func() (*x509.CertPool, error) {
		return nil, monkey.Error
	}
	pg := monkey.Patch(certpool.System, patch)
	defer pg.Unpatch()

	loadSystemCertPool()
}

func TestCertPool(t *testing.T) {
	pool := CertPool(t)

	systemCertPool, err := certpool.System()
	require.NoError(t, err)
	certs := systemCertPool.Certs()
	require.Len(t, pool.GetPublicRootCACerts(), len(certs))

	require.Len(t, pool.GetPublicClientCACerts(), PublicClientCANum)
	require.Len(t, pool.GetPublicClientPairs(), PublicClientCertNum)

	require.Len(t, pool.GetPrivateRootCAPairs(), PrivateRootCANum)
	require.Len(t, pool.GetPrivateClientCAPairs(), PrivateClientCANum)
	require.Len(t, pool.GetPrivateClientPairs(), PrivateClientCertNum)

	testsuite.IsDestroyed(t, pool)
}
