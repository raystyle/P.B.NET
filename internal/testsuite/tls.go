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

	"project/internal/option"
	"project/internal/random"
)

// TLSCertificate is used to generate CA ASN1 data, signed certificate
func TLSCertificate(t testing.TB) (caASN1 []byte, cPEMBlock, cPriPEMBlock []byte) {
	// generate CA certificate
	caCert := &x509.Certificate{
		SerialNumber: big.NewInt(random.Int64()),
		SubjectKeyId: random.Bytes(4),
		NotBefore:    time.Now().AddDate(0, 0, -1),
		NotAfter:     time.Now().AddDate(0, 0, 1),
	}
	caCert.Subject.CommonName = "testsuite CA " + random.String(4)
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

	// sign certificate
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(random.Int64()),
		SubjectKeyId: random.Bytes(4),
		NotBefore:    time.Now().AddDate(0, 0, -1),
		NotAfter:     time.Now().AddDate(0, 0, 1),
	}
	cert.Subject.CommonName = "testsuite certificate " + random.String(4)
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
	// certificates about server
	caASN1, certPEMBlock, keyPEMBlock := TLSCertificate(t)
	caCert, err := x509.ParseCertificate(caASN1)
	require.NoError(t, err)
	tlsCert, err := tls.X509KeyPair(certPEMBlock, keyPEMBlock)
	require.NoError(t, err)

	server = &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}
	client = &tls.Config{RootCAs: x509.NewCertPool()}
	client.RootCAs.AddCert(caCert)

	// certificates about client
	caASN1, certPEMBlock, keyPEMBlock = TLSCertificate(t)
	caCert, err = x509.ParseCertificate(caASN1)
	require.NoError(t, err)
	tlsCert, err = tls.X509KeyPair(certPEMBlock, keyPEMBlock)
	require.NoError(t, err)

	server.ClientCAs = x509.NewCertPool()
	server.ClientCAs.AddCert(caCert)
	client.Certificates = []tls.Certificate{tlsCert}
	return
}

// TLSConfigOptionPair is used to build server and client *options.TLSConfig
func TLSConfigOptionPair(t testing.TB) (server, client option.TLSConfig) {
	// certificates about server
	caASN1, certPEMBlock, keyPEMBlock := TLSCertificate(t)
	caPEMBlock := string(pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caASN1,
	}))
	server.Certificates = []option.X509KeyPair{
		{
			Cert: string(certPEMBlock),
			Key:  string(keyPEMBlock),
		},
	}
	server.ClientAuth = tls.RequireAndVerifyClientCert
	server.ServerSide = true
	client.RootCAs = []string{caPEMBlock}

	// certificates about client
	caASN1, certPEMBlock, keyPEMBlock = TLSCertificate(t)
	caPEMBlock = string(pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caASN1,
	}))
	server.ClientCAs = []string{caPEMBlock}
	client.Certificates = []option.X509KeyPair{
		{
			Cert: string(certPEMBlock),
			Key:  string(keyPEMBlock),
		},
	}
	return
}
