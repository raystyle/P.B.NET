package test

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/crypto/cert"
	"project/internal/logger"
	"project/internal/messages"
	"project/internal/option"
	"project/internal/testsuite"
	"project/internal/xnet"

	"project/beacon"
	"project/controller"
	"project/node"
	"project/testdata"
)

var (
	ctrl     *controller.Ctrl
	initOnce sync.Once
)

func TestMain(m *testing.M) {
	m.Run()

	if ctrl != nil {
		// wait to print log
		time.Sleep(time.Second)
		ctrl.Exit(nil)
	}

	testdata.Clean()

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

	if ctrl != nil {
		// must copy, because global variable
		ctrlC := ctrl
		ctrl = nil

		if !testsuite.Destroyed(ctrlC) {
			fmt.Println("[warning] controller is not destroyed")
			time.Sleep(time.Minute)
			os.Exit(1)
		}
	}
}

// in some test, log level will be changed.
var logLevel = "debug"

func generateControllerConfig() *controller.Config {
	cfg := controller.Config{}

	cfg.Test.SkipTestClientDNS = true
	cfg.Test.SkipSynchronizeTime = true

	cfg.Database.Dialect = "mysql"
	cfg.Database.DSN = "pbnet:pbnet@tcp(127.0.0.1:3306)/pbnet_test?loc=Local&parseTime=true"
	cfg.Database.MaxOpenConns = 16
	cfg.Database.MaxIdleConns = 16
	cfg.Database.LogFile = "log/database.log"
	cfg.Database.GORMLogFile = "log/gorm.log"
	cfg.Database.GORMDetailedLog = false
	cfg.Database.LogWriter = logger.NewWriterWithPrefix(os.Stdout, "Ctrl")

	cfg.Logger.Level = logLevel
	cfg.Logger.File = "log/controller.log"
	cfg.Logger.Writer = logger.NewWriterWithPrefix(os.Stdout, "Ctrl")

	cfg.Global.DNSCacheExpire = 10 * time.Second
	cfg.Global.TimeSyncSleepFixed = 15
	cfg.Global.TimeSyncSleepRandom = 10
	cfg.Global.TimeSyncInterval = time.Minute

	cfg.Client.Timeout = 10 * time.Second

	cfg.Sender.MaxConns = 16 * runtime.NumCPU()
	cfg.Sender.Worker = 64
	cfg.Sender.Timeout = 15 * time.Second
	cfg.Sender.QueueSize = 512
	cfg.Sender.MaxBufferSize = 16 << 10

	cfg.Syncer.ExpireTime = 3 * time.Second

	cfg.Worker.Number = 64
	cfg.Worker.QueueSize = 512
	cfg.Worker.MaxBufferSize = 16 << 10

	cfg.Web.Dir = "web"
	cfg.Web.CertFile = "ca/cert.pem"
	cfg.Web.KeyFile = "ca/key.pem"
	cfg.Web.CertOpts.DNSNames = []string{"localhost"}
	cfg.Web.CertOpts.IPAddresses = []string{"127.0.0.1", "::1"}
	cfg.Web.Network = "tcp"
	cfg.Web.Address = "localhost:1657"
	cfg.Web.Username = "pbnet" // # super user, password = "pbnet"
	cfg.Web.Password = "$2a$12$zWgjYi0aAq.958UtUyDi5.QDmq4LOWsvv7I9ulvf1rHzd9/dWWmTi"
	return &cfg
}

func generateNodeConfig(tb testing.TB, name string) *node.Config {
	cfg := node.Config{}

	cfg.Test.SkipSynchronizeTime = true

	cfg.Logger.Level = logLevel
	cfg.Logger.QueueSize = 512
	cfg.Logger.Writer = logger.NewWriterWithPrefix(os.Stdout, name)

	cfg.Global.DNSCacheExpire = 10 * time.Second
	cfg.Global.TimeSyncSleepFixed = 15
	cfg.Global.TimeSyncSleepRandom = 10
	cfg.Global.TimeSyncInterval = time.Minute
	cfg.Global.Certificates = testdata.Certificates(tb)
	cfg.Global.ProxyClients = testdata.ProxyClients(tb)
	cfg.Global.DNSServers = testdata.DNSServers()
	cfg.Global.TimeSyncerClients = testdata.TimeSyncerClients()

	cfg.Client.ProxyTag = testdata.Socks5Tag
	cfg.Client.Timeout = 10 * time.Second

	cfg.Register.SleepFixed = 10
	cfg.Register.SleepRandom = 20

	cfg.Forwarder.MaxClientConns = 7
	cfg.Forwarder.MaxCtrlConns = 10
	cfg.Forwarder.MaxNodeConns = 8
	cfg.Forwarder.MaxBeaconConns = 128

	cfg.Sender.Worker = 64
	cfg.Sender.QueueSize = 512
	cfg.Sender.MaxBufferSize = 16 << 10
	cfg.Sender.Timeout = 15 * time.Second

	cfg.Syncer.ExpireTime = 3 * time.Second

	cfg.Worker.Number = 16
	cfg.Worker.QueueSize = 1024
	cfg.Worker.MaxBufferSize = 16 << 10

	cfg.Server.MaxConns = 16 * runtime.NumCPU()
	cfg.Server.Timeout = 15 * time.Second

	cfg.Ctrl.KexPublicKey = ctrl.KeyExchangePublicKey()
	cfg.Ctrl.PublicKey = ctrl.PublicKey()
	cfg.Ctrl.BroadcastKey = ctrl.BroadcastKey()
	return &cfg
}

