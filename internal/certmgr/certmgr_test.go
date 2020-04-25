package certmgr

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/cert"
)

var testPassword = []byte("pbnet")

func testGenerateCertPool(t *testing.T) *cert.Pool {
	// load system certificates
	pool, err := cert.NewPoolWithSystemCerts()
	require.NoError(t, err)

	// create Root CA certificate
	rootCA, err := cert.GenerateCA(nil)
	require.NoError(t, err)
	err = pool.AddPrivateRootCACert(rootCA.Encode())
	require.NoError(t, err)

	// create Client CA certificate
	clientCA, err := cert.GenerateCA(nil)
	require.NoError(t, err)
	err = pool.AddPublicClientCACert(clientCA.ASN1())
	require.NoError(t, err)
	err = pool.AddPrivateClientCACert(clientCA.Encode())
	require.NoError(t, err)

	// generate a client certificate and use client CA sign it
	clientCert, err := cert.Generate(clientCA.Certificate, clientCA.PrivateKey, nil)
	require.NoError(t, err)
	err = pool.AddPublicClientCert(clientCert.Encode())
	require.NoError(t, err)
	err = pool.AddPrivateClientCert(clientCert.Encode())
	require.NoError(t, err)

	return pool
}

func testReadKeyFile(t *testing.T) ([]byte, []byte) {
	certsData, err := ioutil.ReadFile(CertFilePath)
	require.NoError(t, err)
	hashData, err := ioutil.ReadFile(HashFilePath)
	require.NoError(t, err)
	return certsData, hashData
}

func testRemoveKeyFile(t *testing.T) {
	err := os.Remove(CertFilePath)
	require.NoError(t, err)
	err = os.Remove(HashFilePath)
	require.NoError(t, err)
}

func TestSaveCtrlCertPool(t *testing.T) {
	t.Run("common", func(t *testing.T) {
		pool := testGenerateCertPool(t)
		err := SaveCtrlCertPool(pool, testPassword)
		require.NoError(t, err)

		testRemoveKeyFile(t)
	})

}

func TestLoadCtrlCertPool(t *testing.T) {
	t.Run("common", func(t *testing.T) {
		pool := testGenerateCertPool(t)
		err := SaveCtrlCertPool(pool, testPassword)
		require.NoError(t, err)
		defer testRemoveKeyFile(t)

		pool = cert.NewPool()
		certsData, hashData := testReadKeyFile(t)
		err = LoadCtrlCertPool(pool, certsData, hashData, testPassword)
		require.NoError(t, err)
	})

}

func testGenerateCert(t *testing.T) *cert.Pair {
	pair, err := cert.GenerateCA(nil)
	require.NoError(t, err)
	return pair
}

