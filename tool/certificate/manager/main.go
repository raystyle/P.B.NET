package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/crypto/ssh/terminal"

	"project/internal/crypto/aes"
	"project/internal/crypto/cert"
	"project/internal/crypto/cert/certutil"
	"project/internal/patch/msgpack"
	"project/internal/security"
)

const (
	certFile    = "key/certs.dat"
	certHash    = "key/certs.hash"
	certFileBak = certFile + ".bak"
	certHashBak = certHash + ".bak"
)

func main() {
	var init bool
	flag.BoolVar(&init, "init", false, "initialize certificate manager")
	flag.Parse()
	if init {
		initialize()
		return
	}
	manage()
}

func initialize() {
	// check exists
	_, err := os.OpenFile(certFile, os.O_RDONLY, 0600)
	if err == nil {
		fmt.Printf("%s has already exists\n", certFile)
		os.Exit(0)
	}
	_, err = os.OpenFile(certHash, os.O_RDONLY, 0600)
	if err == nil {
		fmt.Printf("%s has already exists\n", certHash)
		os.Exit(0)
	}

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
	_ = os.Mkdir("key", 0750)

	// load system certificates
	pool, err := cert.NewPoolWithSystemCerts()
	checkError(err, true)

	// create Root CA certificate
	rootCA, err := cert.GenerateCA(nil)
	checkError(err, true)
	err = pool.AddPrivateRootCACert(rootCA.Encode())
	checkError(err, true)

	// create Client CA certificate
	clientCA, err := cert.GenerateCA(nil)
	checkError(err, true)
	err = pool.AddPrivateClientCACert(clientCA.Encode())
	checkError(err, true)

	// generate a client certificate and use client CA sign it
	clientCert, err := cert.Generate(clientCA.Certificate, clientCA.PrivateKey, nil)
	checkError(err, true)
	err = pool.AddPrivateClientCert(clientCert.Encode())
	checkError(err, true)

	saveCertPool(pool, pwd)
	fmt.Println("initialize certificate manager successfully")
}

func manage() {
	// input password
	fmt.Print("password: ")
	pwd, err := terminal.ReadPassword(int(syscall.Stdin))
	checkError(err, true)
	fmt.Println()
	// load certificate
	data, err := ioutil.ReadFile(certFile)
	checkError(err, true)
	pool := LoadCertPool(data, pwd)
	// start manage
	manager := manager{
		pwd:  security.NewBytes(pwd),
		pool: pool,
	}
	security.CoverBytes(pwd)
	manager.Manage()
}

type rawCertPool struct {
	PublicRootCACerts   [][]byte `msgpack:"a"`
	PublicClientCACerts [][]byte `msgpack:"b"`
	PublicClientPairs   []struct {
		Cert []byte `msgpack:"a"`
		Key  []byte `msgpack:"b"`
	} `msgpack:"c"`
	PrivateRootCAPairs []struct {
		Cert []byte `msgpack:"a"`
		Key  []byte `msgpack:"b"`
	} `msgpack:"d"`
	PrivateClientCAPairs []struct {
		Cert []byte `msgpack:"a"`
		Key  []byte `msgpack:"b"`
	} `msgpack:"e"`
	PrivateClientPairs []struct {
		Cert []byte `msgpack:"a"`
		Key  []byte `msgpack:"b"`
	} `msgpack:"f"`
}

