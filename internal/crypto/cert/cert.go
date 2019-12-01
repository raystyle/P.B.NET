package cert

import (
	"bytes"
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
	"project/internal/logger"
	"project/internal/random"
)

// Options include options about generate certificate
type Options struct {
	Algorithm   string    `toml:"algorithm"` // rsa, ecdsa, ed25519
	DNSNames    []string  `toml:"dns_names"`
	IPAddresses []string  `toml:"ip_addresses"` // IP SANS
	Subject     Subject   `toml:"subject"`
	NotBefore   time.Time `toml:"not_before"`
	NotAfter    time.Time `toml:"not_after"`
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

// EncodeToPEM is used to encode certificate and private key to ASN1 and PKCS8
func (kp *KeyPair) Encode() (cert, key []byte) {
	cert = make([]byte, len(kp.asn1Data))
	copy(cert, kp.asn1Data)
	key, _ = x509.MarshalPKCS8PrivateKey(kp.PrivateKey)
	return
}

// EncodeToPEM is used to encode certificate and private key to PEM data
func (kp *KeyPair) EncodeToPEM() (cert, key []byte) {
	c, k := kp.Encode()
	certBlock := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: c,
	}
	cert = pem.EncodeToMemory(certBlock)
	keyBlock := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: k,
	}
	key = pem.EncodeToMemory(keyBlock)
	return
}

// EncodeToPEM is used to generate tls certificate
func (kp *KeyPair) TLSCertificate() (tls.Certificate, error) {
	return tls.X509KeyPair(kp.EncodeToPEM())
}

func genCertificate(opts *Options) *x509.Certificate {
	cert := x509.Certificate{}
	cert.SerialNumber = big.NewInt(random.Int64())
	cert.SubjectKeyId = random.Bytes(4)

	// Subject.CommonName
	if opts.Subject.CommonName == "" {
		cert.Subject.CommonName = random.Cookie(6 + random.Int(8))
	} else {
		cert.Subject.CommonName = opts.Subject.CommonName
	}
	// Subject.Organization
	if opts.Subject.Organization == nil {
		cert.Subject.Organization = []string{random.Cookie(6 + random.Int(8))}
	} else {
		copy(cert.Subject.Organization, opts.Subject.Organization)
	}
	cert.Subject.Country = make([]string, len(opts.Subject.Country))
	copy(cert.Subject.Country, opts.Subject.Country)
	cert.Subject.OrganizationalUnit = make([]string, len(opts.Subject.OrganizationalUnit))
	copy(cert.Subject.OrganizationalUnit, opts.Subject.OrganizationalUnit)
	cert.Subject.Locality = make([]string, len(opts.Subject.Locality))
	copy(cert.Subject.Locality, opts.Subject.Locality)
	cert.Subject.Province = make([]string, len(opts.Subject.Province))
	copy(cert.Subject.Province, opts.Subject.Province)
	cert.Subject.StreetAddress = make([]string, len(opts.Subject.StreetAddress))
	copy(cert.Subject.StreetAddress, opts.Subject.StreetAddress)
	cert.Subject.PostalCode = make([]string, len(opts.Subject.PostalCode))
	copy(cert.Subject.PostalCode, opts.Subject.PostalCode)
	cert.Subject.SerialNumber = opts.Subject.SerialNumber

	// set time
	now := time.Time{}.AddDate(2017, 10, 26) // 2018-11-27
	if opts.NotBefore.Equal(time.Time{}) {
		years := random.Int(10)
		months := random.Int(12)
		days := random.Int(31)
		cert.NotBefore = now.AddDate(-years, -months, -days)
	} else {
		cert.NotBefore = opts.NotBefore
	}
	if opts.NotAfter.Equal(time.Time{}) {
		years := 10 + random.Int(10)
		months := random.Int(12)
		days := random.Int(31)
		cert.NotAfter = now.AddDate(years, months, days)
	} else {
		cert.NotAfter = opts.NotAfter
	}
	return &cert
}

func genKey(algorithm string) (interface{}, interface{}, error) {
	switch algorithm {
	case "", "rsa":
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

// GenerateCA is used to generate a CA certificate from Options
func GenerateCA(opts *Options) (*KeyPair, error) {
	if opts == nil {
		opts = new(Options)
	}

	ca := genCertificate(opts)
	ca.KeyUsage = x509.KeyUsageCertSign
	ca.BasicConstraintsValid = true
	ca.IsCA = true

	privateKey, publicKey, err := genKey(opts.Algorithm)
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

// Generate is used to generate a signed certificate by CA or
// self-sign certificate from options
func Generate(parent *x509.Certificate, pri interface{}, opts *Options) (*KeyPair, error) {
	if opts == nil {
		opts = new(Options)
	}

	cert := genCertificate(opts)
	cert.KeyUsage = x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment

	// check dns
	dn := opts.DNSNames
	for i := 0; i < len(dn); i++ {
		if !isDomainName(dn[i]) {
			return nil, fmt.Errorf("%s is not a domain name", dn[i])
		}
		cert.DNSNames = append(cert.DNSNames, dn[i])
	}

	// check ip
	ips := opts.IPAddresses
	for i := 0; i < len(ips); i++ {
		ip := net.ParseIP(ips[i])
		if ip == nil {
			return nil, fmt.Errorf("%s is not a IP", ips[i])
		}
		cert.IPAddresses = append(cert.IPAddresses, ip)
	}

	// generate certificate
	privateKey, publicKey, err := genKey(opts.Algorithm)
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

func printStrings(s []string) string {
	var ss string
	for i, s := range s {
		if i == 0 {
			ss = s
		} else {
			ss += ", " + s
		}
	}
	return ss
}

// Print is used to print certificate information
func Print(cert *x509.Certificate) *bytes.Buffer {
	output := new(bytes.Buffer)
	const certFormat = `subject
  common name:  %s
  organization: %s
issuer
  common name:  %s
  organization: %s
public key algorithm: %s
signature algorithm:  %s
not before: %s
not after:  %s`
	_, _ = fmt.Fprintf(output, certFormat,
		cert.Subject.CommonName, printStrings(cert.Subject.Organization),
		cert.Issuer.CommonName, printStrings(cert.Issuer.Organization),
		cert.PublicKeyAlgorithm, cert.SignatureAlgorithm,
		cert.NotBefore.Local().Format(logger.TimeLayout),
		cert.NotAfter.Local().Format(logger.TimeLayout),
	)
	return output
}
