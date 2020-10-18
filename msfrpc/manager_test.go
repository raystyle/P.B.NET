package msfrpc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
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

	testUserToken    = "test-user-token"
	testAnotherToken = "test-another-token"
	testAdminToken   = "test-admin-token"

	testConsoleCommand     = []byte("version\r\n")
	testShellCommand       = []byte("whoami\r\n")
	testMeterpreterCommand = []byte("sysinfo\r\n")
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

func TestIOManager_log(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)

	manager := NewIOManager(client, testIOManagerHandlers, nil)

	manager.log(logger.Debug, "test log")
	manager.logf(logger.Debug, "test %s", "log")

	err := manager.Close()
	require.NoError(t, err)

	manager.log(logger.Debug, "test log")
	manager.logf(logger.Debug, "test %s", "log")

	testsuite.IsDestroyed(t, manager)

	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestIOManager_trackIOObject(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)

	manager := NewIOManager(client, testIOManagerHandlers, nil)

	console := &IOObject{object: new(Console)}
	shell := &IOObject{object: new(Shell)}
	meterpreter := &IOObject{object: new(Meterpreter)}

	// before close
	err := manager.trackConsole(console, true)
	require.NoError(t, err)
	err = manager.trackConsole(console, false)
	require.NoError(t, err)

	err = manager.trackShell(shell, true)
	require.NoError(t, err)
	err = manager.trackShell(shell, false)
	require.NoError(t, err)

	err = manager.trackMeterpreter(meterpreter, true)
	require.NoError(t, err)
	err = manager.trackMeterpreter(meterpreter, false)
	require.NoError(t, err)

	err = manager.Close()
	require.NoError(t, err)

	// after manager closed
	err = manager.trackConsole(console, true)
	require.Equal(t, ErrIOManagerClosed, err)
	err = manager.trackShell(shell, true)
	require.Equal(t, ErrIOManagerClosed, err)
	err = manager.trackMeterpreter(meterpreter, true)
	require.Equal(t, ErrIOManagerClosed, err)

	testsuite.IsDestroyed(t, manager)

	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestIOManager_IOObjects(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)

	manager := NewIOManager(client, testIOManagerHandlers, nil)

	console := &IOObject{object: new(Console)}
	err := manager.trackConsole(console, true)
	require.NoError(t, err)

	shell := &IOObject{object: new(Shell)}
	err = manager.trackShell(shell, true)
	shell.ToShell().id = 1
	err = manager.trackShell(shell, true)
	require.NoError(t, err)

	meterpreter := &IOObject{object: new(Meterpreter)}
	err = manager.trackMeterpreter(meterpreter, true)
	require.NoError(t, err)
	meterpreter.ToMeterpreter().id = 1
	err = manager.trackMeterpreter(meterpreter, true)
	require.NoError(t, err)
	meterpreter.ToMeterpreter().id = 2
	err = manager.trackMeterpreter(meterpreter, true)
	require.NoError(t, err)

	consoles := manager.Consoles()
	require.Len(t, consoles, 1)
	shells := manager.Shells()
	require.Len(t, shells, 2)
	meterpreters := manager.Meterpreters()
	require.Len(t, meterpreters, 3)

	err = manager.trackConsole(console, false)
	require.NoError(t, err)

	err = manager.trackShell(shell, false)
	require.NoError(t, err)
	shell.ToShell().id = 0
	err = manager.trackShell(shell, false)
	require.NoError(t, err)

	err = manager.trackMeterpreter(meterpreter, false)
	require.NoError(t, err)
	meterpreter.ToMeterpreter().id = 1
	err = manager.trackMeterpreter(meterpreter, false)
	require.NoError(t, err)
	meterpreter.ToMeterpreter().id = 0
	err = manager.trackMeterpreter(meterpreter, false)
	require.NoError(t, err)

	err = manager.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, manager)

	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestIOManager_GetIOObject(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)

	manager := NewIOManager(client, testIOManagerHandlers, nil)

	t.Run("console", func(t *testing.T) {
		console, err := manager.GetConsole("-1")
		require.EqualError(t, err, "console -1 is not exist")
		require.Nil(t, console)
	})

	t.Run("shell", func(t *testing.T) {
		shell, err := manager.GetShell(999)
		require.EqualError(t, err, "shell session 999 is not exist")
		require.Nil(t, shell)
	})

	t.Run("meterpreter", func(t *testing.T) {
		meterpreter, err := manager.GetMeterpreter(999)
		require.EqualError(t, err, "meterpreter session 999 is not exist")
		require.Nil(t, meterpreter)
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

	// common write and read
	err = manager.ConsoleWrite(id, testUserToken, testConsoleCommand)
	require.NoError(t, err)
	err = manager.ConsoleWrite(id, testAnotherToken, testConsoleCommand)
	require.NoError(t, err)
	data, err := manager.ConsoleRead(id, 0)
	require.NoError(t, err)
	fmt.Println(string(data))

	// lock and another want write
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

	// clean
	err = manager.ConsoleLock(id, testUserToken)
	require.NoError(t, err)
	err = manager.ConsoleClean(id, testAnotherToken)
	require.Equal(t, ErrAnotherUserLocked, err)
	err = manager.ConsoleClean(id, testUserToken)
	require.NoError(t, err)
	err = manager.ConsoleUnlock(id, testUserToken)
	require.NoError(t, err)

	// force unlock
	err = manager.ConsoleLock(id, testUserToken)
	require.NoError(t, err)
	err = manager.ConsoleForceUnlock(id, testAdminToken)
	require.NoError(t, err)

	// close
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

		fmt.Println(string(console.Read(0)))
		id := console.ToConsole().ID()

		err = manager.ConsoleDestroy(id, testUserToken)
		require.NoError(t, err)

		testsuite.IsDestroyed(t, console)
	})

	t.Run("failed to track", func(t *testing.T) {
		manager := NewIOManager(client, testIOManagerHandlers, nil)

		// simulate already track console(new created)
		for i := 0; i < 999; i++ {
			manager.consoles[strconv.Itoa(i)] = nil
		}

		console, err := manager.NewConsole(ctx, defaultWorkspace)
		require.Error(t, err)
		require.Nil(t, console)

		for i := 0; i < 999; i++ {
			delete(manager.consoles, strconv.Itoa(i))
		}
		err = manager.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, manager)
	})

	t.Run("failed to destroy console", func(t *testing.T) {
		patch := func(interface{}, context.Context, string, time.Duration) (*Console, error) {
			fakeConsole := Console{
				ctx: client,
				id:  "-1",
			}
			return &fakeConsole, nil
		}
		pg := monkey.PatchInstanceMethod(client, "NewConsole", patch)
		defer pg.Unpatch()

		manager := NewIOManager(client, testIOManagerHandlers, nil)

		// simulate already track console(new created)
		manager.consoles["-1"] = nil

		console, err := manager.NewConsole(ctx, defaultWorkspace)
		require.Error(t, err)
		require.Nil(t, console)

		delete(manager.consoles, "-1")
		err = manager.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, manager)
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

	manager := NewIOManager(client, testIOManagerHandlers, nil)

	console, err := client.NewConsole(ctx, defaultWorkspace, minReadInterval)
	require.NoError(t, err)
	id := console.ID()

	t.Run("success", func(t *testing.T) {
		console, err := manager.NewConsoleWithIDAndLocker(ctx, id, testUserToken)
		require.NoError(t, err)

		fmt.Println(string(console.Read(0)))

		err = console.Close(testUserToken)
		require.NoError(t, err)

		testsuite.IsDestroyed(t, console)
	})

	t.Run("console is not exist", func(t *testing.T) {
		console, err := manager.NewConsoleWithID(ctx, "999")
		require.EqualError(t, err, "console 999 is not exist")
		require.Nil(t, console)
	})

	t.Run("already tracked", func(t *testing.T) {
		console, err := manager.NewConsoleWithID(ctx, id)
		require.NoError(t, err)

		_, err = manager.NewConsoleWithID(ctx, id)
		require.EqualError(t, err, fmt.Sprintf("console %s is already being tracked", id))

		err = console.Close("")
		require.NoError(t, err)
	})

	t.Run("failed to close console", func(t *testing.T) {
		var c *Console
		patch := func(interface{}) error {
			return monkey.Error
		}
		pg := monkey.PatchInstanceMethod(c, "Close", patch)
		defer pg.Unpatch()

		console, err := manager.NewConsoleWithID(ctx, id)
		require.NoError(t, err)

		_, err = manager.NewConsoleWithID(ctx, id)
		require.EqualError(t, err, fmt.Sprintf("console %s is already being tracked", id))

		pg.Unpatch()
		err = console.Close("")
		require.NoError(t, err)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			console, err := manager.NewConsoleWithID(ctx, "999")
			monkey.IsMonkeyError(t, err)
			require.Nil(t, console)
		})
	})

	t.Run("call after close", func(t *testing.T) {
		err := manager.Close()
		require.NoError(t, err)

		console, err := manager.NewConsoleWithID(ctx, "999")
		require.Equal(t, ErrIOManagerClosed, err)
		require.Nil(t, console)
	})

	t.Run("failed to create io object", func(t *testing.T) {
		err := manager.Close()
		require.NoError(t, err)

		console, err := manager.createConsoleIOObject(new(Console), testUserToken)
		require.Equal(t, ErrIOManagerClosed, err)
		require.Nil(t, console)
	})

	err = console.Destroy()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, console)

	err = manager.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, manager)

	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestIOManager_Console_Full(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	manager := NewIOManager(client, testIOManagerHandlers, nil)

	console, err := manager.NewConsole(ctx, defaultWorkspace)
	require.NoError(t, err)
	id := console.ToConsole().ID()

	t.Run("not exist", func(t *testing.T) {
		err = manager.ConsoleLock("-1", testUserToken)
		require.Error(t, err)
		err = manager.ConsoleUnlock("-1", testUserToken)
		require.Error(t, err)
		err = manager.ConsoleForceUnlock("-1", testUserToken)
		require.Error(t, err)
		_, err = manager.ConsoleRead("-1", 0)
		require.Error(t, err)
		err = manager.ConsoleWrite("-1", testUserToken, nil)
		require.Error(t, err)
		err = manager.ConsoleClean("-1", testUserToken)
		require.Error(t, err)
		err = manager.ConsoleClose("-1", testUserToken)
		require.Error(t, err)
		err = manager.ConsoleDetach(ctx, "-1", testUserToken)
		require.Error(t, err)
		err = manager.ConsoleInterrupt(ctx, "-1", testUserToken)
		require.Error(t, err)
		err = manager.ConsoleDestroy("-1", testUserToken)
		require.Error(t, err)
	})

	t.Run("lock", func(t *testing.T) {
		err := manager.ConsoleLock(id, testUserToken)
		require.NoError(t, err)
		defer func() {
			err = manager.ConsoleUnlock(id, testUserToken)
			require.NoError(t, err)
		}()

		err = manager.ConsoleLock(id, testUserToken)
		require.NoError(t, err)
		err = manager.ConsoleLock(id, testAnotherToken)
		require.Equal(t, ErrAnotherUserLocked, err)
	})

	t.Run("unlock", func(t *testing.T) {
		err := manager.ConsoleLock(id, testUserToken)
		require.NoError(t, err)
		defer func() {
			err = manager.ConsoleUnlock(id, testUserToken)
			require.NoError(t, err)
		}()

		err = manager.ConsoleUnlock(id, testAnotherToken)
		require.Equal(t, ErrInvalidLockToken, err)
	})

	t.Run("detach", func(t *testing.T) {
		err := manager.ConsoleLock(id, testUserToken)
		require.NoError(t, err)
		defer func() {
			err = manager.ConsoleUnlock(id, testUserToken)
			require.NoError(t, err)
		}()

		err = manager.ConsoleDetach(ctx, id, testUserToken)
		require.NoError(t, err)
		err = manager.ConsoleDetach(ctx, id, testAnotherToken)
		require.Equal(t, ErrAnotherUserLocked, err)
	})

	t.Run("interrupt", func(t *testing.T) {
		err := manager.ConsoleLock(id, testUserToken)
		require.NoError(t, err)
		defer func() {
			err = manager.ConsoleUnlock(id, testUserToken)
			require.NoError(t, err)
		}()

		err = manager.ConsoleInterrupt(ctx, id, testUserToken)
		require.NoError(t, err)
		err = manager.ConsoleInterrupt(ctx, id, testAnotherToken)
		require.Equal(t, ErrAnotherUserLocked, err)
	})

	err = manager.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, manager)

	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestIOManager_Shell(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	manager := NewIOManager(client, testIOManagerHandlers, nil)

	id := testCreateShellSession(t, client, "55600")
	shell, err := manager.NewShell(ctx, id)
	require.NoError(t, err)

	// common write and read
	err = manager.ShellWrite(id, testUserToken, testShellCommand)
	require.NoError(t, err)
	err = manager.ShellWrite(id, testAnotherToken, testShellCommand)
	require.NoError(t, err)
	data, err := manager.ShellRead(id, 0)
	require.NoError(t, err)
	fmt.Println(string(data))

	// lock and another want write
	err = manager.ShellLock(id, testUserToken)
	require.NoError(t, err)
	err = manager.ShellWrite(id, testUserToken, testShellCommand)
	require.NoError(t, err)
	err = manager.ShellWrite(id, testAnotherToken, testShellCommand)
	require.Equal(t, ErrAnotherUserLocked, err)
	data, err = manager.ShellRead(id, 0)
	require.NoError(t, err)
	fmt.Println(string(data))
	err = manager.ShellUnlock(id, testUserToken)
	require.NoError(t, err)
	err = manager.ShellWrite(id, testAnotherToken, testShellCommand)
	require.NoError(t, err)
	fmt.Println(string(data))

	// clean
	err = manager.ShellLock(id, testUserToken)
	require.NoError(t, err)
	err = manager.ShellClean(id, testAnotherToken)
	require.Equal(t, ErrAnotherUserLocked, err)
	err = manager.ShellClean(id, testUserToken)
	require.NoError(t, err)
	err = manager.ShellUnlock(id, testUserToken)
	require.NoError(t, err)

	// force unlock
	err = manager.ShellLock(id, testUserToken)
	require.NoError(t, err)
	err = manager.ShellForceUnlock(id, testAdminToken)
	require.NoError(t, err)

	modules, err := manager.ShellCompatibleModules(ctx, id)
	require.NoError(t, err)
	for _, module := range modules {
		fmt.Println(module)
	}

	// close
	err = manager.ShellLock(id, testUserToken)
	require.NoError(t, err)
	err = manager.ShellClose(id, testAnotherToken)
	require.Equal(t, ErrAnotherUserLocked, err)
	err = manager.ShellClose(id, testUserToken)
	require.NoError(t, err)

	testsuite.IsDestroyed(t, shell)

	err = client.SessionStop(ctx, id)
	require.NoError(t, err)

	err = manager.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, manager)

	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestIOManager_NewShell(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	manager := NewIOManager(client, testIOManagerHandlers, nil)

	id := testCreateShellSession(t, client, "55601")

	t.Run("success", func(t *testing.T) {
		shell, err := manager.NewShellWithLocker(ctx, id, testUserToken)
		require.NoError(t, err)

		fmt.Println(string(shell.Read(0)))

		err = shell.Close(testUserToken)
		require.NoError(t, err)

		testsuite.IsDestroyed(t, shell)
	})

	t.Run("shell session is not exist", func(t *testing.T) {
		shell, err := manager.NewShell(ctx, 999)
		require.EqualError(t, err, "shell session 999 is not exist")
		require.Nil(t, shell)
	})

	t.Run("already tracked", func(t *testing.T) {
		shell, err := manager.NewShell(ctx, id)
		require.NoError(t, err)

		_, err = manager.NewShell(ctx, id)
		require.EqualError(t, err, fmt.Sprintf("shell session %d is already being tracked", id))

		err = shell.Close("")
		require.NoError(t, err)
	})

	t.Run("failed to close shell", func(t *testing.T) {
		var s *Shell
		patch := func(interface{}) error {
			return monkey.Error
		}
		pg := monkey.PatchInstanceMethod(s, "Close", patch)
		defer pg.Unpatch()

		shell, err := manager.NewShell(ctx, id)
		require.NoError(t, err)

		_, err = manager.NewShell(ctx, id)
		require.EqualError(t, err, fmt.Sprintf("shell session %d is already being tracked", id))

		pg.Unpatch()
		err = shell.Close("")
		require.NoError(t, err)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			shell, err := manager.NewShell(ctx, id)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, shell)
		})
	})

	t.Run("call after close", func(t *testing.T) {
		err := manager.Close()
		require.NoError(t, err)

		shell, err := manager.NewShell(ctx, id)
		require.Equal(t, ErrIOManagerClosed, err)
		require.Nil(t, shell)
	})

	err := client.SessionStop(ctx, id)
	require.NoError(t, err)

	err = manager.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, manager)

	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestIOManager_Meterpreter(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	manager := NewIOManager(client, testIOManagerHandlers, nil)

	id := testCreateMeterpreterSession(t, client, "55602")
	meterpreter, err := manager.NewMeterpreter(ctx, id)
	require.NoError(t, err)

	// common write and read
	err = manager.MeterpreterWrite(id, testUserToken, testMeterpreterCommand)
	require.NoError(t, err)
	err = manager.MeterpreterWrite(id, testAnotherToken, testMeterpreterCommand)
	require.NoError(t, err)
	data, err := manager.MeterpreterRead(id, 0)
	require.NoError(t, err)
	fmt.Println(string(data))

	// lock and another want write
	err = manager.MeterpreterLock(id, testUserToken)
	require.NoError(t, err)
	err = manager.MeterpreterWrite(id, testUserToken, testMeterpreterCommand)
	require.NoError(t, err)
	err = manager.MeterpreterWrite(id, testAnotherToken, testMeterpreterCommand)
	require.Equal(t, ErrAnotherUserLocked, err)
	data, err = manager.MeterpreterRead(id, 0)
	require.NoError(t, err)
	fmt.Println(string(data))
	err = manager.MeterpreterUnlock(id, testUserToken)
	require.NoError(t, err)
	err = manager.MeterpreterWrite(id, testAnotherToken, testMeterpreterCommand)
	require.NoError(t, err)
	fmt.Println(string(data))

	// clean
	err = manager.MeterpreterLock(id, testUserToken)
	require.NoError(t, err)
	err = manager.MeterpreterClean(id, testAnotherToken)
	require.Equal(t, ErrAnotherUserLocked, err)
	err = manager.MeterpreterClean(id, testUserToken)
	require.NoError(t, err)
	err = manager.MeterpreterUnlock(id, testUserToken)
	require.NoError(t, err)

	// force unlock
	err = manager.MeterpreterLock(id, testUserToken)
	require.NoError(t, err)
	err = manager.MeterpreterForceUnlock(id, testAdminToken)
	require.NoError(t, err)

	err = manager.MeterpreterRunSingle(ctx, id, testUserToken, string(testMeterpreterCommand))
	require.NoError(t, err)

	modules, err := manager.MeterpreterCompatibleModules(ctx, id)
	require.NoError(t, err)
	for _, module := range modules {
		fmt.Println(module)
	}

	// close
	err = manager.MeterpreterLock(id, testUserToken)
	require.NoError(t, err)
	err = manager.MeterpreterClose(id, testAnotherToken)
	require.Equal(t, ErrAnotherUserLocked, err)
	err = manager.MeterpreterClose(id, testUserToken)
	require.NoError(t, err)

	testsuite.IsDestroyed(t, meterpreter)

	err = client.SessionStop(ctx, id)
	require.NoError(t, err)

	err = manager.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, manager)

	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestIOManager_NewMeterpreter(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	manager := NewIOManager(client, testIOManagerHandlers, nil)

	id := testCreateMeterpreterSession(t, client, "55603")

	t.Run("success", func(t *testing.T) {
		meterpreter, err := manager.NewMeterpreterWithLocker(ctx, id, testUserToken)
		require.NoError(t, err)

		fmt.Println(string(meterpreter.Read(0)))

		err = meterpreter.Close(testUserToken)
		require.NoError(t, err)

		testsuite.IsDestroyed(t, meterpreter)
	})

	t.Run("meterpreter session is not exist", func(t *testing.T) {
		meterpreter, err := manager.NewMeterpreter(ctx, 999)
		require.EqualError(t, err, "meterpreter session 999 is not exist")
		require.Nil(t, meterpreter)
	})

	t.Run("already tracked", func(t *testing.T) {
		meterpreter, err := manager.NewMeterpreter(ctx, id)
		require.NoError(t, err)

		_, err = manager.NewMeterpreter(ctx, id)
		require.EqualError(t, err, fmt.Sprintf("meterpreter session %d is already being tracked", id))

		err = meterpreter.Close("")
		require.NoError(t, err)
	})

	t.Run("failed to close meterpreter", func(t *testing.T) {
		var m *Meterpreter
		patch := func(interface{}) error {
			return monkey.Error
		}
		pg := monkey.PatchInstanceMethod(m, "Close", patch)
		defer pg.Unpatch()

		meterpreter, err := manager.NewMeterpreter(ctx, id)
		require.NoError(t, err)

		_, err = manager.NewMeterpreter(ctx, id)
		require.EqualError(t, err, fmt.Sprintf("meterpreter session %d is already being tracked", id))

		pg.Unpatch()
		err = meterpreter.Close("")
		require.NoError(t, err)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			meterpreter, err := manager.NewMeterpreter(ctx, id)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, meterpreter)
		})
	})

	t.Run("call after close", func(t *testing.T) {
		err := manager.Close()
		require.NoError(t, err)

		meterpreter, err := manager.NewMeterpreter(ctx, id)
		require.Equal(t, ErrIOManagerClosed, err)
		require.Nil(t, meterpreter)
	})

	err := client.SessionStop(ctx, id)
	require.NoError(t, err)

	err = manager.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, manager)

	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}
