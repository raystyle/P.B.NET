package testsuite

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGoRoutineMark(t *testing.T) {
	gm := MarkGoRoutines(t)
	defer gm.Compare()

	c := make(chan struct{})
	go func() {
		c <- struct{}{}
	}()
	<-c
}

func TestGoRoutineMark_Leak(t *testing.T) {
	gm := MarkGoRoutines(t)
	defer func() { require.Equal(t, 1, gm.calculate()) }()

	c := make(chan struct{})
	go func() {
		c <- struct{}{}
	}()
}

func TestMemoryMark(t *testing.T) {
	mm := MarkMemory(t)

	asd := make([]byte, 1024)
	asd[1023] = 1
	asd[1022] = 2

	TestConn(t)

	mm.Compare()
}

func TestMemoryMark_Leak(t *testing.T) {
	mm := MarkMemory(t)
	defer mm.Compare()

	server, client := net.Pipe()
	server2, client2 := net.Pipe()

	asd := make([]byte, 1024)
	asd[1023] = 1
	asd[1022] = 2

	go func() {
		time.Sleep(time.Minute)
		client.Write(asd)
		server.Write(asd)
		_ = server.Close()
		_ = client.Close()
		client2.Write(asd)
		server2.Write(asd)
	}()

	TestConn(t)
}
