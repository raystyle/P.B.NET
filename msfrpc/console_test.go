package msfrpc

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/module/shellcode"
	"project/internal/patch/monkey"
	"project/internal/testsuite"
)

func TestMSFRPC_ConsoleList(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		consoles, err := msfrpc.ConsoleList(ctx)
		require.NoError(t, err)
		for _, console := range consoles {
			t.Log("id:", console.ID)
			t.Log("prompt:", console.Prompt)
			t.Log("prompt(byte):", []byte(console.Prompt))
			t.Log("busy:", console.Busy)
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		consoles, err := msfrpc.ConsoleList(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, consoles)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			consoles, err := msfrpc.ConsoleList(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, consoles)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_ConsoleCreate(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()
	const workspace = ""

	t.Run("success", func(t *testing.T) {
		result, err := msfrpc.ConsoleCreate(ctx, workspace)
		require.NoError(t, err)
		t.Log("id:", result.ID)
		t.Log("prompt:", result.Prompt)
		t.Log("busy:", result.Busy)

		err = msfrpc.ConsoleDestroy(ctx, result.ID)
		require.NoError(t, err)
	})

	t.Run("with valid workspace", func(t *testing.T) {
		result, err := msfrpc.ConsoleCreate(ctx, "default")
		require.NoError(t, err)
		t.Log("id:", result.ID)
		t.Log("prompt:", result.Prompt)
		t.Log("busy:", result.Busy)

		err = msfrpc.ConsoleDestroy(ctx, result.ID)
		require.NoError(t, err)
	})

	t.Run("with invalid workspace", func(t *testing.T) {
		result, err := msfrpc.ConsoleCreate(ctx, "foo")
		require.NoError(t, err)
		t.Log("id:", result.ID)
		t.Log("prompt:", result.Prompt)
		t.Log("busy:", result.Busy)

		err = msfrpc.ConsoleDestroy(ctx, result.ID)
		require.NoError(t, err)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		result, err := msfrpc.ConsoleCreate(ctx, workspace)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, result)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			result, err := msfrpc.ConsoleCreate(ctx, workspace)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, result)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_ConsoleDestroy(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()
	const workspace = ""

	t.Run("success", func(t *testing.T) {
		result, err := msfrpc.ConsoleCreate(ctx, workspace)
		require.NoError(t, err)

		err = msfrpc.ConsoleDestroy(ctx, result.ID)
		require.NoError(t, err)
	})

	t.Run("invalid console id", func(t *testing.T) {
		err = msfrpc.ConsoleDestroy(ctx, "999")
		require.EqualError(t, err, "invalid console id: 999")
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		err := msfrpc.ConsoleDestroy(ctx, "foo")
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			err := msfrpc.ConsoleDestroy(ctx, "foo")
			monkey.IsMonkeyError(t, err)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_ConsoleRead(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()
	const workspace = ""

	t.Run("success", func(t *testing.T) {
		console, err := msfrpc.ConsoleCreate(ctx, workspace)
		require.NoError(t, err)

		output, err := msfrpc.ConsoleRead(ctx, console.ID)
		require.NoError(t, err)
		t.Log(output.Data)

		err = msfrpc.ConsoleDestroy(ctx, console.ID)
		require.NoError(t, err)
	})

	t.Run("invalid console id", func(t *testing.T) {
		output, err := msfrpc.ConsoleRead(ctx, "999")
		require.EqualError(t, err, "failed to read from console 999: failure")
		require.Nil(t, output)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		output, err := msfrpc.ConsoleRead(ctx, "999")
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, output)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			output, err := msfrpc.ConsoleRead(ctx, "999")
			monkey.IsMonkeyError(t, err)
			require.Nil(t, output)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_ConsoleWrite(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()
	const workspace = ""

	t.Run("success", func(t *testing.T) {
		console, err := msfrpc.ConsoleCreate(ctx, workspace)
		require.NoError(t, err)

		output, err := msfrpc.ConsoleRead(ctx, console.ID)
		require.NoError(t, err)
		t.Log(output.Data)

		const data = "version\r\n"
		n, err := msfrpc.ConsoleWrite(ctx, console.ID, data)
		require.NoError(t, err)
		require.Equal(t, uint64(len(data)), n)

		output, err = msfrpc.ConsoleRead(ctx, console.ID)
		require.NoError(t, err)
		t.Logf("%s\n%s\n", output.Prompt, output.Data)

		err = msfrpc.ConsoleDestroy(ctx, console.ID)
		require.NoError(t, err)
	})

	t.Run("no data", func(t *testing.T) {
		n, err := msfrpc.ConsoleWrite(ctx, "999", "")
		require.NoError(t, err)
		require.Zero(t, n)
	})

	const (
		id   = "999"
		data = "foo"
	)

	t.Run("invalid console id", func(t *testing.T) {
		n, err := msfrpc.ConsoleWrite(ctx, id, data)
		require.EqualError(t, err, "failed to write to console 999: failure")
		require.Equal(t, uint64(0), n)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		n, err := msfrpc.ConsoleWrite(ctx, id, data)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Equal(t, uint64(0), n)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			n, err := msfrpc.ConsoleWrite(ctx, id, data)
			monkey.IsMonkeyError(t, err)
			require.Equal(t, uint64(0), n)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_ConsoleSessionDetach(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()
	const workspace = ""

	t.Run("success", func(t *testing.T) {
		console, err := msfrpc.ConsoleCreate(ctx, workspace)
		require.NoError(t, err)

		output, err := msfrpc.ConsoleRead(ctx, console.ID)
		require.NoError(t, err)
		t.Log(output.Data)

		// detach
		err = msfrpc.ConsoleSessionDetach(ctx, console.ID)
		require.NoError(t, err)
		time.Sleep(time.Second)
		output, err = msfrpc.ConsoleRead(ctx, console.ID)
		require.NoError(t, err)
		t.Logf("%s\n%s\n", output.Prompt, output.Data)

		err = msfrpc.ConsoleDestroy(ctx, console.ID)
		require.NoError(t, err)
	})

	t.Run("invalid console id", func(t *testing.T) {
		err := msfrpc.ConsoleSessionDetach(ctx, "999")
		require.EqualError(t, err, "failed to detach session about console 999: failure")
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		err := msfrpc.ConsoleSessionDetach(ctx, "999")
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			err := msfrpc.ConsoleSessionDetach(ctx, "999")
			monkey.IsMonkeyError(t, err)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_ConsoleSessionKill(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()
	const workspace = ""

	t.Run("success", func(t *testing.T) {
		console, err := msfrpc.ConsoleCreate(ctx, workspace)
		require.NoError(t, err)

		output, err := msfrpc.ConsoleRead(ctx, console.ID)
		require.NoError(t, err)
		t.Log(output.Data)

		// start a handler
		for _, command := range []string{
			"use exploit/multi/handler\r\n",
			"set payload windows/meterpreter/reverse_tcp\r\n",
			"set LHOST 127.0.0.1\r\n",
			"set LPORT 0\r\n",
			"show options\r\n",
			"exploit\r\n",
		} {
			n, err := msfrpc.ConsoleWrite(ctx, console.ID, command)
			require.NoError(t, err)
			require.Equal(t, uint64(len(command)), n)
			// don't wait exploit
			if command == "exploit\r\n" {
				break
			}
			for {
				output, err := msfrpc.ConsoleRead(ctx, console.ID)
				require.NoError(t, err)
				if !output.Busy {
					t.Logf("%s\n%s\n", output.Prompt, output.Data)
					break
				} else if len(output.Data) != 0 {
					t.Logf("%s", output.Data)
				}
				time.Sleep(100 * time.Millisecond)
			}
		}

		// kill
		err = msfrpc.ConsoleSessionKill(ctx, console.ID)
		require.NoError(t, err)
		time.Sleep(time.Second)
		output, err = msfrpc.ConsoleRead(ctx, console.ID)
		require.NoError(t, err)
		t.Logf("%s\n%s\n", output.Prompt, output.Data)

		err = msfrpc.ConsoleDestroy(ctx, console.ID)
		require.NoError(t, err)
	})

	t.Run("invalid console id", func(t *testing.T) {
		err := msfrpc.ConsoleSessionKill(ctx, "999")
		require.EqualError(t, err, "failed to kill session about console 999: failure")
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		err := msfrpc.ConsoleSessionKill(ctx, "999")
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			err := msfrpc.ConsoleSessionKill(ctx, "999")
			monkey.IsMonkeyError(t, err)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestConsole(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	const (
		workspace = ""
		interval  = 25 * time.Millisecond
	)

	console, err := msfrpc.NewConsole(ctx, workspace, interval)
	require.NoError(t, err)

	go func() { _, _ = io.Copy(os.Stdout, console) }()

	for _, command := range []string{
		"version\r\n",
		"use exploit/multi/handler\r\n",
		"set payload windows/meterpreter/reverse_tcp\r\n",
		"set LHOST 127.0.0.1\r\n",
		"set LPORT 0\r\n",
		"show options\r\n",
		"exploit\r\n",
	} {
		_, err = console.Write([]byte(command))
		require.NoError(t, err)
	}

	time.Sleep(time.Second)

	err = console.Interrupt(ctx)
	require.NoError(t, err)

	time.Sleep(time.Second)

	// print new line
	fmt.Println()
	fmt.Println()

	err = console.Close()
	require.NoError(t, err)
	testsuite.IsDestroyed(t, console)

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_NewConsole(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)

	// not login
	console, err := msfrpc.NewConsole(context.Background(), "", 0)
	require.Error(t, err)
	require.Nil(t, console)

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestConsole_readLoop(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	const (
		workspace = ""
		interval  = 25 * time.Millisecond
	)

	t.Run("after msfrpc closed", func(t *testing.T) {
		atomic.StoreInt32(&msfrpc.inShutdown, 1)
		defer atomic.StoreInt32(&msfrpc.inShutdown, 0)

		console, err := msfrpc.NewConsole(ctx, workspace, 0)
		require.NoError(t, err)

		// wait close self
		time.Sleep(time.Second)

		_ = console.Close()

		testsuite.IsDestroyed(t, console)
	})

	t.Run("failed to read", func(t *testing.T) {
		console, err := msfrpc.NewConsole(ctx, workspace, interval)
		require.NoError(t, err)

		time.Sleep(time.Second)

		err = console.Close()
		require.NoError(t, err)
		testsuite.IsDestroyed(t, console)
	})

	t.Run("panic", func(t *testing.T) {
		_, w := io.Pipe()
		defer func() { _ = w.Close() }()

		patchFunc := func(interface{}) bool {
			panic(monkey.Panic)
		}
		pg := monkey.PatchInstanceMethod(w, "Write", patchFunc)
		defer pg.Unpatch()

		console, err := msfrpc.NewConsole(ctx, workspace, interval)
		require.NoError(t, err)

		time.Sleep(time.Second)

		err = console.Close()
		require.NoError(t, err)
		testsuite.IsDestroyed(t, console)
	})

	t.Run("auto close", func(t *testing.T) {
		console, err := msfrpc.NewConsole(ctx, workspace, interval)
		require.NoError(t, err)

		// wait self add
		time.Sleep(time.Second)

		err = msfrpc.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, console)
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestConsole_read(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	const (
		workspace = ""
		interval  = 25 * time.Millisecond
	)

	t.Run("cancel in busy", func(t *testing.T) {
		console, err := msfrpc.NewConsole(ctx, workspace, interval)
		require.NoError(t, err)

		go func() { _, _ = io.Copy(os.Stdout, console) }()

		_, err = console.Write([]byte("use exploit/multi/handler\r\n"))
		require.NoError(t, err)

		time.Sleep(3 * minReadInterval)

		err = console.Close()
		require.NoError(t, err)
		testsuite.IsDestroyed(t, console)
	})

	t.Run("failed to write in busy", func(t *testing.T) {
		console, err := msfrpc.NewConsole(ctx, workspace, interval)
		require.NoError(t, err)

		go func() { _, _ = io.Copy(os.Stdout, console) }()

		for _, command := range []string{
			"version\r\n",
			"use exploit/multi/handler\r\n",
			"set payload windows/meterpreter/reverse_tcp\r\n",
			"set LHOST 127.0.0.1\r\n",
			"set LPORT 0\r\n",
			"show options\r\n",
		} {
			_, err = console.Write([]byte(command))
			require.NoError(t, err)
		}

		time.Sleep(time.Second)

		// failed to write output
		err = console.pw.Close()
		require.NoError(t, err)

		// also failed
		_, err = console.Write([]byte("exploit\r\n"))
		require.Error(t, err)

		time.Sleep(time.Second)

		err = console.Interrupt(ctx)
		require.NoError(t, err)

		time.Sleep(time.Second)

		err = console.Close()
		require.NoError(t, err)
		testsuite.IsDestroyed(t, console)
	})

	t.Run("failed to write last in busy", func(t *testing.T) {
		console, err := msfrpc.NewConsole(ctx, workspace, interval)
		require.NoError(t, err)

		go func() { _, _ = io.Copy(os.Stdout, console) }()

		for _, command := range []string{
			"version\r\n",
			"use exploit/multi/handler\r\n",
			"set payload windows/meterpreter/reverse_tcp\r\n",
			"set LHOST 127.0.0.1\r\n",
			"set LPORT 0\r\n",
			"show options\r\n",
			"exploit\r\n",
		} {
			_, err = console.Write([]byte(command))
			require.NoError(t, err)
		}

		time.Sleep(time.Second)

		// failed to write output
		err = console.pw.Close()
		require.NoError(t, err)

		time.Sleep(time.Second)

		// maybe lost output, interrupt 3 times
		for i := 0; i < 3; i++ {
			err = console.Interrupt(ctx)
			require.NoError(t, err)
			time.Sleep(time.Second)
		}

		err = console.Close()
		require.NoError(t, err)
		testsuite.IsDestroyed(t, console)
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestConsole_writeLimiter(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)

	// force set for prevent net/http call time.Reset()
	msfrpc.client.Transport.(*http.Transport).IdleConnTimeout = 0

	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	const (
		workspace = ""
		interval  = 25 * time.Millisecond
	)

	t.Run("cancel", func(t *testing.T) {
		console, err := msfrpc.NewConsole(ctx, workspace, interval)
		require.NoError(t, err)

		time.Sleep(minReadInterval)

		err = console.Close()
		require.NoError(t, err)
		testsuite.IsDestroyed(t, console)
	})

	t.Run("panic", func(t *testing.T) {
		timer := time.NewTimer(time.Second)
		defer timer.Stop()

		patchFunc := func(interface{}, time.Duration) bool {
			panic(monkey.Panic)
		}
		pg := monkey.PatchInstanceMethod(timer, "Reset", patchFunc)
		defer pg.Unpatch()

		console, err := msfrpc.NewConsole(ctx, workspace, interval)
		require.NoError(t, err)

		time.Sleep(time.Second)

		select {
		case <-console.token:
		case <-time.After(time.Second):
		}

		// prevent select context
		time.Sleep(time.Second)

		pg.Unpatch()

		err = console.Close()
		require.NoError(t, err)
		testsuite.IsDestroyed(t, console)
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestConsole_Write(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	const (
		workspace = ""
		interval  = 25 * time.Millisecond
	)

	t.Run("block before mutex", func(t *testing.T) {
		console, err := msfrpc.NewConsole(ctx, workspace, interval)
		require.NoError(t, err)

		go func() { _, _ = io.Copy(os.Stdout, console) }()

		err = console.Close()
		require.NoError(t, err)

		_, err = console.Write([]byte("version"))
		require.Equal(t, context.Canceled, err)

		err = console.Close()
		require.NoError(t, err)
		testsuite.IsDestroyed(t, console)
	})

	t.Run("block after mutex", func(t *testing.T) {
		console, err := msfrpc.NewConsole(ctx, workspace, interval)
		require.NoError(t, err)

		go func() { _, _ = io.Copy(os.Stdout, console) }()

		go func() {
			time.Sleep(3 * minReadInterval)
			err := console.Close()
			require.NoError(t, err)
		}()

		_, err = console.Write([]byte("version"))
		require.Equal(t, context.Canceled, err)

		err = console.Close()
		require.NoError(t, err)
		testsuite.IsDestroyed(t, console)
	})

	t.Run("failed to write", func(t *testing.T) {
		console, err := msfrpc.NewConsole(ctx, workspace, interval)
		require.NoError(t, err)

		go func() { _, _ = io.Copy(os.Stdout, console) }()

		time.Sleep(time.Second)
		err = msfrpc.AuthLogout(msfrpc.GetToken())
		require.NoError(t, err)

		_, err = console.Write([]byte("version"))
		require.Error(t, err)

		err = msfrpc.AuthLogin()
		require.NoError(t, err)

		err = console.Close()
		require.NoError(t, err)
		testsuite.IsDestroyed(t, console)
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestConsole_Detach(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	const (
		workspace = ""
		interval  = 25 * time.Millisecond
	)

	console, err := msfrpc.NewConsole(ctx, workspace, interval)
	require.NoError(t, err)

	go func() { _, _ = io.Copy(os.Stdout, console) }()

	// select payload
	var payload string
	switch runtime.GOOS {
	case "windows":
		switch runtime.GOARCH {
		case "386":
			payload = "windows/meterpreter/reverse_tcp"
		case "amd64":
			payload = "windows/x64/meterpreter/reverse_tcp"
		default:
			t.Skip("only support 386 and amd64")
		}
	case "linux":
		switch runtime.GOARCH {
		case "386":
			payload = "linux/meterpreter/reverse_tcp"
		case "amd64":
			payload = "linux/x64/meterpreter/reverse_tcp"
		default:
			t.Skip("only support 386 and amd64")
		}
	default:
		t.Skip("only support windows and linux")
	}

	for _, command := range []string{
		"version\r\n",
		"use exploit/multi/handler\r\n",
		"set payload " + payload + "\r\n",
		"set LHOST 127.0.0.1\r\n",
		"set LPORT 55200\r\n",
		"set EXITFUNC thread\r\n",
		"show options\r\n",
		"exploit -z\r\n",
	} {
		_, err = console.Write([]byte(command))
		require.NoError(t, err)
	}

	// generate payload and execute shellcode
	payloadOpts := NewModuleExecuteOptions()
	payloadOpts.Format = "raw"
	payloadOpts.DataStore["EXITFUNC"] = "thread"
	payloadOpts.DataStore["LHOST"] = "127.0.0.1"
	payloadOpts.DataStore["LPORT"] = 55200
	pResult, err := msfrpc.ModuleExecute(ctx, "payload", payload, payloadOpts)
	require.NoError(t, err)
	sc := []byte(pResult.Payload)
	// execute shellcode and wait some time
	go func() { _ = shellcode.Execute("", sc) }()
	time.Sleep(8 * time.Second)

	for _, command := range []string{
		"sessions\r\n",
	} {
		_, err = console.Write([]byte(command))
		require.NoError(t, err)
	}

	time.Sleep(time.Second)

	err = console.Detach(ctx)
	require.NoError(t, err)

	time.Sleep(time.Second)

	// print new line
	fmt.Println()
	fmt.Println()

	err = console.Close()
	require.NoError(t, err)
	testsuite.IsDestroyed(t, console)

	// kill session
	sessions, err := msfrpc.SessionList(ctx)
	require.NoError(t, err)
	for id, session := range sessions {
		const format = "id: %d type: %s remote: %s\n"
		t.Logf(format, id, session.Type, session.TunnelPeer)

		err = msfrpc.SessionStop(ctx, id)
		require.NoError(t, err)
	}

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestConsole_Close(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = msfrpc.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	const (
		workspace = ""
		interval  = 25 * time.Millisecond
	)

	t.Run("twice", func(t *testing.T) {
		console, err := msfrpc.NewConsole(ctx, workspace, interval)
		require.NoError(t, err)

		err = console.Close()
		require.NoError(t, err)
		err = console.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, console)
	})

	t.Run("failed to destroy", func(t *testing.T) {
		console, err := msfrpc.NewConsole(ctx, workspace, interval)
		require.NoError(t, err)

		err = msfrpc.AuthLogout(msfrpc.GetToken())
		require.NoError(t, err)

		err = console.Close()
		require.Error(t, err)

		err = console.Close()
		require.Error(t, err)

		// login and destroy
		err = msfrpc.AuthLogin()
		require.NoError(t, err)

		err = console.Close()
		require.NoError(t, err)
		testsuite.IsDestroyed(t, console)
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}
