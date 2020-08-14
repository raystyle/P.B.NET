// +build windows

package netstat

import (
	"github.com/StackExchange/wmi"
	"github.com/go-ole/go-ole"
)

// Temp test
func Temp() {
	wmi.CreateQuery(nil, "")

	ole.CoUninitialize()
}
