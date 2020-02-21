package bootstrap

import (
	"bytes"
	"context"
	"encoding/hex"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/aes"
	"project/internal/crypto/ed25519"
	"project/internal/dns"
	"project/internal/patch/msgpack"
	"project/internal/patch/toml"
	"project/internal/testsuite"
	"project/internal/testsuite/testdns"
)

func testGenerateHTTP(t *testing.T) *HTTP {
	HTTP := NewHTTP(context.Background(), nil, nil)
	HTTP.AESKey = strings.Repeat("FF", aes.Key256Bit)
	HTTP.AESIV = strings.Repeat("FF", aes.IVSize)
	privateKey, err := ed25519.GenerateKey()
	require.NoError(t, err)
	HTTP.PrivateKey = privateKey
	return HTTP
}

func TestHTTP(t *testing.T) {
	dnsClient, proxyPool, manager := testdns.DNSClient(t)
	defer func() { _ = manager.Close() }()

	t.Run("http", func(t *testing.T) {
		// set test http server mux
		var listenersData []byte
		serveMux := http.NewServeMux()
		serveMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(listenersData)
		})

		if testsuite.IPv4Enabled {
			listeners := testGenerateListeners()
			HTTP := testGenerateHTTP(t)
			listenersInfo, err := HTTP.Generate(listeners)
			require.NoError(t, err)
			listenersData = listenersInfo
			t.Logf("(http-IPv4) bootstrap node listeners info: %s\n", listenersInfo)

			// run HTTP server
			httpServer := http.Server{
				Addr:    "localhost:0",
				Handler: serveMux,
			}
			port := testsuite.RunHTTPServer(t, "tcp4", &httpServer)
			defer func() { _ = httpServer.Close() }()

			// config
			HTTP.Request.URL = "http://localhost:" + port
			HTTP.DNSOpts.Mode = dns.ModeSystem
			HTTP.DNSOpts.Type = dns.TypeIPv4
			// marshal
			b, err := HTTP.Marshal()
			require.NoError(t, err)
			// unmarshal
			HTTP = NewHTTP(context.Background(), proxyPool, dnsClient)
			err = HTTP.Unmarshal(b)
			require.NoError(t, err)

			for i := 0; i < 10; i++ {
				resolved, err := HTTP.Resolve()
				require.NoError(t, err)
				resolved = testDecryptListeners(resolved)
				require.Equal(t, listeners, resolved)
			}

			testsuite.IsDestroyed(t, HTTP)
		}

		if testsuite.IPv6Enabled {
			listeners := testGenerateListeners()
			HTTP := testGenerateHTTP(t)
			listenersInfo, err := HTTP.Generate(listeners)
			require.NoError(t, err)
			listenersData = listenersInfo
			t.Logf("(http-IPv6) bootstrap node listeners info: %s\n", listenersInfo)

			// run HTTP server
			httpServer := http.Server{
				Addr:    "localhost:0",
				Handler: serveMux,
			}
			port := testsuite.RunHTTPServer(t, "tcp6", &httpServer)
			defer func() { _ = httpServer.Close() }()

			// config
			HTTP.Request.URL = "http://localhost:" + port
			HTTP.DNSOpts.Mode = dns.ModeSystem
			HTTP.DNSOpts.Type = dns.TypeIPv6
			// marshal
			b, err := HTTP.Marshal()
			require.NoError(t, err)
			// unmarshal
			HTTP = NewHTTP(context.Background(), proxyPool, dnsClient)
			err = HTTP.Unmarshal(b)
			require.NoError(t, err)

			for i := 0; i < 10; i++ {
				resolved, err := HTTP.Resolve()
				require.NoError(t, err)
				resolved = testDecryptListeners(resolved)
				require.Equal(t, listeners, resolved)
			}

			testsuite.IsDestroyed(t, HTTP)
		}
	})

	t.Run("https", func(t *testing.T) {
		// set test http server mux
		var listenersData []byte
		serveMux := http.NewServeMux()
		serveMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(listenersData)
		})
		serverCfg, clientCfg := testsuite.TLSConfigOptionPair(t)

		if testsuite.IPv4Enabled {
			listeners := testGenerateListeners()
			HTTP := testGenerateHTTP(t)
			listenersInfo, err := HTTP.Generate(listeners)
			require.NoError(t, err)
			t.Logf("(https-IPv4) bootstrap node listeners info: %s\n", listenersInfo)
			listenersData = listenersInfo

			// run HTTPS server
			tlsConfig, err := serverCfg.Apply()
			require.NoError(t, err)
			httpsServer := http.Server{
				Addr:      "localhost:0",
				Handler:   serveMux,
				TLSConfig: tlsConfig,
			}
			port := testsuite.RunHTTPServer(t, "tcp4", &httpsServer)
			defer func() { _ = httpsServer.Close() }()

			// config
			HTTP.Request.URL = "https://localhost:" + port
			HTTP.DNSOpts.Mode = dns.ModeSystem
			HTTP.DNSOpts.Type = dns.TypeIPv4
			HTTP.Transport.TLSClientConfig = clientCfg
			// marshal
			b, err := HTTP.Marshal()
			require.NoError(t, err)
			// unmarshal
			HTTP = NewHTTP(context.Background(), proxyPool, dnsClient)
			err = HTTP.Unmarshal(b)
			require.NoError(t, err)

			for i := 0; i < 10; i++ {
				resolved, err := HTTP.Resolve()
				require.NoError(t, err)
				resolved = testDecryptListeners(resolved)
				require.Equal(t, listeners, resolved)
			}

			testsuite.IsDestroyed(t, HTTP)
		}

		if testsuite.IPv6Enabled {
			listeners := testGenerateListeners()
			HTTP := testGenerateHTTP(t)
			listenersInfo, err := HTTP.Generate(listeners)
			require.NoError(t, err)
			t.Logf("(https-IPv6) bootstrap node listeners info: %s\n", listenersInfo)
			listenersData = listenersInfo

			// run HTTPS server
			tlsConfig, err := serverCfg.Apply()
			require.NoError(t, err)
			httpsServer := http.Server{
				Addr:      "localhost:0",
				Handler:   serveMux,
				TLSConfig: tlsConfig,
			}
			port := testsuite.RunHTTPServer(t, "tcp6", &httpsServer)
			defer func() { _ = httpsServer.Close() }()

			// config
			HTTP.Request.URL = "https://localhost:" + port
			HTTP.DNSOpts.Mode = dns.ModeSystem
			HTTP.DNSOpts.Type = dns.TypeIPv6
			HTTP.Transport.TLSClientConfig = clientCfg
			// marshal
			b, err := HTTP.Marshal()
			require.NoError(t, err)
			// unmarshal
			HTTP = NewHTTP(context.Background(), proxyPool, dnsClient)
			err = HTTP.Unmarshal(b)
			require.NoError(t, err)

			for i := 0; i < 10; i++ {
				resolved, err := HTTP.Resolve()
				require.NoError(t, err)
				resolved = testDecryptListeners(resolved)
				require.Equal(t, listeners, resolved)
			}

			testsuite.IsDestroyed(t, HTTP)
		}
	})
}

