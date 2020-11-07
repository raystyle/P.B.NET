// +build windows

package kiwi

import (
	"fmt"

	"golang.org/x/sys/windows"

	"project/internal/logger"
)

type lsaNT5 struct {
	ctx *Kiwi
}

func newLSA5(ctx *Kiwi) *lsaNT5 {
	return &lsaNT5{ctx: ctx}
}

func (lsa *lsaNT5) logf(lv logger.Level, format string, log ...interface{}) {
	lsa.ctx.logger.Printf(lv, "kiwi-lsa", format, log...)
}

func (lsa *lsaNT5) log(lv logger.Level, log ...interface{}) {
	lsa.ctx.logger.Println(lv, "kiwi-lsa", log...)
}

func (lsa *lsaNT5) acquireKeys(pHandle windows.Handle) error {
	fmt.Println(pHandle)
	lsa.ctx.log(logger.Info, "acquire NT5 LSA keys successfully")
	return nil
}

func (lsa *lsaNT5) Close() {
	lsa.ctx = nil
}
