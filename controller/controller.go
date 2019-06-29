package controller

import (
	"os"
	"sync"

	"github.com/jinzhu/gorm"

	"project/internal/logger"
	"project/internal/protocol"
)

const (
	Name    = "P.B.NET"
	version = protocol.V1_0_0
)

type CTRL struct {
	db        *gorm.DB
	log_level logger.Level
	global    *global
	bser      map[string]*bootstrapper
	bser_m    sync.Mutex
	wg        sync.WaitGroup
}

func New(c *Config) (*CTRL, error) {
	// debug
	if c.bin_path != "" {
		err := os.Chdir(c.bin_path)
		if err != nil {
			return nil, err
		}
	}
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
		bser:      make(map[string]*bootstrapper),
	}
	// init global
	g, err := new_global(ctrl, c)
	if err != nil {
		return nil, err
	}
	ctrl.global = g
	return ctrl, nil
}

func (this *CTRL) Main() error {
	err := this.global.Start_Timesync()
	if err != nil {
		return err
	}
	now := this.global.Now().Format(logger.Time_Layout)
	this.Printf(logger.INFO, src_init, "timesync: %s", now)
	this.Print(logger.INFO, src_init, "start discover bootstrap nodes")
	bs, err := this.Select_Bootstrapper()
	if err != nil {
		this.Fatalln("select bootstrapper failed:", err)
	} else {
		for i := 0; i < len(bs); i++ {
			err := this.Add_Bootstrapper(bs[i])
			if err != nil {
				this.Fatalln("add bootstrapper failed:", err)
			}
		}
	}
	this.Print(logger.INFO, src_init, "controller is running")
	// <view> start web server
	go func() {
		this.global.Wait_Load_Keys()
		this.Print(logger.INFO, src_init, "load keys successfully")
	}()
	return nil
}

func (this *CTRL) Exit() {
	this.bser_m.Lock()
	for _, bser := range this.bser {
		bser.Stop()
	}
	this.bser_m.Unlock()
	this.wg.Wait()
	this.Print(logger.INFO, src_init, "controller is stopped")
	_ = this.db.Close()
}
