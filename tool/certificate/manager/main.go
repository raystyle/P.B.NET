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
	"project/internal/crypto/cert/certutil"
	"project/internal/crypto/rand"
)

func main() {
	var init bool
	flag.BoolVar(&init, "init", false, "initialize certificate manager")
	flag.Parse()
	if init {
		initManager()
		return
	}
	load()
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
		if bytes.Compare(pwd, retype) != 0 {
			fmt.Print("\ndifferent password")
		} else {
			fmt.Println()
			break
		}
	}
	_ = os.Mkdir("key", 0750)

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
	err = ioutil.WriteFile("key/certs.pem", buf.Bytes(), 0600)
	checkError(err, true)

	// encrypt private key
	block, err = x509.EncryptPEMBlock(rand.Reader, "PRIVATE KEY",
		caKey, pwd, x509.PEMCipherAES256)
	checkError(err, true)
	buf.Reset()
	err = pem.Encode(buf, block)
	checkError(err, true)
	err = ioutil.WriteFile("key/keys.pem", buf.Bytes(), 0600)
	checkError(err, true)

	// create system.pem
	err = ioutil.WriteFile("key/system.pem", nil, 0600)
	checkError(err, true)

	// calculate hash
	hash := sha256.New()
	hash.Write(pwd)
	hash.Write(caCert)
	hash.Write(caKey)
	err = ioutil.WriteFile("key/pem.hash", hash.Sum(nil), 0600)
	checkError(err, true)

	fmt.Println("initialize certificate manager successfully")
}

var (
	pwd []byte

	certs     map[int]*x509.Certificate // certificate
	certsASN1 map[int][]byte            // certificate ASN1 data
	keys      map[int]interface{}       // private key
	keysBytes map[int][]byte            // private key data
	number    int                       // the number of the certificates(private keys)

	system     map[int]*x509.Certificate // only certificate
	systemASN1 map[int][]byte            // certificate ASN1 data
	sNumber    int                       // the number of the system certificates
)

func load() {
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
	err = ioutil.WriteFile("key/certs.bak", certPEMBlock, 0600)
	checkError(err, true)
	err = ioutil.WriteFile("key/keys.bak", keyPEMBlock, 0600)
	checkError(err, true)
	err = ioutil.WriteFile("key/system.bak", systemPEMBlock, 0600)
	checkError(err, true)
	err = ioutil.WriteFile("key/hash.bak", PEMHash, 0600)
	checkError(err, true)

	var block *pem.Block
	hash := sha256.New()
	hash.Write(pwd)

	// decrypt certificates and private key
	certs = make(map[int]*x509.Certificate)
	certsASN1 = make(map[int][]byte)
	keys = make(map[int]interface{})
	keysBytes = make(map[int][]byte)
	index := 1
	for {
		if len(certPEMBlock) == 0 {
			break
		}

		// load certificate
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
		key, err := certutil.ParsePrivateKeyBytes(b)
		checkError(err, true)
		keyBytes := b
		hash.Write(b)

		// add
		certs[index] = c
		certsASN1[index] = certASN1
		keys[index] = key
		keysBytes[index] = keyBytes
		index++
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
		index++
	}
	sNumber = len(system)

	// compare hash
	if subtle.ConstantTimeCompare(PEMHash, hash.Sum(nil)) != 1 {
		log.Fatalln("warning: PEM files has been tampered")
	}

	fmt.Println()
}

