package anko

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/system"

	"project/script/internal/config"
)

func TestExportThirdParty(t *testing.T) {
	const template = `// Package thirdparty generate by script/code/anko/package.go, don't edit it.
package thirdparty

import (
	"reflect"

%s	"github.com/mattn/anko/env"
)

func init() {
%s}
%s
`
	// get module directory
	goMod, err := config.GoModCache()
	require.NoError(t, err)

	pkgBuf := new(bytes.Buffer)
	initBuf := new(bytes.Buffer)
	srcBuf := new(bytes.Buffer)

	for _, item := range [...]*struct {
		path string
		dir  string
		init string
	}{
		{
			path: "github.com/pelletier/go-toml",
			dir:  "github.com/pelletier/go-toml@v1.8.1",
			init: "GithubComPelletierGoTOML",
		},
		{
			path: "github.com/pkg/errors",
			dir:  "github.com/pkg/errors@v0.9.1",
			init: "GithubComPkgErrors",
		},
		{
			path: "github.com/vmihailenco/msgpack/v5",
			dir:  "github.com/vmihailenco/msgpack/v5@v5.0.0",
			init: "GithubComVmihailencoMsgpackV5",
		},
		{
			path: "github.com/vmihailenco/msgpack/v5/msgpcode",
			dir:  "github.com/vmihailenco/msgpack/v5@v5.0.0/msgpcode",
			init: "GithubComVmihailencoMsgpackV5Msgpcode",
		},
	} {
		_, _ = fmt.Fprintf(pkgBuf, `	"%s"`+"\n", item.path)
		_, _ = fmt.Fprintf(initBuf, "\tinit%s()\n", item.init)
		src, err := exportDeclaration(goMod, item.path, item.dir, item.init)
		require.NoError(t, err)
		srcBuf.WriteString(src)
	}

	// generate code
	src := fmt.Sprintf(template, pkgBuf, initBuf, srcBuf)

	// fix code
	// for _, item := range [...]*struct {
	// 	old string
	// 	new string
	// }{
	// 	{"interface service.Interface", "iface service.Interface"},
	// 	{"(&interface)", "(&iface)"},
	//
	// 	{"service service.Service", "svc service.Service"},
	// 	{"(&service)", "(&svc)"},
	// } {
	// 	src = strings.ReplaceAll(src, item.old, item.new)
	// }

	// delete code
	for _, item := range []string{
		`		"DecodeDatastoreKey": reflect.ValueOf(msgpack.DecodeDatastoreKey),` + "\n",
		`		"EncodeDatastoreKey": reflect.ValueOf(msgpack.EncodeDatastoreKey),` + "\n",
	} {
		src = strings.ReplaceAll(src, item, "")
	}

	// print and save code
	fmt.Println(src)
	const path = "../../../internal/anko/thirdparty/bundle.go"
	err = system.WriteFile(path, []byte(src))
	require.NoError(t, err)
}

func TestExportThirdPartyWindows(t *testing.T) {
	const template = `// +build windows

// Package thirdparty generate by script/code/anko/package.go, don't edit it.
package thirdparty

import (
	"reflect"

%s	"github.com/mattn/anko/env"
)

func init() {
%s}
%s
`
	// get module directory
	goMod, err := config.GoModCache()
	require.NoError(t, err)

	pkgBuf := new(bytes.Buffer)
	initBuf := new(bytes.Buffer)
	srcBuf := new(bytes.Buffer)

	for _, item := range [...]*struct {
		path string
		dir  string
		init string
	}{
		{
			path: "github.com/go-ole/go-ole",
			dir:  "github.com/go-ole/go-ole@v1.2.5-0.20201122170103-d467d8080fc3",
			init: "GithubComGoOLEGoOLE",
		},
		{
			path: "github.com/go-ole/go-ole/oleutil",
			dir:  "github.com/go-ole/go-ole@v1.2.5-0.20201122170103-d467d8080fc3/oleutil",
			init: "GithubComGoOLEGoOLEOLEUtil",
		},
	} {
		_, _ = fmt.Fprintf(pkgBuf, `	"%s"`+"\n", item.path)
		_, _ = fmt.Fprintf(initBuf, "\tinit%s()\n", item.init)
		src, err := exportDeclaration(goMod, item.path, item.dir, item.init)
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
		// overflows int
		{"(ole.CO_E_CLASSSTRING)", "(uint32(ole.CO_E_CLASSSTRING))"},
		{"(ole.E_ABORT)", "(uint32(ole.E_ABORT))"},
		{"(ole.E_ACCESSDENIED)", "(uint32(ole.E_ACCESSDENIED))"},
		{"(ole.E_FAIL)", "(uint32(ole.E_FAIL))"},
		{"(ole.E_HANDLE)", "(uint32(ole.E_HANDLE))"},
		{"(ole.E_INVALIDARG)", "(uint32(ole.E_INVALIDARG))"},
		{"(ole.E_NOINTERFACE)", "(uint32(ole.E_NOINTERFACE))"},
		{"(ole.E_NOTIMPL)", "(uint32(ole.E_NOTIMPL))"},
		{"(ole.E_OUTOFMEMORY)", "(uint32(ole.E_OUTOFMEMORY))"},
		{"(ole.E_PENDING)", "(uint32(ole.E_PENDING))"},
		{"(ole.E_POINTER)", "(uint32(ole.E_POINTER))"},
		{"(ole.E_UNEXPECTED)", "(uint32(ole.E_UNEXPECTED))"},
	} {
		src = strings.ReplaceAll(src, item.old, item.new)
	}

	// delete code
	// for _, item := range []string{
	// 	`		"DecodeDatastoreKey": reflect.ValueOf(msgpack.DecodeDatastoreKey),` + "\n",
	// 	`		"EncodeDatastoreKey": reflect.ValueOf(msgpack.EncodeDatastoreKey),` + "\n",
	// } {
	// 	src = strings.ReplaceAll(src, item, "")
	// }

	// print and save code
	fmt.Println(src)
	const path = "../../../internal/anko/thirdparty/windows.go"
	err = system.WriteFile(path, []byte(src))
	require.NoError(t, err)
}