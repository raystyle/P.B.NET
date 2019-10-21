package aes

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
)

const (
	Key128Bit = 16
	Key192Bit = 24
	Key256Bit = 32
	IVSize    = 16
	BlockSize = 16
)

var (
	ErrInvalidIVSize     = errors.New("invalid iv size")
	ErrInvalidCipherData = errors.New("invalid cipher data")
	ErrEmptyData         = errors.New("empty data")
	ErrUnPadding         = errors.New("un padding error")
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
	cbc := &CBC{
		key:   make([]byte, len(key)),
		iv:    make([]byte, IVSize),
		block: block,
	}
	copy(cbc.key, key)
	copy(cbc.iv, iv)
	return cbc, nil
}

func (c *CBC) Encrypt(plainData []byte) ([]byte, error) {
	plainDataSize := len(plainData)
	if plainDataSize == 0 {
		return nil, ErrEmptyData
	}
	paddingSize := BlockSize - plainDataSize%BlockSize
	totalSize := plainDataSize + paddingSize
	plain := make([]byte, totalSize)
	copy(plain, plainData)
	padding := byte(paddingSize)
	for i := 0; i < paddingSize; i++ {
		plain[plainDataSize+i] = padding
	}
	encrypter := cipher.NewCBCEncrypter(c.block, c.iv)
	cipherData := make([]byte, totalSize)
	encrypter.CryptBlocks(cipherData, plain)
	return cipherData, nil
}

func (c *CBC) Decrypt(cipherData []byte) ([]byte, error) {
	cipherDataSize := len(cipherData)
	if cipherDataSize == 0 {
		return nil, ErrEmptyData
	}
	if cipherDataSize < BlockSize {
		return nil, ErrInvalidCipherData
	}
	if cipherDataSize%BlockSize != 0 {
		return nil, ErrInvalidCipherData
	}
	decrypter := cipher.NewCBCDecrypter(c.block, c.iv)
	plainData := make([]byte, cipherDataSize)
	decrypter.CryptBlocks(plainData, cipherData)
	plainDataSize := len(plainData)
	paddingSize := int(plainData[plainDataSize-1])
	if plainDataSize-paddingSize < 0 {
		return nil, ErrUnPadding
	}
	return plainData[:plainDataSize-paddingSize], nil
}

func (c *CBC) KeyIV() ([]byte, []byte) {
	key := make([]byte, len(c.key))
	iv := make([]byte, IVSize)
	copy(key, c.key)
	copy(iv, c.iv)
	return key, iv
}

func CBCEncrypt(plainData, key, iv []byte) ([]byte, error) {
	plainDataSize := len(plainData)
	if plainDataSize == 0 {
		return nil, ErrEmptyData
	}
	if len(iv) != IVSize {
		return nil, ErrInvalidIVSize
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	paddingSize := BlockSize - plainDataSize%BlockSize
	totalSize := plainDataSize + paddingSize
	plain := make([]byte, totalSize)
	copy(plain, plainData)
	padding := byte(paddingSize)
	for i := 0; i < paddingSize; i++ {
		plain[plainDataSize+i] = padding
	}
	encrypter := cipher.NewCBCEncrypter(block, iv)
	cipherData := make([]byte, totalSize)
	encrypter.CryptBlocks(cipherData, plain)
	return cipherData, nil
}

func CBCDecrypt(cipherData, key, iv []byte) ([]byte, error) {
	cipherDataSize := len(cipherData)
	if cipherDataSize == 0 {
		return nil, ErrEmptyData
	}
	if len(iv) != IVSize {
		return nil, ErrInvalidIVSize
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if cipherDataSize < BlockSize {
		return nil, ErrInvalidCipherData
	}
	if cipherDataSize%BlockSize != 0 {
		return nil, ErrInvalidCipherData
	}
	decrypter := cipher.NewCBCDecrypter(block, iv)
	plainData := make([]byte, cipherDataSize)
	decrypter.CryptBlocks(plainData, cipherData)
	plainDataSize := len(plainData)
	paddingSize := int(plainData[plainDataSize-1])
	if plainDataSize-paddingSize < 0 {
		return nil, ErrUnPadding
	}
	return plainData[:plainDataSize-paddingSize], nil
}
