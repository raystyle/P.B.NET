package testutil

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io"
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
)

// PPROF is used to open pprof
func PPROF() {
	go func() { _ = http.ListenAndServe("localhost:1999", nil) }()
}

// Bytes is used to generate test data: []byte{0, 1, .... 254, 255}
func Bytes() []byte {
	testdata := make([]byte, 256)
	for i := 0; i < 256; i++ {
		testdata[i] = byte(i)
	}
	return testdata
}

func isDestroyed(object interface{}, gcNum int) bool {
	destroyed := false
	runtime.SetFinalizer(object, func(_ interface{}) {
		destroyed = true
	})
	for i := 0; i < gcNum; i++ {
		runtime.GC()
		time.Sleep(100 * time.Millisecond)
	}
	return destroyed
}

// IsDestroyed is used to check if the object has been recycled by the GC
func IsDestroyed(t testing.TB, object interface{}, gcNum int) {
	require.True(t, isDestroyed(object, gcNum), "object not destroyed")
}

// TLSConfigPair is used to build server and client tls.Config
func TLSConfigPair(t testing.TB) (server, client *tls.Config) {
	// generate CA
	caCert := &x509.Certificate{
		SerialNumber: big.NewInt(1234),
		SubjectKeyId: []byte{0x00, 0x01, 0x02, 0x03},
		NotBefore:    time.Now().AddDate(0, 0, -1),
		NotAfter:     time.Now().AddDate(0, 0, 1),
	}
	caCert.Subject.CommonName = "testutil CA"
	caCert.KeyUsage = x509.KeyUsageCertSign
	caCert.BasicConstraintsValid = true
	caCert.IsCA = true
	caPri, _ := rsa.GenerateKey(rand.Reader, 2048)
	caBytes, _ := x509.CreateCertificate(rand.Reader, caCert, caCert,
		&caPri.PublicKey, caPri)
	caCert, _ = x509.ParseCertificate(caBytes)

	// sign cert
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(5678),
		SubjectKeyId: []byte{0x04, 0x05, 0x06, 0x07},
		NotBefore:    time.Now().AddDate(0, 0, -1),
		NotAfter:     time.Now().AddDate(0, 0, 1),
	}
	cert.Subject.CommonName = "testutil certificate"
	cert.KeyUsage = x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment
	cert.DNSNames = []string{"localhost"}
	cert.IPAddresses = []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
	cPri, _ := rsa.GenerateKey(rand.Reader, 2048)
	cASN1, _ := x509.CreateCertificate(rand.Reader, cert, caCert,
		&cPri.PublicKey, caPri)

	cPEMBlock := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cASN1,
	})
	cPriPEMBlock := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(cPri),
	})

	// server tls config
	tlsCert, err := tls.X509KeyPair(cPEMBlock, cPriPEMBlock)
	require.NoError(t, err)
	server = &tls.Config{Certificates: []tls.Certificate{tlsCert}}
	// client tls config
	client = &tls.Config{RootCAs: x509.NewCertPool()}
	client.RootCAs.AddCert(caCert)
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
	IsDestroyed(t, l, 1)
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

		IsDestroyed(t, server, 1)
		IsDestroyed(t, client, 1)
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
