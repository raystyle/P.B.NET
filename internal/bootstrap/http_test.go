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
	"project/internal/option"
	"project/internal/patch/msgpack"
	"project/internal/patch/toml"
	"project/internal/testsuite"
	"project/internal/testsuite/testdns"
	"project/internal/testsuite/testproxy"
	"project/internal/testsuite/testtls"
)

func TestFlushRequestOption(t *testing.T) {
	// can't use const
	req := option.HTTPRequest{
		URL:    strings.Repeat("http://test.com/", 1),
		Header: make(http.Header),
	}
	header := strings.Repeat("a", 1)
	req.Header.Set(header, header)

	req1, err := req.Apply()
	require.NoError(t, err)
	flushRequestOption(&req)
	req2, err := req.Apply()
	require.Error(t, err)

	require.NotEqual(t, req1, req2, "failed to flush request options")
}

func TestCoverHTTPRequest(t *testing.T) {
	// can't use const
	url := strings.Repeat("http://test.com/", 1)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	header := strings.Repeat("a", 1)
	req.Header.Set(header, header)

	f1 := req.URL.String()
	coverHTTPRequest(req)
	f2 := req.URL.String()

	require.NotEqual(t, f1, f2, "failed to cover string fields")
}

func TestHTTP_Validate(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	HTTP := HTTP{}

	t.Run("invalid request", func(t *testing.T) {
		err := HTTP.Validate()
		require.Error(t, err)
	})

	t.Run("invalid transport", func(t *testing.T) {
		HTTP.Request.URL = "http://abc.com/"
		HTTP.Transport.TLSClientConfig.RootCAs = []string{"foo ca"}
		defer func() { HTTP.Transport.TLSClientConfig.RootCAs = nil }()

		err := HTTP.Validate()
		require.Error(t, err)
	})

	t.Run("invalid AES Key", func(t *testing.T) {
		HTTP.AESKey = "foo key"
		defer func() {
			key := bytes.Repeat([]byte{0}, aes.Key256Bit)
			HTTP.AESKey = hex.EncodeToString(key)
		}()

		err := HTTP.Validate()
		require.Error(t, err)
	})

	t.Run("invalid AES IV", func(t *testing.T) {
		HTTP.AESIV = "foo iv"
		defer func() {
			iv := bytes.Repeat([]byte{0}, aes.IVSize)
			HTTP.AESIV = hex.EncodeToString(iv)
		}()

		err := HTTP.Validate()
		require.Error(t, err)
	})

	t.Run("invalid AES Key and IV", func(t *testing.T) {
		HTTP.AESKey = hex.EncodeToString(bytes.Repeat([]byte{0}, aes.Key256Bit+1))
		HTTP.AESIV = hex.EncodeToString(bytes.Repeat([]byte{0}, aes.IVSize+1))
		defer func() {
			key := bytes.Repeat([]byte{0}, aes.Key256Bit)
			HTTP.AESKey = hex.EncodeToString(key)
			iv := bytes.Repeat([]byte{0}, aes.IVSize)
			HTTP.AESIV = hex.EncodeToString(iv)
		}()

		err := HTTP.Validate()
		require.Error(t, err)
	})

	t.Run("invalid public key", func(t *testing.T) {
		HTTP.PublicKey = "foo public key"
		defer func() {
			publicKey := bytes.Repeat([]byte{0}, ed25519.PublicKeySize)
			HTTP.PublicKey = hex.EncodeToString(publicKey)
		}()

		err := HTTP.Validate()
		require.Error(t, err)
	})

	t.Run("ok", func(t *testing.T) {
		HTTP.Request.URL = "http://abc.com/"

		key := bytes.Repeat([]byte{0}, aes.Key256Bit)
		HTTP.AESKey = hex.EncodeToString(key)
		iv := bytes.Repeat([]byte{0}, aes.IVSize)
		HTTP.AESIV = hex.EncodeToString(iv)

		publicKey := bytes.Repeat([]byte{0}, ed25519.PublicKeySize)
		HTTP.PublicKey = hex.EncodeToString(publicKey)

		err := HTTP.Validate()
		require.NoError(t, err)
	})

	testsuite.IsDestroyed(t, &HTTP)
}

