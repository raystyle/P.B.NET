package controller

import (
	"fmt"
	"os"
	"sync"

	"github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"

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
	boot    map[string]*boot
	boot_m  sync.Mutex
	wg      sync.WaitGroup
	once    sync.Once
	exit    chan error
}

func New(c *Config) (*CTRL, error) {
	// for test
	if c.bin_path != "" {
		err := os.Chdir(c.bin_path)
		if err != nil {
			return nil, err
		}
	}
	// init logger
	l, err := logger.Parse(c.Log_Level)
	if err != nil {
		return nil, err
	}
	// set db logger
	db_lg, err := new_db_logger(c.Dialect, c.DB_Log_Path)
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
		return nil, fmt.Errorf("connect %s server failed", c.Dialect)
	}
	db.SingularTable(true) // not add s
	// connection
	db.DB().SetMaxOpenConns(c.DB_Max_Open_Conns)
	db.DB().SetMaxIdleConns(c.DB_Max_Idle_Conns)
	// gorm logger
	gorm_lg, err := new_gorm_logger(c.GORM_Log_Path)
	if err != nil {
		return nil, err
	}
	db.SetLogger(gorm_lg)
	if c.GORM_Detailed_Log {
		db.LogMode(true)
	}
	ctrl := &CTRL{
		log_lv:  l,
		db_lg:   db_lg,
		db:      db,
		gorm_lg: gorm_lg,
	}
	// init global
	err = new_global(ctrl, c)
	if err != nil {
		return nil, err
	}
	// init http server
	err = new_web(ctrl, c)
	if err != nil {
		return nil, err
	}
	ctrl.boot = make(map[string]*boot)
	ctrl.exit = make(chan error)
	return ctrl, nil
}

func (this *CTRL) Main() error {
	// print time
	now := this.global.Now().Format(logger.Time_Layout)
	this.Printf(logger.INFO, log_init, "time: %s", now)
	// start web server
	err := this.web.Deploy()
	if err != nil {
		return this.fatal(err, "deploy web server failed")
	}
	hs_address := this.web.Address()
	this.Println(logger.INFO, log_init, "http server:", hs_address)
	this.Print(logger.INFO, log_init, "controller is running")
	// wait to load controller keys
	go func() {
		this.global.Wait_Load_Keys()
		this.Print(logger.INFO, log_init, "load keys successfully")
		// load boot
		/*
			this.Print(logger.INFO, log_init, "start discover bootstrap nodes")
			bs, err := this.Select_boot()
			if err != nil {
				return this.fatal(err, "select boot failed")
			}
			for i := 0; i < len(bs); i++ {
				err := this.Add_boot(bs[i])
				if err != nil {
					return this.fatal(err, "add boot failed")
				}
			}
		*/
	}()
	return <-this.exit
}

func (this *CTRL) fatal(err error, msg string) error {
	err = errors.WithMessage(err, msg)
	this.Println(logger.FATAL, log_init, err)
	this.Exit(nil)
	return err
}

func (this *CTRL) Exit(err error) {
	this.once.Do(func() {
		// stop all running boot
		this.boot_m.Lock()
		for _, b := range this.boot {
			b.Stop()
		}
		this.boot_m.Unlock()
		this.exit_log("all boot stopped")
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
	this.Print(logger.INFO, log_exit, log)
}

func (this *CTRL) Load_Keys(password string) error {
	return this.global.Load_Keys(password)
}
