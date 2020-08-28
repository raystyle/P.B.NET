// +build !windows

package netmon

import (
	"errors"
	"runtime"
)

func newNetstat() (*netStat, error) {
	return nil, errors.New("netstat is not implemented on " + runtime.GOOS)
}
