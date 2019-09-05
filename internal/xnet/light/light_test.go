package light

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLight(t *testing.T) {
	listener, err := Listen("tcp", "localhost:0", 0)
	require.NoError(t, err)
	go func() {
		conn, err := listener.Accept()
		require.NoError(t, err)
		write := func() {
			testdata := testGenerateTestdata()
			_, err = conn.Write(testdata)
			require.NoError(t, err)
			require.Equal(t, testGenerateTestdata(), testdata)
		}
		read := func() {
			data := make([]byte, 256)
			_, err = io.ReadFull(conn, data)
			require.NoError(t, err)
			require.Equal(t, testGenerateTestdata(), data)
		}
		read()
		write()
		write()
		read()
	}()
	conn, err := Dial("tcp", listener.Addr().String(), 0)
	require.NoError(t, err)
	write := func() {
		testdata := testGenerateTestdata()
		_, err = conn.Write(testdata)
		require.NoError(t, err)
		require.Equal(t, testGenerateTestdata(), testdata)
	}
	read := func() {
		data := make([]byte, 256)
		_, err = io.ReadFull(conn, data)
		require.NoError(t, err)
		require.Equal(t, testGenerateTestdata(), data)
	}
	write()
	read()
	read()
	write()
}

func testGenerateTestdata() []byte {
	testdata := make([]byte, 256)
	for i := 0; i < 256; i++ {
		testdata[i] = byte(i)
	}
	return testdata
}

/*
func Test_Dial_With_Dialer(t *testing.T) {
	listener, err := Listen("tcp", ":0", 0)
	require.NoError(t, err)
	go func() {
		conn, err := listener.Accept()
		require.NoError(t, err)
		_, _ = conn.Read(nil)
		_ = conn.Close()
	}()
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
	}
	conn, err := Dial_With_Dialer(dialer, "tcp", listener.Addr().String())
	require.NoError(t, err)
	_ = conn.Close()
}
*/
