package lsb

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	"image/png"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/aes"
	"project/internal/patch/monkey"
	"project/internal/random"
)

func TestCalculateStorageSize(t *testing.T) {
	for _, testdata := range [...]*struct {
		width  int
		height int
		output int
	}{
		{width: 100, height: 200, output: 19951},
		{width: 20, height: 3, output: 15},
		{width: 20, height: 2, output: 0},
		{width: 8, height: 4, output: 0},
		{width: 0, height: 0, output: 0},
		{width: 1, height: 1, output: 0},
	} {
		rect := image.Rect(0, 0, testdata.width, testdata.height)
		size := CalculateStorageSize(rect)
		require.Equal(t, testdata.output, size)
	}
}

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

func testGeneratePNG() *image.NRGBA64 {
	rect := image.Rect(0, 0, 160, 90)
	return image.NewNRGBA64(rect)
}

func testGeneratePNGBytes(t *testing.T) []byte {
	img := testGeneratePNG()
	buf := bytes.NewBuffer(make([]byte, 0, 128))
	err := png.Encode(buf, img)
	require.NoError(t, err)
	return buf.Bytes()
}

func TestEncryptToPNG(t *testing.T) {
	t.Run("invalid png file", func(t *testing.T) {
		img, err := EncryptToPNG(nil, nil, nil, nil)
		require.Error(t, err)
		require.Nil(t, img)
	})

	t.Run("failed to encrypt", func(t *testing.T) {
		pic := testGeneratePNGBytes(t)
		img, err := EncryptToPNG(pic, nil, nil, nil)
		require.Error(t, err)
		require.Nil(t, img)
	})

	t.Run("failed to encode", func(t *testing.T) {
		// must before patch, because testGeneratePNGBytes call png.Encode
		pic := testGeneratePNGBytes(t)

		encoder := new(png.Encoder)
		patch := func(interface{}, io.Writer, image.Image) error {
			return monkey.Error
		}
		pg := monkey.PatchInstanceMethod(encoder, "Encode", patch)
		defer pg.Unpatch()

		plainData := []byte{1, 2, 3, 4}
		key := random.Bytes(aes.Key256Bit)
		iv := random.Bytes(aes.IVSize)

		img, err := EncryptToPNG(pic, plainData, key, iv)
		monkey.IsMonkeyError(t, err)
		require.Nil(t, img)
	})
}

func TestDecryptFromPNG(t *testing.T) {

}

func TestEncrypt(t *testing.T) {
	t.Skip()

	file, err := os.Open("testdata/desktop.png")
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

	file2, err := os.Create("testdata/desktop2.png")
	require.NoError(t, err)
	err = png.Encode(file2, img)
	require.NoError(t, err)

	err = file2.Close()
	require.NoError(t, err)

	fmt.Println("---------------------")

	file2, err = os.Open("testdata/desktop2.png")
	require.NoError(t, err)

	img2, err := png.Decode(file2)
	require.NoError(t, err)

	data, err := Decrypt(img2.(*image.NRGBA64), key, iv)
	require.NoError(t, err)

	require.Equal(t, rawData, data)

	nowHash := sha256.Sum256(data)

	fmt.Println("decrypted data hash", nowHash[:])
}
