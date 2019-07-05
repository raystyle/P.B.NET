package controller

import (
	"fmt"
	"sync"

	"github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"

	"project/internal/global/dnsclient"
	"project/internal/global/proxyclient"
	"project/internal/global/timesync"
	"project/internal/logger"
	"project/internal/protocol"
)

const (
	Name    = "P.B.NET"
	Version = protocol.V1_0_0
)

type CTRL struct {
	log_lv  logger.Level
	db      *gorm.DB
	db_lg   *db_logger
	gorm_lg *gorm_logger
	global  *global
	web     *web
	boots   map[string]*boot
	boots_m sync.Mutex
	wg      sync.WaitGroup
	once    sync.Once
	exit    chan error
}

func New(c *Config) (*CTRL, error) {
	// init logger
	lv, err := logger.Parse(c.Log_Level)
	if err != nil {
		return nil, err
	}
	// set db logger
	db_lg, err := new_db_logger(c.Dialect, c.DB_Log_File)
	if err != nil {
		return nil, err
	}
	// if you need, add DB Driver
	switch c.Dialect {
	case "mysql":
		_ = mysql.SetLogger(db_lg)
	default:
		return nil, fmt.Errorf("unknown dialect: %s", c.Dialect)
	}
	// connect database
	db, err := gorm.Open(c.Dialect, c.DSN)
	if err != nil {
		return nil, errors.Wrapf(err, "connect %s server failed", c.Dialect)
	}
	db.SingularTable(true) // not add s
	// connection
	db.DB().SetMaxOpenConns(c.DB_Max_Open_Conns)
	db.DB().SetMaxIdleConns(c.DB_Max_Idle_Conns)
	// gorm logger
	gorm_lg, err := new_gorm_logger(c.GORM_Log_File)
	if err != nil {
		return nil, err
	}
	db.SetLogger(gorm_lg)
	if c.GORM_Detailed_Log {
		db.LogMode(true)
	}
	ctrl := &CTRL{
		log_lv:  lv,
		db:      db,
		db_lg:   db_lg,
		gorm_lg: gorm_lg,
	}
	// init global
	g, err := new_global(ctrl, c)
	if err != nil {
		return nil, err
	}
	ctrl.global = g
	// load proxy clients from database
	pcs, err := ctrl.Select_Proxy_Client()
	if err != nil {
		return nil, errors.Wrap(err, "load proxy clients failed")
	}
	for i := 0; i < len(pcs); i++ {
		tag := pcs[i].Tag
		c := &proxyclient.Client{
			Mode:   pcs[i].Mode,
			Config: pcs[i].Config,
		}
		err = ctrl.global.Add_Proxy_Client(tag, c)
		if err != nil {
			return nil, errors.Wrapf(err, "add proxy client %s failed", tag)
		}
	}
	// load dns clients from database
	dcs, err := ctrl.Select_DNS_Client()
	if err != nil {
		return nil, errors.Wrap(err, "load dns clients failed")
	}
	for i := 0; i < len(dcs); i++ {
		tag := dcs[i].Tag
		c := &dnsclient.Client{
			Method:  dcs[i].Method,
			Address: dcs[i].Address,
		}
		err = ctrl.global.Add_DNS_Client(tag, c)
		if err != nil {
			return nil, errors.Wrapf(err, "add dns client %s failed", tag)
		}
	}
	// load timesync client from database
	ts, err := ctrl.Select_Timesync()
	if err != nil {
		return nil, errors.Wrap(err, "load timesync clients failed")
	}
	for i := 0; i < len(ts); i++ {
		c := &timesync.Client{}
		err = toml.Unmarshal([]byte(ts[i].Config), c)
		if err != nil {
			return nil, errors.Wrapf(err, "load timesync: %s failed", ts[i].Tag)
		}
		tag := ts[i].Tag
		err = ctrl.global.Add_Timesync_Client(tag, c)
		if err != nil {
			return nil, errors.Wrapf(err, "add timesync client %s failed", tag)
		}
	}
	// init http server
	web, err := new_web(ctrl, c)
	if err != nil {
		return nil, errors.WithMessage(err, "init web server failed")
	}
	ctrl.web = web
	ctrl.boots = make(map[string]*boot)
	ctrl.exit = make(chan error)
	return ctrl, nil
}

func (this *CTRL) Main() error {
	// first synchronize time
	err := this.global.Start_Timesync()
	if err != nil {
		return this.fatal(err, "synchronize time failed")
	}
	now := this.global.Now().Format(logger.Time_Layout)
	this.Printf(logger.INFO, "init", "time: %s", now)
	// start web server
	err = this.web.Deploy()
	if err != nil {
		return this.fatal(err, "deploy web server failed")
	}
	hs_address := this.web.Address()
	this.Println(logger.INFO, "init", "http server:", hs_address)
	this.Print(logger.INFO, "init", "controller is running")
	go func() {
		// wait to load controller keys
		this.global.Wait_Load_Keys()
		this.Print(logger.INFO, "init", "load keys successfully")
		// load boot
		this.Print(logger.INFO, "init", "start discover bootstrap nodes")
		bs, err := this.Select_Boot()
		if err != nil {
			this.Println(logger.ERROR, "init", "select boot failed:", err)
		}
		for i := 0; i < len(bs); i++ {
			_ = this.Add_Boot(bs[i])
		}
	}()
	return <-this.exit
}

func (this *CTRL) fatal(err error, msg string) error {
	err = errors.WithMessage(err, msg)
	this.Println(logger.FATAL, "init", err)
	this.Exit(nil)
	return err
}

func (this *CTRL) Exit(err error) {
	this.once.Do(func() {
		// stop all running boot
		this.boots_m.Lock()
		for _, boot := range this.boots {
			boot.Stop()
		}
		this.boots_m.Unlock()
		this.exit_log("all boots stopped")
		this.web.Close()
		this.exit_log("web server is stopped")
		this.wg.Wait()
		this.global.Close()
		this.exit_log("global is stopped")
		this.exit_log("controller is stopped")
		_ = this.db.Close()
		this.gorm_lg.Close()
		this.db_lg.Close()
		if this.exit != nil {
			this.exit <- err
			close(this.exit)
		}
	})
}

func (this *CTRL) exit_log(log string) {
	this.Print(logger.INFO, "exit", log)
}

func (this *CTRL) Load_Keys(password string) error {
	return this.global.Load_Keys(password)
}
