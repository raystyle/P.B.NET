// +build !windows

package shellcode

import (
	"errors"
)

// Execute is a padding
func Execute(method string, shellcode []byte) error {
	return errors.New("current system don't support")
}
