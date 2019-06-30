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
	db        *gorm.DB
	log_level logger.Level
	global    *global
	web       *web
	boot      map[string]*bootstrapper
	boot_m    sync.Mutex
	wg        sync.WaitGroup
	exit      chan struct{}
}

func New(c *Config) (*CTRL, error) {
	// debug
	if c.bin_path != "" {
		err := os.Chdir(c.bin_path)
		if err != nil {
			return nil, err
		}
	}
	// init database
	db, err := connect_database(c)
	if err != nil {
		return nil, err
	}
	// init logger
	l, err := logger.Parse(c.Log_Level)
	if err != nil {
		return nil, err
	}
	ctrl := &CTRL{
		db:        db,
		log_level: l,
		boot:      make(map[string]*bootstrapper),
		exit:      make(chan struct{}),
	}
	// init global
	g, err := new_global(ctrl, c)
	if err != nil {
		return nil, err
	}
	ctrl.global = g
	// sync time
	err = ctrl.global.Start_Timesync()
	if err != nil {
		return nil, err
	}
	// init http server
	hs, err := new_web(ctrl, c)
	if err != nil {
		return nil, err
	}
	ctrl.web = hs
	return ctrl, nil
}

func (this *CTRL) Main() error {
	now := this.global.Now().Format(logger.Time_Layout)
	this.Printf(logger.INFO, src_init, "timesync: %s", now)
	// <view> start web server
	err := this.web.Deploy()
	if err != nil {
		err = errors.WithMessage(err, "start web server failed")
		this.Fatalln(err)
		return err
	}
	hs_address := this.web.Address()
	this.Println(logger.INFO, src_init, "http server:", hs_address)
	// load bootstrapper
	this.Print(logger.INFO, src_init, "start discover bootstrap nodes")
	bs, err := this.Select_Bootstrapper()
	if err != nil {
		err = errors.WithMessage(err, "select bootstrapper failed")
		this.Fatalln(err)
		return err
	}
	for i := 0; i < len(bs); i++ {
		err := this.Add_Bootstrapper(bs[i])
		if err != nil {
			err = errors.WithMessage(err, "add bootstrapper failed")
			this.Fatalln(err)
			return err
		}
	}
	this.Print(logger.INFO, src_init, "controller is running")
	// wait to load controller keys
	this.global.Wait_Load_Keys()
	this.Print(logger.INFO, src_init, "load keys successfully")
	<-this.exit
	return nil
}

func (this *CTRL) Exit() {
	this.boot_m.Lock()
	for _, b := range this.boot {
		b.Stop()
	}
	this.boot_m.Unlock()
	this.web.Close()
	this.wg.Wait()
	this.Print(logger.INFO, src_init, "controller is stopped")
	_ = this.db.Close()
	close(this.exit)
}