func saveCertPool(pool *cert.Pool, pwd []byte) {
	rcp := rawCertPool{}
	pubRootCACerts := pool.GetPublicRootCACerts()
	for i := 0; i < len(pubRootCACerts); i++ {
		rcp.PublicRootCACerts = append(rcp.PublicRootCACerts, pubRootCACerts[i].Raw)
	}
	pubClientCACerts := pool.GetPublicClientCACerts()
	for i := 0; i < len(pubClientCACerts); i++ {
		rcp.PublicClientCACerts = append(rcp.PublicClientCACerts, pubClientCACerts[i].Raw)
	}
	pubClientPairs := pool.GetPublicClientPairs()
	for i := 0; i < len(pubClientPairs); i++ {
		c, k := pubClientPairs[i].Encode()
		rcp.PublicClientPairs = append(rcp.PublicClientPairs, struct {
			Cert []byte `msgpack:"a"`
			Key  []byte `msgpack:"b"`
		}{Cert: c, Key: k})
	}
	priRootCAPairs := pool.GetPrivateRootCAPairs()
	for i := 0; i < len(priRootCAPairs); i++ {
		c, k := priRootCAPairs[i].Encode()
		rcp.PrivateRootCAPairs = append(rcp.PrivateRootCAPairs, struct {
			Cert []byte `msgpack:"a"`
			Key  []byte `msgpack:"b"`
		}{Cert: c, Key: k})
	}
	priClientCAPairs := pool.GetPrivateClientCAPairs()
	for i := 0; i < len(priClientCAPairs); i++ {
		c, k := priClientCAPairs[i].Encode()
		rcp.PrivateClientCAPairs = append(rcp.PrivateClientCAPairs, struct {
			Cert []byte `msgpack:"a"`
			Key  []byte `msgpack:"b"`
		}{Cert: c, Key: k})
	}
	priClientPairs := pool.GetPrivateClientPairs()
	for i := 0; i < len(priClientPairs); i++ {
		c, k := priClientPairs[i].Encode()
		rcp.PrivateClientPairs = append(rcp.PrivateClientPairs, struct {
			Cert []byte `msgpack:"a"`
			Key  []byte `msgpack:"b"`
		}{Cert: c, Key: k})
	}

	// clean private key
	defer func() {
		for i := 0; i < len(rcp.PublicClientPairs); i++ {
			security.CoverBytes(rcp.PublicClientPairs[i].Key)
		}
		for i := 0; i < len(rcp.PrivateRootCAPairs); i++ {
			security.CoverBytes(rcp.PrivateRootCAPairs[i].Key)
		}
		for i := 0; i < len(rcp.PrivateClientCAPairs); i++ {
			security.CoverBytes(rcp.PrivateClientCAPairs[i].Key)
		}
		for i := 0; i < len(rcp.PrivateClientPairs); i++ {
			security.CoverBytes(rcp.PrivateClientPairs[i].Key)
		}
	}()

	data, err := msgpack.Marshal(&rcp)
	checkError(err, true)
	defer security.CoverBytes(data)

	keyIV := sha256.Sum256(pwd)
	defer security.CoverBytes(keyIV[:])
	cipherData, err := aes.CBCEncrypt(data, keyIV[:], keyIV[:aes.IVSize])
	checkError(err, true)

	// save encrypted certificates
	err = writeFile(certFile, cipherData)
	checkError(err, true)

	// calculate hash
	hash := sha256.New()
	hash.Write(pwd)
	hash.Write(data)
	err = writeFile(certHash, hash.Sum(nil))
	checkError(err, true)
}

// LoadCertPool is used to load certificate pool.
func LoadCertPool(data, pwd []byte) *cert.Pool {
	// decrypt
	keyIV := sha256.Sum256(pwd)
	defer security.CoverBytes(keyIV[:])
	plainData, err := aes.CBCDecrypt(data, keyIV[:], keyIV[:aes.IVSize])
	checkError(err, true)
	defer security.CoverBytes(plainData)

	// compare hash
	rawHash, err := ioutil.ReadFile(certHash)
	checkError(err, true)

	hash := sha256.New()
	hash.Write(pwd)
	hash.Write(plainData)
	if subtle.ConstantTimeCompare(rawHash, hash.Sum(nil)) != 1 {
		fmt.Printf("exploit: %s has been tampered or incorrect password\n", certFile)
		os.Exit(0)
	}

	// load
	pool := rawCertPool{}
	err = msgpack.Unmarshal(plainData, &pool)
	checkError(err, true)

	memory := security.NewMemory()
	defer memory.Flush()

	certPool := cert.NewPool()
	for i := 0; i < len(pool.PublicRootCACerts); i++ {
		err := certPool.AddPublicRootCACert(pool.PublicRootCACerts[i])
		checkError(err, true)
	}
	for i := 0; i < len(pool.PublicClientCACerts); i++ {
		err := certPool.AddPublicClientCACert(pool.PublicClientCACerts[i])
		checkError(err, true)
	}
	for i := 0; i < len(pool.PublicClientPairs); i++ {
		memory.Padding()
		pair := pool.PublicClientPairs[i]
		err := certPool.AddPublicClientCert(pair.Cert, pair.Key)
		checkError(err, true)
	}
	for i := 0; i < len(pool.PrivateRootCAPairs); i++ {
		memory.Padding()
		pair := pool.PrivateRootCAPairs[i]
		err := certPool.AddPrivateRootCACert(pair.Cert, pair.Key)
		checkError(err, true)
	}
	for i := 0; i < len(pool.PrivateClientCAPairs); i++ {
		memory.Padding()
		pair := pool.PrivateClientCAPairs[i]
		err := certPool.AddPrivateClientCACert(pair.Cert, pair.Key)
		checkError(err, true)
	}
	for i := 0; i < len(pool.PrivateClientPairs); i++ {
		memory.Padding()
		pair := pool.PrivateClientPairs[i]
		err := certPool.AddPrivateClientCert(pair.Cert, pair.Key)
		checkError(err, true)
	}
	return certPool
}

