// +build windows

package hook

type arch interface {
	DisassembleMode() int
	JumperSize() int
	NewJumpAsm(from, to uintptr) []byte
}
