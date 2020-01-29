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

	"project/controller"
	"project/node"
	"project/testdata"
)

var (
	ctrl     *controller.CTRL
	initOnce sync.Once
)

func TestMain(m *testing.M) {
	m.Run()

	// wait to print log
	time.Sleep(time.Second)
	ctrl.Exit(nil)

	testdata.Clean()

	// one test main goroutine and
	// two goroutine about pprof server in testsuite
	leaks := true
	for i := 0; i < 300; i++ {
		if runtime.NumGoroutine() == 3 {
			leaks = false
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if leaks {
		fmt.Println("[Warning] goroutine leaks!")
		os.Exit(1)
	}

	// must copy
	ctrlC := ctrl
	ctrl = nil
	if !testsuite.Destroyed(ctrlC) {
		fmt.Println("[Warning] controller is not destroyed")
		os.Exit(1)
	}
}

func generateControllerConfig() *controller.Config {
	cfg := controller.Config{}

	cfg.Test.SkipTestClientDNS = true
	cfg.Test.SkipSynchronizeTime = true
	cfg.Test.NodeSend = make(chan []byte, 4)
	cfg.Test.BeaconSend = make(chan []byte, 4)

	cfg.Database.Dialect = "mysql"
	cfg.Database.DSN = "pbnet:pbnet@tcp(127.0.0.1:3306)/pbnet_test?loc=Local&parseTime=true"
	cfg.Database.MaxOpenConns = 16
	cfg.Database.MaxIdleConns = 16
	cfg.Database.LogFile = "log/database.log"
	cfg.Database.GORMLogFile = "log/gorm.log"
	cfg.Database.GORMDetailedLog = false
	cfg.Database.LogWriter = logger.NewWriterWithPrefix(os.Stdout, "CTRL")

	cfg.Logger.Level = "debug"
	cfg.Logger.File = "log/controller.log"
	cfg.Logger.Writer = logger.NewWriterWithPrefix(os.Stdout, "CTRL")

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
	cfg.Web.Address = "localhost:1657"
	cfg.Web.Username = "pbnet" // # super user, password = sha256(sha256("pbnet"))
	cfg.Web.Password = "d6b3ced503b70f7894bd30f36001de4af84a8c2af898f06e29bca95f2dcf5100"
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

func generateNodeConfig(tb testing.TB) *node.Config {
	cfg := node.Config{}

	cfg.Test.SkipSynchronizeTime = true
	cfg.Test.BroadcastTestMsg = make(chan []byte, 4)
	cfg.Test.SendTestMsg = make(chan []byte, 4)

	cfg.Logger.Level = "debug"
	cfg.Logger.Writer = logger.NewWriterWithPrefix(os.Stdout, "Node")

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

	cfg.CTRL.KexPublicKey = ctrl.KeyExchangePub()
	cfg.CTRL.PublicKey = ctrl.PublicKey()
	cfg.CTRL.BroadcastKey = ctrl.BroadcastKey()
	return &cfg
}

// -----------------------------------------initial Node-------------------------------------------

const InitialNodeListenerTag = "init_tcp"

func generateInitialNode(t testing.TB) *node.Node {
	initializeController(t)

	cfg := generateNodeConfig(t)
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

func generateInitialNodeAndTrust(t testing.TB) *node.Node {
	Node := generateInitialNode(t)
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
	err = ctrl.Connect(bListener, Node.GUID())
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
