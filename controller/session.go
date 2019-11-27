package controller

import (
	"bytes"
	"io/ioutil"
	"os"

	"github.com/pkg/errors"

	"project/internal/crypto/aes"
	"project/internal/crypto/ed25519"
	"project/internal/crypto/sha256"
	"project/internal/random"
	"project/internal/security"
)

// GenerateSessionKey is used to generate session key
func GenerateSessionKey(path, password string) error {
	_, err := os.Stat(path)
	if !os.IsNotExist(err) {
		return errors.Errorf("file: %s already exist", path)
	}
	if len(password) < 12 {
		return errors.New("password is too short")
	}
	// generate ed25519 private key(for sign message)
	privateKey, err := ed25519.GenerateKey()
	if err != nil {
		return errors.WithStack(err)
	}
	// generate aes key & iv(for broadcast message)
	aesKey := random.Bytes(aes.Key256Bit)
	aesIV := random.Bytes(aes.IVSize)
	// write
	buffer := new(bytes.Buffer)
	buffer.Write(privateKey)
	buffer.Write(aesKey)
	buffer.Write(aesIV)
	// encrypt
	key := sha256.Bytes([]byte(password))
	iv := sha256.Bytes([]byte{20, 18, 11, 27})[:aes.IVSize]
	keyEnc, err := aes.CBCEncrypt(buffer.Bytes(), key, iv)
	if err != nil {
		return errors.WithStack(err)
	}
	return ioutil.WriteFile(path, keyEnc, 644)
}

// return ed25519 private key & aes key & aes iv
func loadSessionKey(path, password string) ([3][]byte, error) {
	var keys [3][]byte
	if len(password) < 12 {
		return keys, errors.New("password is too short")
	}
	file, err := ioutil.ReadFile(path)
	if err != nil {
		return keys, errors.WithStack(err)
	}
	// decrypt
	memory := security.NewMemory()
	key := sha256.Bytes([]byte(password))
	iv := sha256.Bytes([]byte{20, 18, 11, 27})[:aes.IVSize]
	keyDec, err := aes.CBCDecrypt(file, key, iv)
	if err != nil {
		return keys, errors.WithStack(err)
	}
	if len(keyDec) != ed25519.PrivateKeySize+aes.Key256Bit+aes.IVSize {
		return keys, errors.New("invalid controller keys size")
	}
	// ed25519 private key
	memory.Padding()
	privateKey := keyDec[:ed25519.PrivateKeySize]
	// aes key & aes iv
	memory.Padding()
	offset := ed25519.PrivateKeySize
	aesKey := keyDec[offset : offset+aes.Key256Bit]
	memory.Padding()
	offset += aes.Key256Bit
	aesIV := keyDec[offset : offset+aes.IVSize]
	// write
	keys[0] = privateKey
	keys[1] = aesKey
	keys[2] = aesIV
	return keys, nil
}
