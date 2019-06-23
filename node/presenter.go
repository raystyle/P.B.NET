package node

import (
	"project/internal/logger"
)

type presenter struct {
	ctx *NODE
}

func new_presenter(ctx *NODE) (*presenter, error) {
	p := &presenter{
		ctx: ctx,
	}
	return p, nil
}

func (this *presenter) Start() {
	this.register()
}

func (this *presenter) Shutdown() {
	this.ctx.server.Shutdown()
}

func (this *presenter) log(l logger.Level, log ...interface{}) {
	this.ctx.logger.Println(l, "presenter", log...)
}

func (this *presenter) logf(l logger.Level, format string, log ...interface{}) {
	this.ctx.logger.Printf(l, "presenter", format, log...)
}
