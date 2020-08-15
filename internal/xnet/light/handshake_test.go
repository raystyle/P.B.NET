package light

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/convert"
	"project/internal/crypto/aes"
	"project/internal/crypto/curve25519"
	"project/internal/patch/monkey"
	"project/internal/testsuite"
)

func testGenerateConnPair() (*Conn, *Conn) {
	serverPipe, clientPipe := net.Pipe()
	server := Server(context.Background(), serverPipe, 0)
	client := Client(context.Background(), clientPipe, 0)
	return server, client
}

func testConnClientHandshake(t *testing.T, f func(t *testing.T, server *Conn), expected error) {
	server, client := testGenerateConnPair()

	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(ioutil.Discard, server.Conn)
	}()
	go func() {
		defer wg.Done()
		f(t, server)
	}()
	err := client.clientHandshake()
	if expected != nil {
		require.Equal(t, expected, err)
	} else {
		require.Error(t, err)
	}

	err = server.Close()
	require.NoError(t, err)
	err = client.Close()
	require.NoError(t, err)

	wg.Wait()

	testsuite.IsDestroyed(t, server)
	testsuite.IsDestroyed(t, client)
}

func TestConn_clientHandshake(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("curve25519.ScalarBaseMult", func(t *testing.T) {
		patch := func([]byte) ([]byte, error) {
			return nil, monkey.Error
		}
		pg := monkey.Patch(curve25519.ScalarBaseMult, patch)
		defer pg.Unpatch()
		err := new(Conn).clientHandshake()
		monkey.IsMonkeyError(t, err)
	})

	t.Run("invalid padding size", func(t *testing.T) {
		testConnClientHandshake(t, func(t *testing.T, server *Conn) {
			_, err := server.Conn.Write(convert.BEUint16ToBytes(1))
			require.NoError(t, err)
		}, ErrInvalidPaddingSize)
	})

	t.Run("failed to receive padding data", func(t *testing.T) {
		testConnClientHandshake(t, func(t *testing.T, server *Conn) {
			data := convert.BEUint16ToBytes(paddingMinSize + 10)
			data = append(data, 1)

			_, err := server.Conn.Write(data)
			require.NoError(t, err)

			err = server.Close()
			require.NoError(t, err)
		}, io.ErrUnexpectedEOF)
	})

	sendPaddingData := func(server *Conn) {
		const paddingSize = paddingMinSize + 10
		size := convert.BEUint16ToBytes(paddingSize)
		paddingData := bytes.Repeat([]byte{1}, paddingSize)

		_, err := server.Conn.Write(append(size, paddingData...))
		require.NoError(t, err)
	}

	t.Run("failed to receive server curve25519 out", func(t *testing.T) {
		testConnClientHandshake(t, func(t *testing.T, server *Conn) {
			sendPaddingData(server)
			_, err := server.Conn.Write([]byte{1})
			require.NoError(t, err)

			err = server.Close()
			require.NoError(t, err)
		}, io.ErrUnexpectedEOF)
	})

	sendCurve25519Out := func(server *Conn) {
		_, err := server.Conn.Write(bytes.Repeat([]byte{1}, curve25519.ScalarSize))
		require.NoError(t, err)
	}

	t.Run("failed to calculate AES key", func(t *testing.T) {
		testConnClientHandshake(t, func(t *testing.T, server *Conn) {
			sendPaddingData(server)

			// must here, curve25519.ScalarBaseMult call curve25519.ScalarMult
			patch := func([]byte, []byte) ([]byte, error) {
				return nil, monkey.Error
			}
			pg := monkey.Patch(curve25519.ScalarMult, patch)
			defer pg.Unpatch()

			sendCurve25519Out(server)

			// must sleep for wait client Read
			time.Sleep(100 * time.Millisecond)
		}, monkey.Error)
	})

	t.Run("failed to receive encrypted password", func(t *testing.T) {
		testConnClientHandshake(t, func(t *testing.T, server *Conn) {
			sendPaddingData(server)
			sendCurve25519Out(server)

			// failed to send encrypted password
			_, err := server.Conn.Write(bytes.Repeat([]byte{1}, 1))
			require.NoError(t, err)

			err = server.Close()
			require.NoError(t, err)
		}, io.ErrUnexpectedEOF)
	})

	sendInvalidEncryptedPassword := func(server *Conn) {
		password := bytes.Repeat([]byte{1}, 256)
		password = append(password, bytes.Repeat([]byte{0}, aes.BlockSize)...)
		_, err := server.Conn.Write(password)
		require.NoError(t, err)
	}

	t.Run("failed to decrypt password", func(t *testing.T) {
		testConnClientHandshake(t, func(t *testing.T, server *Conn) {
			sendPaddingData(server)
			sendCurve25519Out(server)

			patch := func([]byte, []byte, []byte) ([]byte, error) {
				return nil, monkey.Error
			}
			pg := monkey.Patch(aes.CBCDecrypt, patch)
			defer pg.Unpatch()

			sendInvalidEncryptedPassword(server)

			// must sleep for wait client Read
			time.Sleep(100 * time.Millisecond)
		}, monkey.Error)
	})

	t.Run("invalid password size", func(t *testing.T) {
		testConnClientHandshake(t, func(t *testing.T, server *Conn) {
			sendPaddingData(server)
			sendCurve25519Out(server)
			sendInvalidEncryptedPassword(server)
		}, ErrInvalidPasswordSize)
	})
}

