package controller

import (
	"time"

	"github.com/pkg/errors"

	"project/internal/bootstrap"
	"project/internal/logger"
)

type bootstrapper struct {
	ctx         *CTRL
	tag         string
	interval    time.Duration
	stop_signal chan struct{}
	bootstrap.Bootstrap
}

func (this *CTRL) Add_Bootstrapper(b *m_bootstrapper) error {
	g := this.global
	boot, err := bootstrap.Load(b.Mode, []byte(b.Config), g.proxy, g.dns)
	if err != nil {
		e := errors.Wrapf(err, "add %s failed", b.Tag)
		this.Println(logger.ERROR, src_boot, e)
		return e
	}
	bo := &bootstrapper{
		ctx:         this,
		Bootstrap:   boot,
		tag:         b.Tag,
		interval:    time.Duration(b.Interval) * time.Second,
		stop_signal: make(chan struct{}, 1),
	}
	this.boot_m.Lock()
	defer this.boot_m.Unlock()
	if _, exist := this.boot[b.Tag]; !exist {
		this.boot[b.Tag] = bo
	} else {
		e := errors.Errorf("%s is running", b.Tag)
		this.Println(logger.ERROR, src_boot, e)
		return e
	}
	this.wg.Add(1)
	go bo.run()
	this.Printf(logger.INFO, src_boot, "add %s", b.Tag)
	return nil
}

func (this *bootstrapper) run() {
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

func (this *bootstrapper) boot() error {
	nodes, err := this.Resolve()
	if err != nil {
		return err
	}
	nodes[0] = nil
	return nil
}

func (this *bootstrapper) Stop() {
	close(this.stop_signal)
}
