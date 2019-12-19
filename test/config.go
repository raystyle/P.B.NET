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

func initializeController(t require.TestingT) {
	initCtrlOnce.Do(func() {
		err := os.Chdir("../app")
		require.NoError(t, err)
		cfg := generateControllerConfig()
		ctrl, err = controller.New(cfg)
		require.NoError(t, err)
		// set controller keys
		err = ctrl.LoadSessionKey([]byte("pbnet"))
		require.NoError(t, err)
		go func() {
			err := ctrl.Main()
			require.NoError(t, err)
		}()
		ctrl.Wait()
	})
}

func generateControllerConfig() *controller.Config {
	c := controller.Config{}

	c.Debug.SkipTestClientDNS = true
	c.Debug.SkipSynchronizeTime = true

	c.Database.Dialect = "mysql"
	c.Database.DSN = "pbnet:pbnet@tcp(127.0.0.1:3306)/pbnet_test?loc=Local&parseTime=true"
	c.Database.MaxOpenConns = 16
	c.Database.MaxIdleConns = 16
	c.Database.LogFile = "log/database.log"
	c.Database.GORMLogFile = "log/gorm.log"
	c.Database.GORMDetailedLog = false

	c.Logger.Level = "debug"
	c.Logger.File = "log/controller.log"
	c.Logger.Writer = os.Stdout

	c.Global.DNSCacheExpire = time.Minute
	c.Global.TimeSyncInterval = time.Minute

	c.Client.Timeout = 10 * time.Second

	c.Sender.MaxConns = 16 * runtime.NumCPU()
	c.Sender.Worker = 64
	c.Sender.Timeout = 15 * time.Second
	c.Sender.QueueSize = 512
	c.Sender.MaxBufferSize = 16 << 10

	c.Syncer.ExpireTime = 3 * time.Second

	c.Worker.Number = 64
	c.Worker.QueueSize = 512
	c.Worker.MaxBufferSize = 16 << 10

	c.Web.Dir = "web"
	c.Web.CertFile = "ca/cert.pem"
	c.Web.KeyFile = "ca/key.pem"
	c.Web.Address = "localhost:1657"
	c.Web.Username = "pbnet" // # super user, password = sha256(sha256("pbnet"))
	c.Web.Password = "d6b3ced503b70f7894bd30f36001de4af84a8c2af898f06e29bca95f2dcf5100"
	return &c
}

func generateNodeConfig(tb testing.TB) *node.Config {
	cfg := node.Config{}

	cfg.Debug.SkipSynchronizeTime = true
	cfg.Debug.Broadcast = make(chan []byte, 4)
	cfg.Debug.Send = make(chan []byte, 4)

	cfg.Logger.Level = "debug"
	cfg.Logger.Writer = os.Stdout

	cfg.Global.DNSCacheExpire = 3 * time.Minute
	cfg.Global.TimeSyncInterval = 1 * time.Minute
	cfg.Global.Certificates = testdata.Certificates(tb)
	cfg.Global.ProxyClients = testdata.ProxyClients(tb)
	cfg.Global.DNSServers = testdata.DNSServers()
	cfg.Global.TimeSyncerClients = testdata.TimeSyncerClients(tb)

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

const initNodeListenerTag = "init_node_tls"

// generateNodeWithListener is used to create init Node
// controller will trust it
func generateNodeWithListener(t testing.TB) *node.Node {
	initializeController(t)

	cfg := generateNodeConfig(t)

	NODE, err := node.New(cfg)
	require.NoError(t, err)

	// generate certificate
	pks := ctrl.GetSelfCA()
	opts := cert.Options{
		DNSNames:    []string{"localhost"},
		IPAddresses: []string{"127.0.0.1", "::1"},
	}
	caCert := pks[0].Certificate
	caKey := pks[0].PrivateKey
	kp, err := cert.Generate(caCert, caKey, &opts)
	require.NoError(t, err)

	// generate listener config
	listener := messages.Listener{
		Tag:     initNodeListenerTag,
		Mode:    xnet.ModeTLS,
		Network: "tcp",
		Address: "localhost:0",
	}
	c, k := kp.EncodeToPEM()
	listener.TLSConfig.Certificates = []options.X509KeyPair{
		{
			Cert: string(c),
			Key:  string(k),
		},
	}

	go func() {
		err := NODE.Main()
		require.NoError(t, err)
	}()
	NODE.Wait()
	require.NoError(t, NODE.AddListener(&listener))
	return NODE
}