func TestHTTP_Generate(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	HTTP := HTTP{}

	t.Run("no listeners", func(t *testing.T) {
		data, err := HTTP.Generate(nil)
		require.Error(t, err)
		require.Zero(t, data)
	})

	t.Run("invalid AES Key", func(t *testing.T) {
		privateKey, err := ed25519.GenerateKey()
		require.NoError(t, err)
		HTTP.PrivateKey = privateKey
		listeners := testGenerateListeners()

		HTTP.AESKey = "foo key"
		defer func() {
			key := bytes.Repeat([]byte{0}, aes.Key256Bit)
			HTTP.AESKey = hex.EncodeToString(key)
		}()

		_, err = HTTP.Generate(listeners)
		require.Error(t, err)
	})

	t.Run("invalid AES IV", func(t *testing.T) {
		listeners := testGenerateListeners()

		HTTP.AESIV = "foo iv"
		defer func() {
			iv := bytes.Repeat([]byte{0}, aes.IVSize)
			HTTP.AESIV = hex.EncodeToString(iv)
		}()

		_, err := HTTP.Generate(listeners)
		require.Error(t, err)
	})

	t.Run("invalid AES Key and IV", func(t *testing.T) {
		listeners := testGenerateListeners()

		HTTP.AESKey = hex.EncodeToString(bytes.Repeat([]byte{0}, aes.Key256Bit+1))
		HTTP.AESIV = hex.EncodeToString(bytes.Repeat([]byte{0}, aes.IVSize+1))
		defer func() {
			key := bytes.Repeat([]byte{0}, aes.Key256Bit)
			HTTP.AESKey = hex.EncodeToString(key)
			iv := bytes.Repeat([]byte{0}, aes.IVSize)
			HTTP.AESIV = hex.EncodeToString(iv)
		}()

		_, err := HTTP.Generate(listeners)
		require.Error(t, err)
	})

	t.Run("ok", func(t *testing.T) {
		listeners := testGenerateListeners()

		data, err := HTTP.Generate(listeners)
		require.NoError(t, err)

		t.Log(string(data))
	})

	testsuite.IsDestroyed(t, &HTTP)
}

func TestHTTP_Marshal(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	HTTP := HTTP{}

	HTTP.Request.URL = "http://abc.com/"

	key := bytes.Repeat([]byte{0}, aes.Key256Bit)
	HTTP.AESKey = hex.EncodeToString(key)
	iv := bytes.Repeat([]byte{0}, aes.IVSize)
	HTTP.AESIV = hex.EncodeToString(iv)

	privateKey, err := ed25519.GenerateKey()
	require.NoError(t, err)
	HTTP.PrivateKey = privateKey

	t.Run("ok", func(t *testing.T) {
		data, err := HTTP.Marshal()
		require.NoError(t, err)

		t.Log(string(data))
	})

	t.Run("failed", func(t *testing.T) {
		HTTP.AESIV = "foo iv"
		data, err := HTTP.Marshal()
		require.Error(t, err)
		require.Nil(t, data)
	})

	testsuite.IsDestroyed(t, &HTTP)
}

