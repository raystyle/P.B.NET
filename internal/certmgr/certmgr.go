package certmgr

import (
	"bytes"
	"compress/flate"
	"crypto/sha256"
	"crypto/subtle"

	"github.com/pkg/errors"

	"project/internal/crypto/aes"
	"project/internal/crypto/cert"
	"project/internal/patch/msgpack"
	"project/internal/security"
	"project/internal/system"
)

// file path about certificate pool.
const (
	CertFilePath = "key/certs.dat"
	HashFilePath = "key/certs.hash"
)

// ctrlCertPool include bytes about certificates and private keys.
// package controller and tool/certificate/manager will use it.
type ctrlCertPool struct {
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

// calculateAESKeyFromPassword is used to generate aes key for encrypt certificate pool.
func calculateAESKeyFromPassword(password []byte) ([]byte, []byte) {
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

// SaveCtrlCertPool is used to compress and encrypt certificate pool.
func SaveCtrlCertPool(pool *cert.Pool, password []byte) error {
	cp := new(ctrlCertPool)
	// clean private key at once
	defer func() {
		for i := 0; i < len(cp.PublicClientPairs); i++ {
			security.CoverBytes(cp.PublicClientPairs[i].Key)
		}
		for i := 0; i < len(cp.PrivateRootCAPairs); i++ {
			security.CoverBytes(cp.PrivateRootCAPairs[i].Key)
		}
		for i := 0; i < len(cp.PrivateClientCAPairs); i++ {
			security.CoverBytes(cp.PrivateClientCAPairs[i].Key)
		}
		for i := 0; i < len(cp.PrivateClientPairs); i++ {
			security.CoverBytes(cp.PrivateClientPairs[i].Key)
		}
	}()
	getCertsFromPool(pool, cp)
	// marshal
	certsData, err := msgpack.Marshal(cp)
	if err != nil {
		return err
	}
	defer security.CoverBytes(certsData)
	// compress
	buf := bytes.NewBuffer(make([]byte, 0, len(certsData)/2))
	writer, err := flate.NewWriter(buf, flate.BestCompression)
	if err != nil {
		return errors.Wrap(err, "failed to create deflate writer")
	}
	_, err = writer.Write(certsData)
	if err != nil {
		return errors.Wrap(err, "failed to compress certificate data")
	}
	err = writer.Close()
	if err != nil {
		return errors.Wrap(err, "failed to close deflate writer")
	}
	// encrypt certificates
	aesKey, aesIV := calculateAESKeyFromPassword(password)
	defer func() {
		security.CoverBytes(aesKey)
		security.CoverBytes(aesIV)
	}()
	cipherData, err := aes.CBCEncrypt(buf.Bytes(), aesKey, aesIV)
	if err != nil {
		return errors.Wrap(err, "failed to encrypt certificate data")
	}
	// save encrypted certificates
	err = system.WriteFile(CertFilePath, cipherData)
	if err != nil {
		return err
	}
	// calculate hash and save
	hash := sha256.New()
	hash.Write(password)
	hash.Write(certsData)
	return system.WriteFile(HashFilePath, hash.Sum(nil))
}

func getCertsFromPool(pool *cert.Pool, cp *ctrlCertPool) {
	pubRootCACerts := pool.GetPublicRootCACerts()
	for i := 0; i < len(pubRootCACerts); i++ {
		cp.PublicRootCACerts = append(cp.PublicRootCACerts, pubRootCACerts[i].Raw)
	}
	pubClientCACerts := pool.GetPublicClientCACerts()
	for i := 0; i < len(pubClientCACerts); i++ {
		cp.PublicClientCACerts = append(cp.PublicClientCACerts, pubClientCACerts[i].Raw)
	}
	pubClientPairs := pool.GetPublicClientPairs()
	for i := 0; i < len(pubClientPairs); i++ {
		c, k := pubClientPairs[i].Encode()
		cp.PublicClientPairs = append(cp.PublicClientPairs, struct {
			Cert []byte `msgpack:"a"`
			Key  []byte `msgpack:"b"`
		}{Cert: c, Key: k})
	}
	priRootCAPairs := pool.GetPrivateRootCAPairs()
	for i := 0; i < len(priRootCAPairs); i++ {
		c, k := priRootCAPairs[i].Encode()
		cp.PrivateRootCAPairs = append(cp.PrivateRootCAPairs, struct {
			Cert []byte `msgpack:"a"`
			Key  []byte `msgpack:"b"`
		}{Cert: c, Key: k})
	}
	priClientCAPairs := pool.GetPrivateClientCAPairs()
	for i := 0; i < len(priClientCAPairs); i++ {
		c, k := priClientCAPairs[i].Encode()
		cp.PrivateClientCAPairs = append(cp.PrivateClientCAPairs, struct {
			Cert []byte `msgpack:"a"`
			Key  []byte `msgpack:"b"`
		}{Cert: c, Key: k})
	}
	priClientPairs := pool.GetPrivateClientPairs()
	for i := 0; i < len(priClientPairs); i++ {
		c, k := priClientPairs[i].Encode()
		cp.PrivateClientPairs = append(cp.PrivateClientPairs, struct {
			Cert []byte `msgpack:"a"`
			Key  []byte `msgpack:"b"`
		}{Cert: c, Key: k})
	}
}

// LoadCtrlCertPool is used to decrypt and decompress certificate pool.
func LoadCtrlCertPool(pool *cert.Pool, cipherData, hashData, password []byte) error {
	// decrypt certificates
	aesKey, aesIV := calculateAESKeyFromPassword(password)
	defer func() {
		security.CoverBytes(aesKey)
		security.CoverBytes(aesIV)
	}()
	plainData, err := aes.CBCDecrypt(cipherData, aesKey, aesIV)
	if err != nil {
		return errors.Wrap(err, "failed to decrypt certificates data")
	}
	defer security.CoverBytes(plainData)
	// decompress
	buf := bytes.NewBuffer(make([]byte, 0, len(plainData)*2))
	reader := flate.NewReader(bytes.NewReader(plainData))
	_, err = buf.ReadFrom(reader)
	if err != nil {
		return errors.Wrap(err, "failed to decompress")
	}
	err = reader.Close()
	if err != nil {
		return errors.Wrap(err, "failed to close deflate reader")
	}
	certsData := buf.Bytes()
	// compare hash
	hash := sha256.New()
	hash.Write(password)
	hash.Write(certsData)
	if subtle.ConstantTimeCompare(hashData, hash.Sum(nil)) != 1 {
		const msg = "exploit: certificate pool has been tampered or incorrect password"
		return errors.New(msg)
	}
	// unmarshal
	cp := ctrlCertPool{}
	err = msgpack.Unmarshal(certsData, &cp)
	if err != nil {
		return err
	}
	return addCertsToPool(pool, &cp)
}

func addCertsToPool(pool *cert.Pool, cp *ctrlCertPool) error {
	memory := security.NewMemory()
	defer memory.Flush()

	var err error
	for i := 0; i < len(cp.PublicRootCACerts); i++ {
		err = pool.AddPublicRootCACert(cp.PublicRootCACerts[i])
		if err != nil {
			return err
		}
	}
	for i := 0; i < len(cp.PublicClientCACerts); i++ {
		err = pool.AddPublicClientCACert(cp.PublicClientCACerts[i])
		if err != nil {
			return err
		}
	}
	for i := 0; i < len(cp.PublicClientPairs); i++ {
		memory.Padding()
		pair := cp.PublicClientPairs[i]
		err = pool.AddPublicClientCert(pair.Cert, pair.Key)
		if err != nil {
			return err
		}
	}
	for i := 0; i < len(cp.PrivateRootCAPairs); i++ {
		memory.Padding()
		pair := cp.PrivateRootCAPairs[i]
		err = pool.AddPrivateRootCACert(pair.Cert, pair.Key)
		if err != nil {
			return err
		}
	}
	for i := 0; i < len(cp.PrivateClientCAPairs); i++ {
		memory.Padding()
		pair := cp.PrivateClientCAPairs[i]
		err = pool.AddPrivateClientCACert(pair.Cert, pair.Key)
		if err != nil {
			return err
		}
	}
	for i := 0; i < len(cp.PrivateClientPairs); i++ {
		memory.Padding()
		pair := cp.PrivateClientPairs[i]
		err = pool.AddPrivateClientCert(pair.Cert, pair.Key)
		if err != nil {
			return err
		}
	}
	return nil
}

// NBCertPool contains raw certificates, it used for Node and Beacon configuration.
// package node and beacon will use it.
type NBCertPool struct {
	PublicRootCACerts   [][]byte `msgpack:"a"`
	PublicClientCACerts [][]byte `msgpack:"b"`
	PublicClientPairs   []struct {
		Cert []byte `msgpack:"a"`
		Key  []byte `msgpack:"b"`
	} `msgpack:"c"`
	PrivateRootCACerts   [][]byte `msgpack:"d"`
	PrivateClientCACerts [][]byte `msgpack:"e"`
	PrivateClientPairs   []struct {
		Cert []byte `msgpack:"a"`
		Key  []byte `msgpack:"b"`
	} `msgpack:"f"`
}

// GetCertsFromPool is used to add certificates to NBCertPool from certificate pool.
// controller will add certificates to NBCertPool.
func (cp *NBCertPool) GetCertsFromPool(pool *cert.Pool) {
	pubRootCACerts := pool.GetPublicRootCACerts()
	for i := 0; i < len(pubRootCACerts); i++ {
		cp.PublicRootCACerts = append(cp.PublicRootCACerts, pubRootCACerts[i].Raw)
	}
	pubClientCACerts := pool.GetPublicClientCACerts()
	for i := 0; i < len(pubClientCACerts); i++ {
		cp.PublicClientCACerts = append(cp.PublicClientCACerts, pubClientCACerts[i].Raw)
	}
	pubClientPairs := pool.GetPublicClientPairs()
	for i := 0; i < len(pubClientPairs); i++ {
		c, k := pubClientPairs[i].Encode()
		cp.PublicClientPairs = append(cp.PublicClientPairs, struct {
			Cert []byte `msgpack:"a"`
			Key  []byte `msgpack:"b"`
		}{Cert: c, Key: k})
	}
	priRootCACerts := pool.GetPrivateRootCACerts()
	for i := 0; i < len(priRootCACerts); i++ {
		cp.PrivateRootCACerts = append(cp.PrivateRootCACerts, priRootCACerts[i].Raw)
	}
	priClientCACerts := pool.GetPrivateClientCACerts()
	for i := 0; i < len(priClientCACerts); i++ {
		cp.PrivateClientCACerts = append(cp.PrivateClientCACerts, priClientCACerts[i].Raw)
	}
	priClientPairs := pool.GetPrivateClientPairs()
	for i := 0; i < len(priClientPairs); i++ {
		c, k := priClientPairs[i].Encode()
		cp.PrivateClientPairs = append(cp.PrivateClientPairs, struct {
			Cert []byte `msgpack:"a"`
			Key  []byte `msgpack:"b"`
		}{Cert: c, Key: k})
	}
}

// ToPool is used to create a certificate pool from NBCertPool.
func (cp *NBCertPool) ToPool() (*cert.Pool, error) {
	memory := security.NewMemory()
	defer memory.Flush()

	pool := cert.NewPool()
	for i := 0; i < len(cp.PublicRootCACerts); i++ {
		err := pool.AddPublicRootCACert(cp.PublicRootCACerts[i])
		if err != nil {
			return nil, err
		}
	}
	for i := 0; i < len(cp.PublicClientCACerts); i++ {
		err := pool.AddPublicClientCACert(cp.PublicClientCACerts[i])
		if err != nil {
			return nil, err
		}
	}
	for i := 0; i < len(cp.PublicClientPairs); i++ {
		memory.Padding()
		pair := cp.PublicClientPairs[i]
		err := pool.AddPublicClientCert(pair.Cert, pair.Key)
		if err != nil {
			return nil, err
		}
	}
	for i := 0; i < len(cp.PrivateRootCACerts); i++ {
		err := pool.AddPrivateRootCACert(cp.PrivateRootCACerts[i], nil)
		if err != nil {
			return nil, err
		}
	}
	for i := 0; i < len(cp.PrivateClientCACerts); i++ {
		err := pool.AddPrivateClientCACert(cp.PrivateClientCACerts[i], nil)
		if err != nil {
			return nil, err
		}
	}
	for i := 0; i < len(cp.PrivateClientPairs); i++ {
		memory.Padding()
		pair := cp.PrivateClientPairs[i]
		err := pool.AddPrivateClientCert(pair.Cert, pair.Key)
		if err != nil {
			return nil, err
		}
	}
	return pool, nil
}
