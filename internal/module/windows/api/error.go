package api

import (
	"fmt"

	"github.com/pkg/errors"
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
