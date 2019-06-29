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
	var only_ca bool
	flag.BoolVar(&only_ca, "onlyca", false, "only generate CA")
	flag.Parse()
	config := &cert.Config{}
	err = toml.Unmarshal(b, config)
	if err != nil {
		log.Fatalln(err)
	}
	ca_crt, ca_pri := cert.Generate_CA(config)
	err = ioutil.WriteFile("ca.crt", ca_crt, 644)
	if err != nil {
		log.Fatalln(err)
	}
	err = ioutil.WriteFile("ca.key", ca_pri, 644)
	if err != nil {
		log.Fatalln(err)
	}
	if !only_ca {
		parent, err := cert.Parse(ca_crt)
		if err != nil {
			log.Fatalln(err)
		}
		privatekey, err := rsa.Import_PrivateKey_PEM(ca_pri)
		if err != nil {
			log.Fatalln(err)
		}
		crt, pri, err := cert.Generate(parent, privatekey, config)
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
