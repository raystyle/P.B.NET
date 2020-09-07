package privilege

import (
	"fmt"
	"strings"
)

func generateRtlEnableDisable(privilege, comment string) {
	// <p0> without "SE"
	const template = `
// RtlEnable<p0> is used to enable <c> privilege that call RtlAdjustPrivilege.
func RtlEnable<p0>() (bool, error) {
	return RtlAdjustPrivilege(<p>, true, false)
}

// RtlDisable<p0> is used to disable <c> privilege that call RtlAdjustPrivilege.
func RtlDisable<p0>() (bool, error) {
	return RtlAdjustPrivilege(<p>, false, false)
}
`
	src := strings.ReplaceAll(template, "<p0>", privilege[2:])
	src = strings.ReplaceAll(src, "<c>", comment)
	src = strings.ReplaceAll(src, "<p>", privilege)
	fmt.Print(src)
}
