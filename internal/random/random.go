package random

import (
	"math/rand"
	"sync"
	"time"
)

var (
	generator *Generator
)

func init() {
	generator = New()
}

type Generator struct {
	seed *rand.Rand
	m    sync.Mutex
}

func New() *Generator {
	return &Generator{
		seed: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// no "|"
func (this *Generator) String(n int) string {
	if n < 1 {
		return ""
	}
	result := make([]rune, n)
	for i := 0; i < n; i++ {
		this.m.Lock()
		r := this.seed.Intn(90)
		this.m.Unlock()
		result[i] = rune(33 + r)
	}
	return string(result)
}

func (this *Generator) Bytes(n int) []byte {
	if n < 1 {
		return nil
	}
	result := make([]byte, n)
	for i := 0; i < n; i++ {
		this.m.Lock()
		r := this.seed.Intn(256)
		this.m.Unlock()
		result[i] = byte(r)
	}
	return result
}

// only number and A-Z a-z
func (this *Generator) Cookie(n int) string {
	if n < 1 {
		return ""
	}
	result := make([]rune, n)
	for i := 0; i < n; i++ {
		this.m.Lock()
		// after space
		r := 33 + this.seed.Intn(90)
		this.m.Unlock()
		switch {
		case r > 47 && r < 58: //  48-57 number
		case r > 64 && r < 91: //  65-90 A-Z
		case r > 96 && r < 123: // 97-122 a-z
		default:
			i--
			continue
		}
		result[i] = rune(r)
	}
	return string(result)
}

func (this *Generator) Int(n int) int {
	if n < 1 {
		return 0
	}
	this.m.Lock()
	r := this.seed.Intn(n)
	this.m.Unlock()
	return r
}

func (this *Generator) Int64() int64 {
	this.m.Lock()
	r := this.seed.Int63()
	this.m.Unlock()
	return r
}

func (this *Generator) Uint64() uint64 {
	this.m.Lock()
	r := this.seed.Uint64()
	this.m.Unlock()
	return r
}

func String(n int) string {
	return generator.String(n)
}

func Bytes(n int) []byte {
	return generator.Bytes(n)
}

func Cookie(n int) string {
	return generator.Cookie(n)
}

func Int(n int) int {
	return generator.Int(n)
}

func Int64() int64 {
	return generator.Int64()
}

func Uint64() uint64 {
	return generator.Uint64()
}
