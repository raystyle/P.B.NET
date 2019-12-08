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

// Rand is used to generate random data
type Rand struct {
	rand *rand.Rand
	m    sync.Mutex
}

// New is used to create a Rand from seed
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

// String return a string that not include "|"
func (r *Rand) String(n int) string {
	if n < 1 {
		return ""
	}
	result := make([]rune, n)
	r.m.Lock()
	defer r.m.Unlock()
	for i := 0; i < n; i++ {
		ri := r.rand.Intn(90)
		result[i] = rune(33 + ri)
	}
	return string(result)
}

// Bytes is used to generate random []byte that size = n
func (r *Rand) Bytes(n int) []byte {
	if n < 1 {
		return nil
	}
	r.m.Lock()
	defer r.m.Unlock()
	result := make([]byte, n)
	for i := 0; i < n; i++ {
		ri := r.rand.Intn(256)
		result[i] = byte(ri)
	}
	return result
}

// Cookie return a string that only include number and A-Z a-z
func (r *Rand) Cookie(n int) string {
	if n < 1 {
		return ""
	}
	result := make([]rune, n)
	r.m.Lock()
	defer r.m.Unlock()
	for i := 0; i < n; i++ {
		// after space
		ri := 33 + r.rand.Intn(90)
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

// Int returns, as an int, a non-negative pseudo-random number in [0,n).
// It panics if n <= 0.
func (r *Rand) Int(n int) int {
	if n < 1 {
		return 0
	}
	r.m.Lock()
	defer r.m.Unlock()
	return r.rand.Intn(n)
}

// Int64 returns a non-negative pseudo-random 63-bit integer as an int64.
func (r *Rand) Int64() int64 {
	r.m.Lock()
	defer r.m.Unlock()
	return r.rand.Int63()
}

// Uint64 returns a pseudo-random 64-bit value as a uint64.
func (r *Rand) Uint64() uint64 {
	r.m.Lock()
	defer r.m.Unlock()
	return r.rand.Uint64()
}

// String return a string that not include "|"
func String(n int) string {
	return gRand.String(n)
}

// Bytes is used to generate random []byte that size = n
func Bytes(n int) []byte {
	return gRand.Bytes(n)
}

// Cookie return a string that only include number and A-Z a-z
func Cookie(n int) string {
	return gRand.Cookie(n)
}

// Int returns, as an int, a non-negative pseudo-random number in [0,n).
// It panics if n <= 0.
func Int(n int) int {
	return gRand.Int(n)
}

// Int64 returns a non-negative pseudo-random 63-bit integer as an int64.
func Int64() int64 {
	return gRand.Int64()
}

// Uint64 returns a pseudo-random 64-bit value as a uint64.
func Uint64() uint64 {
	return gRand.Uint64()
}

// Sleep is used to sleep random time
// fixed <= time < fixed + random
// all time is fixed time + random time
func Sleep(fixed, random int) {
	time.Sleep(time.Duration(fixed+Int(random)) * time.Second)
}
