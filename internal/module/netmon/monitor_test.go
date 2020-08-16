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

	fmt.Println("Local Address    Remote Address    State    PID")
	callback := func(event uint8, data interface{}) {
		switch event {
		case EventConnCreated:
			testMonitorPrintCreatedConns(data)
		case EventConnRemoved:
			testMonitorPrintDeletedConns(data)
		}
	}
	monitor, err := NewMonitor(logger.Test, callback)
	require.NoError(t, err)

	monitor.SetInterval(200 * time.Millisecond)

	time.Sleep(5 * time.Second)
	monitor.Pause()
	time.Sleep(5 * time.Second)
	monitor.Continue()
	time.Sleep(5 * time.Second)

	monitor.GetTCP4Conns()
	monitor.GetTCP6Conns()
	monitor.GetUDP4Conns()
	monitor.GetUDP6Conns()

	monitor.Close()

	testsuite.IsDestroyed(t, monitor)
}

func testMonitorPrintCreatedConns(conns interface{}) {
	for _, conn := range conns.([]interface{}) {
		switch conn := conn.(type) {
		case *TCP4Conn:
			fmt.Printf("create TCP4 connection\n%s:%d %s:%d %d %d\n",
				conn.LocalAddr, conn.LocalPort,
				conn.RemoteAddr, conn.RemotePort,
				conn.State, conn.PID,
			)
		case *TCP6Conn:
			fmt.Printf("create TCP6 connection\n[%s%%%d]:%d [%s%%%d]:%d %d %d\n",
				conn.LocalAddr, conn.LocalScopeID, conn.LocalPort,
				conn.RemoteAddr, conn.RemoteScopeID, conn.RemotePort,
				conn.State, conn.PID,
			)
		case *UDP4Conn:
			fmt.Printf("create UDP4 connection\n%s:%d *:* %d\n",
				conn.LocalAddr, conn.LocalPort, conn.PID,
			)
		case *UDP6Conn:
			fmt.Printf("create UDP6 connection\n[%s%%%d]:%d *:* %d\n",
				conn.LocalAddr, conn.LocalScopeID, conn.LocalPort, conn.PID,
			)
		}
	}
}

func testMonitorPrintDeletedConns(conns interface{}) {
	for _, conn := range conns.([]interface{}) {
		switch conn := conn.(type) {
		case *TCP4Conn:
			fmt.Printf("remove TCP4 connection\n%s:%d %s:%d %d %d\n",
				conn.LocalAddr, conn.LocalPort,
				conn.RemoteAddr, conn.RemotePort,
				conn.State, conn.PID,
			)
		case *TCP6Conn:
			fmt.Printf("remove TCP6 connection\n[%s%%%d]:%d [%s%%%d]:%d %d %d\n",
				conn.LocalAddr, conn.LocalScopeID, conn.LocalPort,
				conn.RemoteAddr, conn.RemoteScopeID, conn.RemotePort,
				conn.State, conn.PID,
			)
		case *UDP4Conn:
			fmt.Printf("remove UDP4 connection\n%s:%d *:* %d\n",
				conn.LocalAddr, conn.LocalPort, conn.PID,
			)
		case *UDP6Conn:
			fmt.Printf("remove UDP6 connection\n[%s%%%d]:%d *:* %d\n",
				conn.LocalAddr, conn.LocalScopeID, conn.LocalPort, conn.PID,
			)
		}
	}
}
