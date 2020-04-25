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
// controller and tool/certificate/manager will use it.
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
	rcp := new(ctrlCertPool)
	// clean private key at once
	defer func() {
		for i := 0; i < len(rcp.PublicClientPairs); i++ {
			security.CoverBytes(rcp.PublicClientPairs[i].Key)
		}
		for i := 0; i < len(rcp.PrivateRootCAPairs); i++ {
			security.CoverBytes(rcp.PrivateRootCAPairs[i].Key)
		}
		for i := 0; i < len(rcp.PrivateClientCAPairs); i++ {
			security.CoverBytes(rcp.PrivateClientCAPairs[i].Key)
		}
		for i := 0; i < len(rcp.PrivateClientPairs); i++ {
			security.CoverBytes(rcp.PrivateClientPairs[i].Key)
		}
	}()
	getCertsFromPool(pool, rcp)
	// marshal
	certsData, err := msgpack.Marshal(rcp)
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

func getCertsFromPool(pool *cert.Pool, rcp *ctrlCertPool) {
	pubRootCACerts := pool.GetPublicRootCACerts()
	for i := 0; i < len(pubRootCACerts); i++ {
		rcp.PublicRootCACerts = append(rcp.PublicRootCACerts, pubRootCACerts[i].Raw)
	}
	pubClientCACerts := pool.GetPublicClientCACerts()
	for i := 0; i < len(pubClientCACerts); i++ {
		rcp.PublicClientCACerts = append(rcp.PublicClientCACerts, pubClientCACerts[i].Raw)
	}
	pubClientPairs := pool.GetPublicClientPairs()
	for i := 0; i < len(pubClientPairs); i++ {
		c, k := pubClientPairs[i].Encode()
		rcp.PublicClientPairs = append(rcp.PublicClientPairs, struct {
			Cert []byte `msgpack:"a"`
			Key  []byte `msgpack:"b"`
		}{Cert: c, Key: k})
	}
	priRootCAPairs := pool.GetPrivateRootCAPairs()
	for i := 0; i < len(priRootCAPairs); i++ {
		c, k := priRootCAPairs[i].Encode()
		rcp.PrivateRootCAPairs = append(rcp.PrivateRootCAPairs, struct {
			Cert []byte `msgpack:"a"`
			Key  []byte `msgpack:"b"`
		}{Cert: c, Key: k})
	}
	priClientCAPairs := pool.GetPrivateClientCAPairs()
	for i := 0; i < len(priClientCAPairs); i++ {
		c, k := priClientCAPairs[i].Encode()
		rcp.PrivateClientCAPairs = append(rcp.PrivateClientCAPairs, struct {
			Cert []byte `msgpack:"a"`
			Key  []byte `msgpack:"b"`
		}{Cert: c, Key: k})
	}
	priClientPairs := pool.GetPrivateClientPairs()
	for i := 0; i < len(priClientPairs); i++ {
		c, k := priClientPairs[i].Encode()
		rcp.PrivateClientPairs = append(rcp.PrivateClientPairs, struct {
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
	rcp := ctrlCertPool{}
	err = msgpack.Unmarshal(certsData, &rcp)
	if err != nil {
		return err
	}
	return addCertsToPool(pool, &rcp)
}

func addCertsToPool(pool *cert.Pool, rcp *ctrlCertPool) error {
	memory := security.NewMemory()
	defer memory.Flush()
	var err error
	for i := 0; i < len(rcp.PublicRootCACerts); i++ {
		err = pool.AddPublicRootCACert(rcp.PublicRootCACerts[i])
		if err != nil {
			return err
		}
	}
	for i := 0; i < len(rcp.PublicClientCACerts); i++ {
		err = pool.AddPublicClientCACert(rcp.PublicClientCACerts[i])
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
