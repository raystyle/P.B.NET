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

func TestParallelReader_Bytes(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	r, w := io.Pipe()
	onRead := func() {}
	pr := newParallelReader(r, logger.Test, onRead)

	testdata := testsuite.Bytes()
	_, err := w.Write(testdata)
	require.NoError(t, err)

	// wait read
	time.Sleep(100 * time.Millisecond)

	t.Run("common", func(t *testing.T) {
		output := pr.Bytes(0)
		require.Equal(t, testdata, output)
	})

	t.Run("start < 0", func(t *testing.T) {
		output := pr.Bytes(-1)
		require.Equal(t, testdata, output)
	})

	t.Run("start != 0", func(t *testing.T) {
		output := pr.Bytes(10)
		require.Equal(t, testdata[10:], output)
	})

	t.Run("start > len", func(t *testing.T) {
		output := pr.Bytes(257)
		require.Nil(t, output)
	})

	err = pr.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, pr)
}

func TestParallelReader_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	r, w := io.Pipe()
	onRead := func() {}
	pr := newParallelReader(r, logger.Test, onRead)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			_, err := w.Write(testsuite.Bytes())
			require.NoError(t, err)
		}

		err := pr.Close()
		require.NoError(t, err)
	}()

	time.Sleep(time.Millisecond)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				data := pr.Bytes(i)
				if len(data) == 0 {
					return
				}
				data[0] = 1
			}
		}()
	}

	wg.Wait()

	testsuite.IsDestroyed(t, pr)
}

func TestParallelReader_Panic(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	reader := testsuite.NewMockConnWithReadPanic()
	onRead := func() {}
	pr := newParallelReader(reader, logger.Test, onRead)

	time.Sleep(time.Second)

	err := pr.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, pr)
}
