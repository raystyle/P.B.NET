// +build windows

package main

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"log"

	"project/internal/crypto/cert/certutil"
	"project/internal/options"
)

func main() {
	root, err := certutil.LoadSystemCertWithName("ROOT")
	if err != nil {
		log.Fatal(err)
	}
	ca, err := certutil.LoadSystemCertWithName("CA")
	if err != nil {
		log.Fatal(err)
	}
	certs := append(root, ca...)
	l := len(certs)
	buf := new(bytes.Buffer)
	count := 0
	for i := 0; i < l; i++ {
		cert, err := x509.ParseCertificate(certs[i])
		if err != nil {
			log.Println("warning", err)
			continue
		}
		block := pem.Block{
			Type:  "CERTIFICATE",
			Bytes: certs[i],
		}
		err = pem.Encode(buf, &block)
		if err != nil {
			log.Fatal(err)
		}
		count += 1
		// print CA info
		const format = "V%d %s\n"
		switch {
		case cert.Subject.CommonName != "":
			log.Printf(format, cert.Version, cert.Subject.CommonName)
		case len(cert.Subject.Organization) != 0:
			log.Printf(format, cert.Version, cert.Subject.Organization[0])
		default:
			log.Printf(format, cert.Version, cert.Subject)
		}
	}

	log.Println("------------------------------------------------")
	log.Println("the raw number of the system certificates:", l)
	log.Println("the actual number of the system certificates:", count)

	// write pem
	err = ioutil.WriteFile("system.pem", buf.Bytes(), 644)
	if err != nil {
		log.Fatal(err)
	}

	// test generate PEM
	pemData, err := ioutil.ReadFile("system.pem")
	if err != nil {
		log.Fatal(err)
	}

	// load system
	tlsConfig, _ := (&options.TLSConfig{
		InsecureLoadFromSystem: true,
	}).Apply()
	sysNum := len(tlsConfig.RootCAs.Subjects())

	// test load
	tlsConfig, _ = (&options.TLSConfig{
		RootCAs: []string{string(pemData)},
	}).Apply()
	loadNum := len(tlsConfig.RootCAs.Subjects())

	// compare
	if sysNum != loadNum {
		log.Printf("warning: system: %d, test load: %d", sysNum, loadNum)
	} else {
		log.Println("export Windows System Root CAs successfully")
	}
}
