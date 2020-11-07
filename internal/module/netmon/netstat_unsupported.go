// +build !windows

package netmon

import (
	"errors"
	"runtime"
)

// Options is a padding structure.
type Options struct{}

// NewNetStat is a padding function.
func NewNetStat(*Options) (NetStat, error) {
	return nil, errors.New("netstat is not implemented on " + runtime.GOOS)
}

// GetTCPConnState is is a padding function.
func GetTCPConnState(uint8) string {
	return "padding"
}
