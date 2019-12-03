// +build go1.13

package cert

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"fmt"

	"project/internal/crypto/rand"
)

func genKey(algorithm string) (interface{}, interface{}, error) {
	switch algorithm {
	case "", "rsa":
		privateKey, _ := rsa.GenerateKey(rand.Reader, 4096)
		return privateKey, &privateKey.PublicKey, nil
	case "ecdsa":
		privateKey, _ := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
		return privateKey, &privateKey.PublicKey, nil
	case "ed25519":
		publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)
		return privateKey, publicKey, nil
	default:
		return nil, nil, fmt.Errorf("unknown algorithm: %s", algorithm)
	}
}