func generateBeaconConfig(tb testing.TB, name string) *beacon.Config {
	cfg := beacon.Config{}

	cfg.Test.SkipSynchronizeTime = true

	cfg.Logger.Level = logLevel
	cfg.Logger.QueueSize = 512
	cfg.Logger.Writer = logger.NewWriterWithPrefix(os.Stdout, name)

	cfg.Global.DNSCacheExpire = 10 * time.Second
	cfg.Global.TimeSyncSleepFixed = 15
	cfg.Global.TimeSyncSleepRandom = 10
	cfg.Global.TimeSyncInterval = time.Minute
	cfg.Global.Certificates = testdata.Certificates(tb)
	cfg.Global.ProxyClients = testdata.ProxyClients(tb)
	cfg.Global.DNSServers = testdata.DNSServers()
	cfg.Global.TimeSyncerClients = testdata.TimeSyncerClients()

	cfg.Client.ProxyTag = testdata.Socks5Tag
	cfg.Client.Timeout = 10 * time.Second

	cfg.Register.SleepFixed = 10
	cfg.Register.SleepRandom = 20

	cfg.Sender.MaxConns = 7
	cfg.Sender.Worker = 64
	cfg.Sender.QueueSize = 512
	cfg.Sender.MaxBufferSize = 16 << 10
	cfg.Sender.Timeout = 15 * time.Second

	cfg.Syncer.ExpireTime = 3 * time.Second

	cfg.Worker.Number = 16
	cfg.Worker.QueueSize = 1024
	cfg.Worker.MaxBufferSize = 16 << 10

	cfg.Driver.SleepFixed = 5
	cfg.Driver.SleepRandom = 10

	cfg.Ctrl.KexPublicKey = ctrl.KeyExchangePublicKey()
	cfg.Ctrl.PublicKey = ctrl.PublicKey()
	cfg.Ctrl.BroadcastKey = ctrl.BroadcastKey()
	return &cfg
}

func initializeController(t testing.TB) {
	initOnce.Do(func() {
		err := os.Chdir("../app")
		require.NoError(t, err)
		cfg := generateControllerConfig()
		ctrl, err = controller.New(cfg)
		require.NoError(t, err)
		testsuite.IsDestroyed(t, cfg)
		// set controller keys
		err = ctrl.LoadSessionKeyFromFile("key/session.key", []byte("pbnet"))
		require.NoError(t, err)
		go func() {
			err := ctrl.Main()
			require.NoError(t, err)
		}()
		ctrl.Wait()
	})
}

// -----------------------------------------Initial Node-------------------------------------------

const InitialNodeListenerTag = "initial_tcp"

func generateInitialNode(t testing.TB, id int) *node.Node {
	initializeController(t)

	cfg := generateNodeConfig(t, fmt.Sprintf("Initial Node %d", id))
	cfg.Register.Skip = true

	// generate certificate
	pairs := ctrl.GetSelfCerts()
	opts := cert.Options{
		DNSNames:    []string{"localhost"},
		IPAddresses: []string{"127.0.0.1", "::1"},
	}
	caCert := pairs[0].Certificate
	caKey := pairs[0].PrivateKey
	pair, err := cert.Generate(caCert, caKey, &opts)
	require.NoError(t, err)

	// generate listener config
	listener := messages.Listener{
		Tag:     InitialNodeListenerTag,
		Mode:    xnet.ModeTCP,
		Network: "tcp",
		Address: "localhost:0",
	}
	c, k := pair.EncodeToPEM()
	listener.TLSConfig.Certificates = []option.X509KeyPair{
		{Cert: string(c), Key: string(k)},
	}

	// add to node config
	data, key, err := controller.GenerateNodeConfigAboutListeners(&listener)
	require.NoError(t, err)
	cfg.Server.Listeners = data
	cfg.Server.ListenersKey = key

	// run
	Node, err := node.New(cfg)
	require.NoError(t, err)
	testsuite.IsDestroyed(t, cfg)
	go func() {
		err := Node.Main()
		require.NoError(t, err)
	}()
	Node.Wait()
	return Node
}

func generateInitialNodeAndTrust(t testing.TB, id int) *node.Node {
	Node := generateInitialNode(t, id)
	listener, err := Node.GetListener(InitialNodeListenerTag)
	require.NoError(t, err)
	bListener := &bootstrap.Listener{
		Mode:    xnet.ModeTCP,
		Network: "tcp",
		Address: listener.Addr().String(),
	}
	// trust node
	req, err := ctrl.TrustNode(context.Background(), bListener)
	require.NoError(t, err)
	err = ctrl.ConfirmTrustNode(context.Background(), bListener, req)
	require.NoError(t, err)
	// connect node
	err = ctrl.Synchronize(context.Background(), Node.GUID(), bListener)
	require.NoError(t, err)
	return Node
}

func generateBootstrap(t testing.TB, listeners ...*bootstrap.Listener) ([]byte, []byte) {
	direct := bootstrap.NewDirect()
	direct.Listeners = listeners
	directCfg, err := direct.Marshal()
	require.NoError(t, err)
	boot := messages.Bootstrap{
		Tag:    "first",
		Mode:   bootstrap.ModeDirect,
		Config: directCfg,
	}
	data, key, err := controller.GenerateRoleConfigAboutTheFirstBootstrap(&boot)
	require.NoError(t, err)
	return data, key
}
