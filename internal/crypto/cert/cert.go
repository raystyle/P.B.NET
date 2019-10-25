package cert

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"runtime"
	"time"

	"project/internal/crypto/rand"
	"project/internal/crypto/rsa"
	"project/internal/dns"
	"project/internal/random"
)

var now = time.Time{}.AddDate(2017, 10, 26) // 2018-11-27

type Config struct {
	Subject     Subject   `toml:"subject"`
	NotAfter    time.Time `toml:"not_after"`
	DNSNames    []string  `toml:"dns_names"`
	IPAddresses []string  `toml:"ip_addresses"` // IP SANS
}

type Subject struct {
	CommonName         string   `toml:"common_name"`
	SerialNumber       string   `toml:"serial_number"`
	Country            []string `toml:"country"`
	Organization       []string `toml:"organization"`
	OrganizationalUnit []string `toml:"organizational_unit"`
	Locality           []string `toml:"locality"`
	Province           []string `toml:"province"`
	StreetAddress      []string `toml:"street_address"`
	PostalCode         []string `toml:"postal_code"`
}

type KeyPair struct {
	Certificate *x509.Certificate
	PrivateKey  *rsa.PrivateKey
	certBytes   []byte
}

func (kp *KeyPair) EncodeToPEM() (cert, key []byte) {
	certBlock := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: kp.certBytes,
	}
	keyBlock := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: rsa.ExportPrivateKey(kp.PrivateKey),
	}
	return pem.EncodeToMemory(certBlock), pem.EncodeToMemory(keyBlock)
}

func (kp *KeyPair) TLSCertificate() (tls.Certificate, error) {
	return tls.X509KeyPair(kp.EncodeToPEM())
}

func generate(cfg *Config) *x509.Certificate {
	if cfg == nil {
		cfg = new(Config)
	}
	cert := &x509.Certificate{}
	cert.SerialNumber = big.NewInt(random.Int64())
	cert.SubjectKeyId = random.Bytes(4)
	// Subject.CommonName
	if cfg.Subject.CommonName == "" {
		cert.Subject.CommonName = random.String(6 + random.Int(8))
	} else {
		cert.Subject.CommonName = cfg.Subject.CommonName
	}
	// Subject.Organization
	if cfg.Subject.Organization == nil {
		cert.Subject.Organization = []string{random.String(6 + random.Int(8))}
	} else {
		copy(cert.Subject.Organization, cfg.Subject.Organization)
	}
	cert.Subject.Country = make([]string, len(cfg.Subject.Country))
	copy(cert.Subject.Country, cfg.Subject.Country)
	cert.Subject.OrganizationalUnit = make([]string, len(cfg.Subject.OrganizationalUnit))
	copy(cert.Subject.OrganizationalUnit, cfg.Subject.OrganizationalUnit)
	cert.Subject.Locality = make([]string, len(cfg.Subject.Locality))
	copy(cert.Subject.Locality, cfg.Subject.Locality)
	cert.Subject.Province = make([]string, len(cfg.Subject.Province))
	copy(cert.Subject.Province, cfg.Subject.Province)
	cert.Subject.StreetAddress = make([]string, len(cfg.Subject.StreetAddress))
	copy(cert.Subject.StreetAddress, cfg.Subject.StreetAddress)
	cert.Subject.PostalCode = make([]string, len(cfg.Subject.PostalCode))
	copy(cert.Subject.PostalCode, cfg.Subject.PostalCode)
	cert.Subject.SerialNumber = cfg.Subject.SerialNumber

	// set time
	years := 1 + random.Int(4)
	months := random.Int(12)
	days := random.Int(31)
	cert.NotBefore = now.AddDate(-years, -months, -days)

	if cfg.NotAfter.Equal(time.Time{}) {
		years = 10 + random.Int(10)
		months = random.Int(12)
		days = random.Int(31)
		cert.NotAfter = now.AddDate(years, months, days)
	}
	return cert
}

// return certificate pem and private key pem
func GenerateCA(cfg *Config) (*KeyPair, error) {
	ca := generate(cfg)
	ca.KeyUsage = x509.KeyUsageCertSign
	ca.BasicConstraintsValid = true
	ca.IsCA = true

	privateKey, _ := rsa.GenerateKey(2048)
	certBytes, _ := x509.CreateCertificate(rand.Reader, ca, ca,
		&privateKey.PublicKey, privateKey)

	caCert, _ := x509.ParseCertificate(certBytes)

	return &KeyPair{
		Certificate: caCert,
		PrivateKey:  privateKey,
		certBytes:   certBytes,
	}, nil
}

// Generate is used to generate tls.KeyPair
func Generate(parent *x509.Certificate, pri *rsa.PrivateKey, cfg *Config) (*KeyPair, error) {
	cert := generate(cfg)
	cert.KeyUsage = x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment

	dn := cfg.DNSNames
	for i := 0; i < len(dn); i++ {
		if !dns.IsDomainName(dn[i]) {
			return nil, fmt.Errorf("%s is not a domain name", dn[i])
		}
		cert.DNSNames = append(cert.DNSNames, dn[i])
	}

	ips := cfg.IPAddresses
	for i := 0; i < len(ips); i++ {
		ip := net.ParseIP(ips[i])
		if ip == nil {
			return nil, fmt.Errorf("%s is not a IP", ips[i])
		}
		cert.IPAddresses = append(cert.IPAddresses, ip)
	}

	privateKey, _ := rsa.GenerateKey(2048)

	var (
		certBytes []byte
		err       error
	)
	if parent != nil && pri != nil {
		certBytes, err = x509.CreateCertificate(
			rand.Reader, cert, parent, &privateKey.PublicKey, pri)
	} else { // self sign
		certBytes, err = x509.CreateCertificate(
			rand.Reader, cert, cert, &privateKey.PublicKey, privateKey)
	}
	if err != nil {
		return nil, err
	}

	sCert, _ := x509.ParseCertificate(certBytes)

	return &KeyPair{
		Certificate: sCert,
		PrivateKey:  privateKey,
		certBytes:   certBytes,
	}, nil
}

func SystemCertPool() (*x509.CertPool, error) {
	if runtime.GOOS == "windows" {
		return systemCertPool()
	}
	return x509.SystemCertPool()
}