func testConnServerHandshake(t *testing.T, f func(t *testing.T, client *Conn), expected error) {
	server, client := testGenerateConnPair()

	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(ioutil.Discard, client.Conn)
	}()
	go func() {
		defer wg.Done()
		f(t, client)
	}()
	err := server.serverHandshake()
	if expected != nil {
		require.Equal(t, expected, err)
	} else {
		require.Error(t, err)
	}

	err = server.Close()
	require.NoError(t, err)
	err = client.Close()
	require.NoError(t, err)

	wg.Wait()

	testsuite.IsDestroyed(t, server)
	testsuite.IsDestroyed(t, client)
}

func TestConn_serverHandshake(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("invalid padding size", func(t *testing.T) {
		testConnServerHandshake(t, func(t *testing.T, client *Conn) {
			_, err := client.Conn.Write(convert.BEUint16ToBytes(1))
			require.NoError(t, err)
		}, ErrInvalidPaddingSize)
	})

	t.Run("failed to receive padding data", func(t *testing.T) {
		testConnServerHandshake(t, func(t *testing.T, client *Conn) {
			data := convert.BEUint16ToBytes(paddingMinSize + 10)
			data = append(data, 1)

			_, err := client.Conn.Write(data)
			require.NoError(t, err)

			err = client.Close()
			require.NoError(t, err)
		}, io.ErrUnexpectedEOF)
	})

	sendPaddingData := func(client *Conn) {
		const paddingSize = paddingMinSize + 10
		size := convert.BEUint16ToBytes(paddingSize)
		paddingData := bytes.Repeat([]byte{1}, paddingSize)
		_, err := client.Conn.Write(append(size, paddingData...))
		require.NoError(t, err)
	}

	t.Run("failed to receive client public key", func(t *testing.T) {
		testConnServerHandshake(t, func(t *testing.T, client *Conn) {
			sendPaddingData(client)
			_, err := client.Conn.Write([]byte{1})
			require.NoError(t, err)

			err = client.Close()
			require.NoError(t, err)
		}, io.ErrUnexpectedEOF)
	})

	sendCurve25519Out := func(server *Conn) {
		_, err := server.Conn.Write(bytes.Repeat([]byte{1}, curve25519.ScalarSize))
		require.NoError(t, err)
	}

	t.Run("curve25519.ScalarBaseMult", func(t *testing.T) {
		testConnServerHandshake(t, func(t *testing.T, client *Conn) {
			sendPaddingData(client)

			patch := func([]byte) ([]byte, error) {
				return nil, monkey.Error
			}
			pg := monkey.Patch(curve25519.ScalarBaseMult, patch)
			defer pg.Unpatch()

			sendCurve25519Out(client)

			// must sleep for wait server Read
			time.Sleep(100 * time.Millisecond)
		}, monkey.Error)
	})

	t.Run("curve25519.ScalarMult", func(t *testing.T) {
		testConnServerHandshake(t, func(t *testing.T, client *Conn) {
			sendPaddingData(client)

			var (
				pg *monkey.PatchGuard

				// must use mutex, because another goroutine
				// execute curve25519.ScalarBaseMult
				ipg *monkey.PatchGuard
				mu  sync.Mutex
			)
			patch := func(in []byte) ([]byte, error) {
				pg.Unpatch()
				out, err := curve25519.ScalarBaseMult(in)

				// patch after curve25519.ScalarBaseMult
				patch := func([]byte, []byte) ([]byte, error) {
					return nil, monkey.Error
				}
				mu.Lock()
				defer mu.Unlock()
				ipg = monkey.Patch(curve25519.ScalarMult, patch)

				return out, err
			}
			pg = monkey.Patch(curve25519.ScalarBaseMult, patch)
			defer func() {
				mu.Lock()
				defer mu.Unlock()
				if ipg != nil {
					ipg.Unpatch()
				}
			}()

			sendCurve25519Out(client)

			// must sleep for wait server Read
			time.Sleep(100 * time.Millisecond)
		}, monkey.Error)
	})

	t.Run("failed to encrypt password", func(t *testing.T) {
		testConnServerHandshake(t, func(t *testing.T, client *Conn) {
			sendPaddingData(client)

			patch := func([]byte, []byte, []byte) ([]byte, error) {
				return nil, monkey.Error
			}
			pg := monkey.Patch(aes.CBCEncrypt, patch)
			defer pg.Unpatch()

			sendCurve25519Out(client)

			// must sleep for wait server Read
			time.Sleep(100 * time.Millisecond)
		}, monkey.Error)
	})
}
