// +build windows

package hook

type archAMD64 struct{}

func newArch() (arch, error) {
	return archAMD64{}, nil
}

func (archAMD64) DisassembleMode() int {
	return 64
}
