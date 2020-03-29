// +build !windows

package meterpreter

import (
	"errors"
)

func reverseTCP(_ *net.TCPConn, _ []byte, _ string) error {
	return errors.New("current system not support")
}
