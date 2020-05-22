package test

import (
	"context"
	"crypto/tls"
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
	exitCode := m.Run()
	if ctrl != nil {
		// wait to print log
		time.Sleep(250 * time.Millisecond)
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
		// must copy, because it is a global variable
		ctrlC := ctrl
		ctrl = nil
		if !testsuite.Destroyed(ctrlC) {
			fmt.Println("[warning] controller is not destroyed")
			time.Sleep(time.Minute)
			os.Exit(1)
		}
	}
	os.Exit(exitCode)
}

// in some tests, these value will be changed.
var (
	loggerLevel      = "debug"
	senderTimeout    = 15 * time.Second
	syncerExpireTime = 3 * time.Second
)

func generateControllerConfig() *controller.Config {
	cfg := controller.Config{}

	cfg.Database.Dialect = "mysql"
	cfg.Database.DSN = "pbnet:pbnet@tcp(127.0.0.1:3306)/pbnet_dev?loc=Local&parseTime=true"
	cfg.Database.MaxOpenConns = 16
	cfg.Database.MaxIdleConns = 16
	cfg.Database.LogFile = "log/database.log"
	cfg.Database.GORMLogFile = "log/gorm.log"
	cfg.Database.GORMDetailedLog = false
	cfg.Database.LogWriter = logger.NewWriterWithPrefix(os.Stdout, "Ctrl")

	cfg.Logger.Level = loggerLevel
	cfg.Logger.File = "log/controller.log"
	cfg.Logger.Writer = logger.NewWriterWithPrefix(os.Stdout, "Ctrl")

	cfg.Global.DNSCacheExpire = 10 * time.Second
	cfg.Global.TimeSyncSleepFixed = 15
	cfg.Global.TimeSyncSleepRandom = 10
	cfg.Global.TimeSyncInterval = time.Minute

	cfg.Client.Timeout = 10 * time.Second
	cfg.Client.TLSConfig.LoadFromCertPool.LoadPrivateRootCA = true
	cfg.Client.TLSConfig.LoadFromCertPool.LoadPrivateClient = true

	cfg.Sender.MaxConns = 16
	cfg.Sender.Worker = 64
	cfg.Sender.Timeout = senderTimeout
	cfg.Sender.QueueSize = 512
	cfg.Sender.MaxBufferSize = 16 << 10

	cfg.Syncer.ExpireTime = syncerExpireTime

	cfg.Worker.Number = 64
	cfg.Worker.QueueSize = 512
	cfg.Worker.MaxBufferSize = 16 << 10

	cfg.WebServer.Directory = "web"
	cfg.WebServer.CertFile = "ca/cert.pem"
	cfg.WebServer.KeyFile = "ca/key.pem"
	cfg.WebServer.CertOpts.DNSNames = []string{"localhost"}
	cfg.WebServer.CertOpts.IPAddresses = []string{"127.0.0.1", "::1"}
	cfg.WebServer.Network = "tcp"
	cfg.WebServer.Address = "localhost:1657"
	cfg.WebServer.Username = "pbnet" // # super user, password = "pbnet"
	cfg.WebServer.Password = "$2a$12$zWgjYi0aAq.958UtUyDi5.QDmq4LOWsvv7I9ulvf1rHzd9/dWWmTi"

	cfg.Test.SkipSynchronizeTime = true
	cfg.Test.SkipTestClientDNS = true
	return &cfg
}

func generateNodeConfig(t testing.TB, name string) *node.Config {
	cfg := node.Config{}

	cfg.Logger.Level = loggerLevel
	cfg.Logger.QueueSize = 512
	cfg.Logger.Writer = logger.NewWriterWithPrefix(os.Stdout, name)

	cfg.Global.DNSCacheExpire = 10 * time.Second
	cfg.Global.TimeSyncSleepFixed = 15
	cfg.Global.TimeSyncSleepRandom = 10
	cfg.Global.TimeSyncInterval = time.Minute

	cfg.Global.CertPool.GetCertsFromPool(ctrl.GetCertPool())
	cfg.Global.ProxyClients = testdata.ProxyClients(t)
	cfg.Global.DNSServers = testdata.DNSServers()
	cfg.Global.TimeSyncerClients = testdata.TimeSyncerClients()

	cfg.Client.Timeout = 10 * time.Second
	cfg.Client.ProxyTag = testdata.Socks5Tag
	cfg.Client.TLSConfig.LoadFromCertPool.LoadPrivateRootCA = true
	cfg.Client.TLSConfig.LoadFromCertPool.LoadPrivateClient = true

	cfg.Register.SleepFixed = 10
	cfg.Register.SleepRandom = 20

	cfg.Forwarder.MaxClientConns = 7
	cfg.Forwarder.MaxCtrlConns = 10
	cfg.Forwarder.MaxNodeConns = 8
	cfg.Forwarder.MaxBeaconConns = 128

	cfg.Sender.Worker = 64
	cfg.Sender.Timeout = senderTimeout
	cfg.Sender.QueueSize = 512
	cfg.Sender.MaxBufferSize = 16 << 10

	cfg.Syncer.ExpireTime = syncerExpireTime

	cfg.Worker.Number = 16
	cfg.Worker.QueueSize = 1024
	cfg.Worker.MaxBufferSize = 16 << 10

	cfg.Server.MaxConns = 64
	cfg.Server.Timeout = 15 * time.Second

	cfg.Ctrl.KexPublicKey = ctrl.KeyExchangePublicKey()
	cfg.Ctrl.PublicKey = ctrl.PublicKey()
	cfg.Ctrl.BroadcastKey = ctrl.BroadcastKey()

	cfg.Test.SkipSynchronizeTime = true
	return &cfg
}

