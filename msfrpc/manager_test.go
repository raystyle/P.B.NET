package msfrpc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/patch/monkey"
	"project/internal/testsuite"
)

func TestIOReader_Read(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	r, w := io.Pipe()
	var (
		read    bool
		cleaned bool
		closed  bool
	)
	onRead := func() { read = true }
	onClean := func() { cleaned = true }
	onClose := func() { closed = true }
	reader := newIOReader(logger.Test, r, onRead, onClean, onClose)
	reader.ReadLoop()

	testdata := testsuite.Bytes()
	_, err := w.Write(testdata)
	require.NoError(t, err)

	// wait read
	time.Sleep(100 * time.Millisecond)

	t.Run("offset = 0", func(t *testing.T) {
		output := reader.Read(0)
		require.Equal(t, testdata, output)
	})

	t.Run("offset < 0", func(t *testing.T) {
		output := reader.Read(-1)
		require.Equal(t, testdata, output)
	})

	t.Run("offset != 0", func(t *testing.T) {
		output := reader.Read(10)
		require.Equal(t, testdata[10:], output)
	})

	t.Run("offset > len", func(t *testing.T) {
		output := reader.Read(257)
		require.Nil(t, output)
	})

	t.Run("read big size data", func(t *testing.T) {
		reader.Clean()

		testdata := bytes.Repeat(testsuite.Bytes(), 256)
		_, err := w.Write(testdata)
		require.NoError(t, err)

		offset := 0
		for {
			output := reader.Read(offset)
			l := len(output)
			if l == 0 {
				break
			}
			require.True(t, bytes.Equal(testdata[offset:offset+l], output))
			offset += l
		}
	})

	reader.Clean()
	err = reader.Close()
	require.NoError(t, err)

	require.True(t, read)
	require.True(t, cleaned)
	require.True(t, closed)

	testsuite.IsDestroyed(t, reader)
}

func TestIOReader_Clean(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	r, w := io.Pipe()
	var (
		read    bool
		cleaned bool
		closed  bool
	)
	onRead := func() { read = true }
	onClean := func() { cleaned = true }
	onClose := func() { closed = true }
	reader := newIOReader(logger.Test, r, onRead, onClean, onClose)
	reader.ReadLoop()

	testdata := testsuite.Bytes()
	_, err := w.Write(testdata)
	require.NoError(t, err)

	reader.Clean()

	output := reader.Read(257)
	require.Nil(t, output)

	err = reader.Close()
	require.NoError(t, err)

	require.True(t, read)
	require.True(t, cleaned)
	require.True(t, closed)

	testsuite.IsDestroyed(t, reader)
}

func TestIOReader_Panic(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	conn := testsuite.NewMockConnWithReadPanic()
	var (
		read    bool
		cleaned bool
		closed  bool
	)
	onRead := func() { read = true }
	onClean := func() { cleaned = true }
	onClose := func() { closed = true }
	reader := newIOReader(logger.Test, conn, onRead, onClean, onClose)
	reader.ReadLoop()

	time.Sleep(time.Second)

	reader.Clean()
	err := reader.Close()
	require.NoError(t, err)

	require.False(t, read)
	require.True(t, cleaned)
	require.True(t, closed)

	testsuite.IsDestroyed(t, reader)
}

