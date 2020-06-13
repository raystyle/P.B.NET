package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExportGoRoot(t *testing.T) {
	const template = `// Package gosrc generate by resource/code/anko/package.go, don't edit it.
package gosrc

import (
%s	"reflect"

	"github.com/mattn/anko/env"
)

func init() {
%s}
%s
`
	// get GOROOT
	output, err := exec.Command("go", "env", "GOROOT").CombinedOutput()
	require.NoError(t, err)
	goRoot := filepath.Join(strings.TrimSpace(string(output)), "src")

	pkgBuf := new(bytes.Buffer)
	initBuf := new(bytes.Buffer)
	srcBuf := new(bytes.Buffer)

	for _, item := range [...]*struct {
		name string
		init string
	}{
		{"archive/zip", "ArchiveZip"},
		{"bufio", "BufIO"},
		{"bytes", "Bytes"},
		{"compress/bzip2", "CompressBZip2"},
		{"compress/flate", "CompressFlate"},
		{"compress/gzip", "CompressGZip"},
		{"compress/zlib", "CompressZlib"},
		{"container/heap", "ContainerHeap"},
		{"container/list", "ContainerList"},
		{"container/ring", "ContainerRing"},
		{"context", "Context"},
		{"crypto", "Crypto"},
		{"crypto/aes", "CryptoAES"},
		{"crypto/cipher", "CryptoCipher"},
		{"crypto/des", "CryptoDES"},
		{"crypto/dsa", "CryptoDSA"},
		{"crypto/ecdsa", "CryptoECDSA"},
		{"crypto/ed25519", "CryptoEd25519"},
		{"crypto/elliptic", "CryptoElliptic"},
		{"crypto/hmac", "CryptoHMAC"},
		{"crypto/md5", "CryptoMD5"},
		{"crypto/rc4", "CryptoRC4"},
		{"crypto/rsa", "CryptoRSA"},
		{"crypto/sha1", "CryptoSHA1"},
		{"crypto/sha256", "CryptoSHA256"},
		{"crypto/sha512", "CryptoSHA512"},
		{"crypto/subtle", "CryptoSubtle"},
		{"crypto/tls", "CryptoTLS"},
		{"crypto/x509", "CryptoX509"},
		{"crypto/x509/pkix", "CryptoX509PKIX"},
		{"encoding", "Encoding"},
		{"encoding/base64", "EncodingBase64"},
		{"encoding/csv", "EncodingCSV"},
		{"encoding/hex", "EncodingHex"},
		{"encoding/json", "EncodingJSON"},
		{"encoding/pem", "EncodingPEM"},
		{"encoding/xml", "EncodingXML"},
		{"fmt", "FMT"},
		{"hash", "Hash"},
		{"hash/crc32", "HashCRC32"},
		{"hash/crc64", "HashCRC64"},
		{"io", "IO"},
		{"io/ioutil", "IOioutil"},
		{"math", "Math"},
		{"math/big", "MathBig"},
		{"math/rand", "MathRand"},
		{"net", "Net"},
		{"net/http", "NetHTTP"},
		{"net/http/cookiejar", "NetHTTPCookieJar"},
		{"net/url", "NetURL"},
		{"os", "OS"},
		{"os/exec", "OSExec"},
		{"os/signal", "OSSignal"},
		{"os/user", "OSUser"},
		{"path", "Path"},
		{"path/filepath", "PathFilepath"},
		{"regexp", "Regexp"},
		{"sort", "Sort"},
		{"strconv", "Strconv"},
		{"strings", "Strings"},
		{"sync", "Sync"},
		{"sync/atomic", "SyncAtomic"},
		{"time", "Time"},
		{"unicode", "Unicode"},
		{"unicode/utf8", "UnicodeUTF8"},
		{"unicode/utf16", "UnicodeUTF16"},
	} {
		_, _ = fmt.Fprintf(pkgBuf, `	"%s"`+"\n", item.name)
		_, _ = fmt.Fprintf(initBuf, "\tinit%s()\n", item.init)
		src, err := exportDeclaration(goRoot, item.name, item.init)
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
		{"interface heap.Interface", "iface heap.Interface"},
		{"(&interface)", "(&iface)"},

		{"list list.List", "ll list.List"},
		{"(&list)", "(&ll)"},

		{"ring ring.Ring", "r ring.Ring"},
		{"(&ring)", "(&r)"},

		{"context context.Context", "ctx context.Context"},
		{"(&context)", "(&ctx)"},

		{"cipher rc4.Cipher", "cip rc4.Cipher"},
		{"(&cipher)", "(&cip)"},

		{"hash crypto.Hash", "h crypto.Hash"},
		{"(&hash)", "(&h)"},

		{"encoding base64.Encoding", "enc base64.Encoding"},
		{"(&encoding)", "(&enc)"},

		{"hash hash.Hash", "h hash.Hash"},
		{"(&hash)", "(&h)"},

		{"int big.Int", "i big.Int"},
		{"(&int)", "(&i)"},

		{"rand rand.Rand", "r rand.Rand"},
		{"(&rand)", "(&r)"},

		{"error net.Error", "err net.Error"},
		{"(&error)", "(&err)"},

		{"interface net.Interface", "iface net.Interface"},
		{"(&interface)", "(&iface)"},

		{"error url.Error", "err url.Error"},
		{"(&error)", "(&err)"},

		{"signal os.Signal", "sig os.Signal"},
		{"(&signal)", "(&sig)"},

		{"error exec.Error", "err exec.Error"},
		{"(&error)", "(&err)"},

		{"user user.User", "usr user.User"},
		{"(&user)", "(&usr)"},

		{"regexp regexp.Regexp", "reg regexp.Regexp"},
		{"(&regexp)", "(&reg)"},

		{"interface sort.Interface", "iface sort.Interface"},
		{"(&interface)", "(&iface)"},

		{"map sync.Map", "m sync.Map"},
		{"(&map)", "(&m)"},

		{"time time.Time", "t time.Time"},
		{"(&time)", "(&t)"},

		{"time time.Time", "t time.Time"},
		{"(&time)", "(&t)"},

		// amd64
		{"(crc64.ECMA)", "(uint64(crc64.ECMA))"},
		{"(crc64.ISO)", "(uint64(crc64.ISO))"},
		{"(math.MaxUint64)", "(uint64(math.MaxUint64))"},

		// 386
		{"(crc32.IEEE)", "(uint32(crc32.IEEE))"},
		{"(crc32.Castagnoli)", "(uint32(crc32.Castagnoli))"},
		{"(crc32.Koopman)", "(uint32(crc32.Koopman))"},
		{"(math.MaxInt64)", "(int64(math.MaxInt64))"},
		{"(math.MaxUint32)", "(uint32(math.MaxUint32))"},
		{"(math.MinInt64)", "(int64(math.MinInt64))"},
		{"(big.MaxPrec)", "(uint32(big.MaxPrec))"},

		// skip gosec
		{`	"crypto/des"`, `	"crypto/des" // #nosec`},
		{`	"crypto/md5"`, `	"crypto/md5" // #nosec`},
		{`	"crypto/rc4"`, `	"crypto/rc4" // #nosec`},
		{`	"crypto/sha1"`, `	"crypto/sha1" // #nosec`},

		{"unknownUserIdError", "unknownUserIDError"},
		{"unknownGroupIdError", "unknownGroupIDError"},
	} {
		src = strings.ReplaceAll(src, item.old, item.new)
	}
	fmt.Println(src)
	const path = "../../../internal/anko/gosrc/src.go"
	err = ioutil.WriteFile(path, []byte(src), 0600)
	require.NoError(t, err)
}

func TestExportProject(t *testing.T) {
	const template = `// Package project generate by resource/code/anko/package.go, don't edit it.
package project

import (
	"reflect"

%s
	"github.com/mattn/anko/env"
)

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
		name string
		init string
	}{
		{"internal/patch/json", "InternalPatchJSON"},
	} {
		_, _ = fmt.Fprintf(pkgBuf, `	"project/%s"`+"\n", item.name)
		_, _ = fmt.Fprintf(initBuf, "\tinit%s()\n", item.init)
		src, err := exportDeclaration(dir, "$"+item.name, item.init)
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
		// {},
	} {
		src = strings.ReplaceAll(src, item.old, item.new)
	}
	fmt.Println(src)
	const path = "../../../internal/anko/project/project.go"
	err = ioutil.WriteFile(path, []byte(src), 0600)
	require.NoError(t, err)
}
