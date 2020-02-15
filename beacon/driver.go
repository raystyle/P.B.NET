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
	"project/internal/random"
	"project/internal/xpanic"
)

type driver struct {
	ctx *Beacon

	// store node listeners
	nodeListeners      map[guid.GUID]map[uint64]*bootstrap.Listener
	nodeListenersIndex uint64
	nodeListenersRWM   sync.RWMutex

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
		ctx:           ctx,
		nodeListeners: make(map[guid.GUID]map[uint64]*bootstrap.Listener),
	}
	sleepFixed := cfg.SleepFixed
	sleepRandom := cfg.SleepRandom
	driver.sleepFixed.Store(sleepFixed)
	driver.sleepRandom.Store(sleepRandom)
	driver.context, driver.cancel = context.WithCancel(context.Background())
	return &driver, nil
}

func (driver *driver) Drive() {
	driver.wg.Add(2)
	go driver.clientWatcher()
	go driver.queryLoop()
}

func (driver *driver) Close() {
	driver.cancel()
	driver.wg.Wait()
	driver.ctx = nil
}

func (driver *driver) getNodeListeners() map[guid.GUID]map[uint64]*bootstrap.Listener {
	nodeListeners := make(map[guid.GUID]map[uint64]*bootstrap.Listener)
	driver.nodeListenersRWM.RLock()
	defer driver.nodeListenersRWM.RUnlock()
	for nodeGUID, listeners := range driver.nodeListeners {
		nodeListeners[nodeGUID] = make(map[uint64]*bootstrap.Listener)
		for index, listener := range listeners {
			nodeListeners[nodeGUID][index] = listener
		}
	}
	return nodeListeners
}

// AddNodeListeners is used to add Node listeners(must be encrypted).
func (driver *driver) AddNodeListener(guid *guid.GUID, listener *bootstrap.Listener) error {
	driver.nodeListenersRWM.Lock()
	defer driver.nodeListenersRWM.Unlock()
	// check Node GUID is exist
	nodeListeners, ok := driver.nodeListeners[*guid]
	if !ok {
		nodeListeners = make(map[uint64]*bootstrap.Listener)
		driver.nodeListeners[*guid] = nodeListeners
	}
	// compare listeners
	for _, nodeListener := range nodeListeners {
		if listener.Equal(nodeListener) {
			return errors.New("node listener already exists")
		}
	}
	index := driver.nodeListenersIndex
	nodeListeners[index] = listener
	driver.nodeListenersIndex++
	return nil
}

// DeleteNodeListener is used to delete Node listener.
func (driver *driver) DeleteNodeListener(guid *guid.GUID, index uint64) error {
	driver.nodeListenersRWM.Lock()
	defer driver.nodeListenersRWM.Unlock()
	// check Node GUID is exist
	nodeListeners, ok := driver.nodeListeners[*guid]
	if !ok {
		return errors.New("node doesn't exist")
	}
	if _, ok := nodeListeners[index]; ok {
		delete(nodeListeners, index)
		return nil
	}
	return errors.New("node listener doesn't exist")
}

func (driver *driver) logf(lv logger.Level, format string, log ...interface{}) {
	driver.ctx.logger.Printf(lv, "driver", format, log...)
}

func (driver *driver) log(lv logger.Level, log ...interface{}) {
	driver.ctx.logger.Println(lv, "driver", log...)
}

// clientWatcher is used to check Beacon is connected enough Nodes.
func (driver *driver) clientWatcher() {
	defer func() {
		if r := recover(); r != nil {
			driver.log(logger.Fatal, xpanic.Print(r, "driver.clientWatcher"))
			// restart queryLoop
			time.Sleep(time.Second)
			go driver.clientWatcher()
		} else {
			driver.wg.Done()
		}
	}()
	sleeper := random.NewSleeper()
	defer sleeper.Stop()
	for {
		select {
		case <-sleeper.Sleep(5, 10):
			driver.watchClient()
		case <-driver.context.Done():
			return
		}
	}
}

func (driver *driver) watchClient() {
	if !driver.ctx.sender.IsInInteractiveMode() {
		return
	}
	// check is enough
	if len(driver.ctx.sender.Clients()) >= driver.ctx.sender.GetMaxConns() {
		return
	}
	// connect node
	for nodeGUID, listeners := range driver.getNodeListeners() {
		var listener *bootstrap.Listener
		for _, listener = range listeners {
			break
		}
		if listener == nil {
			continue
		}
		tempListener := listener.Decrypt()
		_ = driver.ctx.sender.Synchronize(driver.context, &nodeGUID, tempListener)
		tempListener.Destroy()
		if len(driver.ctx.sender.Clients()) >= driver.ctx.sender.GetMaxConns() {
			return
		}
	}
}

// queryLoop is used to query message from Controller.
func (driver *driver) queryLoop() {
	defer func() {
		if r := recover(); r != nil {
			driver.log(logger.Fatal, xpanic.Print(r, "driver.queryLoop"))
			// restart queryLoop
			time.Sleep(time.Second)
			go driver.queryLoop()
		} else {
			driver.wg.Done()
		}
	}()
	sleeper := random.NewSleeper()
	defer sleeper.Stop()
	for {
		sleepFixed := driver.sleepFixed.Load().(uint)
		sleepRandom := driver.sleepRandom.Load().(uint)
		select {
		case <-sleeper.Sleep(sleepFixed, sleepRandom):
			driver.query()
		case <-driver.context.Done():
			return
		}
	}
}

func (driver *driver) query() {
	// check if connect some Nodes(maybe in interactive mode)
	if len(driver.ctx.sender.Clients()) > 0 {
		err := driver.ctx.sender.Query()
		if err != nil {
			driver.log(logger.Warning, "failed to query:", err)
		}
		return
	}
}
