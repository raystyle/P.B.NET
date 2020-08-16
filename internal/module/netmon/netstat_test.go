package netstat

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNetStat(t *testing.T) {
	netstat, err := NewNetStat()
	require.NoError(t, err)

	t.Run("TCP Over IPv4", func(t *testing.T) {
		conns, err := netstat.GetTCP4Conns()
		require.NoError(t, err)
		fmt.Println("Local Address   Remote Address   State   PID")
		for _, conn := range conns {
			fmt.Printf("%s:%d %s:%d %d %d\n",
				conn.LocalAddr,
				conn.LocalPort,
				conn.RemoteAddr,
				conn.RemotePort,
				conn.State,
				conn.PID,
			)
		}
	})

	t.Run("TCP Over IPV6", func(t *testing.T) {
		conns, err := netstat.GetTCP6Conns()
		require.NoError(t, err)
		fmt.Println("Local Address   Remote Address   State   PID")
		for _, conn := range conns {
			fmt.Printf("[%s%%%d]:%d [%s%%%d]:%d %d %d\n",
				conn.LocalAddr,
				conn.LocalScopeID,
				conn.LocalPort,
				conn.RemoteAddr,
				conn.RemoteScopeID,
				conn.RemotePort,
				conn.State,
				conn.PID,
			)
		}
	})

	t.Run("UDP Over IPv4", func(t *testing.T) {
		conns, err := netstat.GetUDP4Conns()
		require.NoError(t, err)
		fmt.Println("Local Address   PID")
		for _, conn := range conns {
			fmt.Printf("%s:%d *:* %d\n",
				conn.LocalAddr,
				conn.LocalPort,
				conn.PID,
			)
		}
	})

	t.Run("UDP Over IPV6", func(t *testing.T) {
		conns, err := netstat.GetUDP6Conns()
		require.NoError(t, err)
		fmt.Println("Local Address   PID")
		for _, conn := range conns {
			fmt.Printf("[%s%%%d]:%d *:* %d\n",
				conn.LocalAddr,
				conn.LocalScopeID,
				conn.LocalPort,
				conn.PID,
			)
		}
	})
}
