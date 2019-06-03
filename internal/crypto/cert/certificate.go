package cert

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"time"

	"project/internal/random"
)

var (
	ERR_INVALID_PEM_BLOCK      = errors.New("invalid PEM block")
	ERR_INVALID_PEM_BLOCK_TYPE = errors.New("invalid PEM block type")
	// NOW = 2018-11-27 00:00:00.000000000 UTC >>>>>> Date of project start
	NOW = time.Time{}.AddDate(2017, 10, 26)
)

// return ca pem and privatekey pem
func Generate_CA() ([]byte, []byte) {
	ca := &x509.Certificate{}
	ca.SerialNumber = big.NewInt(random.Int64())
	ca.Subject = pkix.Name{
		CommonName:   random.String(6 + random.Int(8)),
		Organization: []string{random.String(6 + random.Int(8))},
	}
	ca.NotBefore = NOW
	ca.NotAfter = NOW.AddDate(1000+random.Int(100), random.Int(12), random.Int(31))
	ca.SubjectKeyId = random.Bytes(4)
	ca.BasicConstraintsValid = true
	ca.IsCA = true
	ca.KeyUsage = x509.KeyUsageCertSign
	privatekey, _ := rsa.GenerateKey(rand.Reader, 2048)
	cert_bytes, _ := x509.CreateCertificate(rand.Reader, ca, ca,
		&privatekey.PublicKey, privatekey)
	cert_block := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert_bytes,
	}
	key_block := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privatekey),
	}
	return pem.EncodeToMemory(cert_block), pem.EncodeToMemory(key_block)
}

// return cert pem and privatekey pem
// ip is IP SANS
func Generate(parent *x509.Certificate, pri *rsa.PrivateKey, dns, ip []string) ([]byte, []byte, error) {
	// random cert
	privatekey, _ := rsa.GenerateKey(rand.Reader, 2048)
	cert := &x509.Certificate{}
	cert.SerialNumber = big.NewInt(random.Int64())
	cert.Subject = pkix.Name{
		CommonName:   random.String(6 + random.Int(8)),
		Organization: []string{random.String(6 + random.Int(8))},
	}
	cert.NotBefore = NOW
	cert.NotAfter = NOW.AddDate(1000+random.Int(100), random.Int(12), random.Int(31))
	cert.SubjectKeyId = random.Bytes(4)
	cert.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	cert.KeyUsage = x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment
	cert.DNSNames = dns
	for i := 0; i < len(ip); i++ {
		_ip := net.ParseIP(ip[i])
		if _ip != nil {
			cert.IPAddresses = append(cert.IPAddresses, _ip)
		}
	}
	var (
		cert_bytes []byte
		err        error
	)
	if parent != nil && pri != nil {
		cert_bytes, err = x509.CreateCertificate(
			rand.Reader, cert, parent, &privatekey.PublicKey, pri)
	} else { // self sign
		cert_bytes, err = x509.CreateCertificate(
			rand.Reader, cert, cert, &privatekey.PublicKey, privatekey)
	}
	if err != nil {
		return nil, nil, err
	}
	cert_block := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert_bytes,
	}
	key_block := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privatekey),
	}
	return pem.EncodeToMemory(cert_block), pem.EncodeToMemory(key_block), nil
}

func Parse(cert []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(cert)
	if block == nil {
		return nil, ERR_INVALID_PEM_BLOCK
	}
	if block.Type != "CERTIFICATE" {
		return nil, ERR_INVALID_PEM_BLOCK_TYPE
	}
	return x509.ParseCertificate(block.Bytes)
}
