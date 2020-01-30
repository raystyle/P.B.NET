package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"log"

	"project/internal/crypto/aes"
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
		log.Fatalln(err)
	}
	hash := sha256.New()
	hash.Write([]byte(key))
	aesKey := hash.Sum(nil)
	cipherData, err := aes.CBCEncrypt(sc, aesKey, aesKey[:aes.IVSize])
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println(hex.EncodeToString(cipherData))
}
