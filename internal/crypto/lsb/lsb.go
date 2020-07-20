package lsb

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"

	"project/internal/convert"
	"project/internal/crypto/aes"
	"project/internal/security"
)

// RGBA is uint16, split one byte to 8 bits, and save it to uint16.
//
// R: 1111 000[bit1] 1111 000[bit2]
// G: 1111 000[bit3] 1111 000[bit4]
// B: 1111 000[bit5] 1111 000[bit6]
// A: 1111 000[bit6] 1111 000[bit7]

// data structure stored in PNG
// +--------------+----------+-----------+
// | size(uint32) |  SHA256  | AES(data) |
// +--------------+----------+-----------+
// |   4 bytes    | 32 bytes |    var    |
// +--------------+----------+-----------+

// size is uint32
const headerSize = 4

// CalculateStorageSize is used to calculate the maximum data that can encrypted.
func CalculateStorageSize(rect image.Rectangle) int {
	width := rect.Dx()
	height := rect.Dy()
	size := width * height
	// sha256.Size-1,  "1" is reserved pixel, see writeDataToImage()
	block := (size-headerSize-sha256.Size-1)/aes.BlockSize - 1 // "1" is for aes padding
	// actual data that can store
	max := block*aes.BlockSize + (aes.BlockSize - 1)
	if max < 0 {
		max = 0
	}
	return max
}

