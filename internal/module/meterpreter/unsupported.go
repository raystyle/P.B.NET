// +build !windows

package meterpreter

import (
	"errors"
)

func reverseTCP(*net.TCPConn, []byte, string) error {
	return errors.New("current system not support")
}
