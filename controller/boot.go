package controller

import (
	"time"

	"github.com/pkg/errors"

	"project/internal/bootstrap"
	"project/internal/logger"
)

type boot struct {
	ctx         *CTRL
	tag         string
	interval    time.Duration
	stop_signal chan struct{}
	bootstrap.Bootstrap
}

func (this *CTRL) Add_boot(m *m_boot) error {
	g := this.global
	b, err := bootstrap.Load(m.Mode, []byte(m.Config), g.proxy, g.dns)
	if err != nil {
		e := errors.Wrapf(err, "add %s failed", m.Tag)
		this.Println(logger.ERROR, log_boot, e)
		return e
	}
	boot := &boot{
		ctx:         this,
		Bootstrap:   b,
		tag:         m.Tag,
		interval:    time.Duration(m.Interval) * time.Second,
		stop_signal: make(chan struct{}, 1),
	}
	this.boot_m.Lock()
	defer this.boot_m.Unlock()
	if _, exist := this.boot[m.Tag]; !exist {
		this.boot[m.Tag] = boot
	} else {
		e := errors.Errorf("%s is running", m.Tag)
		this.Println(logger.ERROR, log_boot, e)
		return e
	}
	this.wg.Add(1)
	go boot.run()
	this.Printf(logger.INFO, log_boot, "add %s", m.Tag)
	return nil
}

func (this *boot) run() {
	log_src := "boot-" + this.tag
	defer func() {
		this.ctx.boot_m.Lock()
		delete(this.ctx.boot, this.tag)
		this.ctx.boot_m.Unlock()
		this.ctx.wg.Done()
	}()
	b := func() {
		err := this.boot()
		if err != nil {
			this.ctx.Println(logger.WARNING, log_src, err)
		}
		this.Stop()
	}
	b()
	for {
		select {
		case <-time.After(this.interval):
			b()
		case <-this.stop_signal:
			return
		}
	}
}

func (this *boot) boot() error {
	nodes, err := this.Resolve()
	if err != nil {
		return err
	}
	nodes[0] = nil
	return nil
}

func (this *boot) Stop() {
	close(this.stop_signal)
}
