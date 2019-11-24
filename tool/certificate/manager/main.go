package main

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"syscall"

	"golang.org/x/crypto/ssh/terminal"

	"project/internal/crypto/cert"
	"project/internal/crypto/rand"
	"project/internal/logger"
)

func main() {
	var init bool
	flag.BoolVar(&init, "init", false, "initialize certificate manager")
	flag.Parse()
	if init {
		initManager()
		return
	}
	manage()
}

func initManager() {
	// input password
	fmt.Print("password: ")
	pwd, err := terminal.ReadPassword(int(syscall.Stdin))
	checkError(err)
	for {
		fmt.Print("\nretype: ")
		retype, err := terminal.ReadPassword(int(syscall.Stdin))
		checkError(err)
		if !bytes.Equal(pwd, retype) {
			fmt.Print("\ndifferent password")
		} else {
			fmt.Println()
			break
		}
	}
	// create  CA certificate and private key
	kp, err := cert.GenerateCA(nil)
	checkError(err)
	caCert, caKey := kp.Encode()
	// encrypt certificate
	block, err := x509.EncryptPEMBlock(rand.Reader, "CERTIFICATE",
		caCert, pwd, x509.PEMCipherAES256)
	checkError(err)
	buf := new(bytes.Buffer)
	err = pem.Encode(buf, block)
	checkError(err)
	err = ioutil.WriteFile("key/certs.pem", buf.Bytes(), 644)
	checkError(err)

	// encrypt private key
	block, err = x509.EncryptPEMBlock(rand.Reader, "PRIVATE KEY",
		caKey, pwd, x509.PEMCipherAES256)
	checkError(err)
	buf.Reset()
	err = pem.Encode(buf, block)
	checkError(err)
	err = ioutil.WriteFile("key/keys.pem", buf.Bytes(), 644)
	checkError(err)
	fmt.Println("initialize certificate manager successfully")
}

var (
	pwd    []byte
	certs  map[int]*x509.Certificate
	keys   map[int]interface{}
	number int
)

func loadCertsAndKeys() {
	var err error
	// input password
	fmt.Print("password: ")
	pwd, err = terminal.ReadPassword(int(syscall.Stdin))
	checkError(err)

	// decrypt certificates
	certPEMBlock, err := ioutil.ReadFile("key/certs.pem")
	checkError(err)

	var block *pem.Block
	certs = make(map[int]*x509.Certificate)
	keys = make(map[int]interface{})
	index := 0

	for {
		block, certPEMBlock = pem.Decode(certPEMBlock)
		if block == nil {
			fmt.Println("\nfailed to decode key/certs.pem")
			os.Exit(1)
		}
		b, err := x509.DecryptPEMBlock(block, pwd)
		checkError(err)
		c, err := x509.ParseCertificate(b)
		checkError(err)
		certs[index] = c
		if len(certPEMBlock) == 0 {
			break
		}
		index += 1
	}

	// decrypt private keys
	keyPEMBlock, err := ioutil.ReadFile("key/keys.pem")
	checkError(err)

	index = 0
	for {
		block, keyPEMBlock = pem.Decode(keyPEMBlock)
		if block == nil {
			fmt.Println("\nfailed to decode key/keys.pem")
			os.Exit(1)
		}
		b, err := x509.DecryptPEMBlock(block, pwd)
		checkError(err)
		key, err := x509.ParsePKCS8PrivateKey(b)
		checkError(err)
		keys[index] = key
		if len(certPEMBlock) == 0 {
			break
		}
		index += 1
	}

	number = len(certs)
}

func list() {
	for i := 0; i < number; i++ {
		c := certs[i]
		const format = `
ID: %d
common name: %s
public key algorithm: %s
signature algorithm:  %s
not before: %s
not after:  %s
`
		fmt.Printf(format, i, c.Subject.CommonName,
			c.PublicKeyAlgorithm, c.SignatureAlgorithm,
			c.NotBefore.Format(logger.TimeLayout),
			c.NotAfter.Format(logger.TimeLayout),
		)
	}
}

func manage() {
	loadCertsAndKeys()
	rw := &struct {
		io.Reader
		io.Writer
	}{
		Reader: os.Stdin,
		Writer: os.Stdout,
	}
	term := terminal.NewTerminal(rw, "manager> ")
	for {
		line, err := term.ReadPassword("")
		checkError(err)
		switch line {
		case "list":
			list()
		case "help":

		case "exit":
			os.Exit(0)
		}
	}
}

func checkError(err error) {
	if err != nil {
		if err != io.EOF {
			fmt.Printf("\n%s", err)
		}
		os.Exit(1)
	}
}
