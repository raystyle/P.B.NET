package security

import (
	"reflect"
	"runtime"
	"sync"
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
	slice := make([]byte, stringHeader.Len, stringHeader.Len)
	sliceHeader := (*reflect.SliceHeader)(unsafe.Pointer(&slice)) // #nosec
	sliceHeader.Data = stringHeader.Data
	CoverBytes(slice)
	runtime.KeepAlive(&str)
}

// Bytes make byte slice discontinuous, it safe for use by multiple goroutines.
type Bytes struct {
	data  map[int]byte
	len   int
	cache sync.Pool
}

// NewBytes is used to create Bytes.
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

// Put is used to put byte slice to cache, slice will be cover.
func (b *Bytes) Put(s []byte) {
	for i := 0; i < b.len; i++ {
		s[i] = 0
	}
	b.cache.Put(&s)
}
