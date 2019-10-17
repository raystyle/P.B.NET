package random

import (
	cr "crypto/rand"
	"io"
	"math/rand"
	"sync"
	"time"

	"project/internal/convert"
)

var (
	gRand *Rand
)

func init() {
	gRand = New(0)
}

type Rand struct {
	rand *rand.Rand
	m    sync.Mutex
}

func New(seed int64) *Rand {
	if seed == 0 {
		// try crypto/rand.Reader
		b := make([]byte, 8)
		_, err := io.ReadFull(cr.Reader, b)
		if err == nil {
			seed = convert.BytesToInt64(b)
		} else {
			seed = time.Now().UnixNano()
		}
	}
	return &Rand{
		rand: rand.New(rand.NewSource(seed)),
	}
}

// no "|"
func (r *Rand) String(n int) string {
	if n < 1 {
		return ""
	}
	result := make([]rune, n)
	for i := 0; i < n; i++ {
		r.m.Lock()
		ri := r.rand.Intn(90)
		r.m.Unlock()
		result[i] = rune(33 + ri)
	}
	return string(result)
}

func (r *Rand) Bytes(n int) []byte {
	if n < 1 {
		return nil
	}
	result := make([]byte, n)
	for i := 0; i < n; i++ {
		r.m.Lock()
		ri := r.rand.Intn(256)
		r.m.Unlock()
		result[i] = byte(ri)
	}
	return result
}

// only number and A-Z a-z
func (r *Rand) Cookie(n int) string {
	if n < 1 {
		return ""
	}
	result := make([]rune, n)
	for i := 0; i < n; i++ {
		r.m.Lock()
		// after space
		ri := 33 + r.rand.Intn(90)
		r.m.Unlock()
		switch {
		case ri > 47 && ri < 58: //  48-57 number
		case ri > 64 && ri < 91: //  65-90 A-Z
		case ri > 96 && ri < 123: // 97-122 a-z
		default:
			i--
			continue
		}
		result[i] = rune(ri)
	}
	return string(result)
}

func (r *Rand) Int(n int) int {
	if n < 1 {
		return 0
	}
	r.m.Lock()
	i := r.rand.Intn(n)
	r.m.Unlock()
	return i
}

func (r *Rand) Int64() int64 {
	r.m.Lock()
	i := r.rand.Int63()
	r.m.Unlock()
	return i
}

func (r *Rand) Uint64() uint64 {
	r.m.Lock()
	ui := r.rand.Uint64()
	r.m.Unlock()
	return ui
}

func String(n int) string {
	return gRand.String(n)
}

func Bytes(n int) []byte {
	return gRand.Bytes(n)
}

func Cookie(n int) string {
	return gRand.Cookie(n)
}

func Int(n int) int {
	return gRand.Int(n)
}

func Int64() int64 {
	return gRand.Int64()
}

func Uint64() uint64 {
	return gRand.Uint64()
}

// sleep  fixed <= time < fixed + random
func Sleep(fixed, random int) {
	time.Sleep(time.Duration(fixed+Int(random)) * time.Second)
}
