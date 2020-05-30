package protocol

import (
	"bytes"
	"fmt"
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/convert"
	"project/internal/testsuite"
)

func TestNewSlots(t *testing.T) {
	DestroySlots(NewSlots())
}

func TestHandleConn(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	frameData := []byte{1, 1, 1, 1}
	bigFrame := bytes.Repeat([]byte{1}, 32768)

	testsuite.PipeWithReaderWriter(t,
		func(server net.Conn) {
			var count int
			HandleConn(server, func(frame []byte) {
				count++
				if count != 5 {
					require.Equal(t, frameData, frame)
				} else {
					require.Equal(t, bigFrame, frame)
				}
			})

			err := server.Close()
			require.NoError(t, err)

			require.Equal(t, 5, count)
		},
		func(client net.Conn) {
			// full
			_, err := client.Write([]byte{0, 0, 0, 4, 1, 1, 1, 1})
			require.NoError(t, err)

			// full with incomplete header
			_, err = client.Write([]byte{0, 0, 0, 4, 1, 1, 1, 1, 0, 0})
			require.NoError(t, err)
			_, err = client.Write([]byte{0, 4, 1, 1, 1, 1})
			require.NoError(t, err)

			// incomplete body
			_, err = client.Write([]byte{0, 0, 0, 4, 1, 1})
			require.NoError(t, err)
			_, err = client.Write([]byte{1, 1})
			require.NoError(t, err)

			// big frame
			_, err = client.Write(convert.Uint32ToBytes(32768))
			require.NoError(t, err)
			_, err = client.Write(bigFrame)
			require.NoError(t, err)

			err = client.Close()
			require.NoError(t, err)
		},
	)
}

func TestHandleNULLFrame(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	testsuite.PipeWithReaderWriter(t,
		func(server net.Conn) {
			HandleConn(server, func(frame []byte) {
				require.Equal(t, ConnErrRecvNullFrame, frame[0])
			})

			err := server.Close()
			require.NoError(t, err)
		},
		func(client net.Conn) {
			_, err := client.Write([]byte{0, 0, 0, 0})
			require.NoError(t, err)

			err = client.Close()
			require.NoError(t, err)
		},
	)
}

func TestHandleTooBigFrame(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	testsuite.PipeWithReaderWriter(t,
		func(server net.Conn) {
			HandleConn(server, func(frame []byte) {
				require.Equal(t, ConnErrRecvTooBigFrame, frame[0])
			})

			err := server.Close()
			require.NoError(t, err)
		},
		func(client net.Conn) {
			_, err := client.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF})
			require.NoError(t, err)

			err = client.Close()
			require.NoError(t, err)
		},
	)
}

func BenchmarkHandleConn(b *testing.B) {
	for _, size := range []int{
		128, 2048, 4096, 32768, 1048576,
	} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			benchmarkHandleConn(b, size)
		})
	}
}

func benchmarkHandleConn(b *testing.B, size int) {
	gm := testsuite.MarkGoroutines(b)
	defer gm.Compare()

	server, client := net.Pipe()
	frameData := bytes.Repeat([]byte{1}, size)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()

		count := 0
		HandleConn(server, func(frame []byte) {
			if !bytes.Equal(frame, frameData) {
				b.Fatal("different frame data:", frame, frameData)
			}
			count++
		})

		err := server.Close()
		require.NoError(b, err)

		require.Equal(b, b.N, count)
	}()

	frame := append(convert.Uint32ToBytes(uint32(size)), frameData...)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := client.Write(frame)
		if err != nil {
			b.Fatal(err)
		}
	}

	b.StopTimer()

	err := client.Close()
	require.NoError(b, err)

	wg.Wait()
}

func BenchmarkHandleConnParallel(b *testing.B) {
	for _, size := range []int{
		128, 2048, 4096, 32768, 1048576,
	} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			benchmarkHandleConnParallel(b, size)
		})
	}
}

func benchmarkHandleConnParallel(b *testing.B, size int) {
	gm := testsuite.MarkGoroutines(b)
	defer gm.Compare()

	server, client := net.Pipe()

	frameData := bytes.Repeat([]byte{1}, size)
	frame := append(convert.Uint32ToBytes(uint32(size)), frameData...)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()

		HandleConn(server, func(frame []byte) {
			if !bytes.Equal(frame, frameData) {
				b.Fatal("different frame data:", frame, frameData)
			}
		})

		err := server.Close()
		require.NoError(b, err)
	}()

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := client.Write(frame)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.StopTimer()

	err := client.Close()
	require.NoError(b, err)

	wg.Wait()
}