func TestIOReader_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	testdata := testsuite.Bytes()

	t.Run("without close", func(t *testing.T) {
		t.Run("part", func(t *testing.T) {
			r, w := io.Pipe()
			var (
				bRead   bool
				cleaned bool
				closed  bool
			)
			onRead := func() { bRead = true }
			onClean := func() { cleaned = true }
			onClose := func() { closed = true }
			reader := newIOReader(logger.Test, r, onRead, onClean, onClose)
			reader.ReadLoop()

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
			cleanup := func() {
				// maybe not read finish
				time.Sleep(25 * time.Millisecond)
				reader.Clean()
			}
			testsuite.RunParallel(100, nil, cleanup, write, read, clean)

			err := reader.Close()
			require.NoError(t, err)

			require.True(t, bRead)
			require.True(t, cleaned)
			require.True(t, closed)

			testsuite.IsDestroyed(t, reader)
		})

		t.Run("whole", func(t *testing.T) {
			var (
				r      *io.PipeReader
				w      *io.PipeWriter
				reader *ioReader
			)
			var (
				bRead   bool
				cleaned bool
				closed  bool
			)
			onRead := func() { bRead = true }
			onClean := func() { cleaned = true }
			onClose := func() { closed = true }

			init := func() {
				r, w = io.Pipe()
				reader = newIOReader(logger.Test, r, onRead, onClean, onClose)
				reader.ReadLoop()
			}
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
			cleanup := func() {
				err := reader.Close()
				require.NoError(t, err)

				require.True(t, bRead)
				require.True(t, cleaned)
				require.True(t, closed)
				bRead = false
				cleaned = false
				closed = false
			}
			testsuite.RunParallel(100, init, cleanup, write, read, clean)

			testsuite.IsDestroyed(t, reader)
		})
	})

	t.Run("with close", func(t *testing.T) {
		t.Run("part", func(t *testing.T) {
			r, w := io.Pipe()
			var (
				bRead   bool
				cleaned bool
				closed  bool
			)
			onRead := func() { bRead = true }
			onClean := func() { cleaned = true }
			onClose := func() { closed = true }
			reader := newIOReader(logger.Test, r, onRead, onClean, onClose)
			reader.ReadLoop()

			write := func() {
				// writer pipe maybe closed
				_, _ = w.Write(testdata)
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
			close1 := func() {
				err := reader.Close()
				require.NoError(t, err)
			}
			cleanup := func() {
				reader.Clean()
			}
			testsuite.RunParallel(100, nil, cleanup, write, read, clean, close1)

			require.True(t, bRead)
			require.True(t, cleaned)
			require.True(t, closed)

			testsuite.IsDestroyed(t, reader)
		})

		t.Run("whole", func(t *testing.T) {
			var (
				r      *io.PipeReader
				w      *io.PipeWriter
				reader *ioReader
			)
			var (
				bRead   bool
				cleaned bool
				closed  bool
			)
			onRead := func() { bRead = true }
			onClean := func() { cleaned = true }
			onClose := func() { closed = true }

			init := func() {
				r, w = io.Pipe()
				reader = newIOReader(logger.Test, r, onRead, onClean, onClose)
				reader.ReadLoop()
			}
			write := func() {
				// writer pipe maybe closed
				_, _ = w.Write(testdata)
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
			close1 := func() {
				// maybe not read
				time.Sleep(25 * time.Millisecond)

				err := reader.Close()
				require.NoError(t, err)
			}
			cleanup := func() {
				reader.Clean()

				require.True(t, bRead)
				require.True(t, cleaned)
				require.True(t, closed)
				bRead = false
				cleaned = false
				closed = false
			}
			testsuite.RunParallel(100, init, cleanup, write, read, clean, close1)

			testsuite.IsDestroyed(t, reader)
		})
	})
}

var (
	testIOManagerHandlers = &IOEventHandlers{
		OnConsoleRead:         func(string) {},
		OnConsoleClean:        func(string) {},
		OnConsoleClosed:       func(string) {},
		OnConsoleLocked:       func(string, string) {},
		OnConsoleUnlocked:     func(string, string) {},
		OnShellRead:           func(uint64) {},
		OnShellClean:          func(uint64) {},
		OnShellClosed:         func(uint64) {},
		OnShellLocked:         func(uint64, string) {},
		OnShellUnlocked:       func(uint64, string) {},
		OnMeterpreterRead:     func(uint64) {},
		OnMeterpreterClean:    func(uint64) {},
		OnMeterpreterClosed:   func(uint64) {},
		OnMeterpreterLocked:   func(uint64, string) {},
		OnMeterpreterUnlocked: func(uint64, string) {},
	}
	testUserToken      = "test-user-token"
	testAnotherToken   = "test-another-token"
	testAdminToken     = "test-admin-token"
	testConsoleCommand = []byte("version\r\n")
)

func testReadDataFromIOObject(obj *IOObject) {
	offset := 0
	for i := 0; i < 10; i++ {
		data := obj.Read(offset)
		l := len(data)
		if l != 0 {
			fmt.Println(string(data))
			offset += l
		}
		time.Sleep(minReadInterval)
	}
}

func TestIOObject(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)

	ctx := context.Background()
	var (
		consoleID string
		read      bool
		cleaned   bool
		closed    bool
		locked    bool
		unlocked  bool
	)
	handlers := IOEventHandlers{
		OnConsoleRead: func(id string) {
			require.Equal(t, consoleID, id)
			read = true
		},
		OnConsoleClean: func(id string) {
			require.Equal(t, consoleID, id)
			cleaned = true
		},
		OnConsoleClosed: func(id string) {
			require.Equal(t, consoleID, id)
			closed = true
		},
		OnConsoleLocked: func(id, token string) {
			require.Equal(t, consoleID, id)
			require.Equal(t, testUserToken, token)
			locked = true
		},
		OnConsoleUnlocked: func(id, token string) {
			require.Equal(t, consoleID, id)
			if token != testUserToken && token != testAdminToken {
				t.Fatal("unexpected token:", token)
			}
			unlocked = true
		},
	}
	manager := NewIOManager(client, &handlers, nil)

	console, err := manager.NewConsole(ctx, defaultWorkspace)
	require.NoError(t, err)
	consoleID = console.ToConsole().id

	t.Run("not locked", func(t *testing.T) {
		err := console.Write(testUserToken, testConsoleCommand)
		require.NoError(t, err)

		testReadDataFromIOObject(console)
	})

	t.Run("lock", func(t *testing.T) {
		ok := console.Lock(testUserToken)
		require.True(t, ok)

		fmt.Println("lock at:", console.LockAt())

		err := console.Write(testUserToken, testConsoleCommand)
		require.NoError(t, err)

		testReadDataFromIOObject(console)

		ok = console.Unlock(testUserToken)
		require.True(t, ok)
	})

	t.Run("write after another lock", func(t *testing.T) {
		ok := console.Lock(testUserToken)
		require.True(t, ok)

		err := console.Write(testUserToken, testConsoleCommand)
		require.NoError(t, err)

		testReadDataFromIOObject(console)

		err = console.Write(testAnotherToken, nil)
		require.Equal(t, ErrAnotherUserLocked, err)

		ok = console.Unlock(testUserToken)
		require.True(t, ok)

		err = console.Write(testAnotherToken, testConsoleCommand)
		require.NoError(t, err)

		testReadDataFromIOObject(console)
	})

	t.Run("lock after another lock", func(t *testing.T) {
		ok := console.Lock(testUserToken)
		require.True(t, ok)

		ok = console.Lock(testAnotherToken)
		require.False(t, ok)

		ok = console.Unlock(testUserToken)
		require.True(t, ok)
	})

	t.Run("unlock without lock", func(t *testing.T) {
		ok := console.Unlock(testUserToken)
		require.True(t, ok)
	})

	t.Run("unlock with another token", func(t *testing.T) {
		ok := console.Lock(testUserToken)
		require.True(t, ok)

		ok = console.Unlock(testAnotherToken)
		require.False(t, ok)

		ok = console.Unlock(testUserToken)
		require.True(t, ok)
	})

	t.Run("force unlock", func(t *testing.T) {
		ok := console.Lock(testUserToken)
		require.True(t, ok)

		console.ForceUnlock(testAdminToken)

		err = console.Write(testAnotherToken, testConsoleCommand)
		require.NoError(t, err)

		testReadDataFromIOObject(console)
	})

	t.Run("Clean", func(t *testing.T) {
		err := console.Write(testUserToken, testConsoleCommand)
		require.NoError(t, err)

		err = console.Clean(testUserToken)
		require.NoError(t, err)

		data := console.Read(0)
		require.Nil(t, data)
	})

	t.Run("clean after lock", func(t *testing.T) {
		ok := console.Lock(testUserToken)
		require.True(t, ok)

		err = console.Clean(testAnotherToken)
		require.Equal(t, ErrAnotherUserLocked, err)

		ok = console.Unlock(testUserToken)
		require.True(t, ok)
	})

	t.Run("Close", func(t *testing.T) {
		ok := console.Lock(testUserToken)
		require.True(t, ok)

		err := console.Close(testUserToken)
		require.NoError(t, err)

		err = console.Close(testAnotherToken)
		require.Equal(t, ErrAnotherUserLocked, err)
	})

	err = manager.Close()
	require.NoError(t, err)

	require.True(t, read)
	require.True(t, cleaned)
	require.True(t, closed)
	require.True(t, locked)
	require.True(t, unlocked)

	err = client.ConsoleDestroy(ctx, consoleID)
	require.NoError(t, err)

	testsuite.IsDestroyed(t, console)
	testsuite.IsDestroyed(t, manager)

	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestIOObject_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)

	ctx := context.Background()
	var (
		consoleID string
		bRead     bool
		cleaned   bool
		closed    bool
		locked    bool
		unlocked  bool
	)
	handlers := IOEventHandlers{
		OnConsoleRead: func(id string) {
			require.Equal(t, consoleID, id)
			bRead = true
		},
		OnConsoleClean: func(id string) {
			require.Equal(t, consoleID, id)
			cleaned = true
		},
		OnConsoleClosed: func(id string) {
			require.Equal(t, consoleID, id)
			closed = true
		},
		OnConsoleLocked: func(id, token string) {
			require.Equal(t, consoleID, id)
			require.Equal(t, testUserToken, token)
			locked = true
		},
		OnConsoleUnlocked: func(id, token string) {
			require.Equal(t, consoleID, id)
			if token != testUserToken && token != testAdminToken {
				t.Fatal("unexpected token:", token)
			}
			unlocked = true
		},
	}
	manager := NewIOManager(client, &handlers, nil)

	t.Run("part", func(t *testing.T) {
		console, err := manager.NewConsole(ctx, defaultWorkspace)
		require.NoError(t, err)
		consoleID = console.ToConsole().id

		lock := func() {
			ok := console.Lock(testUserToken)
			require.True(t, ok)
		}
		unlock := func() {
			ok := console.Unlock(testUserToken)
			require.True(t, ok)
		}
		forceUnlock := func() {
			console.ForceUnlock(testAdminToken)
		}
		read := func() {
			testReadDataFromIOObject(console)
		}
		write := func() {
			// maybe locked
			_ = console.Write(testAnotherToken, testConsoleCommand)
		}
		writeWithLock := func() {
			err := console.Write(testUserToken, testConsoleCommand)
			require.NoError(t, err)
		}
		clean := func() {
			err := console.Clean(testUserToken)
			require.NoError(t, err)
		}
		fns := []func(){
			lock, unlock, forceUnlock,
			read, write, writeWithLock, clean,
		}
		testsuite.RunParallel(5, nil, nil, fns...)

		err = console.Close(testUserToken)
		require.NoError(t, err)

		testsuite.IsDestroyed(t, console)

		err = client.ConsoleDestroy(ctx, consoleID)
		require.NoError(t, err)

		require.True(t, bRead)
		require.True(t, cleaned)
		require.True(t, closed)
		require.True(t, locked)
		require.True(t, unlocked)
	})

	t.Run("whole", func(t *testing.T) {
		var (
			console *IOObject
			err     error
		)

		init := func() {
			console, err = manager.NewConsole(ctx, defaultWorkspace)
			require.NoError(t, err)
			consoleID = console.ToConsole().id
		}
		lock := func() {
			ok := console.Lock(testUserToken)
			require.True(t, ok)
		}
		unlock := func() {
			ok := console.Unlock(testUserToken)
			require.True(t, ok)
		}
		forceUnlock := func() {
			console.ForceUnlock(testAdminToken)
		}
		read := func() {
			testReadDataFromIOObject(console)
		}
		write := func() {
			// maybe locked
			_ = console.Write(testAnotherToken, testConsoleCommand)
		}
		writeWithLock := func() {
			err := console.Write(testUserToken, testConsoleCommand)
			require.NoError(t, err)
		}
		clean := func() {
			err := console.Clean(testUserToken)
			require.NoError(t, err)
		}
		cleanup := func() {
			err = console.Close(testUserToken)
			require.NoError(t, err)

			err = client.ConsoleDestroy(ctx, consoleID)
			require.NoError(t, err)
		}
		fns := []func(){
			lock, unlock, forceUnlock,
			read, write, writeWithLock, clean,
		}
		testsuite.RunParallel(5, init, cleanup, fns...)

		testsuite.IsDestroyed(t, console)

		require.True(t, bRead)
		require.True(t, cleaned)
		require.True(t, closed)
		require.True(t, locked)
		require.True(t, unlocked)
	})

	err := manager.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, manager)

	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestIOManager_Console(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)

	ctx := context.Background()

	manager := NewIOManager(client, testIOManagerHandlers, nil)

	console, err := manager.NewConsole(ctx, defaultWorkspace)
	require.NoError(t, err)
	id := console.ToConsole().ID()

	err = manager.ConsoleWrite(id, testUserToken, testConsoleCommand)
	require.NoError(t, err)
	err = manager.ConsoleWrite(id, testAnotherToken, testConsoleCommand)
	require.NoError(t, err)
	data, err := manager.ConsoleRead(id, 0)
	require.NoError(t, err)
	fmt.Println(string(data))

	err = manager.ConsoleLock(id, testUserToken)
	require.NoError(t, err)
	err = manager.ConsoleWrite(id, testUserToken, testConsoleCommand)
	require.NoError(t, err)
	err = manager.ConsoleWrite(id, testAnotherToken, testConsoleCommand)
	require.Equal(t, ErrAnotherUserLocked, err)
	data, err = manager.ConsoleRead(id, 0)
	require.NoError(t, err)
	fmt.Println(string(data))
	err = manager.ConsoleUnlock(id, testUserToken)
	require.NoError(t, err)
	err = manager.ConsoleWrite(id, testAnotherToken, testConsoleCommand)
	require.NoError(t, err)
	fmt.Println(string(data))

	err = manager.ConsoleLock(id, testUserToken)
	require.NoError(t, err)
	err = manager.ConsoleClean(id, testAnotherToken)
	require.Equal(t, ErrAnotherUserLocked, err)
	err = manager.ConsoleClean(id, testUserToken)
	require.NoError(t, err)
	err = manager.ConsoleUnlock(id, testUserToken)
	require.NoError(t, err)

	err = manager.ConsoleLock(id, testUserToken)
	require.NoError(t, err)
	err = manager.ConsoleForceUnlock(id, testAdminToken)
	require.NoError(t, err)

	err = manager.ConsoleLock(id, testUserToken)
	require.NoError(t, err)
	err = manager.ConsoleClose(id, testAnotherToken)
	require.Equal(t, ErrAnotherUserLocked, err)
	err = manager.ConsoleClose(id, testUserToken)
	require.NoError(t, err)

	err = client.ConsoleDestroy(ctx, id)
	require.NoError(t, err)

	err = manager.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, manager)

	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestIOManager_NewConsole(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)

	ctx := context.Background()

	manager := NewIOManager(client, testIOManagerHandlers, nil)

	t.Run("success", func(t *testing.T) {
		console, err := manager.NewConsoleWithLocker(ctx, defaultWorkspace, testUserToken)
		require.NoError(t, err)
		id := console.ToConsole().ID()

		err = client.ConsoleDestroy(ctx, id)
		require.NoError(t, err)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			console, err := manager.NewConsole(ctx, defaultWorkspace)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, console)
		})
	})

	t.Run("call after close", func(t *testing.T) {
		err := manager.Close()
		require.NoError(t, err)

		console, err := manager.NewConsole(ctx, defaultWorkspace)
		require.Equal(t, ErrIOManagerClosed, err)
		require.Nil(t, console)
	})

	err := manager.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, manager)

	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestIOManager_NewConsoleWithID(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)

	ctx := context.Background()
	console, err := client.NewConsole(ctx, defaultWorkspace, minReadInterval)
	require.NoError(t, err)
	id := console.ID()

	manager := NewIOManager(client, testIOManagerHandlers, nil)

	t.Run("success", func(t *testing.T) {
		console, err := manager.NewConsoleWithIDAndLocker(ctx, id, testUserToken)
		require.NoError(t, err)
		fmt.Println(string(console.Read(0)))

		err = client.ConsoleDestroy(ctx, id)
		require.NoError(t, err)
	})

	t.Run("console is not exist", func(t *testing.T) {
		console, err := manager.NewConsoleWithID(ctx, "999")
		require.EqualError(t, err, "console 999 is not exist")
		require.Nil(t, console)
	})

	t.Run("failed to get console list", func(t *testing.T) {
		testPatchClientSend(func() {
			console, err := manager.NewConsoleWithID(ctx, id)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, console)
		})
	})

	t.Run("call after close", func(t *testing.T) {
		err := manager.Close()
		require.NoError(t, err)

		console, err := manager.NewConsoleWithID(ctx, id)
		require.Equal(t, ErrIOManagerClosed, err)
		require.Nil(t, console)
	})

	t.Run("failed to create io object", func(t *testing.T) {
		err := manager.Close()
		require.NoError(t, err)

		obj, err := manager.createConsoleIOObject(console, testUserToken)
		require.Equal(t, ErrIOManagerClosed, err)
		require.Nil(t, obj)
	})

	err = manager.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, manager)

	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}
