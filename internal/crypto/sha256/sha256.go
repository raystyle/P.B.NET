package sha256

import (
	"crypto/sha256"
	"encoding/hex"
)

const (
	Size = sha256.Size
)

// 32 byte
func String(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func Bytes(data []byte) []byte {
	h := sha256.New()
	h.Write(data)
	return h.Sum(nil)
}
