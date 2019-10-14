package controller

import (
	"encoding/hex"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"

	"project/internal/bootstrap"
	"project/internal/dns"
	"project/internal/logger"
	"project/internal/proxy"
	"project/internal/timesync"
	"project/internal/xpanic"
)

var (
	ErrCtrlClosed = errors.New("Controller closed")
	ErrMaxClient  = errors.New("max client")
)

type CTRL struct {
	Debug *Debug

	db      *db      // database
	global  *global  // proxy, dns, time syncer, and ...
	handler *handler // handle message from Node or Beacon
	syncer  *syncer  // sync message
	sender  *sender  // broadcast and send message
	boot    *boot    // auto discover bootstrap nodes
	web     *web     // web server

	logLevel  logger.Level
	maxClient atomic.Value

	// key=hex(guid)
	clients    map[string]*syncerClient
	clientsRWM sync.RWMutex

	inShutdown int32
	closeOnce  sync.Once
	stopSignal chan struct{}
	wg         sync.WaitGroup
	wait       chan struct{}
	exit       chan error
}

func New(cfg *Config) (*CTRL, error) {
	// init logger
	logLevel, err := logger.Parse(cfg.LogLevel)
	if err != nil {
		return nil, err
	}
	// copy debug config
	debug := cfg.Debug
	ctrl := &CTRL{
		Debug:    &debug,
		logLevel: logLevel,
	}
	ctrl.maxClient.Store(cfg.MaxSyncerClient)
	// init database
	db, err := newDB(ctrl, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "init database failed")
	}
	ctrl.db = db
	// init global
	global, err := newGlobal(ctrl, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "init global failed")
	}
	ctrl.global = global
	// handler
	ctrl.handler = &handler{ctx: ctrl}
	// load proxy clients from database
	pcs, err := ctrl.db.SelectProxyClient()
	if err != nil {
		return nil, errors.Wrap(err, "load proxy clients failed")
	}
	for i := 0; i < len(pcs); i++ {
		tag := pcs[i].Tag
		client := &proxy.Client{
			Mode:   pcs[i].Mode,
			Config: pcs[i].Config,
		}
		err = ctrl.global.AddProxyClient(tag, client)
		if err != nil {
			return nil, errors.Wrapf(err, "add proxy client %s failed", tag)
		}
	}
	// load dns servers from database
	dss, err := ctrl.db.SelectDNSServer()
	if err != nil {
		return nil, errors.Wrap(err, "load dns servers failed")
	}
	for i := 0; i < len(dss); i++ {
		tag := dss[i].Tag
		server := &dns.Server{
			Method:  dss[i].Method,
			Address: dss[i].Address,
		}
		err = ctrl.global.AddDNSSever(tag, server)
		if err != nil {
			return nil, errors.Wrapf(err, "add dns server %s failed", tag)
		}
	}
	// load time syncer configs from database
	tcs, err := ctrl.db.SelectTimeSyncer()
	if err != nil {
		return nil, errors.Wrap(err, "select time syncer failed")
	}
	for i := 0; i < len(tcs); i++ {
		cfg := &timesync.Config{}
		err = toml.Unmarshal([]byte(tcs[i].Config), cfg)
		if err != nil {
			return nil, errors.Wrapf(err, "load time syncer config: %s failed", tcs[i].Tag)
		}
		tag := tcs[i].Tag
		err = ctrl.global.AddTimeSyncerConfig(tag, cfg)
		if err != nil {
			return nil, errors.Wrapf(err, "add time syncer config %s failed", tag)
		}
	}
	// init syncer
	syncer, err := newSyncer(ctrl, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "init syncer failed")
	}
	ctrl.syncer = syncer
	// init sender
	sender, err := newSender(ctrl, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "init sender failed")
	}
	ctrl.sender = sender
	// init boot
	ctrl.boot = newBoot(ctrl)
	// init http server
	web, err := newWeb(ctrl, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "init web server failed")
	}
	ctrl.web = web
	// wait and exit
	ctrl.wait = make(chan struct{}, 2)
	ctrl.exit = make(chan error, 1)
	return ctrl, nil
}

func (ctrl *CTRL) Main() error {
	defer func() { ctrl.wait <- struct{}{} }()
	// first synchronize time
	if !ctrl.Debug.SkipTimeSyncer {
		err := ctrl.global.StartTimeSyncer()
		if err != nil {
			return ctrl.fatal(err, "synchronize time failed")
		}
	}
	now := ctrl.global.Now().Format(logger.TimeLayout)
	ctrl.Println(logger.Info, "init", "time:", now)
	// start web server
	err := ctrl.web.Deploy()
	if err != nil {
		return ctrl.fatal(err, "deploy web server failed")
	}
	ctrl.Println(logger.Info, "init", "http server:", ctrl.web.Address())
	ctrl.Print(logger.Info, "init", "controller is running")
	go func() {
		// wait to load controller keys
		ctrl.global.WaitLoadKeys()
		ctrl.Print(logger.Info, "init", "load keys successfully")
		// load boots
		ctrl.Print(logger.Info, "init", "start discover bootstrap nodes")
		boots, err := ctrl.db.SelectBoot()
		if err != nil {
			ctrl.Println(logger.Error, "init", "select boot failed:", err)
			return
		}
		for i := 0; i < len(boots); i++ {
			_ = ctrl.boot.Add(boots[i])
		}
	}()
	ctrl.wait <- struct{}{}
	return <-ctrl.exit
}

