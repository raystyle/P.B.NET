package cert

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"time"

	"project/internal/crypto/rand"
	"project/internal/crypto/rsa"
	"project/internal/random"
)

var (
	ErrInvalidPEMBlock     = errors.New("invalid PEM block")
	ErrInvalidPEMBlockType = errors.New("invalid PEM block type")

	// NOW = 2018-11-27 00:00:00.000000000 UTC >>>>>> date of project start
	NOW = time.Time{}.AddDate(2017, 10, 26)
)

type Config struct {
	Subject     Subject
	NotAfter    time.Time
	DNSNames    []string
	IPAddresses []string // IP SANS
}

type Subject struct {
	CommonName         string
	SerialNumber       string
	Country            []string
	Organization       []string
	OrganizationalUnit []string
	Locality           []string
	Province           []string
	StreetAddress      []string
	PostalCode         []string
}

func generate(c *Config) *x509.Certificate {
	cert := &x509.Certificate{}
	cert.SerialNumber = big.NewInt(random.Int64())
	cert.SubjectKeyId = random.Bytes(4)
	// Subject.CommonName
	if c.Subject.CommonName == "" {
		cert.Subject.CommonName = random.String(6 + random.Int(8))
	} else {
		cert.Subject.CommonName = c.Subject.CommonName
	}
	// Subject.Organization
	if c.Subject.Organization == nil {
		cert.Subject.Organization = []string{random.String(6 + random.Int(8))}
	} else {
		copy(cert.Subject.Organization, c.Subject.Organization)
	}
	cert.Subject.Country = make([]string, len(c.Subject.Country))
	copy(cert.Subject.Country, c.Subject.Country)
	cert.Subject.OrganizationalUnit = make([]string, len(c.Subject.OrganizationalUnit))
	copy(cert.Subject.OrganizationalUnit, c.Subject.OrganizationalUnit)
	cert.Subject.Locality = make([]string, len(c.Subject.Locality))
	copy(cert.Subject.Locality, c.Subject.Locality)
	cert.Subject.Province = make([]string, len(c.Subject.Province))
	copy(cert.Subject.Province, c.Subject.Province)
	cert.Subject.StreetAddress = make([]string, len(c.Subject.StreetAddress))
	copy(cert.Subject.StreetAddress, c.Subject.StreetAddress)
	cert.Subject.PostalCode = make([]string, len(c.Subject.PostalCode))
	copy(cert.Subject.PostalCode, c.Subject.PostalCode)
	cert.Subject.SerialNumber = c.Subject.SerialNumber
	// time
	cert.NotBefore = NOW
	if c.NotAfter.Equal(time.Time{}) {
		years := 10 + random.Int(100)
		months := random.Int(12)
		days := random.Int(31)
		cert.NotAfter = NOW.AddDate(years, months, days)
	}
	return cert
}

// return certificate pem and privatekey pem
func GenerateCA(c *Config) (cert []byte, pri []byte) {
	ca := generate(c)
	ca.KeyUsage = x509.KeyUsageCertSign
	ca.BasicConstraintsValid = true
	ca.IsCA = true
	privateKey, _ := rsa.GenerateKey(2048)
	certBytes, _ := x509.CreateCertificate(rand.Reader, ca, ca,
		&privateKey.PublicKey, privateKey)
	certBlock := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	}
	keyBlock := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: rsa.ExportPrivateKey(privateKey),
	}
	return pem.EncodeToMemory(certBlock), pem.EncodeToMemory(keyBlock)
}

// return cert pem and privatekey pem , ip is IP SANS
func Generate(parent *x509.Certificate, pri *rsa.PrivateKey, c *Config) ([]byte, []byte, error) {
	cert := generate(c)
	cert.KeyUsage = x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment
	cert.DNSNames = make([]string, len(c.DNSNames))
	copy(cert.DNSNames, c.DNSNames)
	ips := c.IPAddresses
	for i := 0; i < len(ips); i++ {
		ip := net.ParseIP(ips[i])
		if ip != nil {
			cert.IPAddresses = append(cert.IPAddresses, ip)
		}
	}
	privatekey, _ := rsa.GenerateKey(2048)
	var (
		certBytes []byte
		err       error
	)
	if parent != nil && pri != nil {
		certBytes, err = x509.CreateCertificate(
			rand.Reader, cert, parent, &privatekey.PublicKey, pri)
	} else { // self sign
		certBytes, err = x509.CreateCertificate(
			rand.Reader, cert, cert, &privatekey.PublicKey, privatekey)
	}
	if err != nil {
		return nil, nil, err
	}
	certBlock := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	}
	keyBlock := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: rsa.ExportPrivateKey(privatekey),
	}
	return pem.EncodeToMemory(certBlock), pem.EncodeToMemory(keyBlock), nil
}

func Parse(cert []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(cert)
	if block == nil {
		return nil, ErrInvalidPEMBlock
	}
	if block.Type != "CERTIFICATE" {
		return nil, ErrInvalidPEMBlockType
	}
	return x509.ParseCertificate(block.Bytes)
}
