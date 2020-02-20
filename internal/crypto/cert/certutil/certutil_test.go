package certutil

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"io/ioutil"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseCertificates(t *testing.T) {
	certPEMBlock, err := ioutil.ReadFile("testdata/certs.pem")
	require.NoError(t, err)
	certs, err := ParseCertificates(certPEMBlock)
	require.NoError(t, err)
	t.Log(certs[0].Issuer)
	t.Log(certs[1].Issuer)

	// parse invalid PEM data
	_, err = ParseCertificates([]byte{0, 1, 2, 3})
	require.Equal(t, ErrInvalidPEMBlock, err)

	// invalid Type
	certPEMBlock = []byte(`
-----BEGIN INVALID TYPE-----
-----END INVALID TYPE-----
`)
	_, err = ParseCertificates(certPEMBlock)
	require.EqualError(t, err, "invalid PEM block type: INVALID TYPE")

	// invalid certificate data
	certPEMBlock = []byte(`
-----BEGIN CERTIFICATE-----
-----END CERTIFICATE-----
`)
	_, err = ParseCertificates(certPEMBlock)
	require.Error(t, err)
}

func TestParseCertificate(t *testing.T) {
	certPEMBlock, err := ioutil.ReadFile("testdata/certs.pem")
	require.NoError(t, err)
	cert, err := ParseCertificate(certPEMBlock)
	require.NoError(t, err)
	t.Log(cert.Issuer)

	// parse invalid PEM data
	_, err = ParseCertificate([]byte{0, 1, 2, 3})
	require.Equal(t, ErrInvalidPEMBlock, err)

	// invalid Type
	certPEMBlock = []byte(`
-----BEGIN INVALID TYPE-----
-----END INVALID TYPE-----
`)
	_, err = ParseCertificate(certPEMBlock)
	require.EqualError(t, err, "invalid PEM block type: INVALID TYPE")

	// invalid certificate data
	certPEMBlock = []byte(`
-----BEGIN CERTIFICATE-----
-----END CERTIFICATE-----
`)
	_, err = ParseCertificate(certPEMBlock)
	require.Error(t, err)
}

func TestParsePrivateKey(t *testing.T) {
	for _, file := range []string{"pkcs1.key", "pkcs8.key", "ecp.key"} {
		keyPEMBlock, err := ioutil.ReadFile("testdata/" + file)
		require.NoError(t, err)
		_, err = ParsePrivateKey(keyPEMBlock)
		require.NoError(t, err)
	}

	// parse invalid PEM data
	_, err := ParsePrivateKey([]byte{0, 1, 2, 3})
	require.Equal(t, ErrInvalidPEMBlock, err)

	// invalid certificate data
	keyPEMBlock := []byte(`
-----BEGIN PRIVATE KEY-----
-----END PRIVATE KEY-----
`)
	_, err = ParsePrivateKey(keyPEMBlock)
	require.Error(t, err)
}

func TestMatch(t *testing.T) {
	cert := new(x509.Certificate)

	t.Run("rsa", func(t *testing.T) {
		t.Run("matched", func(t *testing.T) {
			pri, err := rsa.GenerateKey(rand.Reader, 2048)
			require.NoError(t, err)
			cert.PublicKey = &pri.PublicKey
			require.True(t, Match(cert, pri))
		})

		t.Run("mismatch", func(t *testing.T) {
			pri, err := rsa.GenerateKey(rand.Reader, 2048)
			require.NoError(t, err)
			cert.PublicKey = &pri.PublicKey
			require.False(t, Match(cert, nil))

			pri2, err := rsa.GenerateKey(rand.Reader, 2048)
			require.NoError(t, err)
			require.False(t, Match(cert, pri2))
		})
	})

	t.Run("ecdsa", func(t *testing.T) {
		t.Run("matched", func(t *testing.T) {
			pri, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			require.NoError(t, err)
			cert.PublicKey = &pri.PublicKey
			require.True(t, Match(cert, pri))
		})

		t.Run("mismatch", func(t *testing.T) {
			pri, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			require.NoError(t, err)
			cert.PublicKey = &pri.PublicKey
			require.False(t, Match(cert, nil))

			pri2, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			require.NoError(t, err)
			require.False(t, Match(cert, pri2))
		})
	})

	t.Run("ed25519", func(t *testing.T) {
		t.Run("matched", func(t *testing.T) {
			pub, pri, err := ed25519.GenerateKey(rand.Reader)
			require.NoError(t, err)
			cert.PublicKey = pub
			require.True(t, Match(cert, pri))
		})

		t.Run("mismatched", func(t *testing.T) {
			pub, _, err := ed25519.GenerateKey(rand.Reader)
			require.NoError(t, err)
			cert.PublicKey = pub
			require.False(t, Match(cert, nil))

			_, pri, err := ed25519.GenerateKey(rand.Reader)
			require.NoError(t, err)
			require.False(t, Match(cert, pri))
		})
	})

	t.Run("unknown", func(t *testing.T) {
		cert.PublicKey = []byte{}
		require.False(t, Match(cert, nil))
	})
}

func TestSystemCertPool(t *testing.T) {
	wg := sync.WaitGroup{}
	wg.Add(5)
	for i := 0; i < 5; i++ {
		go func() {
			defer wg.Done()
			pool, err := SystemCertPool()
			require.NoError(t, err)
			t.Log("the number of the system certificates:", len(pool.Subjects()))

			for _, sub := range pool.Subjects() {
				t.Log(string(sub))
			}
		}()
	}
	wg.Wait()
}
