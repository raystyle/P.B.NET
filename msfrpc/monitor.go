package msfrpc

import (
	"context"
	"sync"
	"time"

	"project/internal/logger"
	"project/internal/xpanic"
)

const minWatchInterval = 100 * time.Millisecond

// Callbacks contains about all callback functions
type Callbacks struct {
	OnToken func(token string)
}

// Monitor is used to monitor changes about token list(security), jobs and sessions.
// if msfrpc connected database, it can  monitor hosts, services, browser clients,
// credentials, loots and framework events.
type Monitor struct {
	ctx *MSFRPC

	callbacks *Callbacks
	interval  time.Duration

	context   context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once
	wg        sync.WaitGroup
}

// NewMonitor is used to create a monitor.
func (msf *MSFRPC) NewMonitor(ctx *MSFRPC, callbacks *Callbacks, interval time.Duration) *Monitor {
	if interval < minWatchInterval {
		interval = minWatchInterval
	}
	monitor := &Monitor{
		ctx:       ctx,
		callbacks: callbacks,
		interval:  interval,
	}
	monitor.context, monitor.cancel = context.WithCancel(context.Background())
	monitor.wg.Add(3)
	go monitor.tokensMonitor()
	go monitor.jobsMonitor()
	go monitor.sessionsMonitor()
	return monitor
}

func (monitor *Monitor) log(lv logger.Level, log ...interface{}) {
	monitor.ctx.logger.Println(lv, "msfrpc-monitor", log...)
}

func (monitor *Monitor) tokensMonitor() {
	defer func() {
		if r := recover(); r != nil {
			monitor.log(logger.Fatal, xpanic.Print(r, "Monitor.tokensMonitor"))
			// restart monitor
			time.Sleep(time.Second)
			go monitor.tokensMonitor()
		} else {
			monitor.wg.Done()
		}
	}()
	ticker := time.NewTicker(monitor.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			monitor.watchTokens()
		case <-monitor.context.Done():
			return
		}
	}
}

func (monitor *Monitor) watchTokens() {

}

func (monitor *Monitor) jobsMonitor() {

}

func (monitor *Monitor) sessionsMonitor() {

}

// StartDatabaseMonitors is used to start monitors about database.
func (monitor *Monitor) StartDatabaseMonitors() {

}

// Close is used to close monitor.
func (monitor *Monitor) Close() {
	monitor.closeOnce.Do(func() {
		monitor.cancel()
		monitor.wg.Wait()
	})
}
