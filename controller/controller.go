package controller

import (
	"os"
	"sync"

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
	// debug
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
	ctrl := &CTRL{log_lv: l}
	// database
	err = ctrl.connect_database(c)
	if err != nil {
		return nil, err
	}
	if c.Init_DB {
		return ctrl, nil
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
	now := this.global.Now().Format(logger.Time_Layout)
	this.Printf(logger.INFO, src_init, "timesync: %s", now)
	// <view> start web server
	err := this.web.Deploy()
	if err != nil {
		return this.fatal(err, "deploy web server failed")
	}
	hs_address := this.web.Address()
	this.Println(logger.INFO, src_init, "http server:", hs_address)
	// load boot
	this.Print(logger.INFO, src_init, "start discover bootstrap nodes")
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
	this.Print(logger.INFO, src_init, "controller is running")
	// wait to load controller keys
	go func() {
		this.global.Wait_Load_Keys()
		this.Print(logger.INFO, src_init, "load keys successfully")
	}()
	return <-this.exit
}

func (this *CTRL) fatal(err error, msg string) error {
	err = errors.WithMessage(err, msg)
	this.Println(logger.FATAL, src_init, err)
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
		if this.web != nil {
			this.web.Close()
		}
		this.wg.Wait()
		if this.global != nil {
			this.global.Close()
		}
		this.Print(logger.INFO, src_init, "controller is stopped")
		_ = this.db.Close()
		this.gorm_lg.Close()
		this.db_lg.Close()
		if this.exit != nil {
			this.exit <- err
			close(this.exit)
		}
	})
}

func (this *CTRL) Load_Keys(password string) error {
	return this.global.Load_Keys(password)
}
