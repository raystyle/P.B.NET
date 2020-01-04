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
	"strconv"
	"strings"
	"time"

	"project/internal/crypto/rand"
	"project/internal/dns"
	"project/internal/logger"
	"project/internal/random"
)

// Options contains options about generate certificate
type Options struct {
	Algorithm   string    `toml:"algorithm"` // "rsa|2048", "ecdsa|p256", "ed25519"
	DNSNames    []string  `toml:"dns_names"`
	IPAddresses []string  `toml:"ip_addresses"` // IP SANS
	Subject     Subject   `toml:"subject"`
	NotBefore   time.Time `toml:"not_before"`
	NotAfter    time.Time `toml:"not_after"`
}

// Subject contains certificate subject info
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

// KeyPair contains certificate, certificate ASN1 data and private key
type KeyPair struct {
	Certificate *x509.Certificate
	ASN1Data    []byte
	PrivateKey  interface{}
}

// Encode is used to get certificate ASN1 data and encode private key to PKCS8
func (kp *KeyPair) Encode() ([]byte, []byte) {
	cert := make([]byte, len(kp.ASN1Data))
	copy(cert, kp.ASN1Data)
	key, _ := x509.MarshalPKCS8PrivateKey(kp.PrivateKey)
	return cert, key
}

// EncodeToPEM is used to encode certificate and private key to PEM data
func (kp *KeyPair) EncodeToPEM() ([]byte, []byte) {
	cert, key := kp.Encode()
	certBlock := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert,
	}
	keyBlock := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: key,
	}
	return pem.EncodeToMemory(certBlock), pem.EncodeToMemory(keyBlock)
}

// TLSCertificate is used to generate tls certificate
func (kp *KeyPair) TLSCertificate() (tls.Certificate, error) {
	return tls.X509KeyPair(kp.EncodeToPEM())
}

func generateCertificate(opts *Options) (*x509.Certificate, error) {
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
		cert.Subject.Organization = make([]string, len(opts.Subject.Organization))
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

	// check domain name
	dn := opts.DNSNames
	for i := 0; i < len(dn); i++ {
		if !dns.IsDomainName(dn[i]) {
			return nil, fmt.Errorf("%s is not a domain name", dn[i])
		}
		cert.DNSNames = append(cert.DNSNames, dn[i])
	}

	// check IP address
	ips := opts.IPAddresses
	for i := 0; i < len(ips); i++ {
		ip := net.ParseIP(ips[i])
		if ip == nil {
			return nil, fmt.Errorf("%s is not a IP", ips[i])
		}
		cert.IPAddresses = append(cert.IPAddresses, ip)
	}
	return &cert, nil
}

func generatePrivateKey(algorithm string) (interface{}, interface{}, error) {
	if algorithm == "" {
		privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
		return privateKey, &privateKey.PublicKey, nil
	}
	if algorithm == "ed25519" {
		publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)
		return privateKey, publicKey, nil
	}
	configs := strings.Split(algorithm, "|")
	if len(configs) != 2 {
		return nil, nil, fmt.Errorf("invalid algorithm configs: %s", algorithm)
	}
	switch configs[0] {
	case "rsa":
		bits, err := strconv.Atoi(configs[1])
		if err != nil {
			return nil, nil, fmt.Errorf("invalid RSA bits: %s %s", algorithm, err)
		}
		privateKey, _ := rsa.GenerateKey(rand.Reader, bits)
		return privateKey, &privateKey.PublicKey, nil
	case "ecdsa":
		var privateKey *ecdsa.PrivateKey
		switch configs[1] {
		case "p224":
			privateKey, _ = ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
		case "p256":
			privateKey, _ = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		case "p384":
			privateKey, _ = ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
		case "p521":
			privateKey, _ = ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
		default:
			return nil, nil, fmt.Errorf("unsupported elliptic curve: %s", configs[1])
		}
		return privateKey, &privateKey.PublicKey, nil
	}
	return nil, nil, fmt.Errorf("unknown algorithm: %s", algorithm)
}

// GenerateCA is used to generate a CA certificate from Options
func GenerateCA(opts *Options) (*KeyPair, error) {
	if opts == nil {
		opts = new(Options)
	}

	ca, err := generateCertificate(opts)
	if err != nil {
		return nil, err
	}
	ca.KeyUsage = x509.KeyUsageCertSign
	ca.BasicConstraintsValid = true
	ca.IsCA = true

	privateKey, publicKey, err := generatePrivateKey(opts.Algorithm)
	if err != nil {
		return nil, err
	}
	asn1Data, _ := x509.CreateCertificate(rand.Reader, ca, ca, publicKey, privateKey)
	ca, _ = x509.ParseCertificate(asn1Data)
	return &KeyPair{
		Certificate: ca,
		PrivateKey:  privateKey,
		ASN1Data:    asn1Data,
	}, nil
}

// Generate is used to generate a signed certificate by CA or
// self-sign certificate from options
func Generate(parent *x509.Certificate, pri interface{}, opts *Options) (*KeyPair, error) {
	if opts == nil {
		opts = new(Options)
	}

	cert, err := generateCertificate(opts)
	if err != nil {
		return nil, err
	}
	cert.KeyUsage = x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment

	// generate certificate
	privateKey, publicKey, err := generatePrivateKey(opts.Algorithm)
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
		ASN1Data:    asn1Data,
	}, nil
}

func printStringSlice(s []string) string {
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
		cert.Subject.CommonName, printStringSlice(cert.Subject.Organization),
		cert.Issuer.CommonName, printStringSlice(cert.Issuer.Organization),
		cert.PublicKeyAlgorithm, cert.SignatureAlgorithm,
		cert.NotBefore.Local().Format(logger.TimeLayout),
		cert.NotAfter.Local().Format(logger.TimeLayout),
	)
	return output
}
