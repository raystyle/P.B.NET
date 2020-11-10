// +build windows, !386, !amd64

package hook

import (
	"fmt"
	"runtime"
)

func newArch() (arch, error) {
	return nil, fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
}
