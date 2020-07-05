package security

import (
	"context"
	"reflect"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"project/internal/random"
)

var memory *Memory

func init() {
	memory = NewMemory()
	PaddingMemory()
	FlushMemory()
}

// Memory is used to padding memory for randomized memory address.
type Memory struct {
	rand    *random.Rand
	padding map[string][]byte
	mu      sync.Mutex
}

// NewMemory is used to create Memory.
func NewMemory() *Memory {
	mem := &Memory{
		rand:    random.NewRand(),
		padding: make(map[string][]byte),
	}
	mem.Padding()
	return mem
}

// Padding is used to padding memory.
func (m *Memory) Padding() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := 0; i < 16; i++ {
		data := m.rand.Bytes(8 + m.rand.Int(256))
		m.padding[m.rand.String(8)] = data
	}
}

// Flush is used to flush memory.
func (m *Memory) Flush() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.padding = make(map[string][]byte)
}

// PaddingMemory is used to alloc memory.
func PaddingMemory() {
	memory.Padding()
}

// FlushMemory is used to flush global memory.
func FlushMemory() {
	memory.Flush()
}

// CoverBytes is used to cover byte slice if byte slice has secret.
func CoverBytes(bytes []byte) {
	for i := 0; i < len(bytes); i++ {
		bytes[i] = 0
	}
}

// CoverString is used to cover string if string has secret.
func CoverString(str string) {
	stringHeader := (*reflect.StringHeader)(unsafe.Pointer(&str)) // #nosec
	slice := make([]byte, stringHeader.Len)
	sliceHeader := (*reflect.SliceHeader)(unsafe.Pointer(&slice)) // #nosec
	sliceHeader.Data = stringHeader.Data
	CoverBytes(slice)
	runtime.KeepAlive(&str)
}

// CoverStringMap is used to cover string map.
func CoverStringMap(m map[string]struct{}) {
	for str := range m {
		CoverString(str)
	}
}

// Bytes make byte slice discontinuous, it safe for use by multiple goroutines.
type Bytes struct {
	data  map[int]byte
	len   int
	cache sync.Pool
}

// NewBytes is used to create security Bytes.
func NewBytes(b []byte) *Bytes {
	l := len(b)
	bytes := Bytes{
		data: make(map[int]byte, l),
		len:  l,
	}
	for i := 0; i < l; i++ {
		bytes.data[i] = b[i]
	}
	bytes.cache.New = func() interface{} {
		b := make([]byte, bytes.len)
		return &b
	}
	return &bytes
}

// Get is used to get stored byte slice.
func (b *Bytes) Get() []byte {
	bytes := *b.cache.Get().(*[]byte)
	for i := 0; i < b.len; i++ {
		bytes[i] = b.data[i]
	}
	return bytes
}

// Put is used to put byte slice to cache, slice will be covered.
func (b *Bytes) Put(bytes []byte) {
	for i := 0; i < b.len; i++ {
		bytes[i] = 0
	}
	b.cache.Put(&bytes)
}

// BogoWait is used to use bogo sort to wait time, If timeout, it will interrupt.
func BogoWait(n int, timeout time.Duration) {
	if n < 2 {
		n = 2
	}
	if timeout < 1 || timeout > 5*time.Minute {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	rand := random.NewRand()
	// generate random number
	num := make([]int, n)
	for i := 0; i < n; i++ {
		num[i] = rand.Int(100000)
	}
swap:
	for {
		// check timeout
		select {
		case <-ctx.Done():
			return
		default:
		}
		// swap
		for i := 0; i < n; i++ {
			j := rand.Int(n)
			num[i], num[j] = num[j], num[i]
		}
		// check is sorted
		for i := 1; i < n; i++ {
			if num[i-1] > num[i] {
				continue swap
			}
		}
		return
	}
}
