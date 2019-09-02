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
	generator = New(0)
}

type Generator struct {
	rand *rand.Rand
	m    sync.Mutex
}

func New(seed int64) *Generator {
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	return &Generator{
		rand: rand.New(rand.NewSource(seed)),
	}
}

// no "|"
func (g *Generator) String(n int) string {
	if n < 1 {
		return ""
	}
	result := make([]rune, n)
	for i := 0; i < n; i++ {
		g.m.Lock()
		r := g.rand.Intn(90)
		g.m.Unlock()
		result[i] = rune(33 + r)
	}
	return string(result)
}

func (g *Generator) Bytes(n int) []byte {
	if n < 1 {
		return nil
	}
	result := make([]byte, n)
	for i := 0; i < n; i++ {
		g.m.Lock()
		r := g.rand.Intn(256)
		g.m.Unlock()
		result[i] = byte(r)
	}
	return result
}

// only number and A-Z a-z
func (g *Generator) Cookie(n int) string {
	if n < 1 {
		return ""
	}
	result := make([]rune, n)
	for i := 0; i < n; i++ {
		g.m.Lock()
		// after space
		r := 33 + g.rand.Intn(90)
		g.m.Unlock()
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

func (g *Generator) Int(n int) int {
	if n < 1 {
		return 0
	}
	g.m.Lock()
	r := g.rand.Intn(n)
	g.m.Unlock()
	return r
}

func (g *Generator) Int64() int64 {
	g.m.Lock()
	r := g.rand.Int63()
	g.m.Unlock()
	return r
}

func (g *Generator) Uint64() uint64 {
	g.m.Lock()
	r := g.rand.Uint64()
	g.m.Unlock()
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

// sleep  fixed <= time < fixed + random
func Sleep(fixed, random int) {
	time.Sleep(time.Duration(fixed+Int(random)) * time.Second)
}
