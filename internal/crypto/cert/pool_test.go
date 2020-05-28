package cert

import (
	"crypto/x509"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/cert/certpool"
	"project/internal/patch/monkey"
	"project/internal/security"
	"project/internal/testsuite"
)

func TestPair_ToPair(t *testing.T) {
	defer testsuite.DeferForPanic(t)
	pair := pair{
		PrivateKey: security.NewBytes(make([]byte, 1024)),
	}
	pair.ToPair()
}

func testGenerateCert(t *testing.T) *Pair {
	pair, err := GenerateCA(nil)
	require.NoError(t, err)
	return pair
}

func TestLoadCertWithPrivateKey(t *testing.T) {
	t.Run("no private key", func(t *testing.T) {
		pair := testGenerateCert(t)
		cert, _ := pair.Encode()

		_, err := loadCertWithPrivateKey(cert, nil)
		require.NoError(t, err)
	})

	t.Run("invalid certificate", func(t *testing.T) {
		_, err := loadCertWithPrivateKey(make([]byte, 1024), nil)
		require.Error(t, err)
	})

	t.Run("invalid private key", func(t *testing.T) {
		pair := testGenerateCert(t)
		cert, _ := pair.Encode()

		_, err := loadCertWithPrivateKey(cert, make([]byte, 1024))
		require.Error(t, err)
	})

	t.Run("mismatched private key", func(t *testing.T) {
		pair1 := testGenerateCert(t)
		cert := pair1.ASN1()

		pair2 := testGenerateCert(t)
		_, key := pair2.Encode()

		_, err := loadCertWithPrivateKey(cert, key)
		require.Error(t, err)
	})

	t.Run("MarshalPKCS8PrivateKey", func(t *testing.T) {
		pair := testGenerateCert(t)
		cert, key := pair.Encode() // must before patch

		patch := func(interface{}) ([]byte, error) {
			return nil, monkey.Error
		}
		pg := monkey.Patch(x509.MarshalPKCS8PrivateKey, patch)
		defer pg.Unpatch()

		_, err := loadCertWithPrivateKey(cert, key)
		monkey.IsMonkeyError(t, err)
	})
}

