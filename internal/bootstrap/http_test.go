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
	"project/internal/testsuite/testtls"
)

func TestCoverHTTPRequest(t *testing.T) {
	url := strings.Repeat("http://test.com/", 1)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	f1 := req.URL.String()
	coverHTTPRequest(req)
	f2 := req.URL.String()
	require.NotEqual(t, f1, f2, "failed to cover string fields")
}

func testGenerateHTTP(t *testing.T) *HTTP {
	HTTP := HTTP{
		AESKey: strings.Repeat("FF", aes.Key256Bit),
		AESIV:  strings.Repeat("FF", aes.IVSize),
	}
	privateKey, err := ed25519.GenerateKey()
	require.NoError(t, err)
	HTTP.PrivateKey = privateKey
	return &HTTP
}

func TestHTTP(t *testing.T) {
	dnsClient, proxyPool, proxyMgr, certPool := testdns.DNSClient(t)
	defer func() {
		err := proxyMgr.Close()
		require.NoError(t, err)
	}()

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
			data, err := HTTP.Marshal()
			require.NoError(t, err)

			// unmarshal
			HTTP = NewHTTP(context.Background(), certPool, proxyPool, dnsClient)
			err = HTTP.Unmarshal(data)
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
			data, err := HTTP.Marshal()
			require.NoError(t, err)

			// unmarshal
			HTTP = NewHTTP(context.Background(), certPool, proxyPool, dnsClient)
			err = HTTP.Unmarshal(data)
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
		serverCfg, clientCfg := testtls.OptionPair(t, "127.0.0.1")

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
			data, err := HTTP.Marshal()
			require.NoError(t, err)

			// unmarshal
			HTTP = NewHTTP(context.Background(), certPool, proxyPool, dnsClient)
			err = HTTP.Unmarshal(data)
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
			data, err := HTTP.Marshal()
			require.NoError(t, err)

			// unmarshal
			HTTP = NewHTTP(context.Background(), certPool, proxyPool, dnsClient)
			err = HTTP.Unmarshal(data)
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
	HTTP := HTTP{}
	// invalid request
	err := HTTP.Validate()
	require.Error(t, err)

	// invalid transport
	HTTP.Request.URL = "http://abc.com/"
	HTTP.Transport.TLSClientConfig.RootCAs = []string{"foo ca"}
	err = HTTP.Validate()
	require.Error(t, err)

	HTTP.Transport.TLSClientConfig.RootCAs = nil

	// invalid AES Key
	HTTP.AESKey = "foo key"
	err = HTTP.Validate()
	require.Error(t, err)

	HTTP.AESKey = hex.EncodeToString(bytes.Repeat([]byte{0}, aes.Key256Bit))

	// invalid AES IV
	HTTP.AESIV = "foo iv"
	err = HTTP.Validate()
	require.Error(t, err)
	HTTP.AESIV = hex.EncodeToString(bytes.Repeat([]byte{0}, aes.IVSize+1))
	err = HTTP.Validate()
	require.Error(t, err)

	HTTP.AESIV = hex.EncodeToString(bytes.Repeat([]byte{0}, aes.IVSize))

	// invalid public key
	HTTP.PublicKey = "foo public key"
	err = HTTP.Validate()
	require.Error(t, err)

	HTTP.PrivateKey, err = ed25519.GenerateKey()
	require.NoError(t, err)
	HTTP.AESIV = "foo iv"
	data, err := HTTP.Marshal()
	require.Error(t, err)
	require.Nil(t, data)
}

func TestHTTP_Generate(t *testing.T) {
	HTTP := NewHTTP(context.Background(), nil, nil, nil)

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
	HTTP := HTTP{}

	// unmarshal invalid config
	err := HTTP.Unmarshal([]byte{0x00})
	require.Error(t, err)

	// with incorrect config
	err = HTTP.Unmarshal(nil)
	require.Error(t, err)
}

