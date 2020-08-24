// +build !windows

package shellcode

import (
	"errors"
)

// Execute is a padding function.
func Execute(string, []byte) error {
	return errors.New("current system don't support")
}
