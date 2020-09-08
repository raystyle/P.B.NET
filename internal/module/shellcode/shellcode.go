package shellcode

import (
	"project/internal/security"
)

// supported execute methods
const (
	// Windows
	MethodVirtualProtect = "vp"
	MethodCreateThread   = "thread"
)

const (
	// criticalValue is a flag that when copied shellcode size reach it, call bypass.
	criticalValue = 16 * 1024

	// maxBypassTimes is used to prevent block when execute large shellcode.
	maxBypassTimes = 10
)

func bypass() {
	security.SwitchThread()
}
