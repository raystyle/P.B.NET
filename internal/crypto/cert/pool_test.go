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

func TestLoadPair(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		pair := testGenerateCert(t)

		p, err := loadPair(pair.Encode())
		require.NoError(t, err)
		require.NotNil(t, p)
	})

	t.Run("no certificate", func(t *testing.T) {
		_, err := loadPair(nil, nil)
		require.Error(t, err)
	})

	t.Run("no private key", func(t *testing.T) {
		_, err := loadPair(make([]byte, 1024), nil)
		require.Error(t, err)
	})

	t.Run("invalid certificate", func(t *testing.T) {
		padding := make([]byte, 1024)
		_, err := loadPair(padding, padding)
		require.Error(t, err)
	})

	t.Run("invalid private key", func(t *testing.T) {
		pair := testGenerateCert(t)
		cert, _ := pair.Encode()

		_, err := loadPair(cert, make([]byte, 1024))
		require.Error(t, err)
	})

	t.Run("mismatched private key", func(t *testing.T) {
		pair1 := testGenerateCert(t)
		cert := pair1.ASN1()

		pair2 := testGenerateCert(t)
		_, key := pair2.Encode()

		_, err := loadPair(cert, key)
		require.Error(t, err)
	})

	t.Run("failed to marshal PKCS8 private key", func(t *testing.T) {
		// must before patch
		pair := testGenerateCert(t)
		cert, key := pair.Encode()

		patch := func(interface{}) ([]byte, error) {
			return nil, monkey.Error
		}
		pg := monkey.Patch(x509.MarshalPKCS8PrivateKey, patch)
		defer pg.Unpatch()

		_, err := loadPair(cert, key)
		monkey.IsMonkeyError(t, err)
	})
}

func TestLoadCertToPair(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		pair := testGenerateCert(t)

		p, err := loadCertToPair(pair.ASN1())
		require.NoError(t, err)
		require.NotNil(t, p)
		require.Nil(t, p.PrivateKey)
	})

	t.Run("no certificate", func(t *testing.T) {
		_, err := loadCertToPair(nil)
		require.Error(t, err)
	})

	t.Run("invalid certificate", func(t *testing.T) {
		_, err := loadCertToPair(make([]byte, 1024))
		require.Error(t, err)
	})
}

func TestPool_PublicRootCACert(t *testing.T) {
	pool := NewPool()

	certs := pool.GetPublicRootCACerts()
	require.Empty(t, certs)

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
		gm := testsuite.MarkGoroutines(t)
		defer gm.Compare()

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

		testsuite.IsDestroyed(t, pool)
	})

	t.Run("delete parallel", func(t *testing.T) {
		gm := testsuite.MarkGoroutines(t)
		defer gm.Compare()

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

		testsuite.IsDestroyed(t, pool)
	})

	testsuite.IsDestroyed(t, pool)
}

func TestPool_PublicClientCACert(t *testing.T) {
	pool := NewPool()

	certs := pool.GetPublicClientCACerts()
	require.Empty(t, certs)

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
		gm := testsuite.MarkGoroutines(t)
		defer gm.Compare()

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

		testsuite.IsDestroyed(t, pool)
	})

	t.Run("delete parallel", func(t *testing.T) {
		gm := testsuite.MarkGoroutines(t)
		defer gm.Compare()

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

		testsuite.IsDestroyed(t, pool)
	})

	testsuite.IsDestroyed(t, pool)
}

