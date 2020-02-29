package main

import (
	"bytes"
	"encoding/pem"
	"io/ioutil"
	"log"

	"project/internal/crypto/cert/certutil"
)

func main() {
	// load certificates
	pool, err := certutil.SystemCertPool()
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
		cert := certs[i]
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
	log.Println("the number of the system CA certificates:", l)
	// write pem
	err = ioutil.WriteFile("system.pem", buf.Bytes(), 0600)
	checkError(err)
	// test certificates
	pemData, err := ioutil.ReadFile("system.pem")
	checkError(err)
	certs, err = certutil.ParseCertificates(pemData)
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
