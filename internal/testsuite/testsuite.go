package testsuite

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io"
	"io/ioutil"
	"math/big"
	"net"
	"net/http"
	_ "net/http/pprof"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/options"
)

var (
	ipv6 bool
)

func init() {
	go func() { _ = http.ListenAndServe("localhost:19993", nil) }()
	// check IPv6 available
	conn, err := net.Dial("tcp6", "cloudflare-dns.com:443")
	if err == nil {
		_ = conn.Close()
		ipv6 = true
	}
}

// IPv6 is used to determine whether IPv6 is available
func IPv6() bool {
	return ipv6
}

// Bytes is used to generate test data: []byte{0, 1, .... 254, 255}
func Bytes() []byte {
	testdata := make([]byte, 256)
	for i := 0; i < 256; i++ {
		testdata[i] = byte(i)
	}
	return testdata
}

func isDestroyed(object interface{}) bool {
	destroyed := make(chan struct{})
	runtime.SetFinalizer(object, func(_ interface{}) {
		close(destroyed)
	})
	for i := 0; i < 40; i++ {
		runtime.GC()
		select {
		case <-destroyed:
			return true
		case <-time.After(25 * time.Millisecond):
		}
	}
	return false
}

// IsDestroyed is used to check if the object has been recycled by the GC
func IsDestroyed(t testing.TB, object interface{}) {
	require.True(t, isDestroyed(object), "object not destroyed")
}

// TLSCertificate is used to generate CA ASN1 data, signed certificate
func TLSCertificate(t testing.TB) (caASN1 []byte, cPEMBlock, cPriPEMBlock []byte) {
	// generate CA
	caCert := &x509.Certificate{
		SerialNumber: big.NewInt(1234),
		SubjectKeyId: []byte{0x00, 0x01, 0x02, 0x03},
		NotBefore:    time.Now().AddDate(0, 0, -1),
		NotAfter:     time.Now().AddDate(0, 0, 1),
	}
	caCert.Subject.CommonName = "testsuite CA"
	caCert.KeyUsage = x509.KeyUsageCertSign
	caCert.BasicConstraintsValid = true
	caCert.IsCA = true
	caPri, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	caPub := &caPri.PublicKey
	caASN1, err = x509.CreateCertificate(rand.Reader, caCert, caCert, caPub, caPri)
	require.NoError(t, err)
	caCert, err = x509.ParseCertificate(caASN1)
	require.NoError(t, err)
	// sign cert
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(5678),
		SubjectKeyId: []byte{0x04, 0x05, 0x06, 0x07},
		NotBefore:    time.Now().AddDate(0, 0, -1),
		NotAfter:     time.Now().AddDate(0, 0, 1),
	}
	cert.Subject.CommonName = "testsuite certificate"
	cert.KeyUsage = x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment
	cert.DNSNames = []string{"localhost"}
	cert.IPAddresses = []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
	cPri, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	cPub := &cPri.PublicKey
	cASN1, err := x509.CreateCertificate(rand.Reader, cert, caCert, cPub, caPri)
	require.NoError(t, err)
	cPEMBlock = pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cASN1,
	})
	cPriPEMBlock = pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(cPri),
	})
	return
}

// TLSConfigPair is used to build server and client *tls.Config
func TLSConfigPair(t testing.TB) (server, client *tls.Config) {
	caASN1, cPEMBlock, cPriPEMBlock := TLSCertificate(t)
	// ca certificate
	caCert, err := x509.ParseCertificate(caASN1)
	require.NoError(t, err)
	// server tls config
	tlsCert, err := tls.X509KeyPair(cPEMBlock, cPriPEMBlock)
	require.NoError(t, err)
	server = &tls.Config{Certificates: []tls.Certificate{tlsCert}}
	// client tls config
	client = &tls.Config{RootCAs: x509.NewCertPool()}
	client.RootCAs.AddCert(caCert)
	return
}

// TLSConfigOptionPair is used to build server and client *options.TLSConfig
func TLSConfigOptionPair(t testing.TB) (server, client *options.TLSConfig) {
	caASN1, cPEMBlock, cPriPEMBlock := TLSCertificate(t)
	caPEMBlock := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caASN1,
	})
	// server *options.TLSConfig
	server = &options.TLSConfig{Certificates: make([]options.X509KeyPair, 1)}
	server.Certificates[0] = options.X509KeyPair{
		Cert: string(cPEMBlock),
		Key:  string(cPriPEMBlock),
	}
	// client *options.TLSConfig
	client = &options.TLSConfig{RootCAs: make([]string, 1)}
	client.RootCAs[0] = string(caPEMBlock)
	return
}

