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
	const template = `// Package goroot generate by script/code/anko/package.go, don't edit it.
package goroot

import (
%s
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
		{"encoding/ascii85", "EncodingASCII85"},
		{"encoding/base32", "EncodingBase32"},
		{"encoding/base64", "EncodingBase64"},
		{"encoding/binary", "EncodingBinary"},
		{"encoding/csv", "EncodingCSV"},
		{"encoding/hex", "EncodingHex"},
		{"encoding/json", "EncodingJSON"},
		{"encoding/pem", "EncodingPEM"},
		{"encoding/xml", "EncodingXML"},
		{"fmt", "FMT"},
		{"hash", "Hash"},
		{"hash/crc32", "HashCRC32"},
		{"hash/crc64", "HashCRC64"},
		{"image", "Image"},
		{"image/color", "ImageColor"},
		{"image/draw", "ImageDraw"},
		{"image/gif", "ImageGIF"},
		{"image/jpeg", "ImageJPEG"},
		{"image/png", "ImagePNG"},
		{"io", "IO"},
		{"io/ioutil", "IOioutil"},
		{"log", "Log"},
		{"math", "Math"},
		{"math/big", "MathBig"},
		{"math/bits", "MathBits"},
		{"math/cmplx", "MathCmplx"},
		{"math/rand", "MathRand"},
		{"mime", "MIME"},
		{"mime/multipart", "MIMEMultiPart"},
		{"mime/quotedprintable", "MIMEQuotedPrintable"},
		{"net", "Net"},
		{"net/http", "NetHTTP"},
		{"net/http/cookiejar", "NetHTTPCookieJar"},
		{"net/mail", "NetMail"},
		{"net/smtp", "NetSMTP"},
		{"net/textproto", "NetTextProto"},
		{"net/url", "NetURL"},
		{"os", "OS"},
		{"os/exec", "OSExec"},
		{"os/signal", "OSSignal"},
		{"os/user", "OSUser"},
		{"path", "Path"},
		{"path/filepath", "PathFilepath"},
		{"reflect", "Reflect"},
		{"regexp", "Regexp"},
		{"sort", "Sort"},
		{"strconv", "Strconv"},
		{"strings", "Strings"},
		{"sync", "Sync"},
		{"sync/atomic", "SyncAtomic"},
		{"time", "Time"},
		{"unicode", "Unicode"},
		{"unicode/utf16", "UnicodeUTF16"},
		{"unicode/utf8", "UnicodeUTF8"},
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

		{"encoding base32.Encoding", "enc base32.Encoding"},
		{"(&encoding)", "(&enc)"},

		{"hash hash.Hash", "h hash.Hash"},
		{"(&hash)", "(&h)"},

		{"image image.Image", "img image.Image"},
		{"(&image)", "(&img)"},

		{"color color.Color", "c color.Color"},
		{"(&color)", "(&c)"},

		{"image draw.Image", "img draw.Image"},
		{"(&image)", "(&img)"},

		{"int big.Int", "i big.Int"},
		{"(&int)", "(&i)"},

		{"rand rand.Rand", "r rand.Rand"},
		{"(&rand)", "(&r)"},

		{"error net.Error", "err net.Error"},
		{"(&error)", "(&err)"},

		{"interface net.Interface", "iface net.Interface"},
		{"(&interface)", "(&iface)"},

		{"error textproto.Error", "err textproto.Error"},
		{"(&error)", "(&err)"},

		{"error url.Error", "err url.Error"},
		{"(&error)", "(&err)"},

		{"signal os.Signal", "sig os.Signal"},
		{"(&signal)", "(&sig)"},

		{"error exec.Error", "err exec.Error"},
		{"(&error)", "(&err)"},

		{"user user.User", "usr user.User"},
		{"(&user)", "(&usr)"},

		{"type reflect.Type", "typ reflect.Type"},
		{"(&type)", "(&typ)"},

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

	// print and save code
	fmt.Println(src)
	const path = "../../../internal/anko/goroot/bundle.go"
	err = ioutil.WriteFile(path, []byte(src), 0600)
	require.NoError(t, err)
}

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
	output, err := exec.Command("go", "env", "GOMODCACHE").CombinedOutput()
	require.NoError(t, err)
	goMod := strings.TrimSpace(string(output))

	pkgBuf := new(bytes.Buffer)
	initBuf := new(bytes.Buffer)
	srcBuf := new(bytes.Buffer)

	for _, item := range [...]*struct {
		name string
		path string
		init string
	}{
		{
			name: "github.com/kardianos/service",
			path: "github.com/kardianos/service@v1.1.0",
			init: "GithubComKardianosService",
		},
		{
			name: "github.com/pelletier/go-toml",
			path: "github.com/pelletier/go-toml@v1.8.1",
			init: "GithubComPelletierGoTOML",
		},
		{
			name: "github.com/vmihailenco/msgpack/v4",
			path: "github.com/vmihailenco/msgpack/v4@v4.3.12",
			init: "GithubComVmihailencoMsgpackV4",
		},
	} {
		_, _ = fmt.Fprintf(pkgBuf, `	"%s"`+"\n", item.name)
		_, _ = fmt.Fprintf(initBuf, "\tinit%s()\n", item.init)
		src, err := exportDeclaration(goMod, item.path, item.init)
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
		{"interface service.Interface", "iface service.Interface"},
		{"(&interface)", "(&iface)"},

		{"service service.Service", "svc service.Service"},
		{"(&service)", "(&svc)"},
	} {
		src = strings.ReplaceAll(src, item.old, item.new)
	}

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
	err = ioutil.WriteFile(path, []byte(src), 0600)
	require.NoError(t, err)
}

func TestExportProject(t *testing.T) {
	const template = `// Package project generate by script/code/anko/package.go, don't edit it.
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
		{"internal/patch/msgpack", "InternalPatchMsgpack"},
		{"internal/patch/toml", "InternalPatchToml"},
		{"internal/convert", "InternalConvert"},
		{"internal/httptool", "InternalHTTPTool"},
		{"internal/logger", "InternalLogger"},
		{"internal/nettool", "InternalNetTool"},
		{"internal/xpanic", "InternalXPanic"},
		{"internal/xreflect", "InternalXReflect"},
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
		{"logger logger.Logger", "lg logger.Logger"},
		{"(&logger)", "(&lg)"},
	} {
		src = strings.ReplaceAll(src, item.old, item.new)
	}

	// print and save code
	fmt.Println(src)
	const path = "../../../internal/anko/project/bundle.go"
	err = ioutil.WriteFile(path, []byte(src), 0600)
	require.NoError(t, err)
}
