package aes

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"errors"
)

const (
	Bit128    = 16
	Bit192    = 24
	Bit256    = 32
	IVSize    = 16
	BlockSize = 16
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

func (c *CBC) Encrypt(plainData []byte) ([]byte, error) {
	l := len(plainData)
	if l == 0 {
		return nil, ErrNoData
	}
	padding := IVSize - l%IVSize
	plainData = append(plainData, bytes.Repeat([]byte{byte(padding)}, padding)...)
	encrypter := cipher.NewCBCEncrypter(c.block, c.iv)
	cipherData := make([]byte, len(plainData))
	encrypter.CryptBlocks(cipherData, plainData)
	return cipherData, nil
}

func (c *CBC) Decrypt(cipherData []byte) ([]byte, error) {
	l := len(cipherData)
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
	plainData := make([]byte, l)
	decrypter.CryptBlocks(plainData, cipherData)
	length := len(plainData)
	unpadding := int(plainData[length-1])
	if length-unpadding < 0 {
		return nil, ErrUnpadding
	}
	return plainData[:length-unpadding], nil
}

func (c *CBC) KeyIV() ([]byte, []byte) {
	key := make([]byte, len(c.key))
	iv := make([]byte, IVSize)
	copy(key, c.key)
	copy(iv, c.iv)
	return key, iv
}

func CBCEncrypt(plainData, key, iv []byte) ([]byte, error) {
	l := len(plainData)
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
	plainData = append(plainData, bytes.Repeat([]byte{byte(padding)}, padding)...)
	cipherData := make([]byte, len(plainData))
	encrypter := cipher.NewCBCEncrypter(block, iv)
	encrypter.CryptBlocks(cipherData, plainData)
	return cipherData, nil
}

func CBCDecrypt(cipherData, key, iv []byte) ([]byte, error) {
	l := len(cipherData)
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
	plainData := make([]byte, l)
	decrypter := cipher.NewCBCDecrypter(block, iv)
	decrypter.CryptBlocks(plainData, cipherData)
	// PKCS5
	length := len(plainData)
	unpadding := int(plainData[length-1])
	if length-unpadding < 0 {
		return nil, ErrUnpadding
	}
	return plainData[:length-unpadding], nil
}
