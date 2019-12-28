package security

import (
	"net/http"
	"reflect"
	"sync"
	"unsafe"

	"project/internal/random"
)

var (
	memory *Memory
)

func init() {
	memory = NewMemory()
	PaddingMemory()
	FlushMemory()
}

// Memory include padding memory
type Memory struct {
	rand    *random.Rand
	padding map[string][]byte
	mutex   sync.Mutex
}

// NewMemory is used to create Memory
func NewMemory() *Memory {
	m := &Memory{
		rand:    random.New(),
		padding: make(map[string][]byte),
	}
	m.Padding()
	return m
}

// Padding is used to padding memory
func (m *Memory) Padding() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	for i := 0; i < 16; i++ {
		data := m.rand.Bytes(8 + m.rand.Int(256))
		m.padding[m.rand.String(8)] = data
	}
}

// Flush is used to flush memory
func (m *Memory) Flush() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.padding = make(map[string][]byte)
}

// PaddingMemory is used to alloc memory
func PaddingMemory() {
	memory.Padding()
}

// FlushMemory is used to flush global memory
func FlushMemory() {
	memory.Flush()
}

// CoverBytes is used to cover byte slice if byte slice has secret
func CoverBytes(b []byte) {
	mem := NewMemory()
	mem.Padding()
	rand := random.New()
	randBytes := rand.Bytes(len(b))
	copy(b, randBytes)
	mem.Flush()
}

// CoverString is used to cover string if string has secret
func CoverString(s *string) {
	mem := NewMemory()
	mem.Padding()
	rand := random.New()
	sh := (*reflect.StringHeader)(unsafe.Pointer(s))
	randBytes := rand.Bytes(sh.Len)
	for i := 0; i < sh.Len; i++ {
		mem.Padding()
		b := (*byte)(unsafe.Pointer(sh.Data + uintptr(i)))
		*b = randBytes[i]
		mem.Flush()
	}
}

// CoverHTTPRequest is used to cover http.Request string field if has secret
func CoverHTTPRequest(r *http.Request) {
	CoverString(&r.Host)
	CoverString(&r.URL.Host)
	CoverString(&r.URL.Path)
	CoverString(&r.URL.RawPath)
}

// Bytes make byte slice discontinuous, it safe for use by multiple goroutines
type Bytes struct {
	data  map[int]byte
	len   int
	cache sync.Pool
}

// NewBytes is used to create Bytes
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
		return make([]byte, bytes.len)
	}
	return &bytes
}

// Get is used to get stored byte slice
func (b *Bytes) Get() []byte {
	bytes := b.cache.Get().([]byte)
	for i := 0; i < b.len; i++ {
		bytes[i] = b.data[i]
	}
	return bytes
}

// Put is used to put byte slice to cache, slice will be cover
func (b *Bytes) Put(s []byte) {
	for i := 0; i < b.len; i++ {
		s[i] = 0
	}
	b.cache.Put(s)
}
