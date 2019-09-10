package testdata

import (
	"io/ioutil"

	"github.com/pkg/errors"

	"project/internal/crypto/aes"
	"project/internal/crypto/curve25519"
	"project/internal/crypto/ed25519"
	"project/internal/crypto/sha256"
)

var (
	CtrlED25519    ed25519.PrivateKey
	CtrlCurve25519 []byte
	CtrlAESKey     []byte
)

func init() {
	keys, err := loadCtrlKeys("../app/key/ctrl.key", "123456789012")
	if err != nil {
		panic(err)
	}
	// ed25519
	pri, err := ed25519.ImportPrivateKey(keys[0])
	if err != nil {
		panic(err)
	}
	CtrlED25519 = pri
	// curve25519
	pub, err := curve25519.ScalarBaseMult(pri[:32])
	if err != nil {
		panic(err)
	}
	CtrlCurve25519 = pub
	// broadcast
	CtrlAESKey = append(keys[1], keys[2]...)
}

// from controller/keygen.go
// return ed25519 private key & aes key & aes iv
func loadCtrlKeys(path, password string) ([3][]byte, error) {
	const (
		name     = "P.B.NET"
		nameSize = len(name)
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
	if string(keyDec[:nameSize]) != name {
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
