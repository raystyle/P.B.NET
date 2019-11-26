package main

import (
	"crypto/x509"
	"encoding/pem"
	"flag"
	"io/ioutil"
	"log"

	"github.com/pelletier/go-toml"

	"project/internal/crypto/cert"
)

func main() {
	var (
		ca   bool
		sign bool
	)
	flag.BoolVar(&ca, "ca", false, "generate CA certificate")
	flag.BoolVar(&sign, "sign", false, "sign a certificate by CA")
	flag.Parse()

	options, err := ioutil.ReadFile("options.toml")
	if err != nil {
		log.Fatalln(err)
	}
	opts := &cert.Options{}
	err = toml.Unmarshal(options, opts)
	if err != nil {
		log.Fatalln(err)
	}

	switch {
	case ca:
		ca, err := cert.GenerateCA(opts)
		if err != nil {
			log.Fatalln(err)
		}
		caCert, caKey := ca.EncodeToPEM()
		err = ioutil.WriteFile("ca.crt", caCert, 644)
		if err != nil {
			log.Fatalln(err)
		}
		err = ioutil.WriteFile("ca.key", caKey, 644)
		if err != nil {
			log.Fatalln(err)
		}
	case sign:
		// load CA certificate
		caCert, err := x509.ParseCertificate(parsePEM("ca.crt"))
		if err != nil {
			log.Fatalln(err)
		}
		// load CA private key
		caKey, err := x509.ParsePKCS8PrivateKey(parsePEM("ca.key"))
		if err != nil {
			log.Fatalln(err)
		}
		kp, err := cert.Generate(caCert, caKey, opts)
		if err != nil {
			log.Fatalln(err)
		}
		crt, key := kp.EncodeToPEM()
		err = ioutil.WriteFile("server.crt", crt, 644)
		if err != nil {
			log.Fatalln(err)
		}
		err = ioutil.WriteFile("server.key", key, 644)
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func parsePEM(filename string) []byte {
	pemData, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatalln(err)
	}
	block, _ := pem.Decode(pemData)
	if block == nil {
		log.Fatalf("invalid %s (file format need PEM)", filename)
	}
	return block.Bytes
}
