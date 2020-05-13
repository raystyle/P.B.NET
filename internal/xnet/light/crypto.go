package light

import (
	"project/internal/random"
)

// 0 for encrypt
// 1 for decrypt
type crypto [2][256]byte

func (c *crypto) Encrypt(plainData []byte) []byte {
	l := len(plainData)
	cipherData := make([]byte, l)
	for i := 0; i < len(plainData); i++ {
		cipherData[i] = c[0][plainData[i]]
	}
	return cipherData
}

func (c *crypto) Decrypt(cipherData []byte) {
	for i := 0; i < len(cipherData); i++ {
		cipherData[i] = c[1][cipherData[i]]
	}
}

func newCrypto(password []byte) *crypto {
	var crypto crypto
	if len(password) == 256 {
		// copy encrypt password
		for i := 0; i < 256; i++ {
			crypto[0][i] = password[i]
		}
	} else {
		// generate new encrypt password
		rand := random.NewRand()
		pool := make(map[byte]bool)
		// first select
		crypto[0][0] = byte(rand.Int(256))
		pool[crypto[0][0]] = true
		add := 1
		for {
			// get options
			l := 256 - len(pool)
			options := make([]byte, 0, l)
			for i := 0; i < 256; i++ {
				_, used := pool[byte(i)]
				if !used {
					options = append(options, byte(i))
				}
			}
			// select option
			key := options[rand.Int(l)]
			crypto[0][add] = key
			pool[key] = true
			add++
			if add == 256 {
				break
			}
		}
	}
	// generate decrypt password
	for i := 0; i < 256; i++ {
		crypto[1][crypto[0][i]] = byte(i)
	}
	return &crypto
}
