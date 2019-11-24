package cert

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"

	"project/internal/crypto/rand"
	"project/internal/random"
)

// Config include configuration about generate certificate
type Config struct {
	Algorithm   string    `toml:"algorithm"` // rsa, ecdsa, ed25519
	DNSNames    []string  `toml:"dns_names"`
	IPAddresses []string  `toml:"ip_addresses"` // IP SANS
	NotAfter    time.Time `toml:"not_after"`
	Subject     Subject   `toml:"subject"`
}

// Subject certificate subject info
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

// KeyPair include certificate, certificate ASN1 data and private key
type KeyPair struct {
	Certificate *x509.Certificate
	PrivateKey  interface{}
	asn1Data    []byte // Certificate
}

// EncodeToPEM is used to encode certificate and private key to PEM data
func (kp *KeyPair) EncodeToPEM() (cert, key []byte) {
	certBlock := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: kp.asn1Data,
	}
	b, _ := x509.MarshalPKCS8PrivateKey(kp.PrivateKey)
	keyBlock := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: b,
	}
	return pem.EncodeToMemory(certBlock), pem.EncodeToMemory(keyBlock)
}

// EncodeToPEM is used to generate tls certificate
func (kp *KeyPair) TLSCertificate() (tls.Certificate, error) {
	return tls.X509KeyPair(kp.EncodeToPEM())
}

func genCertificate(cfg *Config) *x509.Certificate {
	cert := x509.Certificate{}
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
	now := time.Time{}.AddDate(2017, 10, 26) // 2018-11-27
	years := 10 + random.Int(10)
	months := random.Int(12)
	days := random.Int(31)
	cert.NotBefore = now.AddDate(-years, -months, -days)
	if cfg.NotAfter.Equal(time.Time{}) {
		years = 10 + random.Int(10)
		months = random.Int(12)
		days = random.Int(31)
		cert.NotAfter = now.AddDate(years, months, days)
	}
	return &cert
}

func genKey(algorithm string) (interface{}, interface{}, error) {
	switch algorithm {
	case "rsa":
		privateKey, _ := rsa.GenerateKey(rand.Reader, 4096)
		return privateKey, &privateKey.PublicKey, nil
	case "ecdsa":
		privateKey, _ := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
		return privateKey, &privateKey.PublicKey, nil
	case "ed25519":
		publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)
		return privateKey, publicKey, nil
	default:
		return nil, nil, fmt.Errorf("unknown algorithm: %s", algorithm)
	}
}

// GenerateCA is used to generate a CA certificate from Config
func GenerateCA(cfg *Config) (*KeyPair, error) {
	ca := genCertificate(cfg)
	ca.KeyUsage = x509.KeyUsageCertSign
	ca.BasicConstraintsValid = true
	ca.IsCA = true

	privateKey, publicKey, err := genKey(cfg.Algorithm)
	if err != nil {
		return nil, err
	}

	asn1Data, _ := x509.CreateCertificate(
		rand.Reader, ca, ca, publicKey, privateKey)
	ca, _ = x509.ParseCertificate(asn1Data)

	return &KeyPair{
		Certificate: ca,
		PrivateKey:  privateKey,
		asn1Data:    asn1Data,
	}, nil
}

// Generate is used to generate a signed certificate by CA or self
func Generate(parent *x509.Certificate, pri interface{}, cfg *Config) (*KeyPair, error) {
	cert := genCertificate(cfg)
	cert.KeyUsage = x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment

	// check dns
	dn := cfg.DNSNames
	for i := 0; i < len(dn); i++ {
		if !isDomainName(dn[i]) {
			return nil, fmt.Errorf("%s is not a domain name", dn[i])
		}
		cert.DNSNames = append(cert.DNSNames, dn[i])
	}

	// check ip
	ips := cfg.IPAddresses
	for i := 0; i < len(ips); i++ {
		ip := net.ParseIP(ips[i])
		if ip == nil {
			return nil, fmt.Errorf("%s is not a IP", ips[i])
		}
		cert.IPAddresses = append(cert.IPAddresses, ip)
	}

	// generate certificate
	privateKey, publicKey, err := genKey(cfg.Algorithm)
	if err != nil {
		return nil, err
	}
	var asn1Data []byte
	if parent != nil && pri != nil { // by CA
		asn1Data, err = x509.CreateCertificate(rand.Reader, cert, parent, publicKey, pri)
	} else { // self-sign
		asn1Data, err = x509.CreateCertificate(rand.Reader, cert, cert, publicKey, privateKey)
	}
	if err != nil {
		return nil, err
	}
	cert, _ = x509.ParseCertificate(asn1Data)
	return &KeyPair{
		Certificate: cert,
		PrivateKey:  privateKey,
		asn1Data:    asn1Data,
	}, nil
}

// from GOROOT/src/net/dnsclient.go

// checks if a string is a presentation-format domain name
// (currently restricted to hostname-compatible "preferred name" LDH labels and
// SRV-like "underscore labels"; see golang.org/issue/12421).
func isDomainName(s string) bool {
	// See RFC 1035, RFC 3696.
	// Presentation format has dots before every label except the first, and the
	// terminal empty label is optional here because we assume fully-qualified
	// (absolute) input. We must therefore reserve space for the first and last
	// labels' length octets in wire format, where they are necessary and the
	// maximum total length is 255.
	// So our _effective_ maximum is 253, but 254 is not rejected if the last
	// character is a dot.
	l := len(s)
	if l == 0 || l > 254 || l == 254 && s[l-1] != '.' {
		return false
	}
	last := byte('.')
	nonNumeric := false // true once we've seen a letter or hyphen
	partLen := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		default:
			return false
		case 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || c == '_':
			nonNumeric = true
			partLen++
		case '0' <= c && c <= '9':
			// fine
			partLen++
		case c == '-':
			// Byte before dash cannot be dot.
			if last == '.' {
				return false
			}
			partLen++
			nonNumeric = true
		case c == '.':
			// Byte before dot cannot be dot, dash.
			if last == '.' || last == '-' {
				return false
			}
			if partLen > 63 || partLen == 0 {
				return false
			}
			partLen = 0
		}
		last = c
	}
	if last == '-' || partLen > 63 {
		return false
	}
	return nonNumeric
}