// ListenerAndDial is used to test net.Listener and Dial
func ListenerAndDial(t testing.TB, l net.Listener, d func() (net.Conn, error), close bool) {
	wg := sync.WaitGroup{}
	for i := 0; i < 3; i++ {
		var server net.Conn
		wg.Add(1)
		go func() {
			defer wg.Done()
			var err error
			server, err = l.Accept()
			require.NoError(t, err)
		}()
		client, err := d()
		require.NoError(t, err)
		wg.Wait()
		Conn(t, server, client, close)
		t.Log("") // new line for Conn
	}
	require.NoError(t, l.Close())
	IsDestroyed(t, l)
}

// Conn is used to test client & server Conn
//
// if close == true, IsDestroyed will be run after Conn.Close()
// if Conn about TLS and use net.Pipe(), set close = false
// server, client := net.Pipe()
// tlsServer = tls.Server(server, tlsConfig)
// tlsClient = tls.Client(client, tlsConfig)
// Conn(t, tlsServer, tlsClient, false) must set false
func Conn(t testing.TB, server, client net.Conn, close bool) {
	// Addr
	t.Log("server remote:", server.RemoteAddr().Network(), server.RemoteAddr())
	t.Log("client local:", client.LocalAddr().Network(), client.LocalAddr())
	t.Log("server local:", server.LocalAddr().Network(), server.LocalAddr())
	t.Log("client remote:", client.RemoteAddr().Network(), client.RemoteAddr())
	// skip udp, because client.LocalAddr() always net.IPv4zero or net.IPv6zero
	if !strings.HasPrefix(server.RemoteAddr().Network(), "udp") {
		require.Equal(t, server.RemoteAddr().Network(), client.LocalAddr().Network())
		require.Equal(t, server.RemoteAddr().String(), client.LocalAddr().String())
	}
	require.Equal(t, server.LocalAddr().Network(), client.RemoteAddr().Network())
	require.Equal(t, server.LocalAddr().String(), client.RemoteAddr().String())

	// Read() and Write()
	write := func(conn net.Conn) {
		data := Bytes()
		_, err := conn.Write(data)
		require.NoError(t, err)
		require.Equal(t, Bytes(), data)
	}
	read := func(conn net.Conn) {
		data := make([]byte, 256)
		_, err := io.ReadFull(conn, data)
		require.NoError(t, err)
		require.Equal(t, Bytes(), data)
	}
	wg := sync.WaitGroup{}
	wg.Add(1)
	// server
	go func() {
		defer wg.Done()
		read(server)
		write(server)
		write(server)
		read(server)
	}()
	// client
	write(client)
	read(client)
	read(client)
	write(client)
	wg.Wait()

	// about Deadline()
	require.NoError(t, server.SetDeadline(time.Now().Add(10*time.Millisecond)))
	require.NoError(t, client.SetDeadline(time.Now().Add(10*time.Millisecond)))
	time.Sleep(20 * time.Millisecond)
	buf := []byte{0, 0, 0, 0}
	_, err := client.Write(buf)
	require.Error(t, err)
	_, err = server.Read(buf)
	require.Error(t, err)

	require.NoError(t, server.SetReadDeadline(time.Now().Add(10*time.Millisecond)))
	require.NoError(t, client.SetWriteDeadline(time.Now().Add(10*time.Millisecond)))
	time.Sleep(20 * time.Millisecond)
	_, err = client.Write(buf)
	require.Error(t, err)
	_, err = server.Read(buf)
	require.Error(t, err)

	// recovery deadline
	require.NoError(t, server.SetDeadline(time.Time{}))
	require.NoError(t, client.SetDeadline(time.Time{}))

	// Close()
	if close {
		require.NoError(t, server.Close())
		require.NoError(t, client.Close())

		IsDestroyed(t, server)
		IsDestroyed(t, client)
	}
}

// RunHTTPServer is used to start a http or https server
func RunHTTPServer(t testing.TB, network string, server *http.Server) string {
	listener, err := net.Listen(network, server.Addr)
	require.NoError(t, err)

	// run
	go func() {
		if server.TLSConfig != nil {
			_ = server.ServeTLS(listener, "", "")
		} else {
			_ = server.Serve(listener)
		}
	}()

	// get port
	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	return port
}

// for test proxy client
type proxyClient interface {
	Dial(network, address string) (net.Conn, error)
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
	DialTimeout(network, address string, timeout time.Duration) (net.Conn, error)
	Connect(conn net.Conn, network, address string) (net.Conn, error)
	HTTP(t *http.Transport)
	Timeout() time.Duration
	Server() (network string, address string)
	Info() string
}

