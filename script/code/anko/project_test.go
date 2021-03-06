package anko

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/system"
)

func TestExportProject(t *testing.T) {
	const template = `// Code generated by script/code/anko/project_test.go. DO NOT EDIT.

package project

import (
	"reflect"

	"project/external/anko/env"

%s)

func init() {
%s}
%s
`
	// get project directory
	dir, err := os.Getwd()
	require.NoError(t, err)
	dir, err = filepath.Abs(dir + "/../../..")
	require.NoError(t, err)

	pkgBuf := new(bytes.Buffer)
	initBuf := new(bytes.Buffer)
	srcBuf := new(bytes.Buffer)

	for _, item := range [...]*struct {
		path string
		init string
	}{
		{"internal/cert", "InternalCert"},
		{"internal/convert", "InternalConvert"},
		{"internal/crypto/aes", "InternalCryptoAES"},
		{"internal/crypto/curve25519", "InternalCryptoCurve25519"},
		{"internal/crypto/ed25519", "InternalCryptoED25519"},
		{"internal/crypto/hmac", "InternalCryptoHMAC"},
		{"internal/crypto/lsb", "InternalCryptoLSB"},
		{"internal/crypto/rand", "InternalCryptoRand"},
		{"internal/dns", "InternalDNS"},
		{"internal/guid", "InternalGUID"},
		{"internal/httptool", "InternalHTTPTool"},
		{"internal/logger", "InternalLogger"},
		{"internal/namer", "InternalNamer"},
		{"internal/nettool", "InternalNetTool"},
		{"internal/option", "InternalOption"},
		{"internal/patch/json", "InternalPatchJSON"},
		{"internal/patch/msgpack", "InternalPatchMsgpack"},
		{"internal/patch/toml", "InternalPatchToml"},
		{"internal/proxy", "InternalProxy"},
		{"internal/proxy/direct", "InternalProxyDirect"},
		{"internal/proxy/http", "InternalProxyHTTP"},
		{"internal/proxy/socks", "InternalProxySocks"},
		{"internal/random", "InternalRandom"},
		{"internal/security", "InternalSecurity"},
		{"internal/system", "InternalSystem"},
		{"internal/timesync", "InternalTimeSync"},
		{"internal/xpanic", "InternalXPanic"},
		{"internal/xreflect", "InternalXReflect"},
		{"internal/xsync", "InternalXSync"},
	} {
		_, _ = fmt.Fprintf(pkgBuf, `	"project/%s"`+"\n", item.path)
		_, _ = fmt.Fprintf(initBuf, "\tinit%s()\n", item.init)
		src, err := exportDeclaration(dir, item.path, "$"+item.path, item.init)
		require.NoError(t, err)
		srcBuf.WriteString(src)
	}

	// generate code
	src := fmt.Sprintf(template, pkgBuf, initBuf, srcBuf)

	// fix code
	for _, item := range [...]*struct {
		old string
		new string
	}{
		{"logger logger.Logger", "lg logger.Logger"},
		{"(&logger)", "(&lg)"},

		{"namer namer.Namer", "n namer.Namer"},
		{"(&namer)", "(&n)"},

		{"direct direct.Direct", "d direct.Direct"},
		{"(&direct)", "(&d)"},

		{"rand random.Rand", "r random.Rand"},
		{"(&rand)", "(&r)"},
	} {
		src = strings.ReplaceAll(src, item.old, item.new)
	}

	// print and save code
	fmt.Println(src)
	const path = "../../../internal/anko/project/bundle.go"
	err = system.WriteFile(path, []byte(src))
	require.NoError(t, err)
}

func TestExportProjectWindows(t *testing.T) {
	const template = `// Code generated by script/code/anko/project_test.go. DO NOT EDIT.

// +build windows

package project

import (
	"reflect"

	"project/external/anko/env"

%s)

func init() {
%s}
%s
`
	// get project directory
	dir, err := os.Getwd()
	require.NoError(t, err)
	dir, err = filepath.Abs(dir + "/../../..")
	require.NoError(t, err)

	pkgBuf := new(bytes.Buffer)
	initBuf := new(bytes.Buffer)
	srcBuf := new(bytes.Buffer)

	for _, item := range [...]*struct {
		path string
		init string
	}{
		{"internal/module/windows/wmi", "InternalModuleWindowsWMI"},
		{"internal/module/windows/privilege", "InternalModuleWindowsPrivilege"},
	} {
		_, _ = fmt.Fprintf(pkgBuf, `	"project/%s"`+"\n", item.path)
		_, _ = fmt.Fprintf(initBuf, "\tinit%s()\n", item.init)
		src, err := exportDeclaration(dir, item.path, "$"+item.path, item.init)
		require.NoError(t, err)
		srcBuf.WriteString(src)
	}

	// generate code
	src := fmt.Sprintf(template, pkgBuf, initBuf, srcBuf)

	// fix code
	for _, item := range [...]*struct {
		old string
		new string
	}{
		// {"logger logger.Logger", "lg logger.Logger"},
		// {"(&logger)", "(&lg)"},
	} {
		src = strings.ReplaceAll(src, item.old, item.new)
	}

	// print and save code
	fmt.Println(src)
	const path = "../../../internal/anko/project/windows.go"
	err = system.WriteFile(path, []byte(src))
	require.NoError(t, err)
}
