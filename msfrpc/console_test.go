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

	"project/internal/module/shellcode"
	"project/internal/patch/monkey"
	"project/internal/testsuite"
)

func TestClient_ConsoleList(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		consoles, err := client.ConsoleList(ctx)
		require.NoError(t, err)
		for _, console := range consoles {
			t.Log("id:", console.ID)
			t.Log("prompt:", console.Prompt)
			t.Log("prompt(byte):", []byte(console.Prompt))
			t.Log("busy:", console.Busy)
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		consoles, err := client.ConsoleList(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, consoles)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			consoles, err := client.ConsoleList(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, consoles)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_ConsoleCreate(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()
	const workspace = ""

	t.Run("success", func(t *testing.T) {
		result, err := client.ConsoleCreate(ctx, workspace)
		require.NoError(t, err)
		t.Log("id:", result.ID)
		t.Log("prompt:", result.Prompt)
		t.Log("busy:", result.Busy)

		err = client.ConsoleDestroy(ctx, result.ID)
		require.NoError(t, err)
	})

	t.Run("with valid workspace", func(t *testing.T) {
		result, err := client.ConsoleCreate(ctx, "default")
		require.NoError(t, err)
		t.Log("id:", result.ID)
		t.Log("prompt:", result.Prompt)
		t.Log("busy:", result.Busy)

		err = client.ConsoleDestroy(ctx, result.ID)
		require.NoError(t, err)
	})

	t.Run("with invalid workspace", func(t *testing.T) {
		result, err := client.ConsoleCreate(ctx, "foo")
		require.NoError(t, err)
		t.Log("id:", result.ID)
		t.Log("prompt:", result.Prompt)
		t.Log("busy:", result.Busy)

		err = client.ConsoleDestroy(ctx, result.ID)
		require.NoError(t, err)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		result, err := client.ConsoleCreate(ctx, workspace)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, result)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			result, err := client.ConsoleCreate(ctx, workspace)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, result)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_ConsoleDestroy(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()
	const workspace = ""

	t.Run("success", func(t *testing.T) {
		result, err := client.ConsoleCreate(ctx, workspace)
		require.NoError(t, err)

		err = client.ConsoleDestroy(ctx, result.ID)
		require.NoError(t, err)
	})

	t.Run("invalid console id", func(t *testing.T) {
		err := client.ConsoleDestroy(ctx, "999")
		require.EqualError(t, err, "invalid console id: 999")
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		err := client.ConsoleDestroy(ctx, "foo")
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			err := client.ConsoleDestroy(ctx, "foo")
			monkey.IsMonkeyError(t, err)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_ConsoleRead(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()
	const workspace = ""

	t.Run("success", func(t *testing.T) {
		console, err := client.ConsoleCreate(ctx, workspace)
		require.NoError(t, err)

		output, err := client.ConsoleRead(ctx, console.ID)
		require.NoError(t, err)
		t.Log(output.Data)

		err = client.ConsoleDestroy(ctx, console.ID)
		require.NoError(t, err)
	})

	t.Run("invalid console id", func(t *testing.T) {
		const errStr = "failed to read from console 999: failure"
		output, err := client.ConsoleRead(ctx, "999")
		require.EqualError(t, err, errStr)
		require.Nil(t, output)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		output, err := client.ConsoleRead(ctx, "999")
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, output)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			output, err := client.ConsoleRead(ctx, "999")
			monkey.IsMonkeyError(t, err)
			require.Nil(t, output)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_ConsoleWrite(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()
	const workspace = ""

	t.Run("success", func(t *testing.T) {
		console, err := client.ConsoleCreate(ctx, workspace)
		require.NoError(t, err)

		output, err := client.ConsoleRead(ctx, console.ID)
		require.NoError(t, err)
		t.Log(output.Data)

		const data = "version\r\n"
		n, err := client.ConsoleWrite(ctx, console.ID, data)
		require.NoError(t, err)
		require.Equal(t, uint64(len(data)), n)

		output, err = client.ConsoleRead(ctx, console.ID)
		require.NoError(t, err)
		t.Logf("%s\n%s\n", output.Prompt, output.Data)

		err = client.ConsoleDestroy(ctx, console.ID)
		require.NoError(t, err)
	})

	t.Run("no data", func(t *testing.T) {
		n, err := client.ConsoleWrite(ctx, "999", "")
		require.NoError(t, err)
		require.Zero(t, n)
	})

	const (
		id   = "999"
		data = "foo"
	)

	t.Run("invalid console id", func(t *testing.T) {
		const errStr = "failed to write to console 999: failure"
		n, err := client.ConsoleWrite(ctx, id, data)
		require.EqualError(t, err, errStr)
		require.Equal(t, uint64(0), n)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		n, err := client.ConsoleWrite(ctx, id, data)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Equal(t, uint64(0), n)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			n, err := client.ConsoleWrite(ctx, id, data)
			monkey.IsMonkeyError(t, err)
			require.Equal(t, uint64(0), n)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_ConsoleSessionDetach(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()
	const workspace = ""

	t.Run("success", func(t *testing.T) {
		console, err := client.ConsoleCreate(ctx, workspace)
		require.NoError(t, err)

		output, err := client.ConsoleRead(ctx, console.ID)
		require.NoError(t, err)
		t.Log(output.Data)

		// detach
		err = client.ConsoleSessionDetach(ctx, console.ID)
		require.NoError(t, err)
		time.Sleep(time.Second)
		output, err = client.ConsoleRead(ctx, console.ID)
		require.NoError(t, err)
		t.Logf("%s\n%s\n", output.Prompt, output.Data)

		err = client.ConsoleDestroy(ctx, console.ID)
		require.NoError(t, err)
	})

	t.Run("invalid console id", func(t *testing.T) {
		const errStr = "failed to detach session about console 999: failure"
		err := client.ConsoleSessionDetach(ctx, "999")
		require.EqualError(t, err, errStr)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		err := client.ConsoleSessionDetach(ctx, "999")
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			err := client.ConsoleSessionDetach(ctx, "999")
			monkey.IsMonkeyError(t, err)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_ConsoleSessionKill(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()
	const workspace = ""

	t.Run("success", func(t *testing.T) {
		console, err := client.ConsoleCreate(ctx, workspace)
		require.NoError(t, err)

		output, err := client.ConsoleRead(ctx, console.ID)
		require.NoError(t, err)
		t.Log(output.Data)

		// start a handler
		for _, command := range [...]string{
			"use exploit/multi/handler\r\n",
			"set payload windows/meterpreter/reverse_tcp\r\n",
			"set LHOST 127.0.0.1\r\n",
			"set LPORT 0\r\n",
			"show options\r\n",
			"exploit\r\n",
		} {
			n, err := client.ConsoleWrite(ctx, console.ID, command)
			require.NoError(t, err)
			require.Equal(t, uint64(len(command)), n)
			// don't wait exploit
			if command == "exploit\r\n" {
				break
			}
			for {
				output, err := client.ConsoleRead(ctx, console.ID)
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

		// session kill
		err = client.ConsoleSessionKill(ctx, console.ID)
		require.NoError(t, err)
		time.Sleep(time.Second)
		output, err = client.ConsoleRead(ctx, console.ID)
		require.NoError(t, err)
		t.Logf("%s\n%s\n", output.Prompt, output.Data)

		err = client.ConsoleDestroy(ctx, console.ID)
		require.NoError(t, err)
	})

	t.Run("invalid console id", func(t *testing.T) {
		const errStr = "failed to kill session about console 999: failure"
		err := client.ConsoleSessionKill(ctx, "999")
		require.EqualError(t, err, errStr)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		err := client.ConsoleSessionKill(ctx, "999")
		require.EqualError(t, err, ErrInvalidTokenFriendly)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			err := client.ConsoleSessionKill(ctx, "999")
			monkey.IsMonkeyError(t, err)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestConsole(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndConnectDB(t)
	ctx := context.Background()

	const (
		workspace = ""
		interval  = 25 * time.Millisecond
	)

	console, err := client.NewConsole(ctx, workspace, interval)
	require.NoError(t, err)

	go func() { _, _ = io.Copy(os.Stdout, console) }()

	for _, command := range [...]string{
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

	fmt.Println(console.ID())

	err = console.Destroy()
	require.NoError(t, err)

	err = console.Close()
	require.NoError(t, err)
	testsuite.IsDestroyed(t, console)

	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_NewConsole(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClient(t)

	// not login
	console, err := client.NewConsole(context.Background(), "", 0)
	require.Error(t, err)
	require.Nil(t, console)

	err = client.Close()
	require.Error(t, err)
	client.Kill()

	testsuite.IsDestroyed(t, client)
}

func TestConsole_readLoop(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)

	ctx := context.Background()

	err := client.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	const (
		workspace = ""
		interval  = 25 * time.Millisecond
	)

	t.Run("after msfrpc client closed", func(t *testing.T) {
		// simulate msfrpc is closed
		atomic.StoreInt32(&client.inShutdown, 1)
		defer atomic.StoreInt32(&client.inShutdown, 0)

		console, err := client.NewConsole(ctx, workspace, 0)
		require.NoError(t, err)

		// wait close self
		time.Sleep(time.Second)

		err = console.Destroy()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, console)
	})

	t.Run("failed to read", func(t *testing.T) {
		console, err := client.NewConsole(ctx, workspace, interval)
		require.NoError(t, err)

		time.Sleep(time.Second)

		err = console.Destroy()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, console)
	})

	t.Run("panic", func(t *testing.T) {
		_, w := io.Pipe()
		defer func() { _ = w.Close() }()

		patch := func(interface{}) bool {
			panic(monkey.Panic)
		}
		pg := monkey.PatchInstanceMethod(w, "Write", patch)
		defer pg.Unpatch()

		console, err := client.NewConsole(ctx, workspace, interval)
		require.NoError(t, err)

		time.Sleep(time.Second)

		err = console.Destroy()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, console)
	})

	t.Run("tracker", func(t *testing.T) {
		console, err := client.NewConsole(ctx, workspace, interval)
		require.NoError(t, err)
		id := console.id

		// wait console tracker
		time.Sleep(time.Second)

		err = client.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, console)

		// destroy opened console
		client = testGenerateClientAndLogin(t)

		err = client.ConsoleDestroy(ctx, id)
		require.NoError(t, err)
	})

	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestConsole_read(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndConnectDB(t)
	ctx := context.Background()

	const (
		workspace = ""
		interval  = 25 * time.Millisecond
	)

	t.Run("failed to read", func(t *testing.T) {
		console, err := client.NewConsole(ctx, workspace, interval)
		require.NoError(t, err)

		go func() { _, _ = io.Copy(os.Stdout, console) }()

		err = console.Destroy()
		require.NoError(t, err)

		ok := console.read()
		require.False(t, ok)

		testsuite.IsDestroyed(t, console)
	})

	t.Run("cancel in busy", func(t *testing.T) {
		console, err := client.NewConsole(ctx, workspace, interval)
		require.NoError(t, err)

		go func() { _, _ = io.Copy(os.Stdout, console) }()

		_, err = console.Write([]byte("use exploit/multi/handler\r\n"))
		require.NoError(t, err)

		time.Sleep(3 * minReadInterval)

		err = console.Destroy()
		require.NoError(t, err)

		ok := console.read()
		require.False(t, ok)

		testsuite.IsDestroyed(t, console)
	})

	t.Run("failed to write in busy", func(t *testing.T) {
		console, err := client.NewConsole(ctx, workspace, interval)
		require.NoError(t, err)

		go func() { _, _ = io.Copy(os.Stdout, console) }()

		for _, command := range [...]string{
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

		err = console.Destroy()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, console)
	})

	t.Run("failed to write last in busy", func(t *testing.T) {
		console, err := client.NewConsole(ctx, workspace, interval)
		require.NoError(t, err)

		go func() { _, _ = io.Copy(os.Stdout, console) }()

		for _, command := range [...]string{
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

		err = console.Destroy()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, console)
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestConsole_writeLimiter(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClient(t)
	// force setting IdleConnTimeout for prevent net/http call time.Reset()
	client.client.Transport.(*http.Transport).IdleConnTimeout = 0
	err := client.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	err = client.DBConnect(ctx, testDBOptions)
	require.NoError(t, err)

	const (
		workspace = ""
		interval  = 25 * time.Millisecond
	)

	t.Run("cancel", func(t *testing.T) {
		console, err := client.NewConsole(ctx, workspace, interval)
		require.NoError(t, err)

		time.Sleep(minReadInterval)

		err = console.Destroy()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, console)
	})

	t.Run("cancel after cancel", func(t *testing.T) {
		console, err := client.NewConsole(ctx, workspace, interval)
		require.NoError(t, err)

		time.Sleep(3 * minReadInterval)

		err = console.Destroy()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, console)
	})

	t.Run("panic", func(t *testing.T) {
		timer := time.NewTimer(time.Second)
		defer timer.Stop()

		patch := func(interface{}, time.Duration) bool {
			panic(monkey.Panic)
		}
		pg := monkey.PatchInstanceMethod(timer, "Reset", patch)
		defer pg.Unpatch()

		console, err := client.NewConsole(ctx, workspace, interval)
		require.NoError(t, err)

		time.Sleep(time.Second)

		select {
		case <-console.token:
		case <-time.After(time.Second):
		}

		// prevent select context
		time.Sleep(time.Second)

		pg.Unpatch()

		err = console.Destroy()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, console)
	})

	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestConsole_Write(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndConnectDB(t)
	ctx := context.Background()

	const (
		workspace = ""
		interval  = 25 * time.Millisecond
	)

	t.Run("block before mutex", func(t *testing.T) {
		console, err := client.NewConsole(ctx, workspace, interval)
		require.NoError(t, err)

		go func() { _, _ = io.Copy(os.Stdout, console) }()

		err = console.Close()
		require.NoError(t, err)

		_, err = console.Write([]byte("version"))
		require.Equal(t, context.Canceled, err)

		err = console.Destroy()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, console)
	})

	t.Run("block after mutex", func(t *testing.T) {
		console, err := client.NewConsole(ctx, workspace, interval)
		require.NoError(t, err)

		go func() { _, _ = io.Copy(os.Stdout, console) }()

		go func() {
			time.Sleep(3 * minReadInterval)
			err := console.Close()
			require.NoError(t, err)
		}()

		_, err = console.Write([]byte("version"))
		require.Equal(t, context.Canceled, err)

		err = console.Destroy()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, console)
	})

	t.Run("failed to write", func(t *testing.T) {
		console := Console{
			ctx:     client,
			context: context.Background(),
		}
		console.token = make(chan struct{}, 2)
		console.token <- struct{}{}
		console.token <- struct{}{}

		_, err := console.Write([]byte("version"))
		require.Error(t, err)
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestConsole_Detach(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndConnectDB(t)
	ctx := context.Background()

	const (
		workspace = ""
		interval  = 25 * time.Millisecond
	)

	console, err := client.NewConsole(ctx, workspace, interval)
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

	for _, command := range [...]string{
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
	payloadOpts := ModuleExecuteOptions{
		DataStore: make(map[string]interface{}),
	}
	payloadOpts.Format = "raw"
	payloadOpts.DataStore["EXITFUNC"] = "thread"
	payloadOpts.DataStore["LHOST"] = "127.0.0.1"
	payloadOpts.DataStore["LPORT"] = 55200
	pResult, err := client.ModuleExecute(ctx, "payload", payload, &payloadOpts)
	require.NoError(t, err)
	sc := []byte(pResult.Payload)
	// execute shellcode and wait some time
	go func() { _ = shellcode.Execute("", sc) }()
	time.Sleep(8 * time.Second)

	for _, command := range [...]string{
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

	err = console.Destroy()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, console)

	// stop session
	sessions, err := client.SessionList(ctx)
	require.NoError(t, err)
	for id, session := range sessions {
		const format = "id: %d type: %s remote: %s\n"
		t.Logf(format, id, session.Type, session.TunnelPeer)

		err = client.SessionStop(ctx, id)
		require.NoError(t, err)
	}

	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestConsole_Destroy(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndConnectDB(t)
	ctx := context.Background()

	const (
		workspace = ""
		interval  = 25 * time.Millisecond
	)

	t.Run("twice", func(t *testing.T) {
		console, err := client.NewConsole(ctx, workspace, interval)
		require.NoError(t, err)

		err = console.Destroy()
		require.NoError(t, err)
		err = console.Destroy()
		require.Error(t, err)

		err = console.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, console)
	})

	t.Run("failed to destroy", func(t *testing.T) {
		console, err := client.NewConsole(ctx, workspace, interval)
		require.NoError(t, err)

		err = client.AuthLogout(client.GetToken())
		require.NoError(t, err)

		err = console.Destroy()
		require.Error(t, err)

		err = console.Destroy()
		require.Error(t, err)

		// login and destroy
		err = client.AuthLogin()
		require.NoError(t, err)

		err = console.Destroy()
		require.NoError(t, err)
		err = console.Close()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, console)
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}