func TestPool(t *testing.T) {
	pool := NewPool()

	t.Run("PublicRootCACert", func(t *testing.T) {
		require.Len(t, pool.GetPublicRootCACerts(), 0)

		pair := testGenerateCert(t)

		t.Run("add", func(t *testing.T) {
			err := pool.AddPublicRootCACert(pair.Certificate.Raw)
			require.NoError(t, err)
			err = pool.AddPublicRootCACert(pair.Certificate.Raw)
			require.Error(t, err)
			err = pool.AddPublicRootCACert(make([]byte, 1024))
			require.Error(t, err)
		})

		t.Run("get", func(t *testing.T) {
			certs := pool.GetPublicRootCACerts()
			require.True(t, certs[0].Equal(pair.Certificate))
		})

		t.Run("delete", func(t *testing.T) {
			err := pool.DeletePublicRootCACert(0)
			require.NoError(t, err)
			err = pool.DeletePublicRootCACert(0)
			require.Error(t, err)
		})

		t.Run("add parallel", func(t *testing.T) {
			var pool *Pool
			pair1 := testGenerateCert(t)
			pair2 := testGenerateCert(t)

			init := func() {
				pool = NewPool()
			}
			add1 := func() {
				err := pool.AddPublicRootCACert(pair1.Certificate.Raw)
				require.NoError(t, err)
			}
			add2 := func() {
				err := pool.AddPublicRootCACert(pair2.Certificate.Raw)
				require.NoError(t, err)
			}
			get := func() {
				pool.GetPublicRootCACerts()
			}
			testsuite.RunParallel(100, init, nil, add1, add2, get)
		})

		t.Run("delete parallel", func(t *testing.T) {
			var pool *Pool
			pair1 := testGenerateCert(t)
			pair2 := testGenerateCert(t)

			init := func() {
				pool = NewPool()

				err := pool.AddPublicRootCACert(pair1.Certificate.Raw)
				require.NoError(t, err)
				err = pool.AddPublicRootCACert(pair2.Certificate.Raw)
				require.NoError(t, err)
			}
			del := func() {
				err := pool.DeletePublicRootCACert(0)
				require.NoError(t, err)
			}
			get := func() {
				pool.GetPublicRootCACerts()
			}
			testsuite.RunParallel(100, init, nil, del, del, get)
		})
	})

	t.Run("PublicClientCACert", func(t *testing.T) {
		require.Len(t, pool.GetPublicClientCACerts(), 0)

		pair := testGenerateCert(t)

		t.Run("add", func(t *testing.T) {
			err := pool.AddPublicClientCACert(pair.Certificate.Raw)
			require.NoError(t, err)
			err = pool.AddPublicClientCACert(pair.Certificate.Raw)
			require.Error(t, err)
			err = pool.AddPublicClientCACert(make([]byte, 1024))
			require.Error(t, err)
		})

		t.Run("get", func(t *testing.T) {
			certs := pool.GetPublicClientCACerts()
			require.True(t, certs[0].Equal(pair.Certificate))
		})

		t.Run("delete", func(t *testing.T) {
			err := pool.DeletePublicClientCACert(0)
			require.NoError(t, err)
			err = pool.DeletePublicClientCACert(0)
			require.Error(t, err)
		})

		t.Run("add parallel", func(t *testing.T) {
			var pool *Pool
			pair1 := testGenerateCert(t)
			pair2 := testGenerateCert(t)

			init := func() {
				pool = NewPool()
			}
			add1 := func() {
				err := pool.AddPublicClientCACert(pair1.Certificate.Raw)
				require.NoError(t, err)
			}
			add2 := func() {
				err := pool.AddPublicClientCACert(pair2.Certificate.Raw)
				require.NoError(t, err)
			}
			get := func() {
				pool.GetPublicClientCACerts()
			}
			testsuite.RunParallel(100, init, nil, add1, add2, get)
		})

		t.Run("delete parallel", func(t *testing.T) {
			var pool *Pool
			pair1 := testGenerateCert(t)
			pair2 := testGenerateCert(t)

			init := func() {
				pool = NewPool()

				err := pool.AddPublicClientCACert(pair1.Certificate.Raw)
				require.NoError(t, err)
				err = pool.AddPublicClientCACert(pair2.Certificate.Raw)
				require.NoError(t, err)
			}
			del := func() {
				err := pool.DeletePublicClientCACert(0)
				require.NoError(t, err)
			}
			get := func() {
				pool.GetPublicClientCACerts()
			}
			testsuite.RunParallel(100, init, nil, del, del, get)
		})
	})

	t.Run("PublicClientCert", func(t *testing.T) {
		require.Len(t, pool.GetPublicClientPairs(), 0)

		pair := testGenerateCert(t)

		t.Run("add", func(t *testing.T) {
			cert, key := pair.Encode()
			err := pool.AddPublicClientCert(cert, key)
			require.NoError(t, err)
			err = pool.AddPublicClientCert(cert, key)
			require.Error(t, err)
			err = pool.AddPublicClientCert(cert, nil)
			require.Error(t, err)

			// loadCertWithPrivateKey
			err = pool.AddPublicClientCert(nil, nil)
			require.Error(t, err)
		})

		t.Run("get", func(t *testing.T) {
			pairs := pool.GetPublicClientPairs()
			require.Equal(t, pair, pairs[0])
		})

		t.Run("delete", func(t *testing.T) {
			err := pool.DeletePublicClientCert(0)
			require.NoError(t, err)
			err = pool.DeletePublicClientCert(0)
			require.Error(t, err)
		})

		t.Run("add parallel", func(t *testing.T) {
			var pool *Pool
			pair1 := testGenerateCert(t)
			pair2 := testGenerateCert(t)

			init := func() {
				pool = NewPool()
			}
			add1 := func() {
				err := pool.AddPublicClientCert(pair1.Encode())
				require.NoError(t, err)
			}
			add2 := func() {
				err := pool.AddPublicClientCert(pair2.Encode())
				require.NoError(t, err)
			}
			get := func() {
				pool.GetPublicClientPairs()
			}
			testsuite.RunParallel(100, init, nil, add1, add2, get)
		})

		t.Run("delete parallel", func(t *testing.T) {
			var pool *Pool
			pair1 := testGenerateCert(t)
			pair2 := testGenerateCert(t)

			init := func() {
				pool = NewPool()

				err := pool.AddPublicClientCert(pair1.Encode())
				require.NoError(t, err)
				err = pool.AddPublicClientCert(pair2.Encode())
				require.NoError(t, err)
			}
			del := func() {
				err := pool.DeletePublicClientCert(0)
				require.NoError(t, err)
			}
			get := func() {
				pool.GetPublicClientPairs()
			}
			testsuite.RunParallel(100, init, nil, del, del, get)
		})
	})

	t.Run("PrivateRootCACert", func(t *testing.T) {
		require.Len(t, pool.GetPrivateRootCACerts(), 0)

		pair := testGenerateCert(t)

		t.Run("add", func(t *testing.T) {
			cert, key := pair.Encode()
			err := pool.AddPrivateRootCACert(cert, key)
			require.NoError(t, err)
			err = pool.AddPrivateRootCACert(cert, key)
			require.Error(t, err)
			err = pool.AddPrivateRootCACert(cert, []byte{})
			require.Error(t, err)

			// loadCertWithPrivateKey
			err = pool.AddPrivateRootCACert(nil, nil)
			require.Error(t, err)
		})

		t.Run("get certs", func(t *testing.T) {
			certs := pool.GetPrivateRootCACerts()
			require.True(t, certs[0].Equal(pair.Certificate))
		})

		t.Run("get pairs", func(t *testing.T) {
			pairs := pool.GetPrivateRootCAPairs()
			require.Equal(t, pair, pairs[0])
		})

		t.Run("delete", func(t *testing.T) {
			err := pool.DeletePrivateRootCACert(0)
			require.NoError(t, err)
			err = pool.DeletePrivateRootCACert(0)
			require.Error(t, err)
		})

		t.Run("add parallel", func(t *testing.T) {
			var pool *Pool
			pair1 := testGenerateCert(t)
			pair2 := testGenerateCert(t)

			init := func() {
				pool = NewPool()
			}
			add1 := func() {
				err := pool.AddPrivateRootCACert(pair1.Encode())
				require.NoError(t, err)
			}
			add2 := func() {
				err := pool.AddPrivateRootCACert(pair2.Encode())
				require.NoError(t, err)
			}
			get1 := func() {
				pool.GetPrivateRootCACerts()
			}
			get2 := func() {
				pool.GetPrivateRootCAPairs()
			}
			testsuite.RunParallel(100, init, nil, add1, add2, get1, get2)
		})

		t.Run("delete parallel", func(t *testing.T) {
			var pool *Pool
			pair1 := testGenerateCert(t)
			pair2 := testGenerateCert(t)

			init := func() {
				pool = NewPool()

				err := pool.AddPrivateRootCACert(pair1.Encode())
				require.NoError(t, err)
				err = pool.AddPrivateRootCACert(pair2.Encode())
				require.NoError(t, err)
			}
			del := func() {
				err := pool.DeletePrivateRootCACert(0)
				require.NoError(t, err)
			}
			get1 := func() {
				pool.GetPrivateRootCACerts()
			}
			get2 := func() {
				pool.GetPrivateRootCAPairs()
			}
			testsuite.RunParallel(100, init, nil, del, del, get1, get2)
		})
	})

	t.Run("PrivateClientCACert", func(t *testing.T) {
		require.Len(t, pool.GetPrivateClientCACerts(), 0)

		pair := testGenerateCert(t)

		t.Run("add", func(t *testing.T) {
			cert, key := pair.Encode()
			err := pool.AddPrivateClientCACert(cert, key)
			require.NoError(t, err)
			err = pool.AddPrivateClientCACert(cert, key)
			require.Error(t, err)
			err = pool.AddPrivateClientCACert(cert, []byte{})
			require.Error(t, err)

			// loadCertWithPrivateKey
			err = pool.AddPrivateClientCACert(nil, nil)
			require.Error(t, err)
		})

		t.Run("get certs", func(t *testing.T) {
			certs := pool.GetPrivateClientCACerts()
			require.True(t, certs[0].Equal(pair.Certificate))
		})

		t.Run("get pairs", func(t *testing.T) {
			pairs := pool.GetPrivateClientCAPairs()
			require.Equal(t, pair, pairs[0])
		})

		t.Run("delete", func(t *testing.T) {
			err := pool.DeletePrivateClientCACert(0)
			require.NoError(t, err)
			err = pool.DeletePrivateClientCACert(0)
			require.Error(t, err)
		})

		t.Run("add parallel", func(t *testing.T) {
			var pool *Pool
			pair1 := testGenerateCert(t)
			pair2 := testGenerateCert(t)

			init := func() {
				pool = NewPool()
			}
			add1 := func() {
				err := pool.AddPrivateClientCACert(pair1.Encode())
				require.NoError(t, err)
			}
			add2 := func() {
				err := pool.AddPrivateClientCACert(pair2.Encode())
				require.NoError(t, err)
			}
			get1 := func() {
				pool.GetPrivateClientCACerts()
			}
			get2 := func() {
				pool.GetPrivateClientCAPairs()
			}
			testsuite.RunParallel(100, init, nil, add1, add2, get1, get2)
		})

		t.Run("delete parallel", func(t *testing.T) {
			var pool *Pool
			pair1 := testGenerateCert(t)
			pair2 := testGenerateCert(t)

			init := func() {
				pool = NewPool()

				err := pool.AddPrivateClientCACert(pair1.Encode())
				require.NoError(t, err)
				err = pool.AddPrivateClientCACert(pair2.Encode())
				require.NoError(t, err)
			}
			del := func() {
				err := pool.DeletePrivateClientCACert(0)
				require.NoError(t, err)
			}
			get1 := func() {
				pool.GetPrivateClientCACerts()
			}
			get2 := func() {
				pool.GetPrivateClientCAPairs()
			}
			testsuite.RunParallel(100, init, nil, del, del, get1, get2)
		})
	})

	t.Run("PrivateClientCert", func(t *testing.T) {
		require.Len(t, pool.GetPrivateClientPairs(), 0)

		pair := testGenerateCert(t)

		t.Run("add", func(t *testing.T) {
			cert, key := pair.Encode()
			err := pool.AddPrivateClientCert(cert, key)
			require.NoError(t, err)
			err = pool.AddPrivateClientCert(cert, key)
			require.Error(t, err)
			err = pool.AddPrivateClientCert(cert, []byte{})
			require.Error(t, err)

			// loadCertWithPrivateKey
			err = pool.AddPrivateClientCert(nil, nil)
			require.Error(t, err)
		})

		t.Run("get", func(t *testing.T) {
			pairs := pool.GetPrivateClientPairs()
			require.Equal(t, pair, pairs[0])
		})

		t.Run("delete", func(t *testing.T) {
			err := pool.DeletePrivateClientCert(0)
			require.NoError(t, err)
			err = pool.DeletePrivateClientCert(0)
			require.Error(t, err)
		})

		t.Run("add parallel", func(t *testing.T) {
			var pool *Pool
			pair1 := testGenerateCert(t)
			pair2 := testGenerateCert(t)

			init := func() {
				pool = NewPool()
			}
			add1 := func() {
				err := pool.AddPrivateClientCert(pair1.Encode())
				require.NoError(t, err)
			}
			add2 := func() {
				err := pool.AddPrivateClientCert(pair2.Encode())
				require.NoError(t, err)
			}
			get := func() {
				pool.GetPrivateClientPairs()
			}
			testsuite.RunParallel(100, init, nil, add1, add2, get)
		})

		t.Run("delete parallel", func(t *testing.T) {
			var pool *Pool
			pair1 := testGenerateCert(t)
			pair2 := testGenerateCert(t)

			init := func() {
				pool = NewPool()

				err := pool.AddPrivateClientCert(pair1.Encode())
				require.NoError(t, err)
				err = pool.AddPrivateClientCert(pair2.Encode())
				require.NoError(t, err)
			}
			del := func() {
				err := pool.DeletePrivateClientCert(0)
				require.NoError(t, err)
			}
			get := func() {
				pool.GetPrivateClientPairs()
			}
			testsuite.RunParallel(100, init, nil, del, del, get)
		})
	})
}

func TestNewPoolWithSystemCerts(t *testing.T) {
	t.Run("common", func(t *testing.T) {
		_, err := NewPoolWithSystemCerts()
		require.NoError(t, err)
	})

	t.Run("failed to call SystemCertPool", func(t *testing.T) {
		patch := func() (*x509.CertPool, error) {
			return nil, monkey.Error
		}
		pg := monkey.Patch(certpool.System, patch)
		defer pg.Unpatch()

		_, err := NewPoolWithSystemCerts()
		monkey.IsMonkeyError(t, err)
	})

	t.Run("failed to AddPublicRootCACert", func(t *testing.T) {
		pool := NewPool()
		patch := func(*Pool, []byte) error {
			return monkey.Error
		}
		pg := monkey.PatchInstanceMethod(pool, "AddPublicRootCACert", patch)
		defer pg.Unpatch()

		_, err := NewPoolWithSystemCerts()
		monkey.IsMonkeyError(t, err)
	})
}
