package beacon

import (
	"context"
	"sync"
	"time"

	"project/internal/logger"
	"project/internal/xpanic"
)

type driver struct {
	ctx *Beacon

	context context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func newDriver(ctx *Beacon, config *Config) (*driver, error) {
	driver := driver{
		ctx: ctx,
	}
	driver.context, driver.cancel = context.WithCancel(context.Background())
	return &driver, nil
}

func (driver *driver) Drive() {
	driver.wg.Add(1)
	go driver.drive()
}

func (driver *driver) Close() {
	driver.cancel()
	driver.wg.Wait()
}

func (driver *driver) logf(lv logger.Level, format string, log ...interface{}) {
	driver.ctx.logger.Printf(lv, "driver", format, log...)
}

func (driver *driver) log(lv logger.Level, log ...interface{}) {
	driver.ctx.logger.Println(lv, "driver", log...)
}

func (driver *driver) drive() {
	defer func() {
		if r := recover(); r != nil {
			driver.log(logger.Fatal, xpanic.Print(r, "driver.Drive"))
			// restart driver
			time.Sleep(time.Second)
			go driver.Drive()
		} else {
			driver.wg.Done()
		}
	}()

}
