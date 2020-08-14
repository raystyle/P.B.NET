package netstat

import (
	"time"

	"project/internal/logger"
)

const defaultInterval = 500 * time.Millisecond

// Callback is used to notice user appear event.
type Callback func()

// Monitor is used tp monitor network status about current system.
type Monitor struct {
	logger   logger.Logger
	interval time.Duration
}

// NewMonitor is used to create a network status monitor.
func NewMonitor(logger logger.Logger) *Monitor {

	return nil
}

// SetInterval is used to set refresh interval, if set zero, it will pause auto refresh.
func (mon *Monitor) SetInterval() {

}

// Refresh is used to get current network status.
func (mon *Monitor) Refresh() {

}

// Pause is used to pause auto refresh.
func (mon *Monitor) Pause() {

}

// Close is used to close network status monitor.
func (mon *Monitor) Close() {

}
