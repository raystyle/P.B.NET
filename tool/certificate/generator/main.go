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
	checkError(err)
	opts := &cert.Options{}
	err = toml.Unmarshal(options, opts)
	checkError(err)

	switch {
	case ca:
		ca, err := cert.GenerateCA(opts)
		checkError(err)
		caCert, caKey := ca.EncodeToPEM()
		err = ioutil.WriteFile("ca.crt", caCert, 0600)
		checkError(err)
		err = ioutil.WriteFile("ca.key", caKey, 0600)
		checkError(err)
	case gen:
		// load CA certificate
		pemData, err := ioutil.ReadFile("ca.crt")
		checkError(err)
		caCert, err := cert.ParseCertificate(pemData)
		checkError(err)
		// load CA private key
		pemData, err = ioutil.ReadFile("ca.key")
		checkError(err)
		caKey, err := cert.ParsePrivateKey(pemData)
		checkError(err)
		// generate
		kp, err := cert.Generate(caCert, caKey, opts)
		checkError(err)
		crt, key := kp.EncodeToPEM()
		err = ioutil.WriteFile("server.crt", crt, 0600)
		checkError(err)
		err = ioutil.WriteFile("server.key", key, 0600)
		checkError(err)
	}
}

func checkError(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}
