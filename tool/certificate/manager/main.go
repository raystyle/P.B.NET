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

	// read PEM files
	certPEMBlock, err := ioutil.ReadFile("key/certs.pem")
	checkError(err)
	keyPEMBlock, err := ioutil.ReadFile("key/keys.pem")
	checkError(err)
	systemPEMBlock, err := ioutil.ReadFile("key/system.pem")
	checkError(err)

	// create backup
	err = ioutil.WriteFile("key/certs.bak", certPEMBlock, 644)
	checkError(err)
	err = ioutil.WriteFile("key/keys.bak", keyPEMBlock, 644)
	checkError(err)
	err = ioutil.WriteFile("key/system.bak", systemPEMBlock, 644)
	checkError(err)

	var block *pem.Block
	// decrypt certificates and private key
	certs = make(map[int]*x509.Certificate)
	certsASN1 = make(map[int][]byte)
	keys = make(map[int]interface{})
	keysPKCS8 = make(map[int][]byte)
	index := 1
	for {
		if len(certPEMBlock) == 0 {
			break
		}

		// load CA certificate
		block, certPEMBlock = pem.Decode(certPEMBlock)
		if block == nil {
			fmt.Println("\nfailed to decode key/certs.pem")
			os.Exit(1)
		}
		b, err := x509.DecryptPEMBlock(block, pwd)
		checkError(err)
		c, err := x509.ParseCertificate(b)
		checkError(err)
		certASN1 := b

		// load private key
		block, keyPEMBlock = pem.Decode(keyPEMBlock)
		if block == nil {
			fmt.Println("\nfailed to decode key/keys.pem")
			os.Exit(1)
		}
		b, err = x509.DecryptPEMBlock(block, pwd)
		checkError(err)
		key, err := x509.ParsePKCS8PrivateKey(b)
		checkError(err)
		keyPKCS8 := b

		// add
		certs[index] = c
		certsASN1[index] = certASN1
		keys[index] = key
		keysPKCS8[index] = keyPKCS8
		index += 1
	}
	number = len(certs)

	// decrypt system certificate
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
	certsPEM := new(bytes.Buffer)
	keysPEM := new(bytes.Buffer)
	systemPEM := new(bytes.Buffer)

	for i := 1; i < number+1; i++ {
		// encrypt certificates
		block, err := x509.EncryptPEMBlock(rand.Reader, "CERTIFICATE",
			certsASN1[i], pwd, x509.PEMCipherAES256)
		checkError(err)
		err = pem.Encode(certsPEM, block)
		checkError(err)

		// encrypt private key
		block, err = x509.EncryptPEMBlock(rand.Reader, "PRIVATE KEY",
			keysPKCS8[i], pwd, x509.PEMCipherAES256)
		checkError(err)
		err = pem.Encode(keysPEM, block)
		checkError(err)
	}

	// encrypt system certificates
	for i := 1; i < sNumber+1; i++ {
		block, err := x509.EncryptPEMBlock(rand.Reader, "CERTIFICATE",
			systemASN1[i], pwd, x509.PEMCipherAES256)
		checkError(err)
		err = pem.Encode(systemPEM, block)
		checkError(err)
	}

	// write
	err := ioutil.WriteFile("key/certs.pem", certsPEM.Bytes(), 644)
	checkError(err)
	err = ioutil.WriteFile("key/keys.pem", keysPEM.Bytes(), 644)
	checkError(err)
	err = ioutil.WriteFile("key/system.pem", systemPEM.Bytes(), 644)
	checkError(err)
}

func exit() {
	checkError(os.Remove("key/certs.bak"))
	checkError(os.Remove("key/keys.bak"))
	checkError(os.Remove("key/system.bak"))
	fmt.Print("Bye!")
	os.Exit(0)
}

func manage() {
	loadCertsAndKeys()
	var input string
	for {
		fmt.Print("manager> ")
		_, err := fmt.Scanln(&input)
		checkError(err)
		switch input {
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
			exit()
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
