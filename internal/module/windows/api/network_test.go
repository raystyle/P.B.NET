package api

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetTCP4Conns(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		t.Run("listeners", func(t *testing.T) {
			conns, err := GetTCP4Conns(TCPTableBasicListener)
			require.NoError(t, err)

			require.NotEmpty(t, conns)
			fmt.Println("Local Address    Remote Address    State    PID    Process")
			for _, conn := range conns {
				fmt.Printf("%s:%d   %s:%d   %s   %d   %s\n",
					conn.LocalAddr, conn.LocalPort,
					conn.RemoteAddr, conn.RemotePort,
					GetTCPStateString(conn.State), conn.PID, conn.Process,
				)
			}
		})

		t.Run("connections", func(t *testing.T) {
			conns, err := GetTCP4Conns(TCPTableBasicConnections)
			require.NoError(t, err)

			require.NotEmpty(t, conns)
		})

		t.Run("all", func(t *testing.T) {
			conns, err := GetTCP4Conns(TCPTableBasicAll)
			require.NoError(t, err)

			require.NotEmpty(t, conns)
		})
	})

	t.Run("owner pid", func(t *testing.T) {

	})

	t.Run("owner module", func(t *testing.T) {

	})
}
