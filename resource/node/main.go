package main

import (
	"bytes"
	"fmt"
	"os"
	"time"

	"project/internal/crypto/aes"
	"project/internal/crypto/ed25519"
	"project/node"
)

func main() {
	cfg := node.Config{}

	cfg.Debug.SkipSynchronizeTime = true

	cfg.Logger.Level = "debug"
	cfg.Logger.Writer = os.Stdout

	cfg.Global.DNSCacheExpire = 3 * time.Minute
	cfg.Global.TimeSyncInterval = 1 * time.Minute
	// cfg.Global.Certificates = testdata.Certificates(tb)
	// cfg.Global.ProxyClients = testdata.ProxyClients(tb)
	// cfg.Global.DNSServers = testdata.DNSServers()
	// cfg.Global.TimeSyncerClients = testdata.TimeSyncerClients(tb)

	cfg.Client.ProxyTag = "balance"
	cfg.Client.Timeout = 15 * time.Second

	cfg.Forwarder.MaxCtrlConns = 10
	cfg.Forwarder.MaxNodeConns = 8
	cfg.Forwarder.MaxBeaconConns = 128

	cfg.Sender.Worker = 64
	cfg.Sender.QueueSize = 512
	cfg.Sender.MaxBufferSize = 512 << 10
	cfg.Sender.Timeout = 15 * time.Second

	cfg.Syncer.ExpireTime = 30 * time.Second

	cfg.Worker.Number = 16
	cfg.Worker.QueueSize = 1024
	cfg.Worker.MaxBufferSize = 16384

	cfg.Server.MaxConns = 10
	cfg.Server.Timeout = 15 * time.Second

	cfg.CTRL.ExPublicKey = bytes.Repeat([]byte{255}, 32)
	cfg.CTRL.PublicKey = bytes.Repeat([]byte{255}, ed25519.PublicKeySize)
	cfg.CTRL.BroadcastKey = bytes.Repeat([]byte{255}, aes.Key256Bit+aes.IVSize)

	fmt.Println(node.New(&cfg))
}