func TestHTTP_Validate(t *testing.T) {
	HTTP := NewHTTP(nil, nil, nil)
	// invalid request
	require.Error(t, HTTP.Validate())

	// invalid transport
	HTTP.Request.URL = "http://abc.com/"
	HTTP.Transport.TLSClientConfig.RootCAs = []string{"foo ca"}
	require.Error(t, HTTP.Validate())

	HTTP.Transport.TLSClientConfig.RootCAs = nil

	// invalid AES Key
	HTTP.AESKey = "foo key"
	require.Error(t, HTTP.Validate())

	HTTP.AESKey = hex.EncodeToString(bytes.Repeat([]byte{0}, aes.Key256Bit))

	// invalid AES IV
	HTTP.AESIV = "foo iv"
	require.Error(t, HTTP.Validate())
	HTTP.AESIV = hex.EncodeToString(bytes.Repeat([]byte{0}, aes.IVSize+1))
	require.Error(t, HTTP.Validate())

	HTTP.AESIV = hex.EncodeToString(bytes.Repeat([]byte{0}, aes.IVSize))

	// invalid public key
	HTTP.PublicKey = "foo public key"
	require.Error(t, HTTP.Validate())

	var err error
	HTTP.PrivateKey, err = ed25519.GenerateKey()
	require.NoError(t, err)
	HTTP.AESIV = "foo iv"
	b, err := HTTP.Marshal()
	require.Error(t, err)
	require.Nil(t, b)
}

