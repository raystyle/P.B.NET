package main

import (
	"flag"
	"io/ioutil"
	"log"

	"project/internal/cert"
	"project/internal/patch/toml"
	"project/internal/system"
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
		err = system.WriteFile("ca.crt", caCert)
		checkError(err)
		err = system.WriteFile("ca.key", caKey)
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
		err = system.WriteFile("server.crt", crt)
		checkError(err)
		err = system.WriteFile("server.key", key)
		checkError(err)
	}
}

func checkError(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}
