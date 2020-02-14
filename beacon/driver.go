package beacon

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"

	"project/internal/bootstrap"
	"project/internal/guid"
	"project/internal/logger"
	"project/internal/xpanic"
)

type driver struct {
	ctx *Beacon

	// store node listeners
	nodeListeners        map[guid.GUID]map[uint64]*bootstrap.Listener
	nodeListenersIndexes map[guid.GUID]uint64
	nodeListenersRWM     sync.RWMutex

	// about random.Sleep() in query
	sleepFixed  atomic.Value
	sleepRandom atomic.Value

	context context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func newDriver(ctx *Beacon, config *Config) (*driver, error) {
	cfg := config.Driver

	if cfg.SleepFixed < 5 {
		return nil, errors.New("driver SleepFixed must >= 5")
	}
	if cfg.SleepRandom < 10 {
		return nil, errors.New("driver SleepRandom must >= 10")
	}

	driver := driver{
		ctx: ctx,
	}
	sleepFixed := cfg.SleepFixed
	sleepRandom := cfg.SleepRandom
	driver.sleepFixed.Store(sleepFixed)
	driver.sleepRandom.Store(sleepRandom)
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
	driver.ctx = nil
}

// AddNodeListeners is used to add Node listeners(must be encrypted)
func (driver *driver) AddNodeListeners(guid *guid.GUID, listeners ...*bootstrap.Listener) error {

	return nil
}

// DeleteNodeListener is used to delete Node listener
func (driver *driver) DeleteNodeListener(guid *guid.GUID, index uint64) error {

	return nil
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
			driver.log(logger.Fatal, xpanic.Print(r, "driver.drive"))
			// restart driver
			time.Sleep(time.Second)
			go driver.drive()
		} else {
			driver.wg.Done()
		}
	}()
}