func TestHTTP_Unmarshal(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	HTTP := HTTP{}

	t.Run("ok", func(t *testing.T) {
		HTTP.Request.URL = "http://abc.com/"

		key := bytes.Repeat([]byte{0}, aes.Key256Bit)
		HTTP.AESKey = hex.EncodeToString(key)
		iv := bytes.Repeat([]byte{0}, aes.IVSize)
		HTTP.AESIV = hex.EncodeToString(iv)

		privateKey, err := ed25519.GenerateKey()
		require.NoError(t, err)
		HTTP.PrivateKey = privateKey

		data, err := HTTP.Marshal()
		require.NoError(t, err)

		err = HTTP.Unmarshal(data)
		require.NoError(t, err)
	})

	t.Run("invalid config", func(t *testing.T) {
		err := HTTP.Unmarshal([]byte{0x00})
		require.Error(t, err)
	})

	t.Run("incorrect config", func(t *testing.T) {
		err := HTTP.Unmarshal(nil)
		require.Error(t, err)
	})

	testsuite.IsDestroyed(t, &HTTP)
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

func TestHTTP_Resolve(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

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
			HTTP := testGenerateHTTP(t)

			listeners := testGenerateListeners()
			info, err := HTTP.Generate(listeners)
			require.NoError(t, err)
			listenersData = info
			t.Logf("(http-IPv4) bootstrap node listeners info: %s\n", info)

			t.Run("IPv4", func(t *testing.T) {
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
				HTTP := NewHTTP(context.Background(), certPool, proxyPool, dnsClient)
				err = HTTP.Unmarshal(data)
				require.NoError(t, err)

				for i := 0; i < 10; i++ {
					resolved, err := HTTP.Resolve()
					require.NoError(t, err)
					resolved = testDecryptListeners(resolved)
					require.Equal(t, listeners, resolved)
				}

				testsuite.IsDestroyed(t, HTTP)
			})

			t.Run("IPv4 with proxy", func(t *testing.T) {
				// run HTTP server
				httpServer := http.Server{
					Addr:    "localhost:80",
					Handler: serveMux,
				}
				_ = testsuite.RunHTTPServer(t, "tcp4", &httpServer)
				defer func() { _ = httpServer.Close() }()

				// config
				HTTP.Request.URL = "http://localhost/"
				HTTP.DNSOpts.Mode = dns.ModeSystem
				HTTP.DNSOpts.Type = dns.TypeIPv4
				HTTP.ProxyTag = testproxy.TagBalance

				// marshal
				data, err := HTTP.Marshal()
				require.NoError(t, err)

				// unmarshal
				HTTP := NewHTTP(context.Background(), certPool, proxyPool, dnsClient)
				err = HTTP.Unmarshal(data)
				require.NoError(t, err)

				for i := 0; i < 10; i++ {
					resolved, err := HTTP.Resolve()
					require.NoError(t, err)
					resolved = testDecryptListeners(resolved)
					require.Equal(t, listeners, resolved)
				}

				testsuite.IsDestroyed(t, HTTP)
			})

			testsuite.IsDestroyed(t, HTTP)
		}

		if testsuite.IPv6Enabled {
			HTTP := testGenerateHTTP(t)

			listeners := testGenerateListeners()
			info, err := HTTP.Generate(listeners)
			require.NoError(t, err)
			listenersData = info
			t.Logf("(http-IPv6) bootstrap node listeners info: %s\n", info)

			t.Run("IPv6", func(t *testing.T) {
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
				HTTP := NewHTTP(context.Background(), certPool, proxyPool, dnsClient)
				err = HTTP.Unmarshal(data)
				require.NoError(t, err)

				for i := 0; i < 10; i++ {
					resolved, err := HTTP.Resolve()
					require.NoError(t, err)
					resolved = testDecryptListeners(resolved)
					require.Equal(t, listeners, resolved)
				}

				// with proxy
				HTTP.ProxyTag = testproxy.TagBalance
				resolved, err := HTTP.Resolve()
				require.NoError(t, err)
				resolved = testDecryptListeners(resolved)
				require.Equal(t, listeners, resolved)

				testsuite.IsDestroyed(t, HTTP)
			})

			t.Run("IPv6 with proxy", func(t *testing.T) {
				// run HTTP server
				httpServer := http.Server{
					Addr:    "localhost:80",
					Handler: serveMux,
				}
				_ = testsuite.RunHTTPServer(t, "tcp6", &httpServer)
				defer func() { _ = httpServer.Close() }()

				// config
				HTTP.Request.URL = "http://localhost/"
				HTTP.DNSOpts.Mode = dns.ModeSystem
				HTTP.DNSOpts.Type = dns.TypeIPv6
				HTTP.ProxyTag = testproxy.TagBalance

				// marshal
				data, err := HTTP.Marshal()
				require.NoError(t, err)

				// unmarshal
				HTTP := NewHTTP(context.Background(), certPool, proxyPool, dnsClient)
				err = HTTP.Unmarshal(data)
				require.NoError(t, err)

				for i := 0; i < 10; i++ {
					resolved, err := HTTP.Resolve()
					require.NoError(t, err)
					resolved = testDecryptListeners(resolved)
					require.Equal(t, listeners, resolved)
				}

				// with proxy
				HTTP.ProxyTag = testproxy.TagBalance
				resolved, err := HTTP.Resolve()
				require.NoError(t, err)
				resolved = testDecryptListeners(resolved)
				require.Equal(t, listeners, resolved)

				testsuite.IsDestroyed(t, HTTP)
			})

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
			HTTP := testGenerateHTTP(t)

			listeners := testGenerateListeners()
			info, err := HTTP.Generate(listeners)
			require.NoError(t, err)
			t.Logf("(https-IPv4) bootstrap node listeners info: %s\n", info)
			listenersData = info

			t.Run("IPv4", func(t *testing.T) {
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
				HTTP := NewHTTP(context.Background(), certPool, proxyPool, dnsClient)
				err = HTTP.Unmarshal(data)
				require.NoError(t, err)

				for i := 0; i < 10; i++ {
					resolved, err := HTTP.Resolve()
					require.NoError(t, err)
					resolved = testDecryptListeners(resolved)
					require.Equal(t, listeners, resolved)
				}

				testsuite.IsDestroyed(t, HTTP)
			})

			t.Run("IPv4 with proxy", func(t *testing.T) {
				// run HTTPS server
				tlsConfig, err := serverCfg.Apply()
				require.NoError(t, err)
				httpsServer := http.Server{
					Addr:      "localhost:443",
					Handler:   serveMux,
					TLSConfig: tlsConfig,
				}
				_ = testsuite.RunHTTPServer(t, "tcp4", &httpsServer)
				defer func() { _ = httpsServer.Close() }()

				// config
				HTTP.Request.URL = "https://localhost/"
				HTTP.DNSOpts.Mode = dns.ModeSystem
				HTTP.DNSOpts.Type = dns.TypeIPv4
				HTTP.Transport.TLSClientConfig = clientCfg
				HTTP.ProxyTag = testproxy.TagBalance

				// marshal
				data, err := HTTP.Marshal()
				require.NoError(t, err)

				// unmarshal
				HTTP := NewHTTP(context.Background(), certPool, proxyPool, dnsClient)
				err = HTTP.Unmarshal(data)
				require.NoError(t, err)

				for i := 0; i < 10; i++ {
					resolved, err := HTTP.Resolve()
					require.NoError(t, err)
					resolved = testDecryptListeners(resolved)
					require.Equal(t, listeners, resolved)
				}

				testsuite.IsDestroyed(t, HTTP)
			})

			testsuite.IsDestroyed(t, HTTP)
		}

		if testsuite.IPv6Enabled {
			HTTP := testGenerateHTTP(t)

			listeners := testGenerateListeners()
			info, err := HTTP.Generate(listeners)
			require.NoError(t, err)
			t.Logf("(https-IPv6) bootstrap node listeners info: %s\n", info)
			listenersData = info

			t.Run("IPv6", func(t *testing.T) {
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
				HTTP := NewHTTP(context.Background(), certPool, proxyPool, dnsClient)
				err = HTTP.Unmarshal(data)
				require.NoError(t, err)

				for i := 0; i < 10; i++ {
					resolved, err := HTTP.Resolve()
					require.NoError(t, err)
					resolved = testDecryptListeners(resolved)
					require.Equal(t, listeners, resolved)
				}

				testsuite.IsDestroyed(t, HTTP)
			})

			t.Run("IPv6 with proxy", func(t *testing.T) {
				// run HTTPS server
				tlsConfig, err := serverCfg.Apply()
				require.NoError(t, err)
				httpsServer := http.Server{
					Addr:      "localhost:443",
					Handler:   serveMux,
					TLSConfig: tlsConfig,
				}
				_ = testsuite.RunHTTPServer(t, "tcp6", &httpsServer)
				defer func() { _ = httpsServer.Close() }()

				// config
				HTTP.Request.URL = "https://localhost/"
				HTTP.DNSOpts.Mode = dns.ModeSystem
				HTTP.DNSOpts.Type = dns.TypeIPv6
				HTTP.Transport.TLSClientConfig = clientCfg
				HTTP.ProxyTag = testproxy.TagBalance

				// marshal
				data, err := HTTP.Marshal()
				require.NoError(t, err)

				// unmarshal
				HTTP := NewHTTP(context.Background(), certPool, proxyPool, dnsClient)
				err = HTTP.Unmarshal(data)
				require.NoError(t, err)

				for i := 0; i < 10; i++ {
					resolved, err := HTTP.Resolve()
					require.NoError(t, err)
					resolved = testDecryptListeners(resolved)
					require.Equal(t, listeners, resolved)
				}

				testsuite.IsDestroyed(t, HTTP)
			})

			testsuite.IsDestroyed(t, HTTP)
		}
	})

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
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

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
	HTTP := HTTP{}
	err = toml.Unmarshal(config, &HTTP)
	require.NoError(t, err)
	err = HTTP.Validate()
	require.NoError(t, err)

	// check zero value
	testsuite.CheckOptions(t, HTTP)

	for _, testdata := range [...]*struct {
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
	} {
		require.Equal(t, testdata.expected, testdata.actual)
	}
}
