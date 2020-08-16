package netstat

import (
	"context"
	"sync"
	"time"

	"project/internal/logger"
	"project/internal/module/control"
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

	controller *control.Controller
	netstat    NetStat
	interval   time.Duration
	rwm        sync.RWMutex

	// about check network status
	tcp4Conns []*TCP4Conn
	tcp6Conns []*TCP6Conn
	udp4Conns []*UDP4Conn
	udp6Conns []*UDP6Conn
	connsRWM  sync.RWMutex

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
	ctx, cancel := context.WithCancel(context.Background())
	monitor := Monitor{
		logger:     logger,
		callback:   callback,
		controller: control.NewController(ctx),
		netstat:    netstat,
		interval:   defaultInterval,
		ctx:        ctx,
		cancel:     cancel,
	}
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
		mon.controller.Paused()
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
		timer.Reset(mon.GetInterval())
	}
}

// Refresh is used to refresh current network status at once.
func (mon *Monitor) Refresh() error {
	tcp4Conns, err := mon.netstat.GetTCP4Conns()
	if err != nil {
		return err
	}
	tcp6Conns, err := mon.netstat.GetTCP6Conns()
	if err != nil {
		return err
	}
	udp4Conns, err := mon.netstat.GetUDP4Conns()
	if err != nil {
		return err
	}
	udp6Conns, err := mon.netstat.GetUDP6Conns()
	if err != nil {
		return err
	}
	if mon.callback != nil {
		mon.compare()
	}
	mon.connsRWM.Lock()
	defer mon.connsRWM.Unlock()
	mon.tcp4Conns = tcp4Conns
	mon.tcp6Conns = tcp6Conns
	mon.udp4Conns = udp4Conns
	mon.udp6Conns = udp6Conns
	return nil
}

// GetTCP4Conns is used to get tcp4 connections that stored in monitor.
func (mon *Monitor) GetTCP4Conns() []*TCP4Conn {
	mon.connsRWM.RLock()
	defer mon.connsRWM.RUnlock()
	return mon.tcp4Conns
}

// GetTCP6Conns is used to get tcp6 connections that stored in monitor.
func (mon *Monitor) GetTCP6Conns() []*TCP6Conn {
	mon.connsRWM.RLock()
	defer mon.connsRWM.RUnlock()
	return mon.tcp6Conns
}

// GetUDP4Conns is used to get udp4 connections that stored in monitor.
func (mon *Monitor) GetUDP4Conns() []*UDP4Conn {
	mon.connsRWM.RLock()
	defer mon.connsRWM.RUnlock()
	return mon.udp4Conns
}

// GetUDP6Conns is used to get udp6 connections that stored in monitor.
func (mon *Monitor) GetUDP6Conns() []*UDP6Conn {
	mon.connsRWM.RLock()
	defer mon.connsRWM.RUnlock()
	return mon.udp6Conns
}

// compare is used to compare between stored in monitor.
func (mon *Monitor) compare() {
	mon.connsRWM.RLock()
	defer mon.connsRWM.RUnlock()
}

// Pause is used to pause auto refresh.
func (mon *Monitor) Pause() {
	mon.controller.Pause()
}

// Continue is used to continue auto refresh.
func (mon *Monitor) Continue() {
	mon.controller.Continue()
}

// Close is used to close network status monitor.
func (mon *Monitor) Close() {
	mon.cancel()
	mon.wg.Wait()
}