// +build !windows

package netstat

import (
	"errors"
	"runtime"
)

func newNetstat() (*netStat, error) {
	return nil, errors.New("netstat is not implemented on " + runtime.GOOS)
}