func TestHTTP_Resolve(t *testing.T) {
	dnsClient, proxyPool, proxyMgr, certPool := testdns.DNSClient(t)
	defer func() {
		err := proxyMgr.Close()
		require.NoError(t, err)
	}()

	t.Run("doesn't exist proxy server", func(t *testing.T) {
		HTTP := testGenerateHTTP(t)
		HTTP.Request.URL = "http://localhost/"
		HTTP.DNSOpts.Mode = dns.ModeSystem
		HTTP.ProxyTag = "doesn't exist"

		data, err := HTTP.Marshal()
		require.NoError(t, err)

		HTTP = NewHTTP(context.Background(), certPool, proxyPool, dnsClient)
		err = HTTP.Unmarshal(data)
		require.NoError(t, err)

		listeners, err := HTTP.Resolve()
		require.Error(t, err)
		require.Nil(t, listeners)
	})

	t.Run("invalid dns options", func(t *testing.T) {
		HTTP := testGenerateHTTP(t)
		HTTP.Request.URL = "http://localhost/"
		HTTP.DNSOpts.Mode = "foo mode"

		data, err := HTTP.Marshal()
		require.NoError(t, err)

		HTTP = NewHTTP(context.Background(), certPool, proxyPool, dnsClient)
		err = HTTP.Unmarshal(data)
		require.NoError(t, err)

		listeners, err := HTTP.Resolve()
		require.Error(t, err)
		require.Nil(t, listeners)
	})

	t.Run("unreachable server", func(t *testing.T) {
		HTTP := testGenerateHTTP(t)
		HTTP.Request.URL = "http://localhost/"
		HTTP.DNSOpts.Mode = dns.ModeSystem

		data, err := HTTP.Marshal()
		require.NoError(t, err)

		HTTP = NewHTTP(context.Background(), certPool, proxyPool, dnsClient)
		err = HTTP.Unmarshal(data)
		require.NoError(t, err)

		listeners, err := HTTP.Resolve()
		require.Error(t, err)
		require.Nil(t, listeners)
	})
}

func TestHTTPPanic(t *testing.T) {
	t.Run("no CBC", func(t *testing.T) {
		HTTP := HTTP{}

		func() {
			defer testsuite.DeferForPanic(t)
			_, _ = HTTP.Resolve()
		}()

		testsuite.IsDestroyed(t, &HTTP)
	})

	t.Run("invalid encrypted data", func(t *testing.T) {
		HTTP := HTTP{}

		func() {
			var err error
			key := bytes.Repeat([]byte{0}, aes.Key128Bit)
			HTTP.cbc, err = aes.NewCBC(key, key)
			require.NoError(t, err)
			HTTP.enc, err = HTTP.cbc.Encrypt(testsuite.Bytes())
			require.NoError(t, err)

			defer testsuite.DeferForPanic(t)
			_, _ = HTTP.Resolve()
		}()

		testsuite.IsDestroyed(t, &HTTP)
	})

	t.Run("invalid http request", func(t *testing.T) {
		dHTTP := HTTP{}

		func() {
			var err error
			key := bytes.Repeat([]byte{0}, aes.Key128Bit)
			dHTTP.cbc, err = aes.NewCBC(key, key)
			require.NoError(t, err)

			data, err := msgpack.Marshal(new(HTTP))
			require.NoError(t, err)
			dHTTP.enc, err = dHTTP.cbc.Encrypt(data)
			require.NoError(t, err)

			defer testsuite.DeferForPanic(t)
			_, _ = dHTTP.Resolve()
		}()

		testsuite.IsDestroyed(t, &dHTTP)
	})

	t.Run("invalid http transport", func(t *testing.T) {
		dHTTP := HTTP{}

		func() {
			var err error
			key := bytes.Repeat([]byte{0}, aes.Key128Bit)
			dHTTP.cbc, err = aes.NewCBC(key, key)
			require.NoError(t, err)

			tHTTP := HTTP{}
			tHTTP.Request.URL = "http://localhost/"
			tHTTP.Transport.TLSClientConfig.RootCAs = []string{"foo ca"}
			data, err := msgpack.Marshal(&tHTTP)
			require.NoError(t, err)
			dHTTP.enc, err = dHTTP.cbc.Encrypt(data)
			require.NoError(t, err)

			defer testsuite.DeferForPanic(t)
			_, _ = dHTTP.Resolve()
		}()

		testsuite.IsDestroyed(t, &dHTTP)
	})

	// resolve
	t.Run("invalid info", func(t *testing.T) {
		defer testsuite.DeferForPanic(t)
		resolve(nil, []byte("foo data"))
	})

	t.Run("invalid AES Key IV", func(t *testing.T) {
		HTTP := HTTP{
			AESKey: strings.Repeat("F", aes.Key128Bit),
			AESIV:  strings.Repeat("F", aes.IVSize),
		}

		defer testsuite.DeferForPanic(t)
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

		defer testsuite.DeferForPanic(t)
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

		defer testsuite.DeferForPanic(t)
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

		defer testsuite.DeferForPanic(t)
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

		defer testsuite.DeferForPanic(t)
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

		defer testsuite.DeferForPanic(t)
		resolve(&HTTP, []byte(hex.EncodeToString(cipherData)))
	})
}

func TestHTTPOptions(t *testing.T) {
	config, err := ioutil.ReadFile("testdata/http.toml")
	require.NoError(t, err)

	// check unnecessary field
	HTTP := new(HTTP)
	err = toml.Unmarshal(config, HTTP)
	require.NoError(t, err)
	err = HTTP.Validate()
	require.NoError(t, err)

	// check zero value
	testsuite.CheckOptions(t, HTTP)

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