// ProxyClient is used to test proxy client
func ProxyClient(t testing.TB, server io.Closer, client proxyClient) {
	defer func() {
		require.NoError(t, server.Close())
		IsDestroyed(t, server)
	}()
	wg := sync.WaitGroup{}

	// test Dial and DialTimeout
	wg.Add(1)
	go func() {
		defer wg.Done()
		var targets = []string{"180.101.49.12:80", "www.baidu.com:443"}
		if IPv6() && !strings.Contains(client.Info(), "socks4") {
			targets = append(targets, "[2606:4700::6810:f9f9]:443")
		}
		for _, target := range targets {
			wg.Add(1)
			go func(target string) {
				defer wg.Done()
				conn, err := client.Dial("tcp", target)
				require.NoError(t, err)
				_ = conn.Close()
				conn, err = client.DialTimeout("tcp", target, 0)
				require.NoError(t, err)
				_ = conn.Close()
			}(target)
		}
	}()

	// test DialContext (http)
	wg.Add(1)
	go func() {
		defer wg.Done()
		transport := http.Transport{DialContext: client.DialContext}
		client := http.Client{
			Transport: &transport,
			Timeout:   time.Minute,
		}
		defer client.CloseIdleConnections()
		resp, err := client.Get("http://www.msftconnecttest.com/connecttest.txt")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		defer func() { _ = resp.Body.Close() }()
		b, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, "Microsoft Connect Test", string(b))
	}()

	// test DialContext (https)
	wg.Add(1)
	go func() {
		defer wg.Done()
		transport := http.Transport{DialContext: client.DialContext}
		client := http.Client{
			Transport: &transport,
			Timeout:   time.Minute,
		}
		defer client.CloseIdleConnections()
		resp, err := client.Get("https://github.com/robots.txt")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		defer func() { _ = resp.Body.Close() }()
		b, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, "# If you w", string(b)[:10])
	}()

	// test HTTP with http target
	wg.Add(1)
	go func() {
		defer wg.Done()
		transport := &http.Transport{}
		client.HTTP(transport)
		client := http.Client{
			Transport: transport,
			Timeout:   time.Minute,
		}
		defer client.CloseIdleConnections()
		resp, err := client.Get("http://www.msftconnecttest.com/connecttest.txt")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		b, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, "Microsoft Connect Test", string(b))
	}()

	// test HTTP with https target
	wg.Add(1)
	go func() {
		defer wg.Done()
		transport := &http.Transport{}
		client.HTTP(transport)
		client := http.Client{
			Transport: transport,
			Timeout:   time.Minute,
		}
		defer client.CloseIdleConnections()
		resp, err := client.Get("https://github.com/robots.txt")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		b, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, "# If you w", string(b)[:10])
	}()

	wg.Wait()

	t.Log("timeout:", client.Timeout())
	network, address := client.Server()
	t.Log("server:", network, address)
	t.Log("info:", client.Info())
	IsDestroyed(t, client)
}

// ProxyClientWithUnreachableProxyServer is used to test proxy client that
// can't connect proxy server
func ProxyClientWithUnreachableProxyServer(t testing.TB, client proxyClient) {
	// unknown network
	_, err := client.Dial("foo", "")
	require.Error(t, err)
	t.Log("Dial:\n", err)
	_, err = client.DialContext(context.Background(), "foo", "")
	require.Error(t, err)
	t.Log("DialContext:\n", err)
	_, err = client.DialTimeout("foo", "", time.Second)
	require.Error(t, err)
	t.Log("DialTimeout:\n", err)
	_, err = client.Connect(nil, "foo", "")
	require.Error(t, err)
	t.Log("Connect:\n", err)

	// unreachable proxy server
	_, err = client.Dial("tcp", "")
	require.Error(t, err)
	t.Log("Dial:\n", err)
	_, err = client.DialContext(context.Background(), "tcp", "")
	require.Error(t, err)
	t.Log("DialContext:\n", err)
	_, err = client.DialTimeout("tcp", "", time.Second)
	require.Error(t, err)
	t.Log("DialTimeout:\n", err)
	IsDestroyed(t, client)
}

// ProxyClientWithUnreachableTarget is used to test proxy client that
// connect unreachable target
func ProxyClientWithUnreachableTarget(t testing.TB, server io.Closer, client proxyClient) {
	defer func() {
		require.NoError(t, server.Close())
		IsDestroyed(t, server)
	}()
	const unreachableTarget = "0.0.0.0:1"
	_, err := client.Dial("tcp", unreachableTarget)
	require.Error(t, err)
	t.Log("Dial -> Connect:\n", err)
	_, err = client.DialContext(context.Background(), "tcp", unreachableTarget)
	require.Error(t, err)
	t.Log("DialContext -> Connect:\n", err)
	_, err = client.DialTimeout("tcp", unreachableTarget, time.Second)
	require.Error(t, err)
	t.Log("DialTimeout -> Connect:\n", err)
	IsDestroyed(t, client)
}
