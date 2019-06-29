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

const (
	name_size = len(Name)
	Key_Path  = "key/ctrl.key"
	key_size  = name_size + ed25519.PrivateKey_Size + aes.BIT256 + aes.IV_SIZE
)

// name & ed25519 & aes key & aes iv
func Gen_CTRL_Keys(path, password string) error {
	_, err := os.Stat(path)
	if !os.IsNotExist(err) {
		return errors.Errorf("file: %s already exist", path)
	}
	if len(password) < 12 {
		return errors.New("password is too short")
	}
	// generate ed25519 private key
	private_key, err := ed25519.Generate_Key()
	if err != nil {
		return errors.WithStack(err)
	}
	// generate aes key & iv
	aes_key := random.Bytes(aes.BIT256)
	aes_iv := random.Bytes(aes.IV_SIZE)
	buffer := new(bytes.Buffer)
	buffer.WriteString(Name)
	buffer.Write(private_key)
	buffer.Write(aes_key)
	buffer.Write(aes_iv)
	// encrypt
	key := sha256.Bytes([]byte(password))
	iv := sha256.Bytes([]byte{20, 18, 11, 27})[:aes.IV_SIZE]
	key_enc, err := aes.CBC_Encrypt(buffer.Bytes(), key, iv)
	if err != nil {
		return errors.WithStack(err)
	}
	err = ioutil.WriteFile(path, key_enc, 644)
	return errors.WithStack(err)
}

// return ed25519 private key & aes key & aes iv
func Load_CTRL_Keys(path, password string) ([3][]byte, error) {
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
	iv := sha256.Bytes([]byte{20, 18, 11, 27})[:aes.IV_SIZE]
	key_dec, err := aes.CBC_Decrypt(data, key, iv)
	if err != nil {
		return keys, errors.WithStack(err)
	}
	if len(key_dec) != key_size {
		return keys, errors.New("invalid controller keys size")
	}
	// check header
	if string(key_dec[:name_size]) != Name {
		return keys, errors.New("invalid controller keys")
	}
	raw := key_dec[name_size:]
	// ed25519 private key
	privatekey := raw[:ed25519.PrivateKey_Size]
	// aes key & aes iv
	offset := ed25519.PrivateKey_Size
	aes_key := raw[offset : offset+aes.BIT256]
	offset += aes.BIT256
	aes_iv := raw[offset : offset+aes.IV_SIZE]
	// write
	keys[0] = privatekey
	keys[1] = aes_key
	keys[2] = aes_iv
	return keys, nil
}
