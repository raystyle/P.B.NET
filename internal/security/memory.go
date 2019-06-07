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
	Padding_Memory()
	Flush_Memory()
}

type Memory struct {
	random  *random.Generator
	padding map[string][]byte
	m       sync.Mutex
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
	this.m.Lock()
	for i := 0; i < 16; i++ {
		this.padding[this.random.String(8)] =
			this.random.Bytes(8 + this.random.Int(256))
	}
	this.m.Unlock()
}

func (this *Memory) Flush() {
	this.m.Lock()
	this.padding = make(map[string][]byte)
	this.m.Unlock()
	runtime.GC()
}

func Padding_Memory() {
	memory.Padding()
}

func Flush_Memory() {
	memory.Flush()
}
