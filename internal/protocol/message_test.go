package protocol

import (
	"bytes"
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/convert"
)

func TestNewSlot(t *testing.T) {
	slot := NewSlot()
	<-slot.Available
}

func TestHandleConn(t *testing.T) {
	var (
		message = []byte{1, 1, 1, 1}
		bigMsg  = bytes.Repeat([]byte{1}, 32768)
		wg      sync.WaitGroup
	)
	server, client := net.Pipe()
	wg.Add(1)
	go func() {
		defer wg.Done()
		var count int
		HandleConn(server, func(msg []byte) {
			count++
			if count != 5 {
				require.Equal(t, message, msg)
			} else {
				require.Equal(t, bigMsg, msg)
			}
		})
		_ = server.Close()
		require.Equal(t, 5, count)
	}()
	// full
	_, _ = client.Write([]byte{0, 0, 0, 4, 1, 1, 1, 1})
	// full with incomplete header
	_, _ = client.Write([]byte{0, 0, 0, 4, 1, 1, 1, 1, 0, 0})
	_, _ = client.Write([]byte{0, 4, 1, 1, 1, 1})
	// incomplete body
	_, _ = client.Write([]byte{0, 0, 0, 4, 1, 1})
	_, _ = client.Write([]byte{1, 1})
	// big message
	_, _ = client.Write(convert.Uint32ToBytes(32768))
	_, _ = client.Write(bigMsg)
	_ = client.Close()
	wg.Wait()
}

func TestHandleNULLMessage(t *testing.T) {
	var wg sync.WaitGroup
	server, client := net.Pipe()
	wg.Add(1)
	go func() {
		defer wg.Done()
		HandleConn(server, func(msg []byte) {
			require.Equal(t, ConnErrRecvNullMsg, msg[0])
		})
		_ = server.Close()
	}()
	_, _ = client.Write([]byte{0, 0, 0, 0})
	_ = client.Close()
	wg.Wait()
}

func TestHandleTooBigMessage(t *testing.T) {
	var wg sync.WaitGroup
	server, client := net.Pipe()
	wg.Add(1)
	go func() {
		defer wg.Done()
		HandleConn(server, func(msg []byte) {
			require.Equal(t, ConnErrRecvTooBigMsg, msg[0])
		})
		_ = server.Close()
	}()
	_, _ = client.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF})
	_ = client.Close()
	wg.Wait()
}

func BenchmarkHandleConn_128B(b *testing.B) {
	benchmarkHandleConn(b, 128)
}

func BenchmarkHandleConn_2KB(b *testing.B) {
	benchmarkHandleConn(b, 2048)
}

func BenchmarkHandleConn_4KB(b *testing.B) {
	benchmarkHandleConn(b, 4096)
}

func BenchmarkHandleConn_32KB(b *testing.B) {
	benchmarkHandleConn(b, 32768)
}

func BenchmarkHandleConn_1MB(b *testing.B) {
	benchmarkHandleConn(b, 1048576)
}

func benchmarkHandleConn(b *testing.B, size int) {
	var (
		message = bytes.Repeat([]byte{1}, size)
		wg      sync.WaitGroup
	)
	server, client := net.Pipe()
	wg.Add(1)
	go func() {
		defer wg.Done()
		count := 0
		HandleConn(server, func(msg []byte) {
			if !bytes.Equal(msg, message) {
				b.FailNow()
			}
			count++
		})
		_ = server.Close()
		require.Equal(b, b.N, count)
	}()
	msg := append(convert.Uint32ToBytes(uint32(size)), message...)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.Write(msg)
		if err != nil {
			b.FailNow()
		}
	}
	b.StopTimer()
	_ = client.Close()
	wg.Wait()
}

func BenchmarkHandleConnParallel_128B(b *testing.B) {
	benchmarkHandleConnParallel(b, 128)
}

func BenchmarkHandleConnParallel_2KB(b *testing.B) {
	benchmarkHandleConnParallel(b, 2048)
}

func BenchmarkHandleConnParallel_4KB(b *testing.B) {
	benchmarkHandleConnParallel(b, 4096)
}

func BenchmarkHandleConnParallel_32KB(b *testing.B) {
	benchmarkHandleConnParallel(b, 32768)
}

func BenchmarkHandleConnParallel_1MB(b *testing.B) {
	benchmarkHandleConnParallel(b, 1048576)
}

func benchmarkHandleConnParallel(b *testing.B, size int) {
	message := bytes.Repeat([]byte{1}, size)
	wg := sync.WaitGroup{}
	msg := append(convert.Uint32ToBytes(uint32(size)), message...)
	server, client := net.Pipe()

	wg.Add(1)
	go func() {
		defer wg.Done()
		HandleConn(server, func(msg []byte) {
			if !bytes.Equal(msg, message) {
				b.FailNow()
			}
		})
		_ = server.Close()
	}()

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := client.Write(msg)
			if err != nil {
				b.FailNow()
			}
		}
	})

	b.StopTimer()
	_ = client.Close()
	wg.Wait()
}
