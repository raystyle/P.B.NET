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
	"strings"
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
	_ = os.Mkdir("key", 644)

	// create CA certificate and private key
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

	// create system.pem
	file, err := os.Create("key/system.pem")
	checkError(err)
	_ = file.Close()
}

var (
	pwd []byte

	certs     map[int]*x509.Certificate // CA certificates
	certsASN1 map[int][]byte            // CA certificates ASN1 data
	keys      map[int]interface{}       // CA private key
	keysPKCS8 map[int][]byte            // CA private key PKCS8 data
	number    int                       // the number of the CA certificates(private keys)

	system     map[int]*x509.Certificate // only certificate
	systemASN1 map[int][]byte            // CA certificates ASN1 data
	sNumber    int                       // the number of the system CA certificates
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
	certsASN1 = make(map[int][]byte)
	index := 1
	for {
		if len(certPEMBlock) == 0 {
			break
		}
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
		certsASN1[index] = b
		index += 1
	}

	// decrypt private keys
	keyPEMBlock, err := ioutil.ReadFile("key/keys.pem")
	checkError(err)

	keys = make(map[int]interface{})
	keysPKCS8 = make(map[int][]byte)
	index = 1
	for {
		if len(keyPEMBlock) == 0 {
			break
		}
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
		keysPKCS8[index] = b
		index += 1
	}
	number = len(certs)

	// decrypt system certificate
	systemPEMBlock, err := ioutil.ReadFile("key/system.pem")
	checkError(err)
	system = make(map[int]*x509.Certificate)
	systemASN1 = make(map[int][]byte)
	index = 1
	for {
		if len(systemPEMBlock) == 0 {
			break
		}
		block, systemPEMBlock = pem.Decode(systemPEMBlock)
		if block == nil {
			fmt.Println("\nfailed to decode key/system.pem")
			os.Exit(1)
		}
		b, err := x509.DecryptPEMBlock(block, pwd)
		checkError(err)
		c, err := x509.ParseCertificate(b)
		checkError(err)
		system[index] = c
		systemASN1[index] = b
		index += 1
	}
	sNumber = len(system)

	fmt.Println()
}

func printCertificate(id int, c *x509.Certificate) {
	const certFormat = `
ID: %d
common name: %s
public key algorithm: %s
signature algorithm:  %s
not before: %s
not after:  %s
`
	fmt.Printf(certFormat, id, c.Subject.CommonName,
		c.PublicKeyAlgorithm, c.SignatureAlgorithm,
		c.NotBefore.Local().Format(logger.TimeLayout),
		c.NotAfter.Local().Format(logger.TimeLayout),
	)
}

func list() {
	for i := 1; i < number+1; i++ {
		printCertificate(i, certs[i])
	}
}

func listSystem() {
	for i := 1; i < sNumber+1; i++ {
		printCertificate(i, system[i])
	}
}

func add() {
	var block *pem.Block
	certPEMBlock, err := ioutil.ReadFile("certs.pem")
	checkError(err)
	keyPEMBlock, err := ioutil.ReadFile("keys.pem")
	checkError(err)
	for {
		if len(certPEMBlock) == 0 {
			break
		}

		// load certificate
		block, certPEMBlock = pem.Decode(certPEMBlock)
		if block == nil {
			fmt.Println("\nfailed to decode certs.pem")
			os.Exit(1)
		}
		c, err := x509.ParseCertificate(block.Bytes)
		checkError(err)
		certASN1 := block.Bytes

		// load private key
		block, keyPEMBlock = pem.Decode(keyPEMBlock)
		if block == nil {
			fmt.Println("\nfailed to decode keys.pem")
			os.Exit(1)
		}
		k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		checkError(err)
		keyPKCS8 := block.Bytes

		// add
		number += 1
		certs[number] = c
		certsASN1[number] = certASN1
		keys[number] = k
		keysPKCS8[number] = keyPKCS8
		printCertificate(number, c)
	}
}

func addSystem() {
	var block *pem.Block
	systemPEMBlock, err := ioutil.ReadFile("system.pem")
	checkError(err)
	for {
		if len(systemPEMBlock) == 0 {
			break
		}
		block, systemPEMBlock = pem.Decode(systemPEMBlock)
		if block == nil {
			fmt.Println("\nfailed to decode system.pem")
			os.Exit(1)
		}
		c, err := x509.ParseCertificate(block.Bytes)
		checkError(err)
		sNumber += 1
		system[sNumber] = c
		systemASN1[sNumber] = block.Bytes
		printCertificate(sNumber, c)
	}
}

func save() {
	// encrypt certificates
	buf := new(bytes.Buffer)
	for i := 1; i < number+1; i++ {
		block, err := x509.EncryptPEMBlock(rand.Reader, "CERTIFICATE",
			certsASN1[i], pwd, x509.PEMCipherAES256)
		checkError(err)
		err = pem.Encode(buf, block)
		checkError(err)
	}
	err := ioutil.WriteFile("key/certs.pem", buf.Bytes(), 644)
	checkError(err)

	// encrypt private key
	buf.Reset()
	for i := 1; i < len(keys)+1; i++ {
		block, err := x509.EncryptPEMBlock(rand.Reader, "PRIVATE KEY",
			keysPKCS8[i], pwd, x509.PEMCipherAES256)
		checkError(err)
		err = pem.Encode(buf, block)
		checkError(err)
	}
	err = ioutil.WriteFile("key/keys.pem", buf.Bytes(), 644)
	checkError(err)

	// encrypt system certificates
	buf.Reset()
	for i := 1; i < sNumber+1; i++ {
		block, err := x509.EncryptPEMBlock(rand.Reader, "CERTIFICATE",
			systemASN1[i], pwd, x509.PEMCipherAES256)
		checkError(err)
		err = pem.Encode(buf, block)
		checkError(err)
	}
	err = ioutil.WriteFile("key/system.pem", buf.Bytes(), 644)
	checkError(err)
}

func manage() {
	loadCertsAndKeys()
	var input string
	for {
		fmt.Print("manager> ")
		_, err := fmt.Scanln(&input)
		checkError(err)
		switch input {
		case "":
		case "list":
			list()
		case "system":
			listSystem()
		case "add":
			add()
		case "adds":
			addSystem()
		case "help":

		case "save":
			save()
		case "exit":
			fmt.Print("Bye!")
			os.Exit(0)
		}
	}
}

func checkError(err error) {
	if err != nil {
		if strings.Contains(err.Error(), "unexpected newline") {
			return
		}
		if err != io.EOF {
			fmt.Printf("\n%s", err)
		}
		os.Exit(1)
	}
}