const (
	prefixManager         = "manager"
	prefixPublic          = "manager/public"
	prefixPublicRootCA    = "manager/public/root-ca"
	prefixPublicClientCA  = "manager/public/client-ca"
	prefixPublicClient    = "manager/public/client"
	prefixPrivate         = "manager/private"
	prefixPrivateRootCA   = "manager/private/root-ca"
	prefixPrivateClientCA = "manager/private/client-ca"
	prefixPrivateClient   = "manager/private/client"
)

type manager struct {
	pwd     *security.Bytes
	pool    *cert.Pool
	prefix  string
	scanner *bufio.Scanner
}

func (m *manager) Manage() {
	// create backup
	m.backup()
	// interrupt input
	go func() {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt)
		for {
			<-signalChan
		}
	}()
	m.prefix = prefixManager
	m.scanner = bufio.NewScanner(os.Stdin)
	for {
		fmt.Printf("%s> ", m.prefix)
		if !m.scanner.Scan() {
			m.scanner = bufio.NewScanner(os.Stdin)
			fmt.Println()
			continue
		}
		switch m.prefix {
		case prefixManager:
			m.manager()
		case prefixPublic:
			m.public()
		case prefixPrivate:
			m.private()
		case prefixPublicRootCA:
			m.publicRootCA()
		case prefixPublicClientCA:
			m.publicClientCA()
		case prefixPublicClient:
			m.publicClient()
		case prefixPrivateRootCA:
			m.privateRootCA()
		case prefixPrivateClientCA:
			m.privateClientCA()
		case prefixPrivateClient:
			m.privateClient()
		default:
			fmt.Printf("unknown prefix: %s\n", m.prefix)
			os.Exit(1)
		}
	}
}

func (m *manager) manager() {
	cmd := m.scanner.Text()
	switch cmd {
	case "":
	case "public":
		m.prefix = prefixPublic
	case "private":
		m.prefix = prefixPrivate
	case "help":
		m.managerHelp()
	case "save":
		m.save()
	case "exit":
		m.exit()
	default:
		fmt.Printf("unknown command: %s\n", cmd)
	}
}

func (m *manager) backup() {
	// certificate
	data, err := ioutil.ReadFile(certFile)
	checkError(err, true)
	err = writeFile(certFileBak, data)
	checkError(err, true)
	// hash
	data, err = ioutil.ReadFile(certHash)
	checkError(err, true)
	err = writeFile(certHashBak, data)
	checkError(err, true)
}

func (m *manager) managerHelp() {
	const help = `
help about manager:
  
  public       switch to public mode
  private      switch to private mode
  help         print help
  save         save certificate pool
  exit         return to the manager
  
`
	fmt.Print(help)
}

func (m *manager) save() {
	pwd := m.pwd.Get()
	defer m.pwd.Put(pwd)
	saveCertPool(m.pool, pwd)
}

func (m *manager) exit() {
	// delete backup
	checkError(os.Remove(certFileBak), true)
	checkError(os.Remove(certHashBak), true)
	fmt.Println("Bye!")
	os.Exit(0)
}

func (m *manager) public() {
	cmd := m.scanner.Text()
	switch cmd {
	case "":
	case "root-ca":
		m.prefix = prefixPublicRootCA
	case "client-ca":
		m.prefix = prefixPublicClientCA
	case "client":
		m.prefix = prefixPublicClient
	case "help":
		m.publicHelp()
	case "save":
		m.save()
	case "exit":
		m.prefix = prefixManager
	default:
		fmt.Printf("unknown command: %s\n", cmd)
	}
}

func (m *manager) publicHelp() {
	const help = `
help about manager/public:
  
  root-ca      switch to public/root-ca mode
  client-ca    switch to public/client-ca mode
  client       switch to public/client mode
  help         print help
  save         save certificate pool
  exit         return to the manager
  
`
	fmt.Print(help)
}

func (m *manager) publicRootCA() {
	cmd := m.scanner.Text()
	// cmd[:3] == "add"
	switch cmd {
	case "":
	case "list":
		m.publicRootCAList()
	case "help":
		m.publicRootCAHelp()
	case "save":
		m.save()
	case "exit":
		m.prefix = prefixPublic
	default:
		if len(cmd) > 4 {
			switch cmd[:4] {
			case "add ":
				m.publicRootCAAdd(cmd[4:])
				return
			case "del ":
				m.publicRootCADel(cmd[4:])
				return
			}
		}
		fmt.Printf("unknown command: %s\n", cmd)
	}
}

func (m *manager) publicRootCAHelp() {
	const help = `
help about manager/public/root-ca:
  
  add          add a certificate
  del          delete a certificate with ID
  list         list all Root CA certificates
  help         print help
  save         save certificate pool
  exit         return to the public
  
`
	fmt.Print(help)
}

