package ed25519

import (
	"bytes"
	"fmt"
	"testing"

	"golang.org/x/crypto/ed25519"
)

func Test_ed25519(t *testing.T) {
	pri := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0}, ed25519.SeedSize))
	fmt.Println(pri)
	pri2 := ed25519.NewKeyFromSeed(pri.Seed())
	fmt.Println(pri2)
}

func Benchmark_ed25519_sign(b *testing.B) {
	_, pri, _ := ed25519.GenerateKey(nil)
	msg := bytes.Repeat([]byte{0}, 256)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bb := ed25519.Sign(pri, msg)
		bb[0] = 0
	}
	b.StopTimer()
}

func Benchmark_ed25519_verify(b *testing.B) {
	pub, pri, _ := ed25519.GenerateKey(nil)
	msg := bytes.Repeat([]byte{0}, 256)
	signature := ed25519.Sign(pri, msg)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ed25519.Verify(pub, msg, signature)
	}
	b.StopTimer()
}
