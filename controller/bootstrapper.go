package controller

import (
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"

	"project/internal/bootstrap"
	"project/internal/logger"
)

type bootstrapper struct {
	ctx         *CTRL
	tag         string
	ticker      *time.Ticker
	stop_signal chan struct{}
	bootstrap.Bootstrap
}

func (this *CTRL) Add_Bootstrapper(b *m_bootstrapper) error {
	g := this.global
	boot, err := bootstrap.Load(b.Mode, []byte(b.Config), g.proxy, g.dns)
	if err != nil {
		e := errors.Wrapf(err, "add %s failed", b.Tag)
		this.Println(logger.ERROR, src_bs, e)
		return e
	}
	interval := time.Duration(b.Interval) * time.Second
	bser := &bootstrapper{
		ctx:         this,
		Bootstrap:   boot,
		tag:         b.Tag,
		ticker:      time.NewTicker(interval),
		stop_signal: make(chan struct{}, 1),
	}
	this.bser_m.Lock()
	defer this.bser_m.Unlock()
	if _, exist := this.bser[b.Tag]; !exist {
		this.bser[b.Tag] = bser
	} else {
		e := errors.Errorf("%s is running", b.Tag)
		this.Println(logger.ERROR, src_bs, e)
		return e
	}
	this.wg.Add(1)
	go bser.run()
	this.Printf(logger.INFO, src_bs, "add %s", b.Tag)
	return nil
}

func (this *bootstrapper) run() {
	log_src := "bser-" + this.tag
	defer func() {
		this.ticker.Stop()
		this.ctx.bser_m.Lock()
		delete(this.ctx.bser, this.tag)
		this.ctx.bser_m.Unlock()
		this.ctx.wg.Done()
	}()
	b := func() {
		err := this.boot()
		if err != nil {
			this.ctx.Println(logger.WARNING, log_src, err)
		}
	}
	b()
	for {
		select {
		case <-this.ticker.C:
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
	spew.Dump(nodes)
	return nil
}

func (this *bootstrapper) Stop() {
	close(this.stop_signal)
}