func (m *manager) publicRootCAList() {
	fmt.Println()
	certs := m.pool.GetPublicRootCACerts()
	for i := 0; i < len(certs); i++ {
		printCertificate(i, certs[i])
	}
}

func (m *manager) publicRootCAAdd(file string) {
	pemData, err := ioutil.ReadFile(file)
	if checkError(err, false) {
		return
	}
	certs, err := certutil.ParseCertificates(pemData)
	if checkError(err, false) {
		return
	}
	for i := 0; i < len(certs); i++ {
		err = m.pool.AddPublicRootCACert(certs[i].Raw)
		if checkError(err, false) {
			fmt.Printf("\n%s\n\n", cert.Print(certs[i]))
		}
	}
}

func (m *manager) publicRootCADel(id string) {
	i, err := strconv.Atoi(id)
	if checkError(err, false) {
		return
	}
	fmt.Println(i)
	// err := m.pool.AddPublicRootCACert()
	// checkError(err, false)
}

func (m *manager) publicClientCA() {
	cmd := m.scanner.Text()
	switch cmd {
	case "":
	case "root-ca":
		m.prefix = prefixPublicRootCA
	case "client-ca":
		m.prefix = prefixPublicClientCA
	case "client":
		m.prefix = prefixPublicClient
	case "help":
		m.publicHelp()
	case "save":
		m.save()
	case "exit":
		m.prefix = prefixPublic
	default:
		fmt.Printf("unknown command: %s\n", cmd)
	}
}

func (m *manager) publicClient() {
	cmd := m.scanner.Text()
	switch cmd {
	case "":
	case "root-ca":
		m.prefix = prefixPublicRootCA
	case "client-ca":
		m.prefix = prefixPublicClientCA
	case "client":
		m.prefix = prefixPublicClient
	case "help":
		m.publicHelp()
	case "save":
		m.save()
	case "exit":
		m.prefix = prefixPublic
	default:
		fmt.Printf("unknown command: %s\n", cmd)
	}
}

func (m *manager) private() {
	cmd := m.scanner.Text()
	switch cmd {
	case "":
	case "root-ca":
		m.prefix = prefixPrivateRootCA
	case "client-ca":
		m.prefix = prefixPrivateClientCA
	case "client":
		m.prefix = prefixPrivateClient
	case "help":
		m.privateHelp()
	case "save":
		m.save()
	case "exit":
		m.prefix = prefixManager
	default:
		fmt.Printf("unknown command: %s\n", cmd)
	}
}

func (m *manager) privateHelp() {
	const help = `
help about manager/private:
  
  root-ca      switch to private/root-ca mode
  client-ca    switch to private/client-ca mode
  client       switch to private/client mode
  help         print help
  save         save certificate pool
  exit         return to the manager
  
`
	fmt.Print(help)
}

func (m *manager) privateRootCA() {
	cmd := m.scanner.Text()
	switch cmd {
	case "":
	case "root-ca":
		m.prefix = prefixPublicRootCA
	case "client-ca":
		m.prefix = prefixPublicClientCA
	case "client":
		m.prefix = prefixPublicClient
	case "help":
		m.publicHelp()
	case "save":
		m.save()
	case "exit":
		m.prefix = prefixPrivate
	default:
		fmt.Printf("unknown command: %s\n", cmd)
	}
}

func (m *manager) privateClientCA() {
	cmd := m.scanner.Text()
	switch cmd {
	case "":
	case "root-ca":
		m.prefix = prefixPublicRootCA
	case "client-ca":
		m.prefix = prefixPublicClientCA
	case "client":
		m.prefix = prefixPublicClient
	case "help":
		m.publicHelp()
	case "save":
		m.save()
	case "exit":
		m.prefix = prefixPrivate
	default:
		fmt.Printf("unknown command: %s\n", cmd)
	}
}

func (m *manager) privateClient() {
	cmd := m.scanner.Text()
	switch cmd {
	case "":
	case "root-ca":
		m.prefix = prefixPublicRootCA
	case "client-ca":
		m.prefix = prefixPublicClientCA
	case "client":
		m.prefix = prefixPublicClient
	case "help":
		m.publicHelp()
	case "save":
		m.save()
	case "exit":
		m.prefix = prefixPrivate
	default:
		fmt.Printf("unknown command: %s\n", cmd)
	}
}

func printCertificate(id int, c *x509.Certificate) {
	fmt.Printf("ID: %d\n%s\n\n", id, cert.Print(c))
}

func writeFile(filename string, data []byte) error {
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	_, err = file.Write(data)
	if err1 := file.Sync(); err == nil {
		err = err1
	}
	if err1 := file.Close(); err == nil {
		err = err1
	}
	return err
}

func checkError(err error, exit bool) bool {
	if err != nil {
		if strings.Contains(err.Error(), "unexpected newline") {
			return true
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
