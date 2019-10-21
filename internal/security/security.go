package security

import (
	"sync"

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
	for i := 0; i < 16; i++ {
		m.padding[m.rand.String(8)] =
			m.rand.Bytes(8 + m.rand.Int(256))
	}
	m.mutex.Unlock()
}

func (m *Memory) Flush() {
	m.mutex.Lock()
	m.padding = make(map[string][]byte)
	m.mutex.Unlock()
}

func PaddingMemory() {
	memory.Padding()
}

func FlushMemory() {
	memory.Flush()
}

func FlushBytes(b []byte) {
	mem := NewMemory()
	mem.Padding()
	rand := random.New(0)
	randBytes := rand.Bytes(len(b))
	copy(b, randBytes)
	mem.Flush()
}
