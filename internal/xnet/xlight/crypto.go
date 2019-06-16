package xlight

import (
	"project/internal/random"
)

// 0 for encrypt
// 1 for decrypt
type cryptor [2][256]byte

func (this *cryptor) encrypt(plaindata []byte) []byte {
	l := len(plaindata)
	cipherdata := make([]byte, l)
	for i := 0; i < len(plaindata); i++ {
		cipherdata[i] = this[0][plaindata[i]]
	}
	return cipherdata
}

func (this *cryptor) decrypt(cipherdata []byte) {
	for i := 0; i < len(cipherdata); i++ {
		cipherdata[i] = this[1][cipherdata[i]]
	}
}

func new_cryptor(encrypt []byte) *cryptor {
	var cryptor cryptor
	if len(encrypt) == 256 {
		// copy encrypt
		for i := 0; i < 256; i++ {
			cryptor[0][i] = encrypt[i]
		}
	} else {
		// generate new encrypt
		generator := random.New()
		pool := make(map[byte]bool)
		// first select
		cryptor[0][0] = byte(generator.Int(256))
		pool[cryptor[0][0]] = true
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
			key := options[generator.Int(l)]
			cryptor[0][add] = key
			pool[key] = true
			add += 1
			if add == 256 {
				break
			}
		}
	}
	// generate decrypt
	for i := 0; i < 256; i++ {
		cryptor[1][cryptor[0][i]] = byte(i)
	}
	return &cryptor
}
