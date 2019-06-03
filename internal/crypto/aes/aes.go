package aes

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"errors"
)

const IV_SIZE = 16

var (
	ERR_INVALID_IV_SIZE    = errors.New("invalid iv size")
	ERR_NO_DATA            = errors.New("no data")
	ERR_INVALID_CIPHERDATA = errors.New("invalid cipherdata")
	ERR_UNPADDING          = errors.New("unpadding error")
)

type CBC_Cryptor struct {
	key   []byte
	iv    []byte
	block cipher.Block
}

func New_CBC_Cryptor(key, iv []byte) (*CBC_Cryptor, error) {
	if len(iv) != IV_SIZE {
		return nil, ERR_INVALID_IV_SIZE
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	c := &CBC_Cryptor{
		key:   make([]byte, len(key)),
		iv:    make([]byte, IV_SIZE),
		block: block,
	}
	copy(c.key, key)
	copy(c.iv, iv)
	return c, nil
}

func (this *CBC_Cryptor) Encrypt(plaindata []byte) ([]byte, error) {
	l := len(plaindata)
	if l == 0 {
		return nil, ERR_NO_DATA
	}
	padding := IV_SIZE - l%IV_SIZE
	plaindata = append(plaindata, bytes.Repeat([]byte{byte(padding)}, padding)...)
	encrypter := cipher.NewCBCEncrypter(this.block, this.iv)
	cipherdata := make([]byte, len(plaindata))
	encrypter.CryptBlocks(cipherdata, plaindata)
	return cipherdata, nil
}

func (this *CBC_Cryptor) Decrypt(cipherdata []byte) ([]byte, error) {
	l := len(cipherdata)
	if l == 0 {
		return nil, ERR_NO_DATA
	}
	if l < IV_SIZE {
		return nil, ERR_INVALID_CIPHERDATA
	}
	if l%IV_SIZE != 0 {
		return nil, ERR_INVALID_CIPHERDATA
	}
	decrypter := cipher.NewCBCDecrypter(this.block, this.iv)
	plaindata := make([]byte, l)
	decrypter.CryptBlocks(plaindata, cipherdata)
	length := len(plaindata)
	unpadding := int(plaindata[length-1])
	if length-unpadding < 0 {
		return nil, ERR_UNPADDING
	}
	return plaindata[:length-unpadding], nil
}

func (this *CBC_Cryptor) Key_IV() ([]byte, []byte) {
	return this.key, this.iv
}

func CBC_Encrypt(plaindata, key, iv []byte) ([]byte, error) {
	l := len(plaindata)
	if l == 0 {
		return nil, ERR_NO_DATA
	}
	if len(iv) != IV_SIZE {
		return nil, ERR_INVALID_IV_SIZE
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	// PKCS5
	padding := IV_SIZE - l%IV_SIZE
	plaindata = append(plaindata, bytes.Repeat([]byte{byte(padding)}, padding)...)
	cipherdata := make([]byte, len(plaindata))
	encrypter := cipher.NewCBCEncrypter(block, iv)
	encrypter.CryptBlocks(cipherdata, plaindata)
	return cipherdata, nil
}

func CBC_Decrypt(cipherdata, key, iv []byte) ([]byte, error) {
	l := len(cipherdata)
	if l == 0 {
		return nil, ERR_NO_DATA
	}
	if len(iv) != IV_SIZE {
		return nil, ERR_INVALID_IV_SIZE
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if l < IV_SIZE {
		return nil, ERR_INVALID_CIPHERDATA
	}
	if l%IV_SIZE != 0 {
		return nil, ERR_INVALID_CIPHERDATA
	}
	plaindata := make([]byte, l)
	decrypter := cipher.NewCBCDecrypter(block, iv)
	decrypter.CryptBlocks(plaindata, cipherdata)
	// PKCS5
	length := len(plaindata)
	unpadding := int(plaindata[length-1])
	if length-unpadding < 0 {
		return nil, ERR_UNPADDING
	}
	return plaindata[:length-unpadding], nil
}
