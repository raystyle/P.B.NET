package testsuite

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/options"
)

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
