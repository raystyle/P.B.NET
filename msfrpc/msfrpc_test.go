package msfrpc

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/patch/monkey"
	"project/internal/testsuite"
)

const (
	testHost         = "127.0.0.1"
	testPort         = "55553"
	testAddress      = testHost + ":" + testPort
	testUsername     = "msf"
	testPassword     = "msf"
	testInvalidToken = "invalid token"
)

func TestMain(m *testing.M) {
	exitCode := m.Run()
	// create msfrpc
	client, err := NewClient(testAddress, testUsername, testPassword, logger.Discard, nil)
	testsuite.CheckErrorInTestMain(err)
	err = client.AuthLogin()
	testsuite.CheckErrorInTestMain(err)
	// check leaks
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, check := range [...]func(context.Context, *Client) bool{
		testMainCheckSession,
		testMainCheckJob,
		testMainCheckConsole,
		testMainCheckToken,
		testMainCheckThread,
	} {
		if !check(ctx, client) {
			time.Sleep(time.Minute)
			os.Exit(1)
		}
	}
	err = client.Close()
	testsuite.CheckErrorInTestMain(err)
	// one test main goroutine and two goroutine about
	// pprof server in internal/testsuite.go
	leaks := true
	for i := 0; i < 300; i++ {
		if runtime.NumGoroutine() == 3 {
			leaks = false
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if leaks {
		fmt.Println("[warning] goroutine leaks!")
		time.Sleep(time.Minute)
		os.Exit(1)
	}
	if !testsuite.Destroyed(client) {
		fmt.Println("[warning] msfrpc client is not destroyed!")
		time.Sleep(time.Minute)
		os.Exit(1)
	}
	os.Exit(exitCode)
}

func testMainCheckSession(ctx context.Context, client *Client) bool {
	var (
		sessions map[uint64]*SessionInfo
		err      error
	)
	for i := 0; i < 30; i++ {
		sessions, err = client.SessionList(ctx)
		testsuite.CheckErrorInTestMain(err)
		if len(sessions) == 0 {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	fmt.Println("[warning] msfrpcd session leaks!")
	const format = "id: %d type: %s remote: %s\n"
	for id, session := range sessions {
		fmt.Printf(format, id, session.Type, session.TunnelPeer)
	}
	return false
}

func testMainCheckJob(ctx context.Context, client *Client) bool {
	var (
		list map[string]string
		err  error
	)
	for i := 0; i < 30; i++ {
		list, err = client.JobList(ctx)
		testsuite.CheckErrorInTestMain(err)
		if len(list) == 0 {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	fmt.Println("[warning] msfrpcd job leaks!")
	const format = "id: %s name: %s\n"
	for id, name := range list {
		fmt.Printf(format, id, name)
	}
	return false
}

func testMainCheckConsole(ctx context.Context, client *Client) bool {
	var (
		consoles []*ConsoleInfo
		err      error
	)
	for i := 0; i < 30; i++ {
		consoles, err = client.ConsoleList(ctx)
		testsuite.CheckErrorInTestMain(err)
		if len(consoles) == 0 {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	fmt.Println("[warning] msfrpcd console leaks!")
	const format = "id: %s prompt: %s\n"
	for i := 0; i < len(consoles); i++ {
		fmt.Printf(format, consoles[i].ID, consoles[i].Prompt)
	}
	return false
}

func testMainCheckToken(ctx context.Context, client *Client) bool {
	var (
		tokens []string
		err    error
	)
	for i := 0; i < 30; i++ {
		tokens, err = client.AuthTokenList(ctx)
		testsuite.CheckErrorInTestMain(err)
		// include self token
		if len(tokens) == 1 {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	fmt.Println("[warning] msfrpcd token leaks!")
	for i := 0; i < len(tokens); i++ {
		fmt.Println(tokens[i])
	}
	return false
}

func testMainCheckThread(ctx context.Context, client *Client) bool {
	var (
		threads map[uint64]*CoreThreadInfo
		err     error
	)
	for i := 0; i < 30; i++ {
		threads, err = client.CoreThreadList(ctx)
		testsuite.CheckErrorInTestMain(err)
		// TODO [external] msfrpcd thread leaks
		// if you call SessionMeterpreterRead() or SessionMeterpreterWrite()
		// when you exit meterpreter shell. this thread is always sleep.
		// so deceive ourselves now.
		for id, thread := range threads {
			if thread.Name == "StreamMonitorRemote" ||
				thread.Name == "MeterpreterRunSingle" {
				delete(threads, id)
			}
		}
		// 3 = internal(do noting)
		// 9 = start sessions scheduler(5) and session manager(1)
		l := len(threads)
		if l == 3 || l == 9 {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	fmt.Println("[warning] msfrpcd thread leaks!")
	const format = "id: %d\nname: %s\ncritical: %t\nstatus: %s\nstarted: %s\n\n"
	for i, t := range threads {
		fmt.Printf(format, i, t.Name, t.Critical, t.Status, t.Started)
	}
	return false
}

func testGenerateClient(t *testing.T) *Client {
	client, err := NewClient(testAddress, testUsername, testPassword, logger.Test, nil)
	require.NoError(t, err)
	return client
}

func testGenerateClientAndLogin(t *testing.T) *Client {
	client := testGenerateClient(t)
	err := client.AuthLogin()
	require.NoError(t, err)
	return client
}

func testPatchClientSend(fn func()) {
	patch := func(context.Context, string, string, io.Reader) (*http.Request, error) {
		return nil, monkey.Error
	}
	pg := monkey.Patch(http.NewRequestWithContext, patch)
	defer pg.Unpatch()
	fn()
}

var testMSFRPCConfig = new(Config)

func init() {
	testMSFRPCConfig.Logger = logger.Test

	testMSFRPCConfig.Client.Address = testAddress
	testMSFRPCConfig.Client.Username = testUsername
	testMSFRPCConfig.Client.Password = testPassword

	testMSFRPCConfig.Monitor = &MonitorOptions{
		EnableDB:  true,
		DBOptions: testDBOptions,
	}

	testMSFRPCConfig.Web.Network = "tcp"
	testMSFRPCConfig.Web.Address = "127.0.0.1:0"
	testMSFRPCConfig.Web.Options = &WebOptions{
		AdminUsername: "admin",
		AdminPassword: "$2a$12$er.iGxcRPUZnmUP.E7JrSOMZsJtoBkqXVIvRQywVaplIplupj7X.G", // "admin"
		DisableTLS:    true,
		HFS:           http.Dir("testdata/web"),
		Users: map[string]string{
			"test": "$2a$12$ADJFbAyjZ5XkekEXewEOeu8UmKMXDkcmu.RPV/AkP.j7CMeGQKz5u", // "test"
		},
	}
}

func TestMSFRPC_HijackLogWriter(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testMSFRPCConfig)
	require.NoError(t, err)
	go func() {
		err := msfrpc.Main()
		require.NoError(t, err)
	}()
	msfrpc.Wait()

	msfrpc.HijackLogWriter()
	log.Print("hijacked log")

	msfrpc.Exit()

	testsuite.IsDestroyed(t, msfrpc)
}
