package protocol

import (
	"bytes"
	"net"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/convert"
	"project/internal/xnet"
)

func TestHandleMessage(t *testing.T) {
	var (
		message = []byte{1, 1, 1, 1}
		bigMsg  = bytes.Repeat([]byte{1}, 32768)
		wg      sync.WaitGroup
	)
	cfg := &xnet.Config{Network: "tcp", Address: "localhost:0"}
	listener, err := xnet.Listen(xnet.LIGHT, cfg)
	require.NoError(t, err)
	wg.Add(1)
	go func() {
		defer wg.Done()
		count := 0
		conn, err := listener.Accept()
		require.NoError(t, err)
		HandleConn(conn, func(msg []byte) {
			count += 1
			if count != 5 {
				require.Equal(t, message, msg)
			} else {
				require.Equal(t, bigMsg, msg)
			}
		})
		_ = conn.Close()
		require.Equal(t, 5, count)
	}()
	// dial
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	cfg.Address = "localhost:" + port
	conn, err := xnet.Dial(xnet.LIGHT, cfg)
	require.NoError(t, err)
	// full
	_, err = conn.Write([]byte{0, 0, 0, 4, 1, 1, 1, 1})
	// full with incomplete header
	_, err = conn.Write([]byte{0, 0, 0, 4, 1, 1, 1, 1, 0, 0})
	time.Sleep(100 * time.Millisecond)
	_, err = conn.Write([]byte{0, 4, 1, 1, 1, 1})
	// incomplete body
	_, err = conn.Write([]byte{0, 0, 0, 4, 1, 1})
	time.Sleep(100 * time.Millisecond)
	_, err = conn.Write([]byte{1, 1})
	// big message
	_, err = conn.Write(convert.Uint32ToBytes(32768))
	_, err = conn.Write(bigMsg)
	_ = conn.Close()
	wg.Wait()
}

func TestHandleNULLMessage(t *testing.T) {
	var (
		wg sync.WaitGroup
	)
	cfg := &xnet.Config{Network: "tcp", Address: "localhost:0"}
	listener, err := xnet.Listen(xnet.LIGHT, cfg)
	require.NoError(t, err)
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, err := listener.Accept()
		require.NoError(t, err)
		HandleConn(conn, func(msg []byte) {
			require.Equal(t, ErrNullMsg, msg[0])
		})
		_ = conn.Close()
	}()
	// dial
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	cfg.Address = "localhost:" + port
	conn, err := xnet.Dial(xnet.LIGHT, cfg)
	require.NoError(t, err)
	_, err = conn.Write([]byte{0, 0, 0, 0})
	_ = conn.Close()
	wg.Wait()
}

func TestHandleTooBigMessage(t *testing.T) {
	var (
		wg sync.WaitGroup
	)
	cfg := &xnet.Config{Network: "tcp", Address: "localhost:0"}
	listener, err := xnet.Listen(xnet.LIGHT, cfg)
	require.NoError(t, err)
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, err := listener.Accept()
		require.NoError(t, err)
		HandleConn(conn, func(msg []byte) {
			require.Equal(t, ErrTooBigMsg, msg[0])
		})
		_ = conn.Close()
		_ = conn.Close()
	}()
	// dial
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	cfg.Address = "localhost:" + port
	conn, err := xnet.Dial(xnet.LIGHT, cfg)
	require.NoError(t, err)
	_, err = conn.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF})
	_ = conn.Close()
	wg.Wait()
}

func BenchmarkHandleMessage_128B(b *testing.B) {
	benchmarkHandleMessage(b, 128)
}

func BenchmarkHandleMessage_2KB(b *testing.B) {
	benchmarkHandleMessage(b, 2048)
}

func BenchmarkHandleMessage_4KB(b *testing.B) {
	benchmarkHandleMessage(b, 4096)
}

func BenchmarkHandleMessage_32KB(b *testing.B) {
	benchmarkHandleMessage(b, 32768)
}