func (ctrl *CTRL) Exit(err error) {
	ctrl.closeOnce.Do(func() {
		ctrl.web.Close()
		ctrl.Print(logger.Info, "exit", "web server is stopped")
		ctrl.boot.Close()
		ctrl.Print(logger.Info, "exit", "boot is stopped")
		ctrl.sender.Close()
		ctrl.Print(logger.Info, "exit", "sender is stopped")
		ctrl.syncer.Close()
		ctrl.Print(logger.Info, "exit", "syncer is stopped")
		ctrl.global.Destroy()
		ctrl.Print(logger.Info, "exit", "global is stopped")
		ctrl.Print(logger.Info, "exit", "controller is stopped")
		ctrl.db.Close()
		ctrl.exit <- err
		close(ctrl.exit)
		// TODO clean point
	})
}

func (ctrl *CTRL) fatal(err error, msg string) error {
	err = errors.WithMessage(err, msg)
	ctrl.Println(logger.Fatal, "init", err)
	ctrl.Exit(nil)
	return err
}

func (ctrl *CTRL) LoadKeys(password string) error {
	return ctrl.global.LoadKeys(password)
}

func (ctrl *CTRL) shuttingDown() bool {
	return atomic.LoadInt32(&ctrl.inShutdown) != 0
}

// getMaxSyncerClient is used to get current max syncer client number
func (ctrl *CTRL) getMaxClient() int {
	return ctrl.maxClient.Load().(int)
}

// Connect is used to connect node for sync message
func (ctrl *CTRL) Connect(node *bootstrap.Node, guid []byte) error {
	if ctrl.shuttingDown() {
		return ErrCtrlClosed
	}
	ctrl.clientsRWM.Lock()
	defer ctrl.clientsRWM.Unlock()
	if len(ctrl.clients) >= ctrl.getMaxClient() {
		return ErrMaxClient
	}
	key := hex.EncodeToString(guid)
	if _, ok := ctrl.clients[key]; ok {
		return errors.Errorf("connect same node %s %s", node.Mode, node.Address)
	}
	cfg := clientCfg{
		Node:     node,
		NodeGUID: guid,
	}
	sc, err := newSyncerClient(ctrl, &cfg)
	if err != nil {
		return errors.WithMessage(err, "connect node failed")
	}
	ctrl.clients[key] = sc
	ctrl.Printf(logger.Info, "connect", "connect node %s", node.Address)
	return nil
}

func (ctrl *CTRL) Disconnect(guid string) error {
	guid = strings.ToLower(guid)
	ctrl.clientsRWM.RLock()
	if sc, ok := ctrl.clients[guid]; ok {
		ctrl.clientsRWM.RUnlock()
		sc.Close()
		ctrl.Printf(logger.Info, "disconnect", "disconnect node %s %s",
			sc.Node.Mode, sc.Node.Address)
		return nil
	} else {
		ctrl.clientsRWM.RUnlock()
		return errors.Errorf("syncer client %s doesn't exist", strings.ToUpper(guid))
	}
}

func (ctrl *CTRL) SyncerClients() map[string]*syncerClient {
	ctrl.clientsRWM.RLock()
	defer ctrl.clientsRWM.RUnlock()
	// copy map
	sClients := make(map[string]*syncerClient, len(ctrl.clients))
	for key, client := range ctrl.clients {
		sClients[key] = client
	}
	return sClients
}

func (ctrl *CTRL) DeleteNode(guid []byte) error {
	err := ctrl.db.DeleteNode(guid)
	if err != nil {
		return errors.Wrapf(err, "delete node %X failed", guid)
	}
	return nil
}

func (ctrl *CTRL) DeleteNodeUnscoped(guid []byte) error {
	err := ctrl.db.DeleteNodeUnscoped(guid)
	if err != nil {
		return errors.Wrapf(err, "unscoped delete node %X failed", guid)
	}
	return nil
}

func (ctrl *CTRL) DeleteBeacon(guid []byte) error {
	err := ctrl.db.DeleteBeacon(guid)
	if err != nil {
		return errors.Wrapf(err, "delete beacon %X failed", guid)
	}
	return nil
}

func (ctrl *CTRL) DeleteBeaconUnscoped(guid []byte) error {
	err := ctrl.db.DeleteBeaconUnscoped(guid)
	if err != nil {
		return errors.Wrapf(err, "unscoped delete beacon %X failed", guid)
	}
	return nil
}

// TODO watcher

// watcher is used to check the number of connect nodes
// connected nodes number < syncer.maxClient, try to connect more node
func (ctrl *CTRL) watcher() {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error("watcher panic:", r)
			ctrl.Print(logger.Fatal, "watcher", err)
			// restart watcher
			time.Sleep(time.Second)
			go ctrl.watcher()
		} else {
			ctrl.wg.Done()
		}
	}()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	isMax := func() bool {
		ctrl.clientsRWM.RLock()
		l := len(ctrl.clients)
		ctrl.clientsRWM.RUnlock()
		return l >= ctrl.getMaxClient()
	}
	watch := func() {
		if isMax() {
			return
		}
		// select nodes
		// TODO watcher
	}
	for {
		select {
		case <-ticker.C:
			watch()
		case <-ctrl.stopSignal:
			return
		}
	}
}

// ------------------------------------test-------------------------------------

// TestWait is used to wait for Main()
func (ctrl *CTRL) TestWait() {
	<-ctrl.wait
}
