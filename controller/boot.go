package controller

import (
	"sync"
	"time"

	"github.com/pkg/errors"

	"project/internal/bootstrap"
	"project/internal/logger"
	"project/internal/xpanic"
)

type boot struct {
	ctx        *CTRL
	tag        string
	interval   time.Duration
	logSrc     string
	bootstrap  bootstrap.Bootstrap
	once       sync.Once
	stopSignal chan struct{}
}

func (ctrl *CTRL) AddBoot(m *mBoot) error {
	g := ctrl.global
	b, err := bootstrap.Load(m.Mode, []byte(m.Config), g.proxyPool, g.dnsClient)
	if err != nil {
		return errors.Wrapf(err, "load boot %s failed", m.Tag)
	}
	boot := &boot{
		ctx:        ctrl,
		tag:        m.Tag,
		interval:   time.Duration(m.Interval) * time.Second,
		logSrc:     "boot-" + m.Tag,
		bootstrap:  b,
		stopSignal: make(chan struct{}),
	}
	ctrl.bootsM.Lock()
	defer ctrl.bootsM.Unlock()
	if _, ok := ctrl.boots[m.Tag]; !ok {
		ctrl.boots[m.Tag] = boot
	} else {
		return errors.Errorf("boot %s is running", m.Tag)
	}
	ctrl.wg.Add(1)
	go boot.Boot()
	ctrl.Printf(logger.INFO, "boot", "add boot %s", m.Tag)
	return nil
}

func (boot *boot) Boot() {
	defer func() {
		boot.ctx.bootsM.Lock()
		delete(boot.ctx.boots, boot.tag)
		boot.ctx.bootsM.Unlock()
		boot.ctx.Printf(logger.INFO, "boot", "boot %s stop", boot.tag)
		boot.ctx.wg.Done()
	}()
	f := func() {
		err := boot.resolve()
		if err != nil {
			boot.ctx.Println(logger.WARNING, boot.logSrc, err)
		} else {
			boot.Stop()
		}
	}
	f()
	for {
		select {
		case <-time.After(boot.interval):
			f()
		case <-boot.stopSignal:
			return
		}
	}
}

func (boot *boot) resolve() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = xpanic.Error("boot.resolve() panic:", r)
			boot.ctx.Print(logger.FATAL, boot.logSrc, err)
		}
	}()
	nodes, err := boot.bootstrap.Resolve()
	if err != nil {
		return
	}
	nodes[0] = nil
	return
}

func (boot *boot) Stop() {
	boot.once.Do(func() {
		close(boot.stopSignal)
	})
}
