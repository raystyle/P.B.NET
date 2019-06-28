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
	key_path  = "key/ctrl.key"
	key_size  = len(Name) + ed25519.PrivateKey_Size + aes.BIT256 + aes.IV_SIZE
)

// name & ed25519 & aes key & aes iv
func Gen_CTRL_Keys(password string) error {
	_, err := os.Stat(key_path)
	if !os.IsNotExist(err) {
		return errors.New("file: " + key_path + " already exist")
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
	ctrl_keys, err := aes.CBC_Encrypt(buffer.Bytes(), key, iv)
	if err != nil {
		return errors.WithStack(err)
	}
	err = ioutil.WriteFile(key_path, ctrl_keys, 644)
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

// ed25519 & aes key & aes iv
func load_ctrl_keys(password string) ([]byte, error) {
	if len(password) < 12 {
		return nil, errors.New("password is too short")
	}
	data, err := ioutil.ReadFile(key_path)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	// decrypt
	key := sha256.Bytes([]byte(password))
	iv := sha256.Bytes([]byte{20, 18, 11, 27})[:aes.IV_SIZE]
	ctrl_keys, err := aes.CBC_Decrypt(data, key, iv)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if len(ctrl_keys) != key_size {
		return nil, errors.New("invalid controller keys size")
	}
	if string(ctrl_keys[:name_size]) != Name {
		return nil, errors.New("invalid controller keys")
	}
	return ctrl_keys[name_size:], nil
}
