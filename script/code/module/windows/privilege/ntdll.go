package privilege

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"text/template"

	"github.com/stretchr/testify/require"
)

func generateRtlEnableDisable(privilege, comment string) {
	// <p0> without "SE"
	const tpl = `
// RtlEnable<p0> is used to enable <c> privilege that call RtlAdjustPrivilege.
func RtlEnable<p0>() (bool, error) {
	return RtlAdjustPrivilege(<p>, true, false)
}

// RtlDisable<p0> is used to disable <c> privilege that call RtlAdjustPrivilege.
func RtlDisable<p0>() (bool, error) {
	return RtlAdjustPrivilege(<p>, false, false)
}
`
	src := strings.ReplaceAll(tpl, "<p0>", privilege[2:])
	src = strings.ReplaceAll(src, "<c>", comment)
	src = strings.ReplaceAll(src, "<p>", privilege)
	fmt.Print(src)
}

func generateTestRtlEnableDisable(t *testing.T, privilege string) {
	const src = `
func TestRtlEnable{{.P0}}(t *testing.T) {
	if !testIsElevated() {
		return
	}
	previous, err := RtlEnable{{.P0}}()
	require.NoError(t, err)
	{{if .First}}require.True(t, previous){{else}}require.False(t, previous){{end}}
	
	testRestorePrivilege(t, {{.P}}, previous, true)
}

func TestRtlDisable{{.P0}}(t *testing.T) {
	if !testIsElevated() {
		return
	}
	first, err := RtlEnable{{.P0}}()
	require.NoError(t, err)
	{{if .First}}require.True(t, first){{else}}require.False(t, first){{end}}

	previous, err := RtlDisable{{.P0}}()
	require.NoError(t, err)
	require.True(t, previous)
	
	testRestorePrivilege(t, {{.P}}, first, false)
}
`
	type p struct {
		P0    string // privilege name without "SE"
		P     string // full privilege name
		First bool   // special
	}
	param := p{
		P0: privilege[2:],
		P:  privilege,
	}
	if privilege == "SEDebug" {
		param.First = true
	}
	tpl := template.New("execute")
	_, err := tpl.Parse(src)
	require.NoError(t, err)
	err = tpl.Execute(os.Stdout, &param)
	require.NoError(t, err)
}
