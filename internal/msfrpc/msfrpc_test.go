package msfrpc

import (
	"context"
	"io"
	"net/http"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
	"project/internal/testsuite"
)

const (
	testCommand  = "msfrpcd"
	testHost     = "127.0.0.1"
	testPort     = 55553
	testUsername = "msf"
	testPassword = "msf"

	testInvalidToken    = "invalid token"
	testErrInvalidToken = "Invalid Authentication Token"
)

func TestMain(m *testing.M) {
	// start Metasploit RPC service
	cmd := exec.Command(testCommand, "-a", testHost, "-U", testUsername, "-P", testPassword)
	err := cmd.Start()
	if err != nil {
		panic(err)
	}
	// wait some time for start Metasploit RPC service
	// stdout and stderr can't read any data, so use time.Sleep
	// TODO remove comment
	// time.Sleep(10 * time.Second)
	// stop Metasploit RPC service
	defer func() {
		_ = cmd.Process.Kill()
	}()
	os.Exit(m.Run())
}

func TestNewMSFRPC(t *testing.T) {
	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func testPatchSend(f func()) {
	patch := func(context.Context, string, string, io.Reader) (*http.Request, error) {
		return nil, monkey.ErrMonkey
	}
	pg := monkey.Patch(http.NewRequestWithContext, patch)
	defer pg.Unpatch()
	f()
}

func TestMSFRPC_Login(t *testing.T) {
	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)

	t.Run("login success", func(t *testing.T) {
		err = msfrpc.Login()
		require.NoError(t, err)
	})

	t.Run("login failed", func(t *testing.T) {
		msfrpc.password = "foo"
		err = msfrpc.Login()
		require.EqualError(t, err, "Login Failed")

		msfrpc.password = testUsername
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			err = msfrpc.Login()
			monkey.IsMonkeyError(t, err)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_Logout(t *testing.T) {
	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)

	t.Run("logout self", func(t *testing.T) {
		err = msfrpc.Login()
		require.NoError(t, err)

		err = msfrpc.Logout(msfrpc.GetToken())
		require.NoError(t, err)
	})

	t.Run("logout invalid token", func(t *testing.T) {
		err = msfrpc.Login()
		require.NoError(t, err)

		err = msfrpc.Logout(testInvalidToken)
		require.EqualError(t, err, testErrInvalidToken)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			err = msfrpc.Logout(msfrpc.GetToken())
			monkey.IsMonkeyError(t, err)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_TokenList(t *testing.T) {
	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		token := msfrpc.GetToken()
		list, err := msfrpc.TokenList()
		require.NoError(t, err)
		var exist bool
		for i := 0; i < len(list); i++ {
			t.Log(list[i])
			if token == list[i] {
				exist = true
			}
		}
		require.True(t, exist)
	})

	t.Run("invalid token", func(t *testing.T) {
		msfrpc.SetToken(testInvalidToken)
		list, err := msfrpc.TokenList()
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, list)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			list, err := msfrpc.TokenList()
			monkey.IsMonkeyError(t, err)
			require.Nil(t, list)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_Close(t *testing.T) {
	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)

	t.Run("ok", func(t *testing.T) {
		err = msfrpc.Login()
		require.NoError(t, err)
		err = msfrpc.Close()
		require.NoError(t, err)
	})

	t.Run("failed", func(t *testing.T) {
		err = msfrpc.Close()
		require.Error(t, err)
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}
