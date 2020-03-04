package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io/ioutil"
	"log"

	"project/internal/crypto/aes"
)

func main() {
	var (
		hexStr string
		file   string
		key    string
	)
	flag.StringVar(&hexStr, "hex", "", "hex encoded payload")
	flag.StringVar(&file, "file", "payload", "payload file")
	flag.StringVar(&key, "k", "test", "aes key")
	flag.Parse()

	var (
		data []byte
		err  error
	)
	if hexStr != "" {
		data, err = hex.DecodeString(hexStr)
	} else {
		data, err = ioutil.ReadFile(file) // #nosec
	}
	if err != nil {
		log.Fatalln(err)
	}
	hash := sha256.New()
	hash.Write([]byte(key))
	aesKey := hash.Sum(nil)
	cipherData, err := aes.CBCEncrypt(data, aesKey, aesKey[:aes.IVSize])
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println(hex.EncodeToString(cipherData))
}
