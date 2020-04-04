package msfrpc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
	"project/internal/testsuite"
)

func TestMSFRPC_ConsoleCreate(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		result, err := msfrpc.ConsoleCreate(ctx)
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

		result, err := msfrpc.ConsoleCreate(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, result)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			result, err := msfrpc.ConsoleCreate(ctx)
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

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		result, err := msfrpc.ConsoleCreate(ctx)
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

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		console, err := msfrpc.ConsoleCreate(ctx)
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

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		console, err := msfrpc.ConsoleCreate(ctx)
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

	t.Run("invalid console id", func(t *testing.T) {
		n, err := msfrpc.ConsoleWrite(ctx, "999", "foo")
		require.EqualError(t, err, "failed to write to console 999: failure")
		require.Equal(t, uint64(0), n)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)

		n, err := msfrpc.ConsoleWrite(ctx, "999", "foo")
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Equal(t, uint64(0), n)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchSend(func() {
			n, err := msfrpc.ConsoleWrite(ctx, "999", "foo")
			monkey.IsMonkeyError(t, err)
			require.Equal(t, uint64(0), n)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_ConsoleList(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
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

func TestMSFRPC_ConsoleSessionDetach(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		console, err := msfrpc.ConsoleCreate(ctx)
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

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.AuthLogin()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		console, err := msfrpc.ConsoleCreate(ctx)
		require.NoError(t, err)

		output, err := msfrpc.ConsoleRead(ctx, console.ID)
		require.NoError(t, err)
		t.Log(output.Data)

		// start a handler
		commands := []string{
			"use exploit/multi/handler\r\n",
			"set payload windows/meterpreter/reverse_tcp\r\n",
			"set LHOST 127.0.0.1\r\n",
			"set LPORT 0\r\n",
			"show options\r\n",
			"exploit\r\n",
		}
		for _, command := range commands {
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
