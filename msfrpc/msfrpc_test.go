package msfrpc

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/nettool"
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
	code := m.Run()
	var (
		msfrpcdLeaks   bool
		msfrpcLeaks    bool
		goroutineLeaks bool
	)
	if code == 0 {
		msfrpcLeaks = testMainCheckMSFRPCLeaks()
		msfrpcdLeaks = testMainCheckMSFRPCDLeaks()
		goroutineLeaks = testsuite.TestMainGoroutineLeaks()
	}
	if msfrpcdLeaks || msfrpcLeaks || goroutineLeaks {
		fmt.Println("[info] wait one minute for fetch pprof")
		time.Sleep(time.Minute)
		os.Exit(1)
	}
	os.Exit(code)
}

func testMainCheckMSFRPCDLeaks() bool {
	// create msfrpc
	client, err := NewClient(testAddress, testUsername, testPassword, logger.Discard, nil)
	testsuite.TestMainCheckError(err)
	err = client.AuthLogin()
	testsuite.TestMainCheckError(err)
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
			return true
		}
	}
	err = client.Close()
	testsuite.TestMainCheckError(err)
	if !testsuite.Destroyed(client) {
		fmt.Println("[warning] msfrpc client is not destroyed!")
		return true
	}
	return false
}

func testMainCheckSession(ctx context.Context, client *Client) bool {
	var (
		sessions map[uint64]*SessionInfo
		err      error
	)
	for i := 0; i < 30; i++ {
		sessions, err = client.SessionList(ctx)
		testsuite.TestMainCheckError(err)
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
		testsuite.TestMainCheckError(err)
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
		testsuite.TestMainCheckError(err)
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
		testsuite.TestMainCheckError(err)
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
		testsuite.TestMainCheckError(err)
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

func testGenerateConfig() *Config {
	cfg := Config{Logger: logger.Test}

	cfg.Client.Address = testAddress
	cfg.Client.Username = testUsername
	cfg.Client.Password = testPassword

	// must set max connection, otherwise testsuite.MarkGoroutines()
	// will not inaccurate, and testInitializeMSFRPC() must wait some time
	cfg.Client.Options = new(ClientOptions)
	clientOpts := cfg.Client.Options
	clientOpts.Transport.MaxIdleConns = 1
	clientOpts.Transport.MaxIdleConnsPerHost = 1
	clientOpts.Transport.MaxConnsPerHost = 1
	clientOpts.Transport.IdleConnTimeout = 3 * time.Minute

	cfg.Monitor = &MonitorOptions{
		EnableDB: true,
		Database: testDBOptions,
	}

	cfg.Web.Network = "tcp"
	cfg.Web.Address = "127.0.0.1:0"
	cfg.Web.Options = &WebOptions{
		AdminPassword: "$2a$12$er.iGxcRPUZnmUP.E7JrSOMZsJtoBkqXVIvRQywVaplIplupj7X.G", // "admin"
		DisableTLS:    true,
		HFS:           http.Dir("testdata/web"),
		Users: map[string]*WebUser{
			"manager": {
				Password:    "$2a$12$ADJFbAyjZ5XkekEXewEOeu8UmKMXDkcmu.RPV/AkP.j7CMeGQKz5u", // "test"
				UserGroup:   UserGroupManagers,
				DisplayName: "Manager",
			},
			"user": {
				Password:    "$2a$12$ADJFbAyjZ5XkekEXewEOeu8UmKMXDkcmu.RPV/AkP.j7CMeGQKz5u", // "test"
				UserGroup:   UserGroupUsers,
				DisplayName: "User",
			},
			"guest": {
				Password:    "$2a$12$ADJFbAyjZ5XkekEXewEOeu8UmKMXDkcmu.RPV/AkP.j7CMeGQKz5u", // "test"
				UserGroup:   UserGroupGuests,
				DisplayName: "Guest",
			},
		},
	}
	return &cfg
}

func testGenerateMSFRPC(t testing.TB, cfg *Config) *MSFRPC {
	msfrpc, err := NewMSFRPC(cfg)
	require.NoError(t, err)
	go func() {
		err := msfrpc.Main()
		require.NoError(t, err)
	}()
	msfrpc.Wait()
	return msfrpc
}

func TestMSFRPC(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	cfg := testGenerateConfig()
	msfrpc := testGenerateMSFRPC(t, cfg)

	// serve a new listener
	errCh := make(chan error, 1)
	go func() {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		errCh <- msfrpc.Serve(listener)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	addrs, err := nettool.WaitServerServe(ctx, errCh, msfrpc, 2)
	require.NoError(t, err)
	msfrpc.logger.Println(logger.Debug, "test", "web server addresses:", addrs)

	// reload
	err = msfrpc.Reload()
	require.NoError(t, err)

	msfrpc.Exit()

	testsuite.IsDestroyed(t, msfrpc)
}
