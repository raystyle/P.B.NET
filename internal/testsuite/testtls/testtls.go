package testtls

import (
	"crypto/tls"
	"encoding/pem"
	"testing"

	"project/internal/option"
	"project/internal/testsuite"
)

// OptionPair is used to build *options.TLSConfig about server and client.
func OptionPair(t testing.TB) (server, client option.TLSConfig) {
	// certificates about server
	caASN1, certPEMBlock, keyPEMBlock := testsuite.TLSCertificate(t)
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
	caASN1, certPEMBlock, keyPEMBlock = testsuite.TLSCertificate(t)
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
