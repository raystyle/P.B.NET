package light

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_xlight(t *testing.T) {
	listener, err := Listen("tcp", ":0", 0)
	require.Nil(t, err, err)
	go func() {
		conn, err := listener.Accept()
		require.Nil(t, err, err)
		write := func() {
			testdata := test_generate_testdata()
			_, err = conn.Write(testdata)
			require.Nil(t, err, err)
			require.Equal(t, test_generate_testdata(), testdata)
		}
		read := func() {
			data := make([]byte, 256)
			_, err = io.ReadFull(conn, data)
			require.Nil(t, err, err)
			require.Equal(t, test_generate_testdata(), data)
		}
		read()
		write()
		write()
		read()
	}()
	conn, err := Dial("tcp", listener.Addr().String(), 0)
	require.Nil(t, err, err)
	write := func() {
		testdata := test_generate_testdata()
		_, err = conn.Write(testdata)
		require.Nil(t, err, err)
		require.Equal(t, test_generate_testdata(), testdata)
	}
	read := func() {
		data := make([]byte, 256)
		_, err = io.ReadFull(conn, data)
		require.Nil(t, err, err)
		require.Equal(t, test_generate_testdata(), data)
	}
	write()
	read()
	read()
	write()
}

func test_generate_testdata() []byte {
	testdata := make([]byte, 256)
	for i := 0; i < 256; i++ {
		testdata[i] = byte(i)
	}
	return testdata
}

/*
func Test_Dial_With_Dialer(t *testing.T) {
	listener, err := Listen("tcp", ":0", 0)
	require.Nil(t, err, err)
	go func() {
		conn, err := listener.Accept()
		require.Nil(t, err, err)
		_, _ = conn.Read(nil)
		_ = conn.Close()
	}()
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
	}
	conn, err := Dial_With_Dialer(dialer, "tcp", listener.Addr().String())
	require.Nil(t, err, err)
	_ = conn.Close()
}
*/
