package main

import (
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"project/internal/cert"
	"project/internal/namer"
	"project/internal/patch/toml"
	"project/internal/system"
)

func main() {
	var (
		ca  bool
		gen bool
		nt  string
		np  string
	)
	flag.BoolVar(&ca, "ca", false, "generate CA certificate")
	flag.BoolVar(&gen, "gen", false, "generate certificate and sign it by CA")
	flag.StringVar(&nt, "namer-type", "english", "namer type")
	flag.StringVar(&np, "namer-path", "namer/english.zip", "namer resource path")
	flag.Parse()

	options, err := ioutil.ReadFile("options.toml")
	system.CheckError(err)
	opts := &cert.Options{
		Namer: loadNamer(nt, np),
	}
	err = toml.Unmarshal(options, opts)
	system.CheckError(err)

	var certificate *x509.Certificate
	switch {
	case ca:
		ca, err := cert.GenerateCA(opts)
		system.CheckError(err)
		caCert, caKey := ca.EncodeToPEM()
		err = system.WriteFile("ca.crt", caCert)
		system.CheckError(err)
		err = system.WriteFile("ca.key", caKey)
		system.CheckError(err)
		certificate = ca.Certificate
	case gen:
		// load CA certificate
		pemData, err := ioutil.ReadFile("ca.crt")
		system.CheckError(err)
		caCert, err := cert.ParseCertificate(pemData)
		system.CheckError(err)
		// load CA private key
		pemData, err = ioutil.ReadFile("ca.key")
		system.CheckError(err)
		caKey, err := cert.ParsePrivateKey(pemData)
		system.CheckError(err)
		// generate
		kp, err := cert.Generate(caCert, caKey, opts)
		system.CheckError(err)
		crt, key := kp.EncodeToPEM()
		err = system.WriteFile("server.crt", crt)
		system.CheckError(err)
		err = system.WriteFile("server.key", key)
		system.CheckError(err)
		certificate = kp.Certificate
	default:
		flag.PrintDefaults()
		return
	}
	fmt.Println(cert.Print(certificate))
}

func loadNamer(typ, path string) namer.Namer {
	resource, err := ioutil.ReadFile(path)
	system.CheckError(err)
	var nr namer.Namer
	switch typ {
	case "english":
		english := namer.NewEnglish()
		err = english.Load(resource)
		system.CheckError(err)
		nr = english
	default:
		fmt.Printf("unsupported namer: %s\n", typ)
		os.Exit(1)
	}
	return nr
}
