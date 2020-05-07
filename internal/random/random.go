package random

import (
	cr "crypto/rand"
	"crypto/sha256"
	"io"
	"math/rand"
	"runtime"
	"sync"
	"time"

	"project/internal/convert"
	"project/internal/xpanic"
)

var (
	gRand    *Rand
	gSleeper *Sleeper
)

func init() {
	gRand = NewRand()
	gSleeper = NewSleeper()
}

// Rand is used to generate random data.
type Rand struct {
	rand *rand.Rand
	mu   sync.Mutex
}

// NewRand is used to create a Rand.
// performance: BenchmarkNew-6    4148    304633 ns/op    35511 B/op
func NewRand() *Rand {
	const (
		goroutines = 4
		times      = 128
	)
	data := make(chan []byte, 16)
	for i := 0; i < goroutines; i++ {
		go sendData(data, times)
	}
	timer := time.NewTimer(time.Second)
	hash := sha256.New()
read:
	for i := 0; i < goroutines*times; i++ {
		timer.Reset(time.Second)
		select {
		case d := <-data:
			if d != nil {
				hash.Write(d)
			}
		case <-timer.C:
			break read
		}
	}
	n, _ := io.CopyN(hash, cr.Reader, 512)
	hash.Write([]byte{byte(n)})
	hashData := hash.Sum(nil)
	r := rand.New(rand.NewSource(time.Now().UnixNano())) // #nosec
	selected := make([]byte, 8)
	for i := 0; i < 8; i++ {
		selected[i] = hashData[r.Intn(sha256.Size)]
	}
	seed := convert.BytesToInt64(selected)
	return &Rand{rand: rand.New(rand.NewSource(seed))} // #nosec
}

func sendData(data chan<- []byte, times int) {
	defer func() {
		if r := recover(); r != nil {
			xpanic.Log(r, "sendData")
		}
	}()
	count := 0
	timer := time.NewTimer(time.Second)
	r := rand.New(rand.NewSource(time.Now().UnixNano())) // #nosec
	for i := 0; i < times; i++ {
		timer.Reset(time.Second)
		select {
		case data <- []byte{byte(r.Intn(256) + i)}:
		case <-timer.C:
			return
		}
		// schedule manually
		if count > 16 {
			runtime.Gosched()
			count = 0
		} else {
			count++
		}
	}
}

// String return a string that not include "|".
func (r *Rand) String(n int) string {
	if n < 1 {
		return ""
	}
	result := make([]rune, n)
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := 0; i < n; i++ {
		ri := r.rand.Intn(90)
		result[i] = rune(33 + ri)
	}
	return string(result)
}

// Bytes is used to generate random []byte that size = n.
func (r *Rand) Bytes(n int) []byte {
	if n < 1 {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]byte, n)
	for i := 0; i < n; i++ {
		ri := r.rand.Intn(256)
		result[i] = byte(ri)
	}
	return result
}

// Cookie return a string that only include number and A-Z a-z.
func (r *Rand) Cookie(n int) string {
	if n < 1 {
		return ""
	}
	result := make([]rune, n)
	r.mu.Lock()
	defer r.mu.Unlock()
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
func (r *Rand) Int(n int) int {
	if n < 1 {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rand.Intn(n)
}

// Int64 returns a non-negative pseudo-random 63-bit integer as an int64.
func (r *Rand) Int64() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rand.Int63()
}

// Uint64 returns a pseudo-random 64-bit value as a uint64.
func (r *Rand) Uint64() uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rand.Uint64()
}

// String return a string that not include "|".
func String(n int) string {
	return gRand.String(n)
}

// Bytes is used to generate random []byte that size = n.
func Bytes(n int) []byte {
	return gRand.Bytes(n)
}

// Cookie return a string that only include number and A-Z a-z.
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

// MaxSleepTime is used to prevent sleep dead!
const MaxSleepTime = 30 * time.Minute

// Sleeper contain a timer and rand for reuse.
type Sleeper struct {
	timer *time.Timer
	rand  *Rand
}

// NewSleeper is used to create a sleeper.
func NewSleeper() *Sleeper {
	timer := time.NewTimer(time.Minute)
	timer.Stop()
	return &Sleeper{
		timer: timer,
		rand:  NewRand(),
	}
}

// Sleep is used to sleep with fixed + random time.
func (s *Sleeper) Sleep(fixed, random uint) <-chan time.Time {
	s.timer.Reset(s.calculateDuration(fixed, random))
	select {
	case <-s.timer.C:
	default:
	}
	return s.timer.C
}

// calculateDuration is used to calculate actual duration.
// fixed <= time < fixed + random
// all time is fixed time + random time
func (s *Sleeper) calculateDuration(fixed, random uint) time.Duration {
	if fixed+random < 1 {
		fixed = 1
	}
	random = uint(s.rand.Int(int(random)))
	total := time.Duration(fixed+random) * time.Second
	if total > MaxSleepTime {
		total = MaxSleepTime
	}
	return total
}

// Stop is used to stop timer.
func (s *Sleeper) Stop() {
	s.timer.Stop()
}

// Sleep is used to sleep a random time.
func Sleep(fixed, random uint) <-chan time.Time {
	return gSleeper.Sleep(fixed, random)
}
