// +build windows

package hook

type arch interface {
	DisassembleMode() int
}
