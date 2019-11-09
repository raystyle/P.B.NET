package security

import (
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

type Memory struct {
	rand    *random.Rand
	padding map[string][]byte
	mutex   sync.Mutex
}

func NewMemory() *Memory {
	m := &Memory{
		rand:    random.New(0),
		padding: make(map[string][]byte),
	}
	m.Padding()
	return m
}

func (m *Memory) Padding() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	for i := 0; i < 16; i++ {
		data := m.rand.Bytes(8 + m.rand.Int(256))
		m.padding[m.rand.String(8)] = data
	}
}

func (m *Memory) Flush() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.padding = make(map[string][]byte)
}

func PaddingMemory() {
	memory.Padding()
}

func FlushMemory() {
	memory.Flush()
}

// FlushBytes is used to cover []byte if []byte has secret
func FlushBytes(b []byte) {
	mem := NewMemory()
	mem.Padding()
	rand := random.New(0)
	randBytes := rand.Bytes(len(b))
	copy(b, randBytes)
	mem.Flush()
}

// FlushString is used to cover string if string has secret
func FlushString(s *string) {
	mem := NewMemory()
	mem.Padding()
	rand := random.New(0)
	sh := (*reflect.StringHeader)(unsafe.Pointer(s))
	randBytes := rand.Bytes(sh.Len)
	for i := 0; i < sh.Len; i++ {
		mem.Padding()
		b := (*byte)(unsafe.Pointer(sh.Data + uintptr(i)))
		*b = randBytes[i]
		mem.Flush()
	}
}
