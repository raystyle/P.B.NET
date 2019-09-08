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
)

// name & ed25519 & aes key & aes iv
func GenCtrlKeys(path, password string) error {
	_, err := os.Stat(path)
	if !os.IsNotExist(err) {
		return errors.Errorf("file: %s already exist", path)
	}
	if len(password) < 12 {
		return errors.New("password is too short")
	}
	// generate ed25519 private key
	privateKey, err := ed25519.GenerateKey()
	if err != nil {
		return errors.WithStack(err)
	}
	// generate aes key & iv
	aesKey := random.Bytes(aes.Bit256)
	aesIV := random.Bytes(aes.IVSize)
	buffer := new(bytes.Buffer)
	buffer.WriteString(Name)
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
	err = ioutil.WriteFile(path, keyEnc, 644)
	return errors.WithStack(err)
}

// return ed25519 private key & aes key & aes iv
func loadCtrlKeys(path, password string) ([3][]byte, error) {
	const (
		nameSize = len(Name)
		keySize  = nameSize + ed25519.PrivateKeySize + aes.Bit256 + aes.IVSize
	)
	var keys [3][]byte
	if len(password) < 12 {
		return keys, errors.New("password is too short")
	}
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return keys, errors.WithStack(err)
	}
	// decrypt
	key := sha256.Bytes([]byte(password))
	iv := sha256.Bytes([]byte{20, 18, 11, 27})[:aes.IVSize]
	keyDec, err := aes.CBCDecrypt(data, key, iv)
	if err != nil {
		return keys, errors.WithStack(err)
	}
	if len(keyDec) != keySize {
		return keys, errors.New("invalid controller keys size")
	}
	// check header
	if string(keyDec[:nameSize]) != Name {
		return keys, errors.New("invalid controller keys")
	}
	raw := keyDec[nameSize:]
	// ed25519 private key
	privateKey := raw[:ed25519.PrivateKeySize]
	// aes key & aes iv
	offset := ed25519.PrivateKeySize
	aesKey := raw[offset : offset+aes.Bit256]
	offset += aes.Bit256
	aesIV := raw[offset : offset+aes.IVSize]
	// write
	keys[0] = privateKey
	keys[1] = aesKey
	keys[2] = aesIV
	return keys, nil
}