func printCertificate(id int, c *x509.Certificate) {
	fmt.Printf("ID: %d\n%s\n\n", id, cert.Print(c))
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

// check if added repeatedly
func checkRepeat(a map[int][]byte, b []byte) bool {
	for _, v := range a {
		if bytes.Compare(v, b) == 0 {
			return true
		}
	}
	return false
}

func add() {
	var block *pem.Block
	certPEMBlock, err := ioutil.ReadFile("certs.pem")
	if checkError(err, false) {
		return
	}
	keyPEMBlock, err := ioutil.ReadFile("keys.pem")
	if checkError(err, false) {
		return
	}
	// test load
	_, err = tls.X509KeyPair(certPEMBlock, keyPEMBlock)
	if checkError(err, false) {
		return
	}

	for {
		if len(certPEMBlock) == 0 {
			break
		}

		// load certificate
		block, certPEMBlock = pem.Decode(certPEMBlock)
		if block == nil {
			fmt.Println("\nfailed to decode certs.pem")
			return
		}
		c, err := x509.ParseCertificate(block.Bytes)
		if checkError(err, false) {
			return
		}
		if checkRepeat(certsASN1, block.Bytes) {
			fmt.Printf("\nthis certificate has already exists:\n\n%s\n", cert.Print(c))
			continue
		}
		certASN1 := block.Bytes

		// load private key
		block, keyPEMBlock = pem.Decode(keyPEMBlock)
		if block == nil {
			fmt.Println("\nfailed to decode keys.pem")
			return
		}
		k, err := certutil.ParsePrivateKeyBytes(block.Bytes)
		if checkError(err, false) {
			return
		}
		keyBytes := block.Bytes

		// add
		number++
		certs[number] = c
		certsASN1[number] = certASN1
		keys[number] = k
		keysBytes[number] = keyBytes
		fmt.Println("add certificate successfully")
		printCertificate(number, c)
	}
}

func addSystem() {
	var block *pem.Block
	systemPEMBlock, err := ioutil.ReadFile("system.pem")
	if checkError(err, false) {
		return
	}
	for {
		if len(systemPEMBlock) == 0 {
			break
		}
		block, systemPEMBlock = pem.Decode(systemPEMBlock)
		if block == nil {
			fmt.Println("\nfailed to decode system.pem")
			return
		}
		c, err := x509.ParseCertificate(block.Bytes)
		if checkError(err, false) {
			return
		}
		if checkRepeat(systemASN1, block.Bytes) {
			fmt.Printf("\nthis certificate has already exists:\n\n%s\n", cert.Print(c))
			continue
		}
		sNumber++
		system[sNumber] = c
		systemASN1[sNumber] = block.Bytes
		fmt.Println("add system certificate successfully")
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
		if checkError(err, false) {
			return
		}
		err = pem.Encode(certsPEM, block)
		if checkError(err, false) {
			return
		}
		hash.Write(certsASN1[i])

		// encrypt private key
		block, err = x509.EncryptPEMBlock(rand.Reader, "PRIVATE KEY",
			keysBytes[i], pwd, x509.PEMCipherAES256)
		if checkError(err, false) {
			return
		}
		err = pem.Encode(keysPEM, block)
		if checkError(err, false) {
			return
		}
		hash.Write(keysBytes[i])
	}

	// encrypt system certificates
	for i := 1; i < sNumber+1; i++ {
		block, err := x509.EncryptPEMBlock(rand.Reader, "CERTIFICATE",
			systemASN1[i], pwd, x509.PEMCipherAES256)
		if checkError(err, false) {
			return
		}
		err = pem.Encode(systemPEM, block)
		if checkError(err, false) {
			return
		}
		hash.Write(systemASN1[i])
	}

	// write PEM files and hash
	for _, p := range []*struct {
		filename string
		data     []byte
	}{
		{filename: "key/certs.pem", data: certsPEM.Bytes()},
		{filename: "key/keys.pem", data: keysPEM.Bytes()},
		{filename: "key/system.pem", data: systemPEM.Bytes()},
		{filename: "key/pem.hash", data: hash.Sum(nil)},
	} {
		err := ioutil.WriteFile(p.filename, p.data, 0600)
		if checkError(err, false) {
			return
		}
	}
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
		input = ""
	}
}

func checkError(err error, exit bool) bool {
	if err != nil {
		if strings.Contains(err.Error(), "unexpected newline") {
			return false
		}
		if err != io.EOF {
			fmt.Printf("%s\n", err)
		}
		if exit {
			os.Exit(1)
		}
		return true
	}
	return false
}
