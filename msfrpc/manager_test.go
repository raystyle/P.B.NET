package msfrpc

import (
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/testsuite"
)

func TestIOReader_Read(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	r, w := io.Pipe()
	onRead := func() {}
	reader := newIOReader(r, logger.Test, onRead)

	testdata := testsuite.Bytes()
	_, err := w.Write(testdata)
	require.NoError(t, err)

	// wait read
	time.Sleep(100 * time.Millisecond)

	t.Run("common", func(t *testing.T) {
		output := reader.Read(0)
		require.Equal(t, testdata, output)
	})

	t.Run("start < 0", func(t *testing.T) {
		output := reader.Read(-1)
		require.Equal(t, testdata, output)
	})

	t.Run("start != 0", func(t *testing.T) {
		output := reader.Read(10)
		require.Equal(t, testdata[10:], output)
	})

	t.Run("start > len", func(t *testing.T) {
		output := reader.Read(257)
		require.Nil(t, output)
	})

	err = reader.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, reader)
}

func TestIOReader_Clean(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	r, w := io.Pipe()
	onRead := func() {}
	reader := newIOReader(r, logger.Test, onRead)

	testdata := testsuite.Bytes()
	_, err := w.Write(testdata)
	require.NoError(t, err)

	reader.Clean()

	output := reader.Read(257)
	require.Nil(t, output)

	err = reader.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, reader)
}

func TestIOReader_Panic(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	conn := testsuite.NewMockConnWithReadPanic()
	onRead := func() {}
	reader := newIOReader(conn, logger.Test, onRead)

	time.Sleep(time.Second)

	err := reader.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, reader)
}

func TestIOReader_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	testdata := testsuite.Bytes()

	t.Run("without close", func(t *testing.T) {
		t.Run("part", func(t *testing.T) {
			r, w := io.Pipe()
			onRead := func() {}
			reader := newIOReader(r, logger.Test, onRead)

			write := func() {
				_, err := w.Write(testdata)
				require.NoError(t, err)
			}
			read := func() {
				for i := 0; i < 100; i++ {
					output := reader.Read(i)
					if len(output) == 0 {
						return
					}
					require.Equal(t, testdata[i:], output)
				}
			}
			clean := func() {
				reader.Clean()
			}
			testsuite.RunParallel(100, nil, nil, write, read, clean)

			err := reader.Close()
			require.NoError(t, err)

			testsuite.IsDestroyed(t, reader)
		})

		t.Run("whole", func(t *testing.T) {

		})
	})

	t.Run("with close", func(t *testing.T) {
		t.Run("part", func(t *testing.T) {

		})

		t.Run("whole", func(t *testing.T) {

		})
	})

	r, w := io.Pipe()
	onRead := func() {}
	reader := newIOReader(r, logger.Test, onRead)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			_, err := w.Write(testsuite.Bytes())
			require.NoError(t, err)
		}

		err := reader.Close()
		require.NoError(t, err)
	}()

	time.Sleep(time.Millisecond)

	wg.Add(1)
	go func() {
		defer wg.Done()
		reader.Clean()
	}()

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				data := reader.Read(i)
				if len(data) == 0 {
					return
				}
				data[0] = 1
			}
		}()
	}

	wg.Wait()

	testsuite.IsDestroyed(t, reader)
}
