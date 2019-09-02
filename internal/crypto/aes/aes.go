package aes

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"errors"
)

const (
	Bit128 = 16
	Bit192 = 24
	Bit256 = 32
	IVSize = 16
)

var (
	ErrInvalidIVSize     = errors.New("invalid iv size")
	ErrNoData            = errors.New("no data")
	ErrInvalidCipherData = errors.New("invalid cipher data")
	ErrUnpadding         = errors.New("unpadding error")
)

type CBC struct {
	key   []byte
	iv    []byte
	block cipher.Block
}

func NewCBC(key, iv []byte) (*CBC, error) {
	if len(iv) != IVSize {
		return nil, ErrInvalidIVSize
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	c := &CBC{
		key:   make([]byte, len(key)),
		iv:    make([]byte, IVSize),
		block: block,
	}
	copy(c.key, key)
	copy(c.iv, iv)
	return c, nil
}

func (c *CBC) Encrypt(plaindata []byte) ([]byte, error) {
	l := len(plaindata)
	if l == 0 {
		return nil, ErrNoData
	}
	padding := IVSize - l%IVSize
	plaindata = append(plaindata, bytes.Repeat([]byte{byte(padding)}, padding)...)
	encrypter := cipher.NewCBCEncrypter(c.block, c.iv)
	cipherdata := make([]byte, len(plaindata))
	encrypter.CryptBlocks(cipherdata, plaindata)
	return cipherdata, nil
}

func (c *CBC) Decrypt(cipherdata []byte) ([]byte, error) {
	l := len(cipherdata)
	if l == 0 {
		return nil, ErrNoData
	}
	if l < IVSize {
		return nil, ErrInvalidCipherData
	}
	if l%IVSize != 0 {
		return nil, ErrInvalidCipherData
	}
	decrypter := cipher.NewCBCDecrypter(c.block, c.iv)
	plaindata := make([]byte, l)
	decrypter.CryptBlocks(plaindata, cipherdata)
	length := len(plaindata)
	unpadding := int(plaindata[length-1])
	if length-unpadding < 0 {
		return nil, ErrUnpadding
	}
	return plaindata[:length-unpadding], nil
}

func (c *CBC) KeyIV() ([]byte, []byte) {
	key := make([]byte, len(c.key))
	iv := make([]byte, IVSize)
	copy(key, c.key)
	copy(iv, c.iv)
	return key, iv
}

func CBCEncrypt(plaindata, key, iv []byte) ([]byte, error) {
	l := len(plaindata)
	if l == 0 {
		return nil, ErrNoData
	}
	if len(iv) != IVSize {
		return nil, ErrInvalidIVSize
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	// PKCS5
	padding := IVSize - l%IVSize
	plaindata = append(plaindata, bytes.Repeat([]byte{byte(padding)}, padding)...)
	cipherdata := make([]byte, len(plaindata))
	encrypter := cipher.NewCBCEncrypter(block, iv)
	encrypter.CryptBlocks(cipherdata, plaindata)
	return cipherdata, nil
}

func CBCDecrypt(cipherdata, key, iv []byte) ([]byte, error) {
	l := len(cipherdata)
	if l == 0 {
		return nil, ErrNoData
	}
	if len(iv) != IVSize {
		return nil, ErrInvalidIVSize
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if l < IVSize {
		return nil, ErrInvalidCipherData
	}
	if l%IVSize != 0 {
		return nil, ErrInvalidCipherData
	}
	plaindata := make([]byte, l)
	decrypter := cipher.NewCBCDecrypter(block, iv)
	decrypter.CryptBlocks(plaindata, cipherdata)
	// PKCS5
	length := len(plaindata)
	unpadding := int(plaindata[length-1])
	if length-unpadding < 0 {
		return nil, ErrUnpadding
	}
	return plaindata[:length-unpadding], nil
}
