package netstat

import (
	"context"
	"sync"
	"time"

	"project/internal/logger"
	"project/internal/xpanic"
)

const (
	defaultInterval = 500 * time.Millisecond
	minInterval     = 100 * time.Millisecond
)

// about events
const (
	_ uint8 = iota
	EventConnCreated
	EventConnRemoved
)

// Callback is used to notice user appear event.
type Callback func(event uint8)

// Monitor is used tp monitor network status about current system.
type Monitor struct {
	logger   logger.Logger
	callback Callback

	netstat  NetStat
	interval time.Duration

	// about pause and continue auto refresh
	paused  bool
	pauseCh chan struct{}

	// about check network status
	tcp4Conns TCP4Conn
	tcp6Conns TCP6Conn
	udp4Conns UDP4Conn
	udp6Conns UDP4Conn

	rwm sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewMonitor is used to create a network status monitor.
func NewMonitor(logger logger.Logger, callback Callback) (*Monitor, error) {
	netstat, err := newNetstat()
	if err != nil {
		return nil, err
	}
	monitor := Monitor{
		logger:   logger,
		callback: callback,
		netstat:  netstat,
		interval: defaultInterval,
		pauseCh:  make(chan struct{}, 1),
	}
	monitor.ctx, monitor.cancel = context.WithCancel(context.Background())
	monitor.wg.Add(1)
	go monitor.refreshLoop()
	return &monitor, nil
}

// GetInterval is used to get refresh interval.
func (mon *Monitor) GetInterval() time.Duration {
	mon.rwm.RLock()
	defer mon.rwm.RUnlock()
	return mon.interval
}

// SetInterval is used to set refresh interval, if set zero, it will pause auto refresh.
func (mon *Monitor) SetInterval(interval time.Duration) {
	if interval < minInterval {
		interval = minInterval
	}
	mon.rwm.Lock()
	defer mon.rwm.Unlock()
	mon.interval = interval
}

func (mon *Monitor) log(lv logger.Level, log ...interface{}) {
	mon.logger.Println(lv, "network monitor", log...)
}

func (mon *Monitor) refreshLoop() {
	defer mon.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			mon.log(logger.Fatal, xpanic.Print(r, "Monitor.refreshLoop"))
			// restart
			time.Sleep(time.Second)
			mon.wg.Add(1)
			go mon.refreshLoop()
		}
	}()
	timer := time.NewTimer(mon.GetInterval())
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			err := mon.Refresh()
			if err != nil {
				mon.log(logger.Error, "failed to refresh:", err)
				return
			}
		case <-mon.ctx.Done():
			return
		}

		mon.isPaused()

		timer.Reset(mon.GetInterval())
	}
}

// Refresh is used to refresh current network status at once.
func (mon *Monitor) Refresh() error {

	return nil
}

// Pause is used to pause auto refresh.
func (mon *Monitor) Pause() {
	mon.rwm.Lock()
	defer mon.rwm.Unlock()
	mon.paused = true
}

// Continue is used to continue auto refresh.
func (mon *Monitor) Continue() {
	mon.rwm.Lock()
	defer mon.rwm.Unlock()
	mon.paused = false
	select {
	case mon.pauseCh <- struct{}{}:
	default:
	}
}

func (mon *Monitor) isPaused() {
	paused := func() bool {
		mon.rwm.RLock()
		defer mon.rwm.RUnlock()
		return mon.paused
	}()
	if paused {
		select {
		case <-mon.pauseCh:
		case <-mon.ctx.Done():
		}
	}
}

// Close is used to close network status monitor.
func (mon *Monitor) Close() {
	mon.cancel()
	mon.wg.Wait()
}
