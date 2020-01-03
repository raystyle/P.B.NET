package test

import (
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/controller"
	"project/internal/crypto/cert"
	"project/internal/logger"
	"project/internal/messages"
	"project/internal/options"
	"project/internal/xnet"
	"project/node"
	"project/testdata"
)

var (
	ctrl         *controller.CTRL
	initCtrlOnce sync.Once
)

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

	cfg.Global.DNSCacheExpire = time.Minute
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

func initializeController(t require.TestingT) {
	initCtrlOnce.Do(func() {
		err := os.Chdir("../app")
		require.NoError(t, err)
		cfg := generateControllerConfig()
		ctrl, err = controller.New(cfg)
		require.NoError(t, err)
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

	cfg.Global.DNSCacheExpire = 3 * time.Minute
	cfg.Global.TimeSyncSleepFixed = 15
	cfg.Global.TimeSyncSleepRandom = 10
	cfg.Global.TimeSyncInterval = 1 * time.Minute
	cfg.Global.Certificates = testdata.Certificates(tb)
	cfg.Global.ProxyClients = testdata.ProxyClients(tb)
	cfg.Global.DNSServers = testdata.DNSServers()
	cfg.Global.TimeSyncerClients = testdata.TimeSyncerClients()

	cfg.Client.ProxyTag = "balance"
	cfg.Client.Timeout = 15 * time.Second

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

	cfg.CTRL.ExPublicKey = ctrl.KeyExchangePub()
	cfg.CTRL.PublicKey = ctrl.PublicKey()
	cfg.CTRL.BroadcastKey = ctrl.BroadcastKey()
	return &cfg
}

const nodeInitListenerTag = "init_tls"

// generateNodeWithListener is used to create init Node
// controller will trust it
func generateNodeWithListener(t testing.TB) *node.Node {
	initializeController(t)

	cfg := generateNodeConfig(t)
	NODE, err := node.New(cfg)
	require.NoError(t, err)

	// generate certificate
	keyPairs := ctrl.GetSelfCA()
	opts := cert.Options{
		DNSNames:    []string{"localhost"},
		IPAddresses: []string{"127.0.0.1", "::1"},
	}
	caCert := keyPairs[0].Certificate
	caKey := keyPairs[0].PrivateKey
	kp, err := cert.Generate(caCert, caKey, &opts)
	require.NoError(t, err)

	// generate listener config
	listener := messages.Listener{
		Tag:     nodeInitListenerTag,
		Mode:    xnet.ModeTLS,
		Network: "tcp",
		Address: "localhost:0",
	}
	c, k := kp.EncodeToPEM()
	listener.TLSConfig.Certificates = []options.X509KeyPair{
		{Cert: string(c), Key: string(k)},
	}

	go func() {
		err := NODE.Main()
		require.NoError(t, err)
	}()
	NODE.Wait()
	require.NoError(t, NODE.AddListener(&listener))
	return NODE
}
