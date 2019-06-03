package security

import (
	"runtime"
	"sync"

	"project/internal/random"
)

var (
	memory *Memory
)

func init() {
	memory = New_Memory()
}

type Memory struct {
	random  *random.Generator
	padding map[string][]byte
	mutex   sync.Mutex
}

func New_Memory() *Memory {
	m := &Memory{
		random:  random.New(),
		padding: make(map[string][]byte),
	}
	m.Padding()
	return m
}

func (this *Memory) Padding() {
	this.mutex.Lock()
	for i := 0; i < 16; i++ {
		this.padding[this.random.String(8)] =
			this.random.Bytes(8 + this.random.Int(256))
	}
	this.mutex.Unlock()
}

func (this *Memory) Flush() {
	this.mutex.Lock()
	this.padding = make(map[string][]byte)
	this.mutex.Unlock()
	runtime.GC()
}

func Padding_Memory() {
	memory.Padding()
}

func Flush_Memory() {
	memory.Flush()
}
