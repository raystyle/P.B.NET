package testcert

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/cert/certutil"
)

func TestCertPool(t *testing.T) {
	pool := CertPool(t)

	systemCertPool, err := certutil.SystemCertPool()
	require.NoError(t, err)
	certs := systemCertPool.Certs()
	require.Equal(t, len(certs), len(pool.GetPublicRootCACerts()))

	require.Equal(t, 2, len(pool.GetPublicClientCACerts()))
	require.Equal(t, 4, len(pool.GetPublicClientPairs()))

	require.Equal(t, 2, len(pool.GetPrivateRootCAPairs()))
	require.Equal(t, 2, len(pool.GetPrivateClientCAPairs()))
	require.Equal(t, 4, len(pool.GetPrivateClientPairs()))
}
