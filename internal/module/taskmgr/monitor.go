package taskmgr

import (
	"context"
	"sync"
	"time"

	"project/internal/logger"
	"project/internal/module/control"
)

const (
	defaultRefreshInterval = 500 * time.Millisecond
	minimumRefreshInterval = 100 * time.Millisecond
)

// about events
const (
	_ uint8 = iota
	EventProcessCreated
	EventProcessTerminated
)

// EventHandler is used to handle appeared event.
type EventHandler func(ctx context.Context, event uint8, data interface{})

// Monitor is used tp monitor system status about current system.
type Monitor struct {
	logger  logger.Logger
	handler EventHandler

	controller *control.Controller
	tasklist   TaskList
	interval   time.Duration
	rwm        sync.RWMutex

	// about check process status
	processes    []*Process
	processesRWM sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewMonitor is used to create a system status monitor.
func NewMonitor(logger logger.Logger, handler EventHandler) (*Monitor, error) {
	tasklist, err := NewTaskList()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	monitor := Monitor{
		logger:     logger,
		controller: control.NewController(ctx),
		tasklist:   tasklist,
		interval:   defaultRefreshInterval,
		ctx:        ctx,
		cancel:     cancel,
	}
	// refresh before refreshLoop, and not set eventHandler.
	err = monitor.Refresh()
	if err != nil {
		return nil, err
	}
	// not trigger eventHandler before first refresh.
	monitor.handler = handler
	monitor.wg.Add(1)
	go monitor.refreshLoop()
	return &monitor, nil
}

func (mon *Monitor) refreshLoop() {

}

// Refresh is used to refresh current system status at once.
func (mon *Monitor) Refresh() error {
	return nil
}
