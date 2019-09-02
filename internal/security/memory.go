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
	random  *random.Generator
	padding map[string][]byte
	mutex   sync.Mutex
}

func NewMemory() *Memory {
	m := &Memory{
		random:  random.New(0),
		padding: make(map[string][]byte),
	}
	m.Padding()
	return m
}

func (m *Memory) Padding() {
	m.mutex.Lock()
	for i := 0; i < 16; i++ {
		m.padding[m.random.String(8)] =
			m.random.Bytes(8 + m.random.Int(256))
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
