package testdata

import (
	"io/ioutil"

	"github.com/pkg/errors"

	"project/internal/crypto/aes"
	"project/internal/crypto/ed25519"
	"project/internal/crypto/sha256"
)

var (
	CTRL_Keys_PWD = "123456789012"
	CTRL_ED25519  ed25519.PrivateKey
	CTRL_AES_Key  []byte
)

func init() {
	keys, err := load_ctrl_keys("../app/key/ctrl.key", CTRL_Keys_PWD)
	if err != nil {
		panic(err)
	}
	// ed25519
	pri, _ := ed25519.Import_PrivateKey(keys[0])
	CTRL_ED25519 = pri
	CTRL_AES_Key = append(keys[1],keys[2]...)
}

// from controller/keygen.go
// return ed25519 private key & aes key & aes iv
func load_ctrl_keys(path, password string) ([3][]byte, error) {
	const (
		name      = "P.B.NET"
		name_size = len(name)
		key_size  = name_size + ed25519.PrivateKey_Size + aes.BIT256 + aes.IV_SIZE
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
	iv := sha256.Bytes([]byte{20, 18, 11, 27})[:aes.IV_SIZE]
	key_dec, err := aes.CBC_Decrypt(data, key, iv)
	if err != nil {
		return keys, errors.WithStack(err)
	}
	if len(key_dec) != key_size {
		return keys, errors.New("invalid controller keys size")
	}
	// check header
	if string(key_dec[:name_size]) != name {
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
