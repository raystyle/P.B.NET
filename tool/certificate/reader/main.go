// +build windows

package main

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"log"

	"project/internal/crypto/cert/certutil"
	"project/internal/option"
)

func main() {
	root, err := certutil.LoadSystemCertWithName("ROOT")
	checkError(err)
	ca, err := certutil.LoadSystemCertWithName("CA")
	checkError(err)
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
		checkError(err)
		count++
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
	log.Println("the raw number of the system CA certificates:", l)
	log.Println("the actual number of the system CA certificates:", count)

	// write pem
	err = ioutil.WriteFile("system.pem", buf.Bytes(), 0600)
	checkError(err)

	// test generated PEM
	pemData, err := ioutil.ReadFile("system.pem")
	checkError(err)
	tlsConfig, _ := (&option.TLSConfig{RootCAs: []string{string(pemData)}}).Apply()
	loadNum := len(tlsConfig.RootCAs.Subjects())
	tlsConfig, _ = (&option.TLSConfig{InsecureLoadFromSystem: true}).Apply()
	sysNum := len(tlsConfig.RootCAs.Subjects())

	// compare
	if sysNum != loadNum {
		log.Printf("warning: system: %d, test load: %d", sysNum, loadNum)
	} else {
		log.Println("export Windows System CA certificates successfully")
	}
}

func checkError(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}
