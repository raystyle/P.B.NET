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
	"project/internal/protocol"
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

	// interactive mode
	interactive  atomic.Value
	interactiveM sync.Mutex

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
	driver.SetSleepTime(cfg.SleepFixed, cfg.SleepRandom)
	interactive := cfg.Interactive
	driver.interactive.Store(interactive)
	driver.context, driver.cancel = context.WithCancel(context.Background())
	return &driver, nil
}

func (driver *driver) Drive() {
	driver.wg.Add(3)
	go driver.clientWatcher()
	go driver.queryLoop()
	go driver.modeWatcher()
}

func (driver *driver) Close() {
	driver.cancel()
	driver.wg.Wait()
	driver.ctx = nil
}

// NodeListener is used to get all Node listeners.
func (driver *driver) NodeListeners() map[guid.GUID]map[uint64]*bootstrap.Listener {
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
func (driver *driver) AddNodeListener(guid *guid.GUID, listener *bootstrap.Listener) {
	driver.nodeListenersRWM.Lock()
	defer driver.nodeListenersRWM.Unlock()
	// check Node GUID is exist
	listeners, ok := driver.nodeListeners[*guid]
	if !ok {
		listeners = make(map[uint64]*bootstrap.Listener)
		driver.nodeListeners[*guid] = listeners
	}
	// compare listeners
	for _, nListener := range listeners {
		if listener.Equal(nListener) {
			return
		}
	}
	index := driver.nodeListenersIndex
	listeners[index] = listener
	driver.nodeListenersIndex++
}

// DeleteNodeListener is used to delete Node listener.
func (driver *driver) DeleteNodeListener(guid *guid.GUID, index uint64) error {
	driver.nodeListenersRWM.Lock()
	defer driver.nodeListenersRWM.Unlock()
	// check Node GUID is exist
	listeners, ok := driver.nodeListeners[*guid]
	if !ok {
		return errors.New("node doesn't exist")
	}
	if _, ok := listeners[index]; ok {
		delete(listeners, index)
		return nil
	}
	return errors.New("node listener doesn't exist")
}

// DeleteAllNodeListener is used to delete Node's all listeners.
func (driver *driver) DeleteAllNodeListener(guid *guid.GUID) {
	driver.nodeListenersRWM.Lock()
	defer driver.nodeListenersRWM.Unlock()
	delete(driver.nodeListeners, *guid)
}

func (driver *driver) SetSleepTime(fixed, random uint) {
	driver.sleepFixed.Store(fixed)
	driver.sleepRandom.Store(random)
}

func (driver *driver) GetSleepTime() (uint, uint) {
	fixed := driver.sleepFixed.Load().(uint)
	rand := driver.sleepRandom.Load().(uint)
	return fixed, rand
}

func (driver *driver) EnableInteractiveMode() {
	driver.interactiveM.Lock()
	defer driver.interactiveM.Unlock()
	driver.interactive.Store(true)
}

func (driver *driver) DisableInteractiveMode() error {
	driver.interactiveM.Lock()
	defer driver.interactiveM.Unlock()
	if !driver.IsInInteractiveMode() {
		return errors.New("already disable interactive mode")
	}
	// check virtual connections manager

	driver.interactive.Store(false)
	return nil
}

// IsInInteractiveMode is used to check is in interactive mode.
func (driver *driver) IsInInteractiveMode() bool {
	return driver.interactive.Load().(bool)
}

// UpdateNode is used to update Node if client find different GUID in certificate.
func (driver *driver) UpdateNode(ctx context.Context, cert *protocol.Certificate) (bool, error) {
	// select a Node's listener
	var listener *bootstrap.Listener
	for _, listeners := range driver.NodeListeners() {
		for _, l := range listeners {
			listener = l
			break
		}
		break
	}
	// use protocol.CtrlGUID to skip check node guid in certificate
	client, err := driver.ctx.NewClient(ctx, listener, protocol.CtrlGUID, nil)
	if err != nil {
		return false, err
	}
	defer client.Close()
	// send connect operation
	_, err = client.Conn.Write([]byte{protocol.BeaconOperationUpdate})
	if err != nil {
		return false, errors.Wrap(err, "failed to send update operation")
	}
	// send request

	return true, nil
}

// func (driver *driver) logf(lv logger.Level, format string, log ...interface{}) {
// 	driver.ctx.logger.Printf(lv, "driver", format, log...)
// }

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
	if !driver.IsInInteractiveMode() {
		return
	}
	// check is enough
	if len(driver.ctx.sender.Clients()) >= driver.ctx.sender.GetMaxConns() {
		return
	}
	// connect node
	for nodeGUID, listeners := range driver.NodeListeners() {
		var listener *bootstrap.Listener
		for _, listener = range listeners {
			break
		}
		if listener == nil {
			continue
		}
		// tempListener := listener.Decrypt()
		_ = driver.ctx.sender.Synchronize(driver.context, &nodeGUID, listener)
		// tempListener.Destroy()
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

func (driver *driver) modeWatcher() {
	defer func() {
		if r := recover(); r != nil {
			driver.log(logger.Fatal, xpanic.Print(r, "driver.modeWatcher"))
			// restart queryLoop
			time.Sleep(time.Second)
			go driver.modeWatcher()
		} else {
			driver.wg.Done()
		}
	}()
	driver.watchMode()
}

func (driver *driver) watchMode() {

}
