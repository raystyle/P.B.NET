// +build windows

package hook

type arch386 struct{}

func newArch() (arch, error) {
	return arch386{}, nil
}

func (arch386) DisassembleMode() int {
	return 32
}
