package aes

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func generateBytes() []byte {
	testdata := make([]byte, 256)
	for i := 0; i < 256; i++ {
		testdata[i] = byte(i)
	}
	return testdata
}

func TestAES(t *testing.T) {
	key128 := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 11, 12, 13, 14, 15, 16}
	key256 := bytes.Repeat(key128, 2)
	iv := []byte{11, 12, 13, 14, 15, 16, 17, 18, 19, 10, 111, 112, 113, 114, 115, 116}

	// encrypt & decrypt
	f := func(key []byte) {
		testdata := generateBytes()

		cipherData, err := CBCEncrypt(testdata, key, iv)
		require.NoError(t, err)
		require.Equal(t, generateBytes(), testdata)
		require.NotEqual(t, testdata, cipherData)

		plainData, err := CBCDecrypt(cipherData, key, iv)
		require.NoError(t, err)
		require.Equal(t, testdata, plainData)
	}

	t.Run("key 128bit", func(t *testing.T) {
		f(key128)
	})

	t.Run("key 256bit", func(t *testing.T) {
		f(key256)
	})

	t.Run("no data", func(t *testing.T) {
		_, err := CBCEncrypt(nil, key128, iv)
		require.Equal(t, ErrEmptyData, err)

		_, err = CBCDecrypt(nil, key128, iv)
		require.Equal(t, ErrEmptyData, err)
	})

	data := bytes.Repeat([]byte{255}, 32)

	t.Run("invalid key", func(t *testing.T) {
		_, err := CBCEncrypt(data, nil, iv)
		require.Error(t, err)

		_, err = CBCDecrypt(data, nil, iv)
		require.Error(t, err)
	})

	t.Run("invalid iv", func(t *testing.T) {
		_, err := CBCEncrypt(data, key128, nil)
		require.Equal(t, ErrInvalidIVSize, err)

		_, err = CBCDecrypt(data, key128, nil)
		require.Equal(t, ErrInvalidIVSize, err)
	})

	t.Run("ErrInvalidCipherData", func(t *testing.T) {
		_, err := CBCDecrypt(bytes.Repeat([]byte{0}, 13), key128, iv)
		require.Equal(t, ErrInvalidCipherData, err)

		_, err = CBCDecrypt(bytes.Repeat([]byte{0}, 63), key128, iv)
		require.Equal(t, ErrInvalidCipherData, err)
	})

	t.Run("ErrInvalidPaddingSize", func(t *testing.T) {
		_, err := CBCDecrypt(bytes.Repeat([]byte{0}, 64), key128, iv)
		require.Equal(t, ErrInvalidPaddingSize, err)
	})
}

func TestCBC(t *testing.T) {
	key128 := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 11, 12, 13, 14, 15, 16}
	key256 := bytes.Repeat(key128, 2)
	iv := []byte{11, 12, 13, 14, 15, 16, 17, 18, 19, 10, 111, 112, 113, 114, 115, 116}

	t.Run("test key", func(t *testing.T) {
		_, err := NewCBC(bytes.Repeat([]byte{0}, Key128Bit), iv)
		require.NoError(t, err)

		_, err = NewCBC(bytes.Repeat([]byte{0}, Key192Bit), iv)
		require.NoError(t, err)

		_, err = NewCBC(bytes.Repeat([]byte{0}, Key256Bit), iv)
		require.NoError(t, err)
	})

	// encrypt & decrypt
	f := func(key []byte) {
		cbc, err := NewCBC(key, iv)
		require.NoError(t, err)
		testdata := generateBytes()

		for i := 0; i < 10; i++ {
			cipherData, err := cbc.Encrypt(testdata)
			require.NoError(t, err)
			require.Equal(t, generateBytes(), testdata)
			require.NotEqual(t, testdata, cipherData)
		}

		cipherData, err := cbc.Encrypt(testdata)
		require.NoError(t, err)
		for i := 0; i < 20; i++ {
			plainData, err := cbc.Decrypt(cipherData)
			require.NoError(t, err)
			require.Equal(t, testdata, plainData)
		}
	}

	t.Run("key 128bit", func(t *testing.T) {
		f(key128)
	})

	t.Run("key 256bit", func(t *testing.T) {
		f(key256)
	})

	t.Run("invalid key", func(t *testing.T) {
		_, err := NewCBC(nil, iv)
		require.Error(t, err)
	})

	t.Run("invalid iv", func(t *testing.T) {
		_, err := NewCBC(key128, nil)
		require.Error(t, err)
	})

	t.Run("no data", func(t *testing.T) {
		cbc, err := NewCBC(key128, iv)
		require.NoError(t, err)

		_, err = cbc.Encrypt(nil)
		require.Equal(t, ErrEmptyData, err)

		_, err = cbc.Decrypt(nil)
		require.Equal(t, ErrEmptyData, err)
	})

	t.Run("invalid data", func(t *testing.T) {
		cbc, err := NewCBC(key128, iv)
		require.NoError(t, err)

		_, err = cbc.Decrypt(bytes.Repeat([]byte{0}, 13))
		require.Equal(t, ErrInvalidCipherData, err)

		_, err = cbc.Decrypt(bytes.Repeat([]byte{0}, 63))
		require.Equal(t, ErrInvalidCipherData, err)

		_, err = cbc.Decrypt(bytes.Repeat([]byte{0}, 64))
		require.Equal(t, ErrInvalidPaddingSize, err)
	})

	t.Run("key iv", func(t *testing.T) {
		cbc, err := NewCBC(key128, iv)
		require.NoError(t, err)

		k, v := cbc.KeyIV()
		require.Equal(t, key128, k)
		require.Equal(t, iv, v)
	})
}

func BenchmarkCBC_Encrypt_128(b *testing.B) {
	key := bytes.Repeat([]byte{0}, 16)
	data := bytes.Repeat([]byte{0}, 64)

	benchmarkCBCEncrypt(b, data, key)
}

func BenchmarkCBC_Encrypt_256(b *testing.B) {
	key := bytes.Repeat([]byte{0}, 32)
	data := bytes.Repeat([]byte{0}, 64)

	benchmarkCBCEncrypt(b, data, key)
}

func benchmarkCBCEncrypt(b *testing.B, data, key []byte) {
	iv := bytes.Repeat([]byte{0}, IVSize)
	cbc, err := NewCBC(key, iv)
	require.NoError(b, err)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = cbc.Encrypt(data)
	}

	b.StopTimer()
}

func BenchmarkCBC_Decrypt_128(b *testing.B) {
	key := bytes.Repeat([]byte{0}, 16)
	iv := bytes.Repeat([]byte{0}, IVSize)
	cipherData, err := CBCEncrypt(bytes.Repeat([]byte{0}, 64), key, iv)
	require.NoError(b, err)

	benchmarkCBCDecrypt(b, cipherData, key, iv)
}

func BenchmarkCBC_Decrypt_256(b *testing.B) {
	key := bytes.Repeat([]byte{0}, 32)
	iv := bytes.Repeat([]byte{0}, IVSize)
	cipherData, err := CBCEncrypt(bytes.Repeat([]byte{0}, 64), key, iv)
	require.NoError(b, err)

	benchmarkCBCDecrypt(b, cipherData, key, iv)
}

func benchmarkCBCDecrypt(b *testing.B, data, key, iv []byte) {
	cbc, err := NewCBC(key, iv)
	require.NoError(b, err)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = cbc.Decrypt(data)
	}

	b.StopTimer()
}
