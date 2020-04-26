package controller

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"

	"github.com/pkg/errors"

	"project/internal/crypto/aes"
	"project/internal/crypto/ed25519"
	"project/internal/random"
	"project/internal/security"
)

// SessionKeyFile is the session key file path.
const SessionKeyFile = "key/session.key"

func calculateAESKeyFromPassword(password []byte) ([]byte, []byte) {
	hash := sha256.New()
	hash.Write(password)
	hash.Write([]byte{20, 18, 11, 27})
	hashed := hash.Sum(nil)
	for i := 0; i < 10000; i++ {
		hash.Write(hashed)
		hashed = hash.Sum(nil)
	}
	keyIV := hash.Sum(nil)
	return keyIV, keyIV[:aes.IVSize]
}

// GenerateSessionKey is used to generate session key.
func GenerateSessionKey(password []byte) ([]byte, error) {
	// generate ed25519 private key(for sign message)
	privateKey, err := ed25519.GenerateKey()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	// generate aes key & iv(for broadcast message)
	broadcastKey := append(random.Bytes(aes.Key256Bit), random.Bytes(aes.IVSize)...)
	// save keys
	buf := new(bytes.Buffer)
	buf.Write(privateKey)
	buf.Write(broadcastKey)
	// calculate hash
	hash := sha256.New()
	hash.Write(buf.Bytes())
	keysHash := hash.Sum(nil)
	// encrypt keys
	aesKey, aesIV := calculateAESKeyFromPassword(password)
	keysEnc, err := aes.CBCEncrypt(buf.Bytes(), aesKey, aesIV)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return append(keysHash, keysEnc...), nil
}

// return ed25519 private key & aes key & aes iv.
func loadSessionKey(data, password []byte) ([][]byte, error) {
	const sessionKeySize = 0 +
		sha256.Size +
		ed25519.PrivateKeySize + aes.Key256Bit + aes.IVSize +
		aes.BlockSize
	if len(data) != sessionKeySize {
		return nil, errors.New("invalid session key size")
	}
	memory := security.NewMemory()
	defer memory.Flush()
	// decrypt session key
	aesKey, aesIV := calculateAESKeyFromPassword(password)
	keysDec, err := aes.CBCDecrypt(data[sha256.Size:], aesKey, aesIV)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	// compare hash
	hash := sha256.New()
	hash.Write(keysDec)
	if subtle.ConstantTimeCompare(data[:sha256.Size], hash.Sum(nil)) != 1 {
		return nil, errors.New("invalid password")
	}
	// ed25519 private key
	memory.Padding()
	privateKey := keysDec[:ed25519.PrivateKeySize]
	// aes key & aes iv
	memory.Padding()
	offset := ed25519.PrivateKeySize
	aesKey = keysDec[offset : offset+aes.Key256Bit]
	memory.Padding()
	offset += aes.Key256Bit
	aesIV = keysDec[offset : offset+aes.IVSize]
	key := make([][]byte, 3)
	key[0] = privateKey
	key[1] = aesKey
	key[2] = aesIV
	return key, nil
}
