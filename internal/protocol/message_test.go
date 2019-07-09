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
		message = []byte{0, 0, 0, 0}
		big_msg = bytes.Repeat([]byte{0}, 32768)
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
		Handle_Message(xconn, func(msg []byte) {
			count += 1
			if count != 5 {
				require.Equal(t, message, msg)
			} else {
				require.Equal(t, big_msg, msg)
			}
		})
		_ = conn.Close()
		require.Equal(t, 5, count)
	}()
	// dial
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	c.Address = "localhost:" + port
	conn, err := xnet.Dial(xnet.LIGHT, c)
	require.Nil(t, err, err)
	// full
	_, err = conn.Write([]byte{0, 0, 0, 4, 0, 0, 0, 0})
	// full with incomplete header
	_, err = conn.Write([]byte{0, 0, 0, 4, 0, 0, 0, 0, 0, 0})
	time.Sleep(100 * time.Millisecond)
	_, err = conn.Write([]byte{0, 4, 0, 0, 0, 0})
	// incomplete body
	_, err = conn.Write([]byte{0, 0, 0, 4, 0, 0})
	time.Sleep(100 * time.Millisecond)
	_, err = conn.Write([]byte{0, 0})
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
		Handle_Message(xconn, func(msg []byte) {
			require.Equal(t, ERR_NULL_MESSAGE, msg)
		})
		_ = conn.Close()
	}()
	// dial
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	c.Address = "localhost:" + port
	conn, err := xnet.Dial(xnet.LIGHT, c)
	require.Nil(t, err, err)
	_, err = conn.Write([]byte{0, 0, 0, 0, 0, 0, 0, 0})
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
		Handle_Message(xconn, func(msg []byte) {
			require.Equal(t, ERR_TOO_BIG_MESSAGE, msg)
		})
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