func generateBeaconConfig(t testing.TB, name string) *beacon.Config {
	cfg := beacon.Config{}

	cfg.Logger.Level = loggerLevel
	cfg.Logger.QueueSize = 512
	cfg.Logger.Writer = logger.NewWriterWithPrefix(os.Stdout, name)

	cfg.Global.DNSCacheExpire = 10 * time.Second
	cfg.Global.TimeSyncSleepFixed = 15
	cfg.Global.TimeSyncSleepRandom = 10
	cfg.Global.TimeSyncInterval = time.Minute

	cfg.Global.CertPool.GetCertsFromPool(ctrl.GetCertPool())
	cfg.Global.ProxyClients = testdata.ProxyClients(t)
	cfg.Global.DNSServers = testdata.DNSServers()
	cfg.Global.TimeSyncerClients = testdata.TimeSyncerClients()

	cfg.Client.Timeout = 10 * time.Second
	cfg.Client.ProxyTag = testdata.Socks5Tag
	cfg.Client.TLSConfig.LoadFromCertPool.LoadPrivateRootCA = true
	cfg.Client.TLSConfig.LoadFromCertPool.LoadPrivateClient = true

	cfg.Register.SleepFixed = 10
	cfg.Register.SleepRandom = 20

	cfg.Sender.MaxConns = 7
	cfg.Sender.Worker = 64
	cfg.Sender.Timeout = senderTimeout
	cfg.Sender.QueueSize = 512
	cfg.Sender.MaxBufferSize = 16 << 10

	cfg.Syncer.ExpireTime = syncerExpireTime

	cfg.Worker.Number = 16
	cfg.Worker.QueueSize = 1024
	cfg.Worker.MaxBufferSize = 16 << 10

	cfg.Driver.SleepFixed = 5
	cfg.Driver.SleepRandom = 10

	cfg.Ctrl.KexPublicKey = ctrl.KeyExchangePublicKey()
	cfg.Ctrl.PublicKey = ctrl.PublicKey()

	cfg.Test.SkipSynchronizeTime = true
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
		err = ctrl.LoadKeyFromFile([]byte("pbnet"), []byte("pbnet"))
		require.NoError(t, err)
		go func() {
			err := ctrl.Main()
			require.NoError(t, err)
		}()
		ctrl.Wait()
	})
}

func generateCert(t testing.TB) *cert.Pair {
	certs := ctrl.GetCertPool().GetPrivateRootCAPairs()
	caCert := certs[0].Certificate
	caKey := certs[0].PrivateKey
	opts := cert.Options{
		DNSNames:    []string{"localhost"},
		IPAddresses: []string{"127.0.0.1", "::1"},
	}
	pair, err := cert.Generate(caCert, caKey, &opts)
	require.NoError(t, err)
	return pair
}

// // must copy because of cover string
//	mode := []byte(listener.Mode())
//	network := []byte(addr.Network())
//	address := []byte(addr.String())
//	return bootstrap.NewListener(string(mode), string(network), string(address))
func getNodeListener(t testing.TB, node *node.Node, tag string) *bootstrap.Listener {
	listener, err := node.GetListener(tag)
	require.NoError(t, err)
	addr := listener.Addr()
	return bootstrap.NewListener(listener.Mode(), addr.Network(), addr.String())
}

// -----------------------------------------Initial Node-------------------------------------------

const initialNodeListenerTag = "initial_tls"

func generateInitialNode(t testing.TB, id int) *node.Node {
	initializeController(t)

	cfg := generateNodeConfig(t, fmt.Sprintf("Initial Node %d", id))
	cfg.Register.Skip = true

	// generate listener config
	certPEM, keyPEM := generateCert(t).EncodeToPEM()
	listener := messages.Listener{
		Tag:     initialNodeListenerTag,
		Mode:    xnet.ModeTLS,
		Network: "tcp",
		Address: "localhost:0",
	}
	listener.TLSConfig.Certificates = []option.X509KeyPair{
		{Cert: string(certPEM), Key: string(keyPEM)},
	}
	listener.TLSConfig.LoadFromCertPool.LoadPrivateClientCA = true
	listener.TLSConfig.ClientAuth = tls.RequireAndVerifyClientCert

	// set to node config
	data, key, err := controller.GenerateListeners(&listener)
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
	listener := getNodeListener(t, Node, initialNodeListenerTag)
	ctx := context.Background()
	// trust node
	nnr, err := ctrl.TrustNode(ctx, listener)
	require.NoError(t, err)
	// confirm
	reply := controller.ReplyNodeRegister{
		ID:   nnr.ID,
		Zone: "test",
	}
	err = ctrl.ConfirmTrustNode(ctx, &reply)
	require.NoError(t, err)
	// connect node
	err = ctrl.Synchronize(ctx, Node.GUID(), listener)
	require.NoError(t, err)
	return Node
}

func generateBootstrap(t testing.TB, listeners ...*bootstrap.Listener) ([]byte, []byte) {
	direct := bootstrap.NewDirect()
	for _, listener := range listeners {
		direct.Listeners = append(direct.Listeners, listener.Decrypt())
	}
	directCfg, err := direct.Marshal()
	require.NoError(t, err)
	boot := messages.Bootstrap{
		Tag:    "first",
		Mode:   bootstrap.ModeDirect,
		Config: directCfg,
	}
	data, key, err := controller.GenerateFirstBootstrap(&boot)
	require.NoError(t, err)
	return data, key
}
