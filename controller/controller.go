package controller

import (
	"fmt"
	"sync"

	"github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"

	"project/internal/dns"
	"project/internal/logger"
	"project/internal/proxy"
	"project/internal/timesync"
)

const (
	Name = "P.B.NET"
)

type CTRL struct {
	debug    *Debug
	logLv    logger.Level
	db       *gorm.DB
	dbLg     *dbLogger
	gormLg   *gormLogger
	global   *global
	web      *web
	boots    map[string]*boot // discover bootstrap node
	bootsM   sync.Mutex
	syncers  map[string]*syncer // sync message
	syncersM sync.RWMutex
	sender   *sender // broadcast and send message
	wg       sync.WaitGroup
	once     sync.Once
	wait     chan struct{}
	exit     chan error
}

func New(cfg *Config) (*CTRL, error) {
	// init logger
	lv, err := logger.Parse(cfg.LogLevel)
	if err != nil {
		return nil, err
	}
	// set db logger
	dbLg, err := newDBLogger(cfg.Dialect, cfg.DBLogFile)
	if err != nil {
		return nil, err
	}
	// if you need, add DB Driver
	switch cfg.Dialect {
	case "mysql":
		_ = mysql.SetLogger(dbLg)
	default:
		return nil, fmt.Errorf("unknown dialect: %s", cfg.Dialect)
	}
	// connect database
	db, err := gorm.Open(cfg.Dialect, cfg.DSN)
	if err != nil {
		return nil, errors.Wrapf(err, "connect %s server failed", cfg.Dialect)
	}
	db.SingularTable(true) // not add s
	// connection
	db.DB().SetMaxOpenConns(cfg.DBMaxOpenConns)
	db.DB().SetMaxIdleConns(cfg.DBMaxIdleConns)
	// gorm logger
	gormLg, err := newGormLogger(cfg.GORMLogFile)
	if err != nil {
		return nil, err
	}
	db.SetLogger(gormLg)
	if cfg.GORMDetailedLog {
		db.LogMode(true)
	}
	// copy debug config
	debug := cfg.Debug
	ctrl := &CTRL{
		debug:  &debug,
		logLv:  lv,
		db:     db,
		dbLg:   dbLg,
		gormLg: gormLg,
	}
	// init global
	g, err := newGlobal(ctrl, cfg)
	if err != nil {
		return nil, err
	}
	ctrl.global = g
	// load proxy clients from database
	pcs, err := ctrl.SelectProxyClient()
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
	dss, err := ctrl.SelectDNSServer()
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
	tcs, err := ctrl.SelectTimeSyncer()
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
	// init sender
	sender, err := newSender(ctrl, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "init sender failed")
	}
	ctrl.sender = sender
	// init http server
	web, err := newWeb(ctrl, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "init web server failed")
	}
	ctrl.web = web
	ctrl.boots = make(map[string]*boot)
	ctrl.wait = make(chan struct{}, 2)
	ctrl.exit = make(chan error, 1)
	return ctrl, nil
}

func (ctrl *CTRL) Main() error {
	defer func() { ctrl.wait <- struct{}{} }()
	// first synchronize time
	if !ctrl.debug.SkipTimeSyncer {
		err := ctrl.global.StartTimeSyncer()
		if err != nil {
			return ctrl.fatal(err, "synchronize time failed")
		}
	}
	now := ctrl.global.Now().Format(logger.TimeLayout)
	ctrl.Println(logger.INFO, "init", "time:", now)
	// start web server
	err := ctrl.web.Deploy()
	if err != nil {
		return ctrl.fatal(err, "deploy web server failed")
	}
	ctrl.Println(logger.INFO, "init", "http server:", ctrl.web.Address())
	ctrl.Print(logger.INFO, "init", "controller is running")
	go func() {
		// wait to load controller keys
		ctrl.global.WaitLoadKeys()
		ctrl.Print(logger.INFO, "init", "load keys successfully")
		// load boot
		ctrl.Print(logger.INFO, "init", "start discover bootstrap nodes")
		boots, err := ctrl.SelectBoot()
		if err != nil {
			ctrl.Println(logger.ERROR, "init", "select boot failed:", err)
			return
		}
		for i := 0; i < len(boots); i++ {
			err = ctrl.AddBoot(boots[i])
			if err != nil {
				ctrl.Print(logger.ERROR, "boot", err)
			}
		}

	}()
	ctrl.wait <- struct{}{}
	return <-ctrl.exit
}

func (ctrl *CTRL) fatal(err error, msg string) error {
	err = errors.WithMessage(err, msg)
	ctrl.Println(logger.FATAL, "init", err)
	ctrl.Exit(nil)
	return err
}

// for Test wait for Main()
func (ctrl *CTRL) Wait() {
	<-ctrl.wait
}

func (ctrl *CTRL) Exit(err error) {
	ctrl.once.Do(func() {
		// stop all running boot
		ctrl.bootsM.Lock()
		for _, boot := range ctrl.boots {
			boot.Stop()
		}
		ctrl.bootsM.Unlock()
		ctrl.Print(logger.INFO, "exit", "all boots stopped")
		ctrl.web.Close()
		ctrl.Print(logger.INFO, "exit", "web server is stopped")
		ctrl.wg.Wait()
		ctrl.global.Destroy()
		ctrl.Print(logger.INFO, "exit", "global is stopped")
		ctrl.Print(logger.INFO, "exit", "controller is stopped")
		_ = ctrl.db.Close()
		ctrl.gormLg.Close()
		ctrl.dbLg.Close()
		ctrl.exit <- err
		close(ctrl.exit)
	})
}

func (ctrl *CTRL) LoadKeys(password string) error {
	return ctrl.global.LoadKeys(password)
}