func BenchmarkHandleMessage_1MB(b *testing.B) {
	benchmarkHandleMessage(b, 1048576)
}

func BenchmarkHandleMessage_16MB(b *testing.B) {
	benchmarkHandleMessage(b, 16*1048576)
}

func BenchmarkHandleMessage_64MB(b *testing.B) {
	benchmarkHandleMessage(b, 64*1048576)
}

func benchmarkHandleMessage(b *testing.B, size int) {
	var (
		message = bytes.Repeat([]byte{1}, size)
		wg      sync.WaitGroup
	)
	cfg := &xnet.Config{Network: "tcp", Address: "localhost:0"}
	listener, err := xnet.Listen(xnet.LIGHT, cfg)
	require.NoError(b, err)
	wg.Add(1)
	go func() {
		defer wg.Done()
		count := 0
		conn, err := listener.Accept()
		require.NoError(b, err)
		HandleConn(conn, func(msg []byte) {
			if !bytes.Equal(msg, message) {
				b.FailNow()
			}
			count += 1
		})
		_ = conn.Close()
		require.Equal(b, b.N, count)
	}()
	// dial
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	cfg.Address = "localhost:" + port
	conn, err := xnet.Dial(xnet.LIGHT, cfg)
	require.NoError(b, err)
	msg := append(convert.Uint32ToBytes(uint32(size)), message...)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := conn.Write(msg)
		if err != nil {
			b.FailNow()
		}
	}
	b.StopTimer()
	_ = conn.Close()
	wg.Wait()
}

func BenchmarkHandleMessageParallel_128B(b *testing.B) {
	benchmarkHandleMessageParallel(b, 128)
}

func BenchmarkHandleMessageParallel_2KB(b *testing.B) {
	benchmarkHandleMessageParallel(b, 2048)
}

func BenchmarkHandleMessageParallel_4KB(b *testing.B) {
	benchmarkHandleMessageParallel(b, 4096)
}

func BenchmarkHandleMessageParallel_32KB(b *testing.B) {
	benchmarkHandleMessageParallel(b, 32768)
}

func BenchmarkHandleMessageParallel_1MB(b *testing.B) {
	benchmarkHandleMessageParallel(b, 1048576)
}

func BenchmarkHandleMessageParallel_16MB(b *testing.B) {
	benchmarkHandleMessageParallel(b, 16*1048576)
}

func BenchmarkHandleMessageParallel_64MB(b *testing.B) {
	benchmarkHandleMessageParallel(b, 64*1048576)
}

func benchmarkHandleMessageParallel(b *testing.B, size int) {
	var (
		nOnce   = b.N / runtime.NumCPU()
		message = bytes.Repeat([]byte{1}, size)
		wg      sync.WaitGroup
	)
	cfg := &xnet.Config{Network: "tcp", Address: "localhost:0"}
	listener, err := xnet.Listen(xnet.LIGHT, cfg)
	require.NoError(b, err)
	wg.Add(1)
	go func() {
		defer wg.Done()
		count := 0
		conn, err := listener.Accept()
		require.NoError(b, err)
		HandleConn(conn, func(msg []byte) {
			if !bytes.Equal(msg, message) {
				b.FailNow()
			}
			count += 1
		})
		_ = conn.Close()
		require.Equal(b, nOnce*runtime.NumCPU(), count)
	}()
	// dial
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	cfg.Address = "localhost:" + port
	conn, err := xnet.Dial(xnet.LIGHT, cfg)
	require.NoError(b, err)
	msg := append(convert.Uint32ToBytes(uint32(size)), message...)
	writeWG := sync.WaitGroup{}
	write := func() {
		for i := 0; i < nOnce; i++ {
			_, err := conn.Write(msg)
			if err != nil {
				b.FailNow()
			}
		}
		writeWG.Done()
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < runtime.NumCPU(); i++ {
		writeWG.Add(1)
		go write()
	}
	writeWG.Wait()
	b.StopTimer()
	_ = conn.Close()
	wg.Wait()
}
