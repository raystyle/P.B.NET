package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"log"

	"project/internal/crypto/aes"
	"project/internal/module/shellcode"
)

func main() {
	var (
		method string
		key    string
		sc     string
	)
	flag.StringVar(&method, "m", "", "execute method")
	flag.StringVar(&key, "k", "test", "aes key")
	flag.StringVar(&sc, "sc", "", "shellcode")
	flag.Parse()

	cipherData, err := hex.DecodeString(sc)
	if err != nil {
		log.Fatal(err)
	}
	hash := sha256.New()
	hash.Write([]byte(key))
	aesKey := hash.Sum(nil)
	s, err := aes.CBCDecrypt(cipherData, aesKey, aesKey[:aes.IVSize])
	if err != nil {
		log.Fatal(err)
	}

	err = shellcode.Execute(method, s)
	if err != nil {
		log.Fatal(err)
	}
}
