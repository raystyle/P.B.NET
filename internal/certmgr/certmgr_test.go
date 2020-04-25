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
