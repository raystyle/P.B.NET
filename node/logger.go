package node

import (
	"fmt"

	"project/internal/logger"
)

type node_log struct {
	ctx *NODE
	l   logger.Level
}

func new_logger(ctx *NODE) (*node_log, error) {
	l, err := logger.Parse(ctx.config.Log_Level)
	if err != nil {
		return nil, err
	}
	return &node_log{ctx: ctx, l: l}, nil
}

func (this *node_log) Printf(l logger.Level, src, format string, log ...interface{}) {
	if l < this.l {
		return
	}
	buffer := logger.Prefix(l, src)
	if buffer == nil {
		return
	}
	buffer.WriteString(fmt.Sprintf(format, log...))
	this.print(buffer.String())
}

func (this *node_log) Print(l logger.Level, src string, log ...interface{}) {
	if l < this.l {
		return
	}
	buffer := logger.Prefix(l, src)
	if buffer == nil {
		return
	}
	buffer.WriteString(fmt.Sprint(log...))
	this.print(buffer.String())
}

func (this *node_log) Println(l logger.Level, src string, log ...interface{}) {
	if l < this.l {
		return
	}
	buffer := logger.Prefix(l, src)
	if buffer == nil {
		return
	}
	buffer.WriteString(fmt.Sprintln(log...))
	this.print(buffer.String()[:buffer.Len()-1]) // delete "\n"
}

func (this *node_log) print(log string) {
	fmt.Println(log)
}
