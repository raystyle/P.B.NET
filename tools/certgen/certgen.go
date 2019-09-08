package main

import (
	"flag"
	"io/ioutil"
	"log"

	"github.com/pelletier/go-toml"

	"project/internal/crypto/cert"
	"project/internal/crypto/rsa"
)

func main() {
	b, err := ioutil.ReadFile("config.toml")
	if err != nil {
		log.Fatalln(err)
	}
	var onlyCA bool
	flag.BoolVar(&onlyCA, "onlyca", false, "only generate CA")
	flag.Parse()
	config := &cert.Config{}
	err = toml.Unmarshal(b, config)
	if err != nil {
		log.Fatalln(err)
	}
	caCert, caPri := cert.GenerateCA(config)
	err = ioutil.WriteFile("ca.crt", caCert, 644)
	if err != nil {
		log.Fatalln(err)
	}
	err = ioutil.WriteFile("ca.key", caPri, 644)
	if err != nil {
		log.Fatalln(err)
	}
	if !onlyCA {
		parent, err := cert.Parse(caCert)
		if err != nil {
			log.Fatalln(err)
		}
		privateKey, err := rsa.ImportPrivateKeyPEM(caPri)
		if err != nil {
			log.Fatalln(err)
		}
		crt, pri, err := cert.Generate(parent, privateKey, config)
		if err != nil {
			log.Fatalln(err)
		}
		err = ioutil.WriteFile("server.crt", crt, 644)
		if err != nil {
			log.Fatalln(err)
		}
		err = ioutil.WriteFile("server.key", pri, 644)
		if err != nil {
			log.Fatalln(err)
		}
	}
}
