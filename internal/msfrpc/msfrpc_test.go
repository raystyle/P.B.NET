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
	testBin      = "msfrpcd"
	testHost     = "127.0.0.1"
	testPort     = 55553
	testUsername = "msf"
	testPassword = "msf"
)

func TestMain(m *testing.M) {
	if testsuite.InGoland {

	}

	cmd := exec.Command(testBin, "-a", testHost, "-U", testUsername, "-P", testPassword)
	if !testsuite.InGoland {
		// start Metasploit RPC service
		err := cmd.Start()
		if err != nil {
			panic(err)
		}
		// wait some time for start Metasploit RPC service
		// stdout and stderr can't read any data, so use time.Sleep
	}

	// TODO remove it
	// time.Sleep(10 * time.Second)

	exitCode := m.Run()

	// stop Metasploit RPC service
	if !testsuite.InGoland {
		_ = cmd.Process.Kill()
	}

	os.Exit(exitCode)
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

		err = msfrpc.Logout(msfrpc.getToken())
		require.NoError(t, err)
	})

	t.Run("logout invalid token", func(t *testing.T) {
		// err = msfrpc.Login()
		// require.NoError(t, err)
		//
		// err = msfrpc.Logout("invalid token")
		// require.Error(t, err)
		// t.Log(err)
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
