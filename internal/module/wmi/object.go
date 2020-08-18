package wmi

import (
	"github.com/go-ole/go-ole"
)

// Object returned by Client.Get().
type Object struct {
	object *ole.VARIANT
}
