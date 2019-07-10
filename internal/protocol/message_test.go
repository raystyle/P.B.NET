package protocol

import (
	"bytes"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/convert"
	"project/internal/xnet"
)

func Test_Handle_Message(t *testing.T) {
	var (
		message = []byte{1, 1, 1, 1}
		big_msg = bytes.Repeat([]byte{1}, 32768)
		wg      sync.WaitGroup
	)
	c := &xnet.Config{Network: "tcp", Address: ":0"}
	listener, err := xnet.Listen(xnet.LIGHT, c)
	require.Nil(t, err, err)
	wg.Add(1)
	go func() {
		defer wg.Done()
		count := 0
		conn, err := listener.Accept()
		require.Nil(t, err, err)
		xconn := xnet.New_Conn(conn, time.Now().Unix())
		Handle_Conn(xconn, func(msg []byte) {
			count += 1
			if count != 5 {
				require.Equal(t, message, msg)
			} else {
				require.Equal(t, big_msg, msg)
			}
		}, func() { _ = conn.Close() })
		require.Equal(t, 5, count)
	}()
	// dial
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	c.Address = "localhost:" + port
	conn, err := xnet.Dial(xnet.LIGHT, c)
	require.Nil(t, err, err)
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
	_, err = conn.Write(convert.Uint32_Bytes(32768))
	_, err = conn.Write(big_msg)
	_ = conn.Close()
	wg.Wait()
}

func Test_Handle_NULL_Message(t *testing.T) {
	var (
		wg sync.WaitGroup
	)
	c := &xnet.Config{Network: "tcp", Address: ":0"}
	listener, err := xnet.Listen(xnet.LIGHT, c)
	require.Nil(t, err, err)
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, err := listener.Accept()
		require.Nil(t, err, err)
		xconn := xnet.New_Conn(conn, time.Now().Unix())
		Handle_Conn(xconn, func(msg []byte) {
			require.Equal(t, ERR_NULL_MSG, msg)
		}, func() { _ = conn.Close() })
	}()
	// dial
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	c.Address = "localhost:" + port
	conn, err := xnet.Dial(xnet.LIGHT, c)
	require.Nil(t, err, err)
	_, err = conn.Write([]byte{0, 0, 0, 0})
	_ = conn.Close()
	wg.Wait()
}

func Test_Handle_Too_Big_Message(t *testing.T) {
	var (
		wg sync.WaitGroup
	)
	c := &xnet.Config{Network: "tcp", Address: ":0"}
	listener, err := xnet.Listen(xnet.LIGHT, c)
	require.Nil(t, err, err)
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, err := listener.Accept()
		require.Nil(t, err, err)
		xconn := xnet.New_Conn(conn, time.Now().Unix())
		Handle_Conn(xconn, func(msg []byte) {
			require.Equal(t, ERR_TOO_BIG_MSG, msg)
		}, func() { _ = conn.Close() })
		_ = conn.Close()
	}()
	// dial
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	c.Address = "localhost:" + port
	conn, err := xnet.Dial(xnet.LIGHT, c)
	require.Nil(t, err, err)
	_, err = conn.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF})
	_ = conn.Close()
	wg.Wait()
}

func Benchmark_Handle_Message_128B(b *testing.B) {
	benchmark_handle_message(b, 128)
}

func Benchmark_Handle_Message_2KB(b *testing.B) {
	benchmark_handle_message(b, 2048)
}

func Benchmark_Handle_Message_4KB(b *testing.B) {
	benchmark_handle_message(b, 4096)
}

func Benchmark_Handle_Message_32KB(b *testing.B) {
	benchmark_handle_message(b, 32768)
}

func Benchmark_Handle_Message_1MB(b *testing.B) {
	benchmark_handle_message(b, 1048576)
}

func Benchmark_Handle_Message_16MB(b *testing.B) {
	benchmark_handle_message(b, 16*1048576)
}

func Benchmark_Handle_Message_64MB(b *testing.B) {
	benchmark_handle_message(b, 64*1048576)
}

func benchmark_handle_message(b *testing.B, size int) {
	var (
		message = bytes.Repeat([]byte{1}, size)
		wg      sync.WaitGroup
	)
	c := &xnet.Config{Network: "tcp", Address: ":0"}
	listener, err := xnet.Listen(xnet.LIGHT, c)
	require.Nil(b, err, err)
	wg.Add(1)
	go func() {
		defer wg.Done()
		count := 0
		conn, err := listener.Accept()
		require.Nil(b, err, err)
		xconn := xnet.New_Conn(conn, time.Now().Unix())
		Handle_Conn(xconn, func(msg []byte) {
			if !bytes.Equal(msg, message) {
				b.FailNow()
			}
			count += 1
		}, func() { _ = conn.Close() })
		require.Equal(b, b.N, count)
	}()
	// dial
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	c.Address = "localhost:" + port
	conn, err := xnet.Dial(xnet.LIGHT, c)
	require.Nil(b, err, err)
	msg := append(convert.Uint32_Bytes(uint32(size)), message...)
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
