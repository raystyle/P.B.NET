package controller

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"io/ioutil"
	"os"

	"github.com/pkg/errors"

	"project/internal/crypto/aes"
	"project/internal/crypto/cert"
	"project/internal/crypto/ed25519"
	"project/internal/patch/msgpack"
	"project/internal/random"
	"project/internal/security"
)

// about key file path.
const (
	sessionKeyFile = "key/session.key"
	CertFile       = "key/certs.dat"
	CertHash       = "key/certs.hash"
)

// GenerateSessionKey is used to generate session key and save to file.
func GenerateSessionKey(password []byte) error {
	_, err := os.Stat(sessionKeyFile)
	if !os.IsNotExist(err) {
		return errors.Errorf("file: %s already exist", sessionKeyFile)
	}
	key, err := generateSessionKey(password)
	if err != nil {
		return nil
	}
	return ioutil.WriteFile(sessionKeyFile, key, 0600)
}

func generateAESKeyIVFromPassword(password []byte) ([]byte, []byte) {
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

func generateSessionKey(password []byte) ([]byte, error) {
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
	aesKey, aesIV := generateAESKeyIVFromPassword(password)
	keysEnc, err := aes.CBCEncrypt(buf.Bytes(), aesKey, aesIV)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return append(keysHash, keysEnc...), nil
}

// return ed25519 private key & aes key & aes iv.
func loadSessionKey(data, password []byte) ([][]byte, error) {
	const sessionKeySize = sha256.Size +
		ed25519.PrivateKeySize + aes.Key256Bit + aes.IVSize +
		aes.BlockSize
	if len(data) != sessionKeySize {
		return nil, errors.New("invalid session key size")
	}
	memory := security.NewMemory()
	defer memory.Flush()
	// decrypt session key
	aesKey, aesIV := generateAESKeyIVFromPassword(password)
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

// RawCertPool include certificates and private keys.
type RawCertPool struct {
	PublicRootCACerts   [][]byte `msgpack:"a"`
	PublicClientCACerts [][]byte `msgpack:"b"`
	PublicClientPairs   []struct {
		Cert []byte `msgpack:"a"`
		Key  []byte `msgpack:"b"`
	} `msgpack:"c"`
	PrivateRootCAPairs []struct {
		Cert []byte `msgpack:"a"`
		Key  []byte `msgpack:"b"`
	} `msgpack:"d"`
	PrivateClientCAPairs []struct {
		Cert []byte `msgpack:"a"`
		Key  []byte `msgpack:"b"`
	} `msgpack:"e"`
	PrivateClientPairs []struct {
		Cert []byte `msgpack:"a"`
		Key  []byte `msgpack:"b"`
	} `msgpack:"f"`
}

// GenerateCertPoolAESKey is used to generate aes key for encrypt certificate pool.
func GenerateCertPoolAESKey(password []byte) ([]byte, []byte) {
	hash := sha256.New()
	hash.Write(password)
	hashed := hash.Sum(nil)
	for i := 0; i < 10000; i++ {
		hash.Write(hashed)
		hashed = hash.Sum(nil)
	}
	keyIV := hash.Sum(nil)
	return keyIV, keyIV[:aes.IVSize]
}

// LoadCertPool is used to load certificate pool.
func LoadCertPool(cipherData, rawHash, password []byte) (*cert.Pool, error) {
	pool := cert.NewPool()
	err := loadCertPool(pool, cipherData, rawHash, password)
	if err != nil {
		return nil, err
	}
	return pool, err
}

func loadCertPool(pool *cert.Pool, cipherData, rawHash, password []byte) error {
	// decrypt certificates
	aesKey, aesIV := GenerateCertPoolAESKey(password)
	defer func() {
		security.CoverBytes(aesKey)
		security.CoverBytes(aesIV)
	}()
	plainData, err := aes.CBCDecrypt(cipherData, aesKey, aesIV)
	if err != nil {
		return err
	}
	defer security.CoverBytes(plainData)
	// compare hash
	hash := sha256.New()
	hash.Write(password)
	hash.Write(plainData)
	if subtle.ConstantTimeCompare(rawHash, hash.Sum(nil)) != 1 {
		const msg = "exploit: certificate pool has been tampered or incorrect password"
		return errors.New(msg)
	}
	return addCertificatesToPool(pool, plainData)
}

func addCertificatesToPool(pool *cert.Pool, plainData []byte) error {
	rcp := RawCertPool{}
	err := msgpack.Unmarshal(plainData, &rcp)
	if err != nil {
		return err
	}
	memory := security.NewMemory()
	defer memory.Flush()
	for i := 0; i < len(rcp.PublicRootCACerts); i++ {
		err = pool.AddPublicRootCACert(rcp.PublicRootCACerts[i])
		if err != nil {
			return err
		}
	}
	for i := 0; i < len(rcp.PublicClientCACerts); i++ {
		err := pool.AddPublicClientCACert(rcp.PublicClientCACerts[i])
		if err != nil {
			return err
		}
	}
	for i := 0; i < len(rcp.PublicClientPairs); i++ {
		memory.Padding()
		pair := rcp.PublicClientPairs[i]
		err = pool.AddPublicClientCert(pair.Cert, pair.Key)
		if err != nil {
			return err
		}
	}
	for i := 0; i < len(rcp.PrivateRootCAPairs); i++ {
		memory.Padding()
		pair := rcp.PrivateRootCAPairs[i]
		err = pool.AddPrivateRootCACert(pair.Cert, pair.Key)
		if err != nil {
			return err
		}
	}
	for i := 0; i < len(rcp.PrivateClientCAPairs); i++ {
		memory.Padding()
		pair := rcp.PrivateClientCAPairs[i]
		err = pool.AddPrivateClientCACert(pair.Cert, pair.Key)
		if err != nil {
			return err
		}
	}
	for i := 0; i < len(rcp.PrivateClientPairs); i++ {
		memory.Padding()
		pair := rcp.PrivateClientPairs[i]
		err = pool.AddPrivateClientCert(pair.Cert, pair.Key)
		if err != nil {
			return err
		}
	}
	return nil
}
