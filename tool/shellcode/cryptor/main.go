package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"log"

	"project/internal/crypto/aes"
	"project/internal/crypto/sha256"
)

func main() {
	var (
		shellcode string
		key       string
	)
	flag.StringVar(&shellcode, "sc", "", "shellcode")
	flag.StringVar(&key, "k", "test", "aes key")
	flag.Parse()

	sc, err := hex.DecodeString(shellcode)
	if err != nil {
		log.Fatal(err)
	}

	aesKey := sha256.Bytes([]byte(key))
	cipherData, err := aes.CBCEncrypt(sc, aesKey, aesKey[:aes.IVSize])
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(hex.EncodeToString(cipherData))
}
