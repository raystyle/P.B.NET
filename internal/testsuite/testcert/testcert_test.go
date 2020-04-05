package testcert

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/cert"
	"project/internal/testsuite"
)

func TestCertPool(t *testing.T) {
	pool := CertPool(t)

	systemCertPool, err := cert.SystemCertPool()
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
