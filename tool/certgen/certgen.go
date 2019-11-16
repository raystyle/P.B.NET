package main

import (
	"flag"
	"io/ioutil"
	"log"

	"github.com/pelletier/go-toml"

	"project/internal/crypto/cert"
)

func main() {
	config, err := ioutil.ReadFile("config.toml")
	if err != nil {
		log.Fatalln(err)
	}

	var onlyCA bool
	flag.BoolVar(&onlyCA, "onlyca", false, "only generate CA")
	flag.Parse()

	certCfg := &cert.Config{}
	err = toml.Unmarshal(config, certCfg)
	if err != nil {
		log.Fatalln(err)
	}

	ca, err := cert.GenerateCA(certCfg)
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

	if !onlyCA {
		kp, err := cert.Generate(ca.Certificate, ca.PrivateKey, certCfg)
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