// EncryptToPNG is used to load PNG image and encrypt data to it.
func EncryptToPNG(pic, plainData, key, iv []byte) ([]byte, error) {
	img, err := png.Decode(bytes.NewReader(pic))
	if err != nil {
		return nil, err
	}
	newImg, err := Encrypt(img, plainData, key, iv)
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(make([]byte, 0, len(pic)))
	encoder := png.Encoder{
		CompressionLevel: png.BestCompression,
	}
	err = encoder.Encode(buf, newImg)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Encrypt is used to encrypt data by aes + hash and save it to a PNG image.
func Encrypt(img image.Image, plainData, key, iv []byte) (*image.NRGBA64, error) {
	// basic information
	rect := img.Bounds()
	storageSize := CalculateStorageSize(rect)
	size := len(plainData)
	if size > storageSize {
		const format = "this image can only store %s data, plain data size is %d"
		str := convert.FormatByte(uint64(storageSize))
		return nil, fmt.Errorf(format, str, size)
	}
	if size > math.MaxInt32-1 { // because aes block size
		return nil, errors.New("plain data size is bigger than 4GB")
	}
	// encrypted data
	cipherData, err := aes.CBCEncrypt(plainData, key, iv)
	if err != nil {
		return nil, err
	}
	defer security.CoverBytes(cipherData)
	h := sha256.Sum256(plainData)
	hash := h[:]
	defer security.CoverBytes(hash)
	// set secret
	secret := make([]byte, 0, headerSize+sha256.Size+len(cipherData))
	secret = append(secret, convert.Uint32ToBytes(uint32(len(cipherData)))...)
	secret = append(secret, hash...)
	secret = append(secret, cipherData...)
	return writeDataToImage(img, rect, secret), nil
}

func writeDataToImage(img image.Image, rect image.Rectangle, data []byte) *image.NRGBA64 {
	newImg := image.NewNRGBA64(rect)
	min := rect.Min
	max := rect.Max
	width, height := rect.Dx(), rect.Dy()
	begin := 0
	stop := len(data)

	for x := min.X; x < width; x++ {
		for y := min.Y; y < height; y++ {
			r, g, b, a := img.At(x, y).RGBA()
			rgba := color.NRGBA64{
				R: uint16(r),
				G: uint16(g),
				B: uint16(b),
				A: uint16(a),
			}
			if begin < stop {
				b := data[begin]

				// write 8 bit to the last bit about 4(RGBA) * 2(front and end) byte
				var m [8]uint8
				m[0] = uint8(rgba.R >> 8) // red front 8 bit
				m[1] = uint8(rgba.R)      // red end 8 bit
				m[2] = uint8(rgba.G >> 8) // green front 8 bit
				m[3] = uint8(rgba.G)      // green end 8 bit
				m[4] = uint8(rgba.B >> 8) // blue front 8 bit
				m[5] = uint8(rgba.B)      // blue end 8 bit
				m[6] = uint8(rgba.A >> 8) // alpha front 8 bit
				m[7] = uint8(rgba.A)      // alpha end 8 bit

				for i := 0; i < 8; i++ {
					// get the bit about the byte
					bit := b << i >> 7 // b << (i + 1 - 1) >> 7
					switch {
					case bit == 0 && m[i]&1 == 1:
						m[i]--
					case bit == 1 && m[i]&1 == 0:
						m[i]++
					}
				}

				rgba.R = uint16(m[0])<<8 + uint16(m[1])
				rgba.G = uint16(m[2])<<8 + uint16(m[3])
				rgba.B = uint16(m[4])<<8 + uint16(m[5])
				rgba.A = uint16(m[6])<<8 + uint16(m[7])

				begin++
			} else {
				switch rgba.A {
				case math.MaxUint16:
				case 0:
				default:
					rgba.A++
				}
			}
			newImg.SetNRGBA64(x, y, rgba)

			// clean pixel
			rgba.R = 0
			rgba.G = 0
			rgba.B = 0
			rgba.A = 0
		}
	}

	// force set the last pixel to make sure image is 64 bit png.
	r, g, b, _ := img.At(max.X-1, max.Y-1).RGBA()
	rgba := color.NRGBA64{
		R: uint16(r),
		G: uint16(g),
		B: uint16(b),
		A: 65534,
	}
	newImg.SetNRGBA64(max.X-1, max.Y-1, rgba)
	return newImg
}

// DecryptFromPNG is used to load a PNG image and  decrypt data from it.
func DecryptFromPNG(pic, key, iv []byte) ([]byte, error) {
	p, err := png.Decode(bytes.NewReader(pic))
	if err != nil {
		return nil, err
	}
	img, ok := p.(*image.NRGBA64)
	if !ok {
		return nil, errors.New("png is not NRGBA64")
	}
	return Decrypt(img, key, iv)
}

// Decrypt is used to decrypt cipher data from a PNG image.
func Decrypt(img *image.NRGBA64, key, iv []byte) ([]byte, error) {
	// basic information
	rect := img.Bounds()
	width, height := rect.Dx(), rect.Dy()
	maxSize := width * height // one pixel one byte
	if maxSize < headerSize+sha256.Size+aes.BlockSize {
		return nil, errors.New("invalid image size")
	}
	min := rect.Min
	// store global position
	x := &min.X
	y := &min.Y
	// read header
	header := readDataFromImage(img, width, height, x, y, headerSize)
	cipherDataSize := int(convert.BytesToUint32(header))
	if headerSize+sha256.Size+cipherDataSize > maxSize {
		return nil, errors.New("invalid size in header")
	}
	// read hash
	rawHash := readDataFromImage(img, width, height, x, y, sha256.Size)
	// read cipher data
	cipherData := readDataFromImage(img, width, height, x, y, cipherDataSize)
	// decrypt
	plainData, err := aes.CBCDecrypt(cipherData, key, iv)
	if err != nil {
		return nil, err
	}
	// check hash
	hash := sha256.Sum256(plainData)
	if subtle.ConstantTimeCompare(hash[:], rawHash) != 1 {
		return nil, errors.New("invalid hash about the plain data")
	}
	return plainData, nil
}

func readDataFromImage(img *image.NRGBA64, width, height int, x, y *int, size int) []byte {
	data := make([]byte, size)
	for i := 0; i < size; i++ {
		rgba := img.NRGBA64At(*x, *y)

		// write 8 bit to the last bit about 4(RGBA) * 2(front and end) byte
		var m [8]uint8
		m[0] = uint8(rgba.R >> 8) // red front 8 bit
		m[1] = uint8(rgba.R)      // red end 8 bit
		m[2] = uint8(rgba.G >> 8) // green front 8 bit
		m[3] = uint8(rgba.G)      // green end 8 bit
		m[4] = uint8(rgba.B >> 8) // blue front 8 bit
		m[5] = uint8(rgba.B)      // blue end 8 bit
		m[6] = uint8(rgba.A >> 8) // alpha front 8 bit
		m[7] = uint8(rgba.A)      // alpha end 8 bit

		// set bits
		var b byte
		for i := 0; i < 8; i++ {
			b += m[i] & 1 << (7 - i)
		}
		data[i] = b

		// check if need go to the next pixel column.
		*y++
		if *y >= height {
			*y = 0
			*x++
		}
		if *x >= width {
			panic("lsb: internal error")
		}
	}
	return data
}
