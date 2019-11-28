package main

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
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
	checkError(err, true)
	for {
		fmt.Print("\nretype: ")
		retype, err := terminal.ReadPassword(int(syscall.Stdin))
		checkError(err, true)
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
	checkError(err, true)
	caCert, caKey := kp.Encode()

	// encrypt certificate
	block, err := x509.EncryptPEMBlock(rand.Reader, "CERTIFICATE",
		caCert, pwd, x509.PEMCipherAES256)
	checkError(err, true)
	buf := new(bytes.Buffer)
	err = pem.Encode(buf, block)
	checkError(err, true)
	err = ioutil.WriteFile("key/certs.pem", buf.Bytes(), 644)
	checkError(err, true)

	// encrypt private key
	block, err = x509.EncryptPEMBlock(rand.Reader, "PRIVATE KEY",
		caKey, pwd, x509.PEMCipherAES256)
	checkError(err, true)
	buf.Reset()
	err = pem.Encode(buf, block)
	checkError(err, true)
	err = ioutil.WriteFile("key/keys.pem", buf.Bytes(), 644)
	checkError(err, true)

	// create system.pem
	file, err := os.Create("key/system.pem")
	checkError(err, true)
	defer func() { _ = file.Close() }()

	// calculate hash
	hash := sha256.New()
	hash.Write(pwd)
	hash.Write(caCert)
	hash.Write(caKey)
	err = ioutil.WriteFile("key/pem.hash", hash.Sum(nil), 644)
	checkError(err, true)

	fmt.Println("initialize certificate manager successfully")
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
	checkError(err, true)

	// read PEM files
	certPEMBlock, err := ioutil.ReadFile("key/certs.pem")
	checkError(err, true)
	keyPEMBlock, err := ioutil.ReadFile("key/keys.pem")
	checkError(err, true)
	systemPEMBlock, err := ioutil.ReadFile("key/system.pem")
	checkError(err, true)
	PEMHash, err := ioutil.ReadFile("key/pem.hash")
	checkError(err, true)

	// create backup
	err = ioutil.WriteFile("key/certs.bak", certPEMBlock, 644)
	checkError(err, true)
	err = ioutil.WriteFile("key/keys.bak", keyPEMBlock, 644)
	checkError(err, true)
	err = ioutil.WriteFile("key/system.bak", systemPEMBlock, 644)
	checkError(err, true)
	err = ioutil.WriteFile("key/hash.bak", PEMHash, 644)
	checkError(err, true)

	var block *pem.Block
	hash := sha256.New()
	hash.Write(pwd)

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
		checkError(err, true)
		c, err := x509.ParseCertificate(b)
		checkError(err, true)
		certASN1 := b
		hash.Write(b)

		// load private key
		block, keyPEMBlock = pem.Decode(keyPEMBlock)
		if block == nil {
			fmt.Println("\nfailed to decode key/keys.pem")
			os.Exit(1)
		}
		b, err = x509.DecryptPEMBlock(block, pwd)
		checkError(err, true)
		key, err := x509.ParsePKCS8PrivateKey(b)
		checkError(err, true)
		keyPKCS8 := b
		hash.Write(b)

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
		checkError(err, true)
		c, err := x509.ParseCertificate(b)
		checkError(err, true)
		hash.Write(b)

		// add
		system[index] = c
		systemASN1[index] = b
		index += 1
	}
	sNumber = len(system)

	// check hash
	if subtle.ConstantTimeCompare(PEMHash, hash.Sum(nil)) != 1 {
		log.Fatal("warning: PEM files has been tampered")
	}

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
	checkError(err, false)
	keyPEMBlock, err := ioutil.ReadFile("keys.pem")
	checkError(err, false)

	// test load
	_, err = tls.X509KeyPair(certPEMBlock, keyPEMBlock)
	checkError(err, false)

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
		checkError(err, false)
		certASN1 := block.Bytes

		// load private key
		block, keyPEMBlock = pem.Decode(keyPEMBlock)
		if block == nil {
			fmt.Println("\nfailed to decode keys.pem")
			os.Exit(1)
		}
		k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		checkError(err, false)
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
	checkError(err, false)
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
		checkError(err, false)
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

	hash := sha256.New()
	hash.Write(pwd)

	for i := 1; i < number+1; i++ {
		// encrypt certificates
		block, err := x509.EncryptPEMBlock(rand.Reader, "CERTIFICATE",
			certsASN1[i], pwd, x509.PEMCipherAES256)
		checkError(err, false)
		err = pem.Encode(certsPEM, block)
		checkError(err, false)
		hash.Write(certsASN1[i])

		// encrypt private key
		block, err = x509.EncryptPEMBlock(rand.Reader, "PRIVATE KEY",
			keysPKCS8[i], pwd, x509.PEMCipherAES256)
		checkError(err, false)
		err = pem.Encode(keysPEM, block)
		checkError(err, false)
		hash.Write(keysPKCS8[i])
	}

	// encrypt system certificates
	for i := 1; i < sNumber+1; i++ {
		block, err := x509.EncryptPEMBlock(rand.Reader, "CERTIFICATE",
			systemASN1[i], pwd, x509.PEMCipherAES256)
		checkError(err, false)
		err = pem.Encode(systemPEM, block)
		checkError(err, false)
		hash.Write(systemASN1[i])
	}

	// write
	err := ioutil.WriteFile("key/certs.pem", certsPEM.Bytes(), 644)
	checkError(err, false)
	err = ioutil.WriteFile("key/keys.pem", keysPEM.Bytes(), 644)
	checkError(err, false)
	err = ioutil.WriteFile("key/system.pem", systemPEM.Bytes(), 644)
	checkError(err, false)
	err = ioutil.WriteFile("key/pem.hash", hash.Sum(nil), 644)
	checkError(err, false)
}

func exit() {
	checkError(os.Remove("key/certs.bak"), true)
	checkError(os.Remove("key/keys.bak"), true)
	checkError(os.Remove("key/system.bak"), true)
	checkError(os.Remove("key/hash.bak"), true)
	fmt.Print("Bye!")
	os.Exit(0)
}

func manage() {
	loadCertsAndKeys()
	var input string
	for {
		fmt.Print("manager> ")
		_, err := fmt.Scanln(&input)
		checkError(err, true)
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

func checkError(err error, exit bool) {
	if err != nil {
		if strings.Contains(err.Error(), "unexpected newline") {
			return
		}
		if err != io.EOF {
			fmt.Printf("\n%s", err)
		}
		if exit {
			os.Exit(1)
		}
	}
}
