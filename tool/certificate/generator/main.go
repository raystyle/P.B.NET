package main

import (
	"flag"
	"io/ioutil"
	"log"

	"project/internal/crypto/cert"
	"project/internal/patch/toml"
)

func main() {
	var (
		ca  bool
		gen bool
	)
	flag.BoolVar(&ca, "ca", false, "generate CA certificate")
	flag.BoolVar(&gen, "gen", false, "generate certificate and sign it by CA")
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
		err = ioutil.WriteFile("ca.crt", caCert, 0600)
		if err != nil {
			log.Fatalln(err)
		}
		err = ioutil.WriteFile("ca.key", caKey, 0600)
		if err != nil {
			log.Fatalln(err)
		}
	case gen:
		// load CA certificate
		pemData, err := ioutil.ReadFile("ca.crt")
		if err != nil {
			log.Fatalln(err)
		}
		caCert, err := cert.ParseCertificate(pemData)
		if err != nil {
			log.Fatalln(err)
		}
		// load CA private key
		pemData, err = ioutil.ReadFile("ca.key")
		if err != nil {
			log.Fatalln(err)
		}
		caKey, err := cert.ParsePrivateKey(pemData)
		if err != nil {
			log.Fatalln(err)
		}
		kp, err := cert.Generate(caCert, caKey, opts)
		if err != nil {
			log.Fatalln(err)
		}
		crt, key := kp.EncodeToPEM()
		err = ioutil.WriteFile("server.crt", crt, 0600)
		if err != nil {
			log.Fatalln(err)
		}
		err = ioutil.WriteFile("server.key", key, 0600)
		if err != nil {
			log.Fatalln(err)
		}
	}
}