func TestHTTP_Generate(t *testing.T) {
	HTTP := NewHTTP(nil, nil, nil)

	// no bootstrap node listeners
	_, err := HTTP.Generate(nil)
	require.Error(t, err)

	// invalid AES Key
	HTTP.PrivateKey, err = ed25519.GenerateKey()
	require.NoError(t, err)
	listeners := testGenerateListeners()
	HTTP.AESKey = "foo key"
	_, err = HTTP.Generate(listeners)
	require.Error(t, err)

	HTTP.AESKey = hex.EncodeToString(bytes.Repeat([]byte{0}, aes.Key128Bit))

	// invalid AES IV
	HTTP.AESIV = "foo iv"
	_, err = HTTP.Generate(listeners)
	require.Error(t, err)

	// invalid Key IV
	HTTP.AESIV = hex.EncodeToString(bytes.Repeat([]byte{0}, 32))
	_, err = HTTP.Generate(listeners)
	require.Error(t, err)
}

func TestHTTP_Unmarshal(t *testing.T) {
	HTTP := NewHTTP(nil, nil, nil)

	// unmarshal invalid config
	require.Error(t, HTTP.Unmarshal([]byte{0x00}))

	// with incorrect config
	require.Error(t, HTTP.Unmarshal(nil))
}

func TestHTTP_Resolve(t *testing.T) {
	dnsClient, proxyPool, manager := testdns.DNSClient(t)
	defer func() { require.NoError(t, manager.Close()) }()

	t.Run("doesn't exist proxy server", func(t *testing.T) {
		HTTP := testGenerateHTTP(t)
		HTTP.Request.URL = "http://localhost/"
		HTTP.DNSOpts.Mode = dns.ModeSystem
		HTTP.ProxyTag = "doesn't exist"
		b, err := HTTP.Marshal()
		require.NoError(t, err)
		HTTP = NewHTTP(context.Background(), proxyPool, dnsClient)
		require.NoError(t, HTTP.Unmarshal(b))
		listeners, err := HTTP.Resolve()
		require.Error(t, err)
		require.Nil(t, listeners)
	})

	t.Run("invalid dns options", func(t *testing.T) {
		HTTP := testGenerateHTTP(t)
		HTTP.Request.URL = "http://localhost/"
		HTTP.DNSOpts.Mode = "foo mode"
		b, err := HTTP.Marshal()
		require.NoError(t, err)
		HTTP = NewHTTP(context.Background(), proxyPool, dnsClient)
		require.NoError(t, HTTP.Unmarshal(b))
		listeners, err := HTTP.Resolve()
		require.Error(t, err)
		require.Nil(t, listeners)
	})

	t.Run("unreachable server", func(t *testing.T) {
		HTTP := testGenerateHTTP(t)
		HTTP.Request.URL = "http://localhost/"
		HTTP.DNSOpts.Mode = dns.ModeSystem
		b, err := HTTP.Marshal()
		require.NoError(t, err)
		HTTP = NewHTTP(context.Background(), proxyPool, dnsClient)
		require.NoError(t, HTTP.Unmarshal(b))
		listeners, err := HTTP.Resolve()
		require.Error(t, err)
		require.Nil(t, listeners)
	})
}

