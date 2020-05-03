package certmgr

import (
	"bytes"
	"compress/flate"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"project/internal/crypto/aes"
	"project/internal/crypto/cert"
	"project/internal/patch/monkey"
	"project/internal/patch/msgpack"
	"project/internal/system"
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

	pool := testGenerateCertPool(t)

	t.Run("invalid structure", func(t *testing.T) {
		patch := func(interface{}) ([]byte, error) {
			return nil, monkey.Error
		}
		pg := monkey.Patch(msgpack.Marshal, patch)
		defer pg.Unpatch()

		err := SaveCtrlCertPool(pool, testPassword)
		monkey.IsMonkeyError(t, err)
	})

	t.Run("failed to NewWriter", func(t *testing.T) {
		patch := func(io.Writer, int) (*flate.Writer, error) {
			return nil, monkey.Error
		}
		pg := monkey.Patch(flate.NewWriter, patch)
		defer pg.Unpatch()

		err := SaveCtrlCertPool(pool, testPassword)
		monkey.IsMonkeyError(t, errors.Cause(err))
	})

	t.Run("failed to write about compress", func(t *testing.T) {
		writer := new(flate.Writer)
		patch := func(interface{}, []byte) (int, error) {
			return 0, monkey.Error
		}
		pg := monkey.PatchInstanceMethod(writer, "Write", patch)
		defer pg.Unpatch()

		err := SaveCtrlCertPool(pool, testPassword)
		monkey.IsMonkeyError(t, errors.Cause(err))
	})

	t.Run("failed to close about compress", func(t *testing.T) {
		writer := new(flate.Writer)
		patch := func(interface{}) error {
			return monkey.Error
		}
		pg := monkey.PatchInstanceMethod(writer, "Close", patch)
		defer pg.Unpatch()

		err := SaveCtrlCertPool(pool, testPassword)
		monkey.IsMonkeyError(t, errors.Cause(err))
	})

	t.Run("failed to encrypt data", func(t *testing.T) {
		patch := func([]byte, []byte, []byte) ([]byte, error) {
			return nil, monkey.Error
		}
		pg := monkey.Patch(aes.CBCEncrypt, patch)
		defer pg.Unpatch()

		err := SaveCtrlCertPool(pool, testPassword)
		monkey.IsMonkeyError(t, errors.Cause(err))
	})

	t.Run("failed to write file", func(t *testing.T) {
		patch := func(string, []byte) error {
			return monkey.Error
		}
		pg := monkey.Patch(system.WriteFile, patch)
		defer pg.Unpatch()

		err := SaveCtrlCertPool(pool, testPassword)
		monkey.IsMonkeyError(t, errors.Cause(err))
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

	pool := cert.NewPool()

	t.Run("invalid cipher data", func(t *testing.T) {
		err := LoadCtrlCertPool(pool, []byte("foo"), nil, testPassword)
		require.Error(t, err)
	})

	t.Run("invalid compressed data", func(t *testing.T) {
		aesKey, aesIV := calculateAESKeyFromPassword(testPassword)
		data := bytes.Repeat([]byte{16}, 32)
		cipherData, err := aes.CBCEncrypt(data, aesKey, aesIV)
		require.NoError(t, err)

		err = LoadCtrlCertPool(pool, cipherData, nil, testPassword)
		require.Error(t, err)
	})

	pool = testGenerateCertPool(t)
	err := SaveCtrlCertPool(pool, testPassword)
	require.NoError(t, err)
	defer testRemoveKeyFile(t)
	certsData, hashData := testReadKeyFile(t)

	t.Run("failed to close deflate reader", func(t *testing.T) {
		reader := flate.NewReader(nil)
		patch := func(interface{}) error {
			return monkey.Error
		}
		pg := monkey.PatchInstanceMethod(reader, "Close", patch)
		defer pg.Unpatch()

		err := LoadCtrlCertPool(pool, certsData, hashData, testPassword)
		require.Error(t, err)
	})

	t.Run("invalid hash", func(t *testing.T) {
		err := LoadCtrlCertPool(pool, certsData, nil, testPassword)
		require.Error(t, err)
	})

	t.Run("failed to unmarshal", func(t *testing.T) {
		patch := func([]byte, interface{}) error {
			return monkey.Error
		}
		pg := monkey.Patch(msgpack.Unmarshal, patch)
		defer pg.Unpatch()

		err := LoadCtrlCertPool(pool, certsData, hashData, testPassword)
		monkey.IsMonkeyError(t, err)
	})
}

func TestAddCertsToPool(t *testing.T) {
	invalidCert := []byte("foo")
	invalidPair := struct {
		Cert []byte `msgpack:"a"`
		Key  []byte `msgpack:"b"`
	}{
		Cert: []byte("foo"),
		Key:  []byte("bar"),
	}

	pool := cert.NewPool()
	cp := new(ctrlCertPool)

	cp.PublicRootCACerts = [][]byte{invalidCert}
	err := addCertsToPool(pool, cp)
	require.Error(t, err)
	cp.PublicRootCACerts = nil

	cp.PublicClientCACerts = [][]byte{invalidCert}
	err = addCertsToPool(pool, cp)
	require.Error(t, err)
	cp.PublicClientCACerts = nil

	cp.PublicClientPairs = []struct {
		Cert []byte `msgpack:"a"`
		Key  []byte `msgpack:"b"`
	}{invalidPair}
	err = addCertsToPool(pool, cp)
	require.Error(t, err)
	cp.PublicClientPairs = nil

	cp.PrivateRootCAPairs = []struct {
		Cert []byte `msgpack:"a"`
		Key  []byte `msgpack:"b"`
	}{invalidPair}
	err = addCertsToPool(pool, cp)
	require.Error(t, err)
	cp.PrivateRootCAPairs = nil

	cp.PrivateClientCAPairs = []struct {
		Cert []byte `msgpack:"a"`
		Key  []byte `msgpack:"b"`
	}{invalidPair}
	err = addCertsToPool(pool, cp)
	require.Error(t, err)
	cp.PrivateClientCAPairs = nil

	cp.PrivateClientPairs = []struct {
		Cert []byte `msgpack:"a"`
		Key  []byte `msgpack:"b"`
	}{invalidPair}
	err = addCertsToPool(pool, cp)
	require.Error(t, err)
	cp.PrivateClientPairs = nil
}

func testGenerateCert(t *testing.T) *cert.Pair {
	pair, err := cert.GenerateCA(nil)
	require.NoError(t, err)
	return pair
}

func TestNBCertPool_GetCertsFromPool(t *testing.T) {
	pair := testGenerateCert(t)
	c, k := pair.Encode()

	pool := cert.NewPool()

	err := pool.AddPublicRootCACert(c)
	require.NoError(t, err)
	err = pool.AddPublicClientCACert(c)
	require.NoError(t, err)
	err = pool.AddPublicClientCert(c, k)
	require.NoError(t, err)
	err = pool.AddPrivateRootCACert(c, k)
	require.NoError(t, err)
	err = pool.AddPrivateClientCACert(c, k)
	require.NoError(t, err)
	err = pool.AddPrivateClientCert(c, k)
	require.NoError(t, err)

	cp := new(NBCertPool)
	cp.GetCertsFromPool(pool)

	require.Len(t, cp.PublicRootCACerts, 1)
	require.Len(t, cp.PublicClientCACerts, 1)
	require.Len(t, cp.PublicClientPairs, 1)
	require.Len(t, cp.PrivateRootCACerts, 1)
	require.Len(t, cp.PrivateClientCACerts, 1)
	require.Len(t, cp.PrivateClientPairs, 1)
}

func TestNBCertPool_ToPool(t *testing.T) {
	cp := new(NBCertPool)

	t.Run("public root ca cert", func(t *testing.T) {
		pair := testGenerateCert(t)
		cp.PublicRootCACerts = [][]byte{pair.ASN1()}
		pool, err := cp.ToPool()
		require.NoError(t, err)
		certs := pool.GetPublicRootCACerts()
		require.Len(t, certs, 1)
		require.Equal(t, pair.ASN1(), certs[0].Raw)

		// already exists
		cp.PublicRootCACerts = append(cp.PublicRootCACerts, pair.ASN1())
		_, err = cp.ToPool()
		require.Error(t, err)

		cp.PublicRootCACerts = [][]byte{pair.ASN1()}
	})

	t.Run("public client ca cert", func(t *testing.T) {
		pair := testGenerateCert(t)
		cp.PublicClientCACerts = [][]byte{pair.ASN1()}
		pool, err := cp.ToPool()
		require.NoError(t, err)
		certs := pool.GetPublicClientCACerts()
		require.Len(t, certs, 1)
		require.Equal(t, pair.ASN1(), certs[0].Raw)

		// already exists
		cp.PublicClientCACerts = append(cp.PublicClientCACerts, pair.ASN1())
		_, err = cp.ToPool()
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

		pool, err := cp.ToPool()
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
		_, err = cp.ToPool()
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
		pool, err := cp.ToPool()
		require.NoError(t, err)
		certs := pool.GetPrivateRootCACerts()
		require.Len(t, certs, 1)
		require.Equal(t, pair.ASN1(), certs[0].Raw)

		// already exists
		cp.PrivateRootCACerts = append(cp.PrivateRootCACerts, pair.ASN1())
		_, err = cp.ToPool()
		require.Error(t, err)

		cp.PrivateRootCACerts = [][]byte{pair.ASN1()}
	})

	t.Run("private client ca cert", func(t *testing.T) {
		pair := testGenerateCert(t)
		cp.PrivateClientCACerts = [][]byte{pair.ASN1()}
		pool, err := cp.ToPool()
		require.NoError(t, err)
		certs := pool.GetPrivateClientCACerts()
		require.Len(t, certs, 1)
		require.Equal(t, pair.ASN1(), certs[0].Raw)

		// already exists
		cp.PrivateClientCACerts = append(cp.PrivateClientCACerts, pair.ASN1())
		_, err = cp.ToPool()
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

		pool, err := cp.ToPool()
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
		_, err = cp.ToPool()
		require.Error(t, err)

		cp.PrivateClientPairs = []struct {
			Cert []byte `msgpack:"a"`
			Key  []byte `msgpack:"b"`
		}{
			{Cert: c, Key: k},
		}
	})
}
