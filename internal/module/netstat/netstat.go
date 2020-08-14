package netstat

import (
	"time"
)

// Monitor is used tp monitor connection status about current system.
type Monitor struct {
	interval time.Duration
}
