package xlight

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_cryptor(t *testing.T) {
	testdata := make([]byte, 256)
	for i := 0; i < 256; i++ {
		testdata[i] = byte(i)
	}
	c := generate_cryptor(nil)
	cipherdata := c.encrypt(testdata)
	c.decrypt(cipherdata)
	require.Equal(t, testdata, cipherdata)
	// has encrypt
	c = generate_cryptor(nil)

	key := make([]byte, 256)
	for i := 0; i < 256; i++ {
		key[i] = c[0][i]
	}
	c = generate_cryptor(key)
	cipherdata = c.encrypt(testdata)
	c.decrypt(cipherdata)
	require.Equal(t, testdata, cipherdata)
}
