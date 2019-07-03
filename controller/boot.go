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
	log_src     string
	bootstrap   bootstrap.Bootstrap
	stop_signal chan struct{}
}

func (this *CTRL) Add_Boot(m *m_boot) error {
	g := this.global
	b, err := bootstrap.Load(m.Mode, []byte(m.Config), g.proxy, g.dns)
	if err != nil {
		e := errors.Wrapf(err, "load boot %s failed", m.Tag)
		this.Println(logger.ERROR, "boot", e)
		return e
	}
	boot := &boot{
		ctx:         this,
		tag:         m.Tag,
		interval:    time.Duration(m.Interval) * time.Second,
		log_src:     "boot-" + m.Tag,
		bootstrap:   b,
		stop_signal: make(chan struct{}, 1),
	}
	this.boots_m.Lock()
	defer this.boots_m.Unlock()
	if _, exist := this.boots[m.Tag]; !exist {
		this.boots[m.Tag] = boot
	} else {
		e := errors.Errorf("boot %s is running", m.Tag)
		this.Println(logger.ERROR, "boot", e)
		return e
	}
	this.wg.Add(1)
	go boot.run()
	this.Printf(logger.INFO, "boot", "add boot %s", m.Tag)
	return nil
}

func (this *boot) run() {
	defer func() {
		this.ctx.boots_m.Lock()
		delete(this.ctx.boots, this.tag)
		this.ctx.boots_m.Unlock()
		this.ctx.wg.Done()
	}()
	b := func() {
		err := this.Resolve()
		if err != nil {
			this.ctx.Println(logger.WARNING, this.log_src, err)
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

func (this *boot) Resolve() error {
	nodes, err := this.bootstrap.Resolve()
	if err != nil {
		return err
	}
	nodes[0] = nil
	return nil
}

func (this *boot) Stop() {
	close(this.stop_signal)
}