func TestNewPoolFromNBCertPool(t *testing.T) {
	cp := new(NBCertPool)

	t.Run("public root ca cert", func(t *testing.T) {
		pair := testGenerateCert(t)
		cp.PublicRootCACerts = [][]byte{pair.ASN1()}
		pool, err := NewPoolFromNBCertPool(cp)
		require.NoError(t, err)
		certs := pool.GetPublicRootCACerts()
		require.Len(t, certs, 1)
		require.Equal(t, pair.ASN1(), certs[0].Raw)

		// already exists
		cp.PublicRootCACerts = append(cp.PublicRootCACerts, pair.ASN1())
		_, err = NewPoolFromNBCertPool(cp)
		require.Error(t, err)

		cp.PublicRootCACerts = [][]byte{pair.ASN1()}
	})

	t.Run("public client ca cert", func(t *testing.T) {
		pair := testGenerateCert(t)
		cp.PublicClientCACerts = [][]byte{pair.ASN1()}
		pool, err := NewPoolFromNBCertPool(cp)
		require.NoError(t, err)
		certs := pool.GetPublicClientCACerts()
		require.Len(t, certs, 1)
		require.Equal(t, pair.ASN1(), certs[0].Raw)

		// already exists
		cp.PublicClientCACerts = append(cp.PublicClientCACerts, pair.ASN1())
		_, err = NewPoolFromNBCertPool(cp)
		require.Error(t, err)

		cp.PublicClientCACerts = [][]byte{pair.ASN1()}
	})

	t.Run("public client cert", func(t *testing.T) {
		pair := testGenerateCert(t)
		c, k := pair.Encode()
		cp.PublicClientPairs = []struct {
			Cert []byte `msgpack:"a"`
			Key  []byte `msgpack:"b"`
		}{
			{Cert: c, Key: k},
		}

		pool, err := NewPoolFromNBCertPool(cp)
		require.NoError(t, err)
		certs := pool.GetPublicClientPairs()
		require.Len(t, certs, 1)
		dCert, dKey := certs[0].Encode()
		require.Equal(t, c, dCert)
		require.Equal(t, k, dKey)

		// already exists
		cp.PublicClientPairs = append(cp.PublicClientPairs, struct {
			Cert []byte `msgpack:"a"`
			Key  []byte `msgpack:"b"`
		}{
			Cert: c, Key: k,
		})
		_, err = NewPoolFromNBCertPool(cp)
		require.Error(t, err)

		cp.PublicClientPairs = []struct {
			Cert []byte `msgpack:"a"`
			Key  []byte `msgpack:"b"`
		}{
			{Cert: c, Key: k},
		}
	})

	t.Run("private root ca cert", func(t *testing.T) {
		pair := testGenerateCert(t)
		cp.PrivateRootCACerts = [][]byte{pair.ASN1()}
		pool, err := NewPoolFromNBCertPool(cp)
		require.NoError(t, err)
		certs := pool.GetPrivateRootCACerts()
		require.Len(t, certs, 1)
		require.Equal(t, pair.ASN1(), certs[0].Raw)

		// already exists
		cp.PrivateRootCACerts = append(cp.PrivateRootCACerts, pair.ASN1())
		_, err = NewPoolFromNBCertPool(cp)
		require.Error(t, err)

		cp.PrivateRootCACerts = [][]byte{pair.ASN1()}
	})

	t.Run("private client ca cert", func(t *testing.T) {
		pair := testGenerateCert(t)
		cp.PrivateClientCACerts = [][]byte{pair.ASN1()}
		pool, err := NewPoolFromNBCertPool(cp)
		require.NoError(t, err)
		certs := pool.GetPrivateClientCACerts()
		require.Len(t, certs, 1)
		require.Equal(t, pair.ASN1(), certs[0].Raw)

		// already exists
		cp.PrivateClientCACerts = append(cp.PrivateClientCACerts, pair.ASN1())
		_, err = NewPoolFromNBCertPool(cp)
		require.Error(t, err)

		cp.PrivateClientCACerts = [][]byte{pair.ASN1()}
	})

	t.Run("private client cert", func(t *testing.T) {
		pair := testGenerateCert(t)
		c, k := pair.Encode()
		cp.PrivateClientPairs = []struct {
			Cert []byte `msgpack:"a"`
			Key  []byte `msgpack:"b"`
		}{
			{Cert: c, Key: k},
		}

		pool, err := NewPoolFromNBCertPool(cp)
		require.NoError(t, err)
		certs := pool.GetPrivateClientPairs()
		require.Len(t, certs, 1)
		dCert, dKey := certs[0].Encode()
		require.Equal(t, c, dCert)
		require.Equal(t, k, dKey)

		// already exists
		cp.PrivateClientPairs = append(cp.PrivateClientPairs, struct {
			Cert []byte `msgpack:"a"`
			Key  []byte `msgpack:"b"`
		}{
			Cert: c, Key: k,
		})
		_, err = NewPoolFromNBCertPool(cp)
		require.Error(t, err)

		cp.PrivateClientPairs = []struct {
			Cert []byte `msgpack:"a"`
			Key  []byte `msgpack:"b"`
		}{
			{Cert: c, Key: k},
		}
	})
}
