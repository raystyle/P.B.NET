package netstat

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/testsuite"
)

func TestMonitor(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	callback := func(event uint8, conn interface{}) {
		switch event {
		case EventConnCreated:
			conn := conn.(*TCP4Conn)
			fmt.Printf("create TCP connection\n%s:%d %s:%d %d %d\n",
				conn.LocalAddr,
				conn.LocalPort,
				conn.RemoteAddr,
				conn.RemotePort,
				conn.State,
				conn.PID,
			)
		case EventConnRemoved:
			conn := conn.(*TCP4Conn)
			fmt.Printf("remove TCP connection\n%s:%d %s:%d %d %d\n",
				conn.LocalAddr,
				conn.LocalPort,
				conn.RemoteAddr,
				conn.RemotePort,
				conn.State,
				conn.PID,
			)
		}
	}

	monitor, err := NewMonitor(logger.Test, callback)
	require.NoError(t, err)

	time.Sleep(time.Minute)

	monitor.Close()

	testsuite.IsDestroyed(t, monitor)
}
