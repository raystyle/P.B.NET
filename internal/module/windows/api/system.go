package api

import (
	"fmt"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"

	"project/internal/convert"
)

func newAPIError(name, reason string, err error) error {
	if err != nil {
		return errors.Errorf("%s: %s, because %s", name, reason, err)
	}
	return errors.Errorf("%s: %s", name, reason)
}

func newAPIErrorf(name string, err error, format string, v ...interface{}) error {
	if err != nil {
		return errors.Errorf("%s: %s, because %s", name, fmt.Sprintf(format, v...), err)
	}
	return errors.Errorf("%s: %s", name, fmt.Sprintf(format, v...))
}

// GetVersion is used to get NT version.
func GetVersion() (major, minor int, err error) {
	const name = "GetVersion"
	ver, err := windows.GetVersion()
	if err != nil {
		return 0, 0, newAPIError(name, "failed to get windows version", err)
	}
	b := convert.LEUint32ToBytes(ver)
	fmt.Println(b)

	fmt.Println(ver)
	return 0, 0, nil
}