func TestPool_PublicClientCert(t *testing.T) {
	pool := NewPool()

	pairs := pool.GetPublicClientPairs()
	require.Empty(t, pairs)

	pair := testGenerateCert(t)

	t.Run("add", func(t *testing.T) {
		cert, key := pair.Encode()
		err := pool.AddPublicClientPair(cert, key)
		require.NoError(t, err)
		err = pool.AddPublicClientPair(cert, key)
		require.Error(t, err)
		err = pool.AddPublicClientPair(cert, nil)
		require.Error(t, err)

		// loadCertWithPrivateKey
		err = pool.AddPublicClientPair(nil, nil)
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
		gm := testsuite.MarkGoroutines(t)
		defer gm.Compare()

		var pool *Pool
		pair1 := testGenerateCert(t)
		pair2 := testGenerateCert(t)

		init := func() {
			pool = NewPool()
		}
		add1 := func() {
			err := pool.AddPublicClientPair(pair1.Encode())
			require.NoError(t, err)
		}
		add2 := func() {
			err := pool.AddPublicClientPair(pair2.Encode())
			require.NoError(t, err)
		}
		get := func() {
			pool.GetPublicClientPairs()
		}
		testsuite.RunParallel(100, init, nil, add1, add2, get)

		testsuite.IsDestroyed(t, pool)
	})

	t.Run("delete parallel", func(t *testing.T) {
		gm := testsuite.MarkGoroutines(t)
		defer gm.Compare()

		var pool *Pool
		pair1 := testGenerateCert(t)
		pair2 := testGenerateCert(t)

		init := func() {
			pool = NewPool()

			err := pool.AddPublicClientPair(pair1.Encode())
			require.NoError(t, err)
			err = pool.AddPublicClientPair(pair2.Encode())
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

		testsuite.IsDestroyed(t, pool)
	})

	testsuite.IsDestroyed(t, pool)
}

func TestPool_PrivateRootCACert(t *testing.T) {
	pool := NewPool()

	certs := pool.GetPrivateRootCACerts()
	require.Empty(t, certs)

	pair := testGenerateCert(t)

	t.Run("add", func(t *testing.T) {
		cert, key := pair.Encode()
		err := pool.AddPrivateRootCAPair(cert, key)
		require.NoError(t, err)
		err = pool.AddPrivateRootCAPair(cert, key)
		require.Error(t, err)
		err = pool.AddPrivateRootCAPair(cert, []byte{})
		require.Error(t, err)

		// loadCertWithPrivateKey
		err = pool.AddPrivateRootCAPair(nil, nil)
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
		gm := testsuite.MarkGoroutines(t)
		defer gm.Compare()

		var pool *Pool
		pair1 := testGenerateCert(t)
		pair2 := testGenerateCert(t)

		init := func() {
			pool = NewPool()
		}
		add1 := func() {
			err := pool.AddPrivateRootCAPair(pair1.Encode())
			require.NoError(t, err)
		}
		add2 := func() {
			err := pool.AddPrivateRootCAPair(pair2.Encode())
			require.NoError(t, err)
		}
		get1 := func() {
			pool.GetPrivateRootCACerts()
		}
		get2 := func() {
			pool.GetPrivateRootCAPairs()
		}
		testsuite.RunParallel(100, init, nil, add1, add2, get1, get2)

		testsuite.IsDestroyed(t, pool)
	})

	t.Run("delete parallel", func(t *testing.T) {
		gm := testsuite.MarkGoroutines(t)
		defer gm.Compare()

		var pool *Pool
		pair1 := testGenerateCert(t)
		pair2 := testGenerateCert(t)

		init := func() {
			pool = NewPool()

			err := pool.AddPrivateRootCAPair(pair1.Encode())
			require.NoError(t, err)
			err = pool.AddPrivateRootCAPair(pair2.Encode())
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

		testsuite.IsDestroyed(t, pool)
	})

	testsuite.IsDestroyed(t, pool)
}

func TestPool_PrivateClientCACert(t *testing.T) {
	pool := NewPool()

	certs := pool.GetPrivateClientCACerts()
	require.Empty(t, certs)

	pair := testGenerateCert(t)

	t.Run("add", func(t *testing.T) {
		cert, key := pair.Encode()
		err := pool.AddPrivateClientCAPair(cert, key)
		require.NoError(t, err)
		err = pool.AddPrivateClientCAPair(cert, key)
		require.Error(t, err)
		err = pool.AddPrivateClientCAPair(cert, []byte{})
		require.Error(t, err)

		// loadCertWithPrivateKey
		err = pool.AddPrivateClientCAPair(nil, nil)
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
		gm := testsuite.MarkGoroutines(t)
		defer gm.Compare()

		var pool *Pool
		pair1 := testGenerateCert(t)
		pair2 := testGenerateCert(t)

		init := func() {
			pool = NewPool()
		}
		add1 := func() {
			err := pool.AddPrivateClientCAPair(pair1.Encode())
			require.NoError(t, err)
		}
		add2 := func() {
			err := pool.AddPrivateClientCAPair(pair2.Encode())
			require.NoError(t, err)
		}
		get1 := func() {
			pool.GetPrivateClientCACerts()
		}
		get2 := func() {
			pool.GetPrivateClientCAPairs()
		}
		testsuite.RunParallel(100, init, nil, add1, add2, get1, get2)

		testsuite.IsDestroyed(t, pool)
	})

	t.Run("delete parallel", func(t *testing.T) {
		gm := testsuite.MarkGoroutines(t)
		defer gm.Compare()

		var pool *Pool
		pair1 := testGenerateCert(t)
		pair2 := testGenerateCert(t)

		init := func() {
			pool = NewPool()

			err := pool.AddPrivateClientCAPair(pair1.Encode())
			require.NoError(t, err)
			err = pool.AddPrivateClientCAPair(pair2.Encode())
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

		testsuite.IsDestroyed(t, pool)
	})

	testsuite.IsDestroyed(t, pool)
}

func TestPool_PrivateClientCert(t *testing.T) {
	pool := NewPool()

	pairs := pool.GetPrivateClientPairs()
	require.Empty(t, pairs)

	pair := testGenerateCert(t)

	t.Run("add", func(t *testing.T) {
		cert, key := pair.Encode()
		err := pool.AddPrivateClientPair(cert, key)
		require.NoError(t, err)
		err = pool.AddPrivateClientPair(cert, key)
		require.Error(t, err)
		err = pool.AddPrivateClientPair(cert, []byte{})
		require.Error(t, err)

		// loadCertWithPrivateKey
		err = pool.AddPrivateClientPair(nil, nil)
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
		gm := testsuite.MarkGoroutines(t)
		defer gm.Compare()

		var pool *Pool
		pair1 := testGenerateCert(t)
		pair2 := testGenerateCert(t)

		init := func() {
			pool = NewPool()
		}
		add1 := func() {
			err := pool.AddPrivateClientPair(pair1.Encode())
			require.NoError(t, err)
		}
		add2 := func() {
			err := pool.AddPrivateClientPair(pair2.Encode())
			require.NoError(t, err)
		}
		get := func() {
			pool.GetPrivateClientPairs()
		}
		testsuite.RunParallel(100, init, nil, add1, add2, get)

		testsuite.IsDestroyed(t, pool)
	})

	t.Run("delete parallel", func(t *testing.T) {
		gm := testsuite.MarkGoroutines(t)
		defer gm.Compare()

		var pool *Pool
		pair1 := testGenerateCert(t)
		pair2 := testGenerateCert(t)

		init := func() {
			pool = NewPool()

			err := pool.AddPrivateClientPair(pair1.Encode())
			require.NoError(t, err)
			err = pool.AddPrivateClientPair(pair2.Encode())
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

		testsuite.IsDestroyed(t, pool)
	})

	testsuite.IsDestroyed(t, pool)
}

func TestPool_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	var pool *Pool
	pair1 := testGenerateCert(t)
	pair2 := testGenerateCert(t)

	init := func() { pool = NewPool() }
	fns := []func(){
		// add
		func() { _ = pool.AddPublicRootCACert(pair1.Certificate.Raw) },
		func() { _ = pool.AddPublicRootCACert(pair2.Certificate.Raw) },
		func() { _ = pool.AddPublicClientCACert(pair1.Certificate.Raw) },
		func() { _ = pool.AddPublicClientCACert(pair2.Certificate.Raw) },
		func() { _ = pool.AddPublicClientPair(pair1.Encode()) },
		func() { _ = pool.AddPublicClientPair(pair2.Encode()) },
		func() { _ = pool.AddPrivateRootCAPair(pair1.Encode()) },
		func() { _ = pool.AddPrivateRootCAPair(pair2.Encode()) },
		func() { _ = pool.AddPrivateClientCAPair(pair1.Encode()) },
		func() { _ = pool.AddPrivateClientCAPair(pair2.Encode()) },
		func() { _ = pool.AddPrivateClientPair(pair1.Encode()) },
		func() { _ = pool.AddPrivateClientPair(pair2.Encode()) },

		// delete
		func() { _ = pool.DeletePublicRootCACert(0) },
		func() { _ = pool.DeletePublicRootCACert(0) },
		func() { _ = pool.DeletePublicClientCACert(0) },
		func() { _ = pool.DeletePublicClientCACert(0) },
		func() { _ = pool.DeletePublicClientCert(0) },
		func() { _ = pool.DeletePublicClientCert(0) },
		func() { _ = pool.DeletePrivateRootCACert(0) },
		func() { _ = pool.DeletePrivateRootCACert(0) },
		func() { _ = pool.DeletePrivateClientCACert(0) },
		func() { _ = pool.DeletePrivateClientCACert(0) },
		func() { _ = pool.DeletePrivateClientCert(0) },
		func() { _ = pool.DeletePrivateClientCert(0) },

		// get
		func() { _ = pool.GetPublicRootCACerts() },
		func() { _ = pool.GetPublicClientCACerts() },
		func() { _ = pool.GetPublicClientPairs() },
		func() { _ = pool.GetPrivateRootCACerts() },
		func() { _ = pool.GetPrivateClientCACerts() },
		func() { _ = pool.GetPrivateRootCAPairs() },
		func() { _ = pool.GetPrivateClientCAPairs() },
		func() { _ = pool.GetPrivateClientPairs() },
	}
	testsuite.RunParallel(100, init, nil, fns...)

	testsuite.IsDestroyed(t, pool)
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

		testsuite.IsDestroyed(t, pool)
	})
}
