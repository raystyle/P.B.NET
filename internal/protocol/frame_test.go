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
		frameData = []byte{1, 1, 1, 1}
		bigFrame  = bytes.Repeat([]byte{1}, 32768)
		wg        sync.WaitGroup
	)
	server, client := net.Pipe()
	wg.Add(1)
	go func() {
		defer wg.Done()
		var count int
		HandleConn(server, func(frame []byte) {
			count++
			if count != 5 {
				require.Equal(t, frameData, frame)
			} else {
				require.Equal(t, bigFrame, frame)
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
	// big frame
	_, _ = client.Write(convert.Uint32ToBytes(32768))
	_, _ = client.Write(bigFrame)
	_ = client.Close()
	wg.Wait()
}

func TestHandleNULLFrame(t *testing.T) {
	var wg sync.WaitGroup
	server, client := net.Pipe()
	wg.Add(1)
	go func() {
		defer wg.Done()
		HandleConn(server, func(frame []byte) {
			require.Equal(t, ConnErrRecvNullFrame, frame[0])
		})
		_ = server.Close()
	}()
	_, _ = client.Write([]byte{0, 0, 0, 0})
	_ = client.Close()
	wg.Wait()
}

func TestHandleTooBigFrame(t *testing.T) {
	var wg sync.WaitGroup
	server, client := net.Pipe()
	wg.Add(1)
	go func() {
		defer wg.Done()
		HandleConn(server, func(frame []byte) {
			require.Equal(t, ConnErrRecvTooBigFrame, frame[0])
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
		frameData = bytes.Repeat([]byte{1}, size)
		wg        sync.WaitGroup
	)
	server, client := net.Pipe()
	wg.Add(1)
	go func() {
		defer wg.Done()
		count := 0
		HandleConn(server, func(frame []byte) {
			if bytes.Compare(frame, frameData) != 0 {
				b.FailNow()
			}
			count++
		})
		_ = server.Close()
		require.Equal(b, b.N, count)
	}()
	frame := append(convert.Uint32ToBytes(uint32(size)), frameData...)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.Write(frame)
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
	frameData := bytes.Repeat([]byte{1}, size)
	wg := sync.WaitGroup{}
	frame := append(convert.Uint32ToBytes(uint32(size)), frameData...)
	server, client := net.Pipe()

	wg.Add(1)
	go func() {
		defer wg.Done()
		HandleConn(server, func(frame []byte) {
			if bytes.Compare(frame, frameData) != 0 {
				b.FailNow()
			}
		})
		_ = server.Close()
	}()

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := client.Write(frame)
			if err != nil {
				b.FailNow()
			}
		}
	})

	b.StopTimer()
	_ = client.Close()
	wg.Wait()
}
