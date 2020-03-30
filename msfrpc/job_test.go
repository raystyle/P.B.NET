package msfrpc

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
	"project/internal/testsuite"
)

func TestMSFRPC_JobList(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		console, err := msfrpc.ConsoleCreate()
		require.NoError(t, err)

		output, err := msfrpc.ConsoleRead(console.ID)
		require.NoError(t, err)
		t.Log(output.Data)

		// start a handler
		commands := []string{
			"use exploit/multi/handler\r\n",
			"set payload windows/meterpreter/reverse_tcp\r\n",
			"set LHOST 127.0.0.1\r\n",
			"set LPORT 0\r\n",
			"show options\r\n",
			"exploit -j\r\n",
		}
		for _, command := range commands {
			n, err := msfrpc.ConsoleWrite(console.ID, command)
			require.NoError(t, err)
			require.Equal(t, uint64(len(command)), n)
			for {
				output, err := msfrpc.ConsoleRead(console.ID)
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

		list, err := msfrpc.JobList()
		require.NoError(t, err)
		for id, name := range list {
			t.Log(id, name)
		}

		err = msfrpc.ConsoleDestroy(console.ID)
		require.NoError(t, err)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		msfrpc.SetToken(testInvalidToken)
		list, err := msfrpc.JobList()
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, list)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			list, err := msfrpc.JobList()
			monkey.IsMonkeyError(t, err)
			require.Nil(t, list)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_JobInfo(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		console, err := msfrpc.ConsoleCreate()
		require.NoError(t, err)

		output, err := msfrpc.ConsoleRead(console.ID)
		require.NoError(t, err)
		t.Log(output.Data)

		// start a handler
		commands := []string{
			"use exploit/multi/handler\r\n",
			"set payload windows/meterpreter/reverse_tcp\r\n",
			"set LHOST 127.0.0.1\r\n",
			"set LPORT 0\r\n",
			"show options\r\n",
			"exploit -j\r\n",
		}
		for _, command := range commands {
			n, err := msfrpc.ConsoleWrite(console.ID, command)
			require.NoError(t, err)
			require.Equal(t, uint64(len(command)), n)
			for {
				output, err := msfrpc.ConsoleRead(console.ID)
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

		list, err := msfrpc.JobList()
		require.NoError(t, err)
		for id := range list {
			info, err := msfrpc.JobInfo(id)
			require.NoError(t, err)
			t.Log(info.Name)
			for key, value := range info.DataStore {
				var typeName string
				typ := reflect.TypeOf(value)
				if typ != nil {
					typeName = typ.Name()
				}
				t.Log(key, value, typeName)
			}
		}

		err = msfrpc.ConsoleDestroy(console.ID)
		require.NoError(t, err)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		msfrpc.SetToken(testInvalidToken)
		info, err := msfrpc.JobInfo("foo")
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, info)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			info, err := msfrpc.JobInfo("foo")
			monkey.IsMonkeyError(t, err)
			require.Nil(t, info)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_JobStop(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		console, err := msfrpc.ConsoleCreate()
		require.NoError(t, err)

		output, err := msfrpc.ConsoleRead(console.ID)
		require.NoError(t, err)
		t.Log(output.Data)

		// start a handler
		commands := []string{
			"use exploit/multi/handler\r\n",
			"set payload windows/meterpreter/reverse_tcp\r\n",
			"set LHOST 127.0.0.1\r\n",
			"set LPORT 0\r\n",
			"show options\r\n",
			"exploit -j\r\n",
		}
		for _, command := range commands {
			n, err := msfrpc.ConsoleWrite(console.ID, command)
			require.NoError(t, err)
			require.Equal(t, uint64(len(command)), n)
			for {
				output, err := msfrpc.ConsoleRead(console.ID)
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

		list, err := msfrpc.JobList()
		require.NoError(t, err)
		for id := range list {
			err = msfrpc.JobStop(id)
			require.NoError(t, err)
		}

		err = msfrpc.ConsoleDestroy(console.ID)
		require.NoError(t, err)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		msfrpc.SetToken(testInvalidToken)
		err := msfrpc.JobStop("foo")
		require.EqualError(t, err, testErrInvalidToken)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			err := msfrpc.JobStop("foo")
			monkey.IsMonkeyError(t, err)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}
