package donut

import (
	"bufio"
	"bytes"
	"encoding/binary"
)

// ChasKey Implementation ported from donut

const (
	// cipherBlockLen - ChasKey Block Length
	cipherBlockLen = uint32(128 / 8)
	// cipherKeyLen - ChasKey Key Length
	cipherKeyLen = uint32(128 / 8)
)

// Rotate32 - rotates a byte right (same as (32 - n) left)
func Rotate32(v uint32, n uint32) uint32 {
	return (v >> n) | (v << (32 - n))
}

// ChasKey Encryption Function
func ChasKey(masterKey []byte, data []byte) []byte {
	// convert inputs to []uint32
	mk := BytesToUint32s(masterKey)
	p := BytesToUint32s(data)

	// add 128-bit master key
	for i := 0; i < 4; i++ {
		p[i] ^= mk[i]
	}
	// apply 16 rounds of permutation
	for i := 0; i < 16; i++ {
		p[0] += p[1]
		p[1] = Rotate32(p[1], 27) ^ p[0]
		p[2] += p[3]
		p[3] = Rotate32(p[3], 24) ^ p[2]
		p[2] += p[1]
		p[0] = Rotate32(p[0], 16) + p[3]
		p[3] = Rotate32(p[3], 19) ^ p[0]
		p[1] = Rotate32(p[1], 25) ^ p[2]
		p[2] = Rotate32(p[2], 16)
	}
	// add 128-bit master key
	for i := 0; i < 4; i++ {
		p[i] ^= mk[i]
	}
	// convert to []byte for XOR phase
	b := bytes.NewBuffer(make([]byte, 0, 4*len(p)))
	w := bufio.NewWriter(b)
	for _, v := range p {
		_ = binary.Write(w, binary.LittleEndian, v)
	}
	_ = w.Flush()
	return b.Bytes()
}

// BytesToUint32s - converts a Byte array to an array of uint32s
func BytesToUint32s(b []byte) []uint32 {
	mb := bytes.NewBuffer(b)
	r := bufio.NewReader(mb)
	var outInt []uint32
	for i := 0; i < len(b); i = i + 4 {
		var tb uint32
		_ = binary.Read(r, binary.LittleEndian, &tb)
		outInt = append(outInt, tb)
	}
	return outInt
}

// Encrypt - encrypt/decrypt data in counter mode
func Encrypt(mk []byte, ctr []byte, data []byte) []byte {
	length := uint32(len(data))
	x := make([]byte, cipherBlockLen)
	p := uint32(0) // data blocks counter
	returnVal := make([]byte, length)
	for length > 0 {
		// copy counter+nonce to local buffer
		copy(x[:cipherBlockLen], ctr[:cipherBlockLen])

		// donut_encrypt x
		x = ChasKey(mk, x)

		// XOR plaintext with ciphertext
		r := uint32(0)
		if length > cipherBlockLen {
			r = cipherBlockLen
		} else {
			r = length
		}
		for i := uint32(0); i < r; i++ {
			returnVal[i+p] = data[i+p] ^ x[i]
		}
		// update length + position
		length -= r
		p += r

		// update counter
		for i := cipherBlockLen - 1; ; i-- {
			ctr[i]++
			if ctr[i] != 0 {
				break
			}
		}
	}
	return returnVal
}

// Speck 64/128
func Speck(mk []byte, p uint64) uint64 {
	w := make([]uint32, 2)
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, p)
	_ = binary.Read(buf, binary.LittleEndian, &w[0])
	_ = binary.Read(buf, binary.LittleEndian, &w[1])
	k := make([]uint32, 4)

	r := bytes.NewBuffer(mk)
	for c := 0; c < 4; c++ {
		_ = binary.Read(r, binary.LittleEndian, &k[c])
	}

	for i := uint32(0); i < 27; i++ {
		// encrypt 64-bit plaintext
		w[0] = (Rotate32(w[0], 8) + w[1]) ^ k[0]
		w[1] = Rotate32(w[1], 29) ^ w[0]

		// create next 32-bit sub key
		t := k[3]
		k[3] = (Rotate32(k[1], 8) + k[0]) ^ i
		k[0] = Rotate32(k[0], 29) ^ k[3]
		k[1] = k[2]
		k[2] = t
	}

	// return 64-bit ciphertext
	b := bytes.NewBuffer([]byte{})
	_ = binary.Write(b, binary.LittleEndian, w[0])
	_ = binary.Write(b, binary.LittleEndian, w[1])
	num := binary.LittleEndian.Uint64(b.Bytes())
	return num
}

// Maru hash
func Maru(input []byte, iv uint64) uint64 {

	// set H to initial value
	// h := binary.LittleEndian.Uint64(iv)
	h := iv
	b := make([]byte, maruBlkLen)

	idx, length, end := 0, 0, 0
	for {
		if end > 0 {
			break
		}
		// end of string or max len?
		if length == len(input) || input[length] == 0 || length == maruMaxStr {
			// zero remainder of M
			for j := idx; j < maruBlkLen; /*-idx*/ j++ {
				b[j] = 0
			}
			// store the end bit
			b[idx] = 0x80
			// have we space in M for api length?
			if idx >= maruBlkLen-4 {
				// no, update H with E
				h ^= Speck(b, h)
				// zero M
				b = make([]byte, maruBlkLen)
			}
			// store total length in bits
			binary.LittleEndian.PutUint32(b[maruBlkLen-4:], uint32(length)*8)
			idx = maruBlkLen
			end++
		} else {
			// store character from api string
			b[idx] = input[length]
			idx++
			length++
		}
		if idx == maruBlkLen {
			// update H with E
			h ^= Speck(b, h)
			// reset idx
			idx = 0
		}
	}
	return h
}
