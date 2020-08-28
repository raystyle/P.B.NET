// +build windows

package kiwimon

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/module/windows/kiwi"
	"project/internal/testsuite"
)

func TestMonitor(t *testing.T) {
	monitor, err := NewMonitor(logger.Test, func(local, remote string, pid int64, cred *kiwi.Credential) {
		fmt.Println(local, remote, pid, cred)
	}, nil)
	require.NoError(t, err)

	time.Sleep(30 * time.Second)

	monitor.Close()

	testsuite.IsDestroyed(t, monitor)
}
