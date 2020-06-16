package lsb

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	"image/png"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/aes"
	"project/internal/random"
)

func TestLSB_White(t *testing.T) {
	testLSB(t, "white")
}

func TestLSB_Black(t *testing.T) {
	testLSB(t, "black")
}

func testLSB(t *testing.T, name string) {
	pic, err := ioutil.ReadFile(fmt.Sprintf("testdata/%s.png", name))
	require.NoError(t, err)

	key := bytes.Repeat([]byte{1}, aes.Key256Bit)
	iv := bytes.Repeat([]byte{2}, aes.IVSize)

	plainData := bytes.Repeat(pic[:128], 5)

	picEnc, err := EncryptToPNG(pic, plainData, key, iv)
	require.NoError(t, err)

	dec, err := DecryptFromPNG(picEnc, key, iv)
	require.NoError(t, err)

	require.Equal(t, plainData, dec)

	fileName := fmt.Sprintf("testdata/%s_enc.png", name)
	err = ioutil.WriteFile(fileName, picEnc, 0600)
	require.NoError(t, err)
}

func TestEncrypt(t *testing.T) {
	file, err := os.Open("desktop.png")
	require.NoError(t, err)
	defer func() { _ = file.Close() }()
	rawImage, err := png.Decode(file)
	require.NoError(t, err)

	key := random.Bytes(aes.Key256Bit)
	iv := random.Bytes(aes.IVSize)

	rawData := random.Bytes(1 << 20)
	rawHash := sha256.Sum256(rawData)

	fmt.Println("plain data hash", rawHash[:])

	img, err := Encrypt(rawImage, rawData, key, iv)
	require.NoError(t, err)

	file2, err := os.Create("desktop2.png")
	require.NoError(t, err)
	err = png.Encode(file2, img)
	require.NoError(t, err)

	err = file2.Close()
	require.NoError(t, err)

	fmt.Println("---------------------")

	file2, err = os.Open("desktop2.png")
	require.NoError(t, err)

	img2, err := png.Decode(file2)
	require.NoError(t, err)

	data, err := Decrypt(img2.(*image.NRGBA64), key, iv)
	require.NoError(t, err)

	require.Equal(t, rawData, data)

	nowHash := sha256.Sum256(data)

	fmt.Println("decrypted data hash", nowHash[:])
}
