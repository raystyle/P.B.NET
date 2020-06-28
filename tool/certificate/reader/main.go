package main

import (
	"bytes"
	"encoding/pem"
	"io/ioutil"
	"log"

	"project/internal/cert"
	"project/internal/cert/certpool"
	"project/internal/system"
)

func main() {
	// load certificates
	pool, err := certpool.System()
	checkError(err)
	certs := pool.Certs()
	l := len(certs)
	buf := new(bytes.Buffer)
	for i := 0; i < l; i++ {
		block := pem.Block{
			Type:  "CERTIFICATE",
			Bytes: certs[i].Raw,
		}
		err = pem.Encode(buf, &block)
		checkError(err)
		// print certificate information
		c := certs[i]
		const format = "V%d %s\n"
		switch {
		case c.Subject.CommonName != "":
			log.Printf(format, c.Version, c.Subject.CommonName)
		case len(c.Subject.Organization) != 0:
			log.Printf(format, c.Version, c.Subject.Organization[0])
		default:
			log.Printf(format, c.Version, c.Subject)
		}
	}
	log.Println("------------------------------------------------")
	log.Println("the number of the system CA certificates:", l)
	// write pem
	err = system.WriteFile("system.pem", buf.Bytes())
	checkError(err)
	// test certificates
	pemData, err := ioutil.ReadFile("system.pem")
	checkError(err)
	certs, err = cert.ParseCertificates(pemData)
	checkError(err)
	// compare
	loadNum := len(certs)
	if loadNum == l {
		log.Println("export System CA certificates successfully")
	} else {
		log.Printf("warning: system: %d, test load: %d\n", l, loadNum)
	}
}

func checkError(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}