func TestHTTPPanic(t *testing.T) {
	t.Run("no CBC", func(t *testing.T) {
		HTTP := NewHTTP(nil, nil, nil)

		func() {
			defer func() {
				r := recover()
				require.NotNil(t, r)
				t.Log(r)
			}()
			_, _ = HTTP.Resolve()
		}()

		testsuite.IsDestroyed(t, HTTP)
	})

	t.Run("invalid encrypted data", func(t *testing.T) {
		HTTP := NewHTTP(nil, nil, nil)

		func() {
			var err error
			key := bytes.Repeat([]byte{0}, aes.Key128Bit)
			HTTP.cbc, err = aes.NewCBC(key, key)
			require.NoError(t, err)

			enc, err := HTTP.cbc.Encrypt(testsuite.Bytes())
			require.NoError(t, err)
			HTTP.enc = enc

			defer func() {
				r := recover()
				require.NotNil(t, r)
				t.Log(r)
			}()
			_, _ = HTTP.Resolve()
		}()

		testsuite.IsDestroyed(t, HTTP)
	})

	t.Run("invalid http request", func(t *testing.T) {
		dHTTP := NewHTTP(nil, nil, nil)

		func() {
			var err error
			key := bytes.Repeat([]byte{0}, aes.Key128Bit)
			dHTTP.cbc, err = aes.NewCBC(key, key)
			require.NoError(t, err)

			b, err := msgpack.Marshal(new(HTTP))
			require.NoError(t, err)
			enc, err := dHTTP.cbc.Encrypt(b)
			require.NoError(t, err)
			dHTTP.enc = enc

			defer func() {
				r := recover()
				require.NotNil(t, r)
				t.Log(r)
			}()
			_, _ = dHTTP.Resolve()
		}()

		testsuite.IsDestroyed(t, dHTTP)
	})

	t.Run("invalid http transport", func(t *testing.T) {
		dHTTP := NewHTTP(nil, nil, nil)

		func() {
			var err error
			key := bytes.Repeat([]byte{0}, aes.Key128Bit)
			dHTTP.cbc, err = aes.NewCBC(key, key)
			require.NoError(t, err)

			tHTTP := HTTP{}
			tHTTP.Request.URL = "http://localhost/"
			tHTTP.Transport.TLSClientConfig.RootCAs = []string{"foo ca"}
			b, err := msgpack.Marshal(&tHTTP)
			require.NoError(t, err)
			enc, err := dHTTP.cbc.Encrypt(b)
			require.NoError(t, err)
			dHTTP.enc = enc

			defer func() {
				r := recover()
				require.NotNil(t, r)
				t.Log(r)
			}()
			_, _ = dHTTP.Resolve()
		}()

		testsuite.IsDestroyed(t, dHTTP)
	})

	// resolve
	t.Run("invalid info", func(t *testing.T) {
		defer func() {
			r := recover()
			require.NotNil(t, r)
			t.Log(r)
		}()
		resolve(nil, []byte("foo data"))
	})

	t.Run("invalid AES Key IV", func(t *testing.T) {
		HTTP := HTTP{
			AESKey: strings.Repeat("F", aes.Key128Bit),
			AESIV:  strings.Repeat("F", aes.IVSize),
		}

		defer func() {
			r := recover()
			require.NotNil(t, r)
			t.Log(r)
		}()
		resolve(&HTTP, []byte("FF"))
	})

	t.Run("invalid info size", func(t *testing.T) {
		key := bytes.Repeat([]byte{0xFF}, aes.Key128Bit)
		cbc, err := aes.NewCBC(key, key)
		require.NoError(t, err)
		cipherData, err := cbc.Encrypt([]byte{0x00})
		require.NoError(t, err)
		HTTP := HTTP{
			AESKey: hex.EncodeToString(key),
			AESIV:  hex.EncodeToString(key),
		}

		defer func() {
			r := recover()
			require.NotNil(t, r)
			t.Log(r)
		}()
		resolve(&HTTP, []byte(hex.EncodeToString(cipherData)))
	})

	t.Run("invalid public key string", func(t *testing.T) {
		key := bytes.Repeat([]byte{0xFF}, aes.Key128Bit)
		cbc, err := aes.NewCBC(key, key)
		require.NoError(t, err)
		data := bytes.Repeat([]byte{0}, ed25519.SignatureSize+1)
		cipherData, err := cbc.Encrypt(data)
		require.NoError(t, err)
		HTTP := HTTP{
			AESKey: hex.EncodeToString(key),
			AESIV:  hex.EncodeToString(key),

			// must generate, because security.FlushString
			PublicKey: strings.Repeat("foo public key", 1),
		}

		defer func() {
			r := recover()
			require.NotNil(t, r)
			t.Log(r)
		}()
		resolve(&HTTP, []byte(hex.EncodeToString(cipherData)))
	})

	t.Run("invalid public key", func(t *testing.T) {
		key := bytes.Repeat([]byte{0xFF}, aes.Key128Bit)
		cbc, err := aes.NewCBC(key, key)
		require.NoError(t, err)
		data := bytes.Repeat([]byte{0}, ed25519.SignatureSize+1)
		cipherData, err := cbc.Encrypt(data)
		require.NoError(t, err)
		HTTP := HTTP{
			AESKey: hex.EncodeToString(key),
			AESIV:  hex.EncodeToString(key),

			// must generate, because security.FlushString
			PublicKey: strings.Repeat("FF", 1),
		}

		defer func() {
			r := recover()
			require.NotNil(t, r)
			t.Log(r)
		}()
		resolve(&HTTP, []byte(hex.EncodeToString(cipherData)))
	})

	t.Run("invalid signature", func(t *testing.T) {
		key := bytes.Repeat([]byte{0xFF}, aes.Key128Bit)
		cbc, err := aes.NewCBC(key, key)
		require.NoError(t, err)
		data := bytes.Repeat([]byte{0}, ed25519.SignatureSize+1)
		cipherData, err := cbc.Encrypt(data)
		require.NoError(t, err)
		HTTP := HTTP{
			AESKey: hex.EncodeToString(key),
			AESIV:  hex.EncodeToString(key),

			// must generate, because security.FlushString
			PublicKey: strings.Repeat("FF", ed25519.PublicKeySize),
		}

		defer func() {
			r := recover()
			require.NotNil(t, r)
			t.Log(r)
		}()
		resolve(&HTTP, []byte(hex.EncodeToString(cipherData)))
	})

	t.Run("invalid node listeners data", func(t *testing.T) {
		key := bytes.Repeat([]byte{0xFF}, aes.Key128Bit)
		cbc, err := aes.NewCBC(key, key)
		require.NoError(t, err)
		data := bytes.Repeat([]byte{0}, ed25519.SignatureSize+1)
		privateKey, err := ed25519.GenerateKey()
		require.NoError(t, err)
		signature := ed25519.Sign(privateKey, data)
		cipherData, err := cbc.Encrypt(append(signature, data...))
		require.NoError(t, err)
		HTTP := HTTP{
			AESKey:    hex.EncodeToString(key),
			AESIV:     hex.EncodeToString(key),
			PublicKey: hex.EncodeToString(privateKey.PublicKey()),
		}

		defer func() {
			r := recover()
			require.NotNil(t, r)
			t.Log(r)
		}()
		resolve(&HTTP, []byte(hex.EncodeToString(cipherData)))
	})
}

func TestHTTPOptions(t *testing.T) {
	config, err := ioutil.ReadFile("testdata/http.toml")
	require.NoError(t, err)
	HTTP := NewHTTP(nil, nil, nil)
	require.NoError(t, toml.Unmarshal(config, HTTP))
	require.NoError(t, HTTP.Validate())

	testdata := [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: 15 * time.Second, actual: HTTP.Timeout},
		{expected: "balance", actual: HTTP.ProxyTag},
		{expected: int64(65535), actual: HTTP.MaxBodySize},
		{expected: strings.Repeat("FF", aes.Key256Bit), actual: HTTP.AESKey},
		{expected: strings.Repeat("FF", aes.IVSize), actual: HTTP.AESIV},
		{expected: strings.Repeat("FF", ed25519.PublicKeySize), actual: HTTP.PublicKey},
		{expected: "https://test.com/", actual: HTTP.Request.URL},
		{expected: 2, actual: HTTP.Transport.MaxIdleConns},
		{expected: dns.ModeSystem, actual: HTTP.DNSOpts.Mode},
	}
	for _, td := range testdata {
		require.Equal(t, td.expected, td.actual)
	}
}
