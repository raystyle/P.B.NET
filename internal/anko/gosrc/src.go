// Package gosrc generate by resource/code/anko/package.go, don't edit it.
package gosrc

import (
	"archive/zip"
	"bufio"
	"bytes"
	"compress/bzip2"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"container/heap"
	"container/list"
	"container/ring"
	"context"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/des" // #nosec
	"crypto/dsa"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/md5" // #nosec
	"crypto/rc4" // #nosec
	"crypto/rsa"
	"crypto/sha1" // #nosec
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"encoding/xml"
	"fmt"
	"hash"
	"hash/crc32"
	"hash/crc64"
	"io"
	"io/ioutil"
	"math"
	"math/big"
	"math/rand"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/mattn/anko/env"
)

func init() {
	initArchiveZip()
	initBufIO()
	initBytes()
	initCompressBZip2()
	initCompressFlate()
	initCompressGZip()
	initCompressZlib()
	initContainerHeap()
	initContainerList()
	initContainerRing()
	initContext()
	initCrypto()
	initCryptoAES()
	initCryptoCipher()
	initCryptoDES()
	initCryptoDSA()
	initCryptoECDSA()
	initCryptoEd25519()
	initCryptoElliptic()
	initCryptoHMAC()
	initCryptoMD5()
	initCryptoRC4()
	initCryptoRSA()
	initCryptoSHA1()
	initCryptoSHA256()
	initCryptoSHA512()
	initCryptoSubtle()
	initCryptoTLS()
	initCryptoX509()
	initCryptoX509PKIX()
	initEncoding()
	initEncodingBase64()
	initEncodingCSV()
	initEncodingHex()
	initEncodingJSON()
	initEncodingPEM()
	initEncodingXML()
	initFMT()
	initHash()
	initHashCRC32()
	initHashCRC64()
	initIO()
	initIOioutil()
	initMath()
	initMathBig()
	initMathRand()
	initNet()
	initNetHTTP()
	initNetHTTPCookieJar()
	initNetURL()
	initOS()
	initOSExec()
	initOSSignal()
	initOSUser()
	initPath()
	initPathFilepath()
	initRegexp()
	initSort()
	initStrconv()
	initStrings()
	initSync()
	initSyncAtomic()
	initTime()
	initUnicode()
	initUnicodeUTF8()
	initUnicodeUTF16()
}

func initArchiveZip() {
	env.Packages["archive/zip"] = map[string]reflect.Value{
		// define constants
		"Store":   reflect.ValueOf(zip.Store),
		"Deflate": reflect.ValueOf(zip.Deflate),

		// define variables
		"ErrAlgorithm": reflect.ValueOf(zip.ErrAlgorithm),
		"ErrChecksum":  reflect.ValueOf(zip.ErrChecksum),
		"ErrFormat":    reflect.ValueOf(zip.ErrFormat),

		// define functions
		"OpenReader":           reflect.ValueOf(zip.OpenReader),
		"NewReader":            reflect.ValueOf(zip.NewReader),
		"RegisterDecompressor": reflect.ValueOf(zip.RegisterDecompressor),
		"RegisterCompressor":   reflect.ValueOf(zip.RegisterCompressor),
		"FileInfoHeader":       reflect.ValueOf(zip.FileInfoHeader),
		"NewWriter":            reflect.ValueOf(zip.NewWriter),
	}
	var (
		reader       zip.Reader
		readCloser   zip.ReadCloser
		file         zip.File
		compressor   zip.Compressor
		decompressor zip.Decompressor
		fileHeader   zip.FileHeader
		writer       zip.Writer
	)
	env.PackageTypes["archive/zip"] = map[string]reflect.Type{
		"Reader":       reflect.TypeOf(&reader).Elem(),
		"ReadCloser":   reflect.TypeOf(&readCloser).Elem(),
		"File":         reflect.TypeOf(&file).Elem(),
		"Compressor":   reflect.TypeOf(&compressor).Elem(),
		"Decompressor": reflect.TypeOf(&decompressor).Elem(),
		"FileHeader":   reflect.TypeOf(&fileHeader).Elem(),
		"Writer":       reflect.TypeOf(&writer).Elem(),
	}
}

func initBufIO() {
	env.Packages["bufio"] = map[string]reflect.Value{
		// define constants
		"MaxScanTokenSize": reflect.ValueOf(bufio.MaxScanTokenSize),

		// define variables
		"ErrTooLong":           reflect.ValueOf(bufio.ErrTooLong),
		"ErrNegativeAdvance":   reflect.ValueOf(bufio.ErrNegativeAdvance),
		"ErrNegativeCount":     reflect.ValueOf(bufio.ErrNegativeCount),
		"ErrInvalidUnreadRune": reflect.ValueOf(bufio.ErrInvalidUnreadRune),
		"ErrBufferFull":        reflect.ValueOf(bufio.ErrBufferFull),
		"ErrAdvanceTooFar":     reflect.ValueOf(bufio.ErrAdvanceTooFar),
		"ErrBadReadCount":      reflect.ValueOf(bufio.ErrBadReadCount),
		"ErrFinalToken":        reflect.ValueOf(bufio.ErrFinalToken),
		"ErrInvalidUnreadByte": reflect.ValueOf(bufio.ErrInvalidUnreadByte),

		// define functions
		"ScanBytes":     reflect.ValueOf(bufio.ScanBytes),
		"ScanRunes":     reflect.ValueOf(bufio.ScanRunes),
		"ScanWords":     reflect.ValueOf(bufio.ScanWords),
		"NewWriterSize": reflect.ValueOf(bufio.NewWriterSize),
		"NewWriter":     reflect.ValueOf(bufio.NewWriter),
		"NewReadWriter": reflect.ValueOf(bufio.NewReadWriter),
		"NewScanner":    reflect.ValueOf(bufio.NewScanner),
		"NewReaderSize": reflect.ValueOf(bufio.NewReaderSize),
		"NewReader":     reflect.ValueOf(bufio.NewReader),
		"ScanLines":     reflect.ValueOf(bufio.ScanLines),
	}
	var (
		reader     bufio.Reader
		writer     bufio.Writer
		readWriter bufio.ReadWriter
		scanner    bufio.Scanner
		splitFunc  bufio.SplitFunc
	)
	env.PackageTypes["bufio"] = map[string]reflect.Type{
		"Reader":     reflect.TypeOf(&reader).Elem(),
		"Writer":     reflect.TypeOf(&writer).Elem(),
		"ReadWriter": reflect.TypeOf(&readWriter).Elem(),
		"Scanner":    reflect.TypeOf(&scanner).Elem(),
		"SplitFunc":  reflect.TypeOf(&splitFunc).Elem(),
	}
}

func initBytes() {
	env.Packages["bytes"] = map[string]reflect.Value{
		// define constants
		"MinRead": reflect.ValueOf(bytes.MinRead),

		// define variables
		"ErrTooLarge": reflect.ValueOf(bytes.ErrTooLarge),

		// define functions
		"LastIndex":       reflect.ValueOf(bytes.LastIndex),
		"ToTitle":         reflect.ValueOf(bytes.ToTitle),
		"ToUpperSpecial":  reflect.ValueOf(bytes.ToUpperSpecial),
		"TrimRight":       reflect.ValueOf(bytes.TrimRight),
		"Runes":           reflect.ValueOf(bytes.Runes),
		"Index":           reflect.ValueOf(bytes.Index),
		"ToUpper":         reflect.ValueOf(bytes.ToUpper),
		"TrimSpace":       reflect.ValueOf(bytes.TrimSpace),
		"ContainsRune":    reflect.ValueOf(bytes.ContainsRune),
		"LastIndexByte":   reflect.ValueOf(bytes.LastIndexByte),
		"IndexAny":        reflect.ValueOf(bytes.IndexAny),
		"LastIndexAny":    reflect.ValueOf(bytes.LastIndexAny),
		"SplitAfterN":     reflect.ValueOf(bytes.SplitAfterN),
		"SplitAfter":      reflect.ValueOf(bytes.SplitAfter),
		"Replace":         reflect.ValueOf(bytes.Replace),
		"Compare":         reflect.ValueOf(bytes.Compare),
		"Split":           reflect.ValueOf(bytes.Split),
		"Join":            reflect.ValueOf(bytes.Join),
		"HasPrefix":       reflect.ValueOf(bytes.HasPrefix),
		"Map":             reflect.ValueOf(bytes.Map),
		"Trim":            reflect.ValueOf(bytes.Trim),
		"NewReader":       reflect.ValueOf(bytes.NewReader),
		"IndexRune":       reflect.ValueOf(bytes.IndexRune),
		"ToLowerSpecial":  reflect.ValueOf(bytes.ToLowerSpecial),
		"ToValidUTF8":     reflect.ValueOf(bytes.ToValidUTF8),
		"IndexFunc":       reflect.ValueOf(bytes.IndexFunc),
		"LastIndexFunc":   reflect.ValueOf(bytes.LastIndexFunc),
		"ReplaceAll":      reflect.ValueOf(bytes.ReplaceAll),
		"Equal":           reflect.ValueOf(bytes.Equal),
		"SplitN":          reflect.ValueOf(bytes.SplitN),
		"Fields":          reflect.ValueOf(bytes.Fields),
		"TrimPrefix":      reflect.ValueOf(bytes.TrimPrefix),
		"EqualFold":       reflect.ValueOf(bytes.EqualFold),
		"NewBuffer":       reflect.ValueOf(bytes.NewBuffer),
		"NewBufferString": reflect.ValueOf(bytes.NewBufferString),
		"HasSuffix":       reflect.ValueOf(bytes.HasSuffix),
		"ToLower":         reflect.ValueOf(bytes.ToLower),
		"Title":           reflect.ValueOf(bytes.Title),
		"TrimLeft":        reflect.ValueOf(bytes.TrimLeft),
		"TrimSuffix":      reflect.ValueOf(bytes.TrimSuffix),
		"Count":           reflect.ValueOf(bytes.Count),
		"ContainsAny":     reflect.ValueOf(bytes.ContainsAny),
		"IndexByte":       reflect.ValueOf(bytes.IndexByte),
		"Repeat":          reflect.ValueOf(bytes.Repeat),
		"TrimRightFunc":   reflect.ValueOf(bytes.TrimRightFunc),
		"TrimFunc":        reflect.ValueOf(bytes.TrimFunc),
		"Contains":        reflect.ValueOf(bytes.Contains),
		"FieldsFunc":      reflect.ValueOf(bytes.FieldsFunc),
		"ToTitleSpecial":  reflect.ValueOf(bytes.ToTitleSpecial),
		"TrimLeftFunc":    reflect.ValueOf(bytes.TrimLeftFunc),
	}
	var (
		buffer bytes.Buffer
		reader bytes.Reader
	)
	env.PackageTypes["bytes"] = map[string]reflect.Type{
		"Buffer": reflect.TypeOf(&buffer).Elem(),
		"Reader": reflect.TypeOf(&reader).Elem(),
	}
}

func initCompressBZip2() {
	env.Packages["compress/bzip2"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
		"NewReader": reflect.ValueOf(bzip2.NewReader),
	}
	var (
		structuralError bzip2.StructuralError
	)
	env.PackageTypes["compress/bzip2"] = map[string]reflect.Type{
		"StructuralError": reflect.TypeOf(&structuralError).Elem(),
	}
}

func initCompressFlate() {
	env.Packages["compress/flate"] = map[string]reflect.Value{
		// define constants
		"DefaultCompression": reflect.ValueOf(flate.DefaultCompression),
		"HuffmanOnly":        reflect.ValueOf(flate.HuffmanOnly),
		"NoCompression":      reflect.ValueOf(flate.NoCompression),
		"BestSpeed":          reflect.ValueOf(flate.BestSpeed),
		"BestCompression":    reflect.ValueOf(flate.BestCompression),

		// define variables

		// define functions
		"NewReader":     reflect.ValueOf(flate.NewReader),
		"NewReaderDict": reflect.ValueOf(flate.NewReaderDict),
		"NewWriter":     reflect.ValueOf(flate.NewWriter),
		"NewWriterDict": reflect.ValueOf(flate.NewWriterDict),
	}
	var (
		corruptInputError flate.CorruptInputError
		internalError     flate.InternalError
		resetter          flate.Resetter
		reader            flate.Reader
		writer            flate.Writer
	)
	env.PackageTypes["compress/flate"] = map[string]reflect.Type{
		"CorruptInputError": reflect.TypeOf(&corruptInputError).Elem(),
		"InternalError":     reflect.TypeOf(&internalError).Elem(),
		"Resetter":          reflect.TypeOf(&resetter).Elem(),
		"Reader":            reflect.TypeOf(&reader).Elem(),
		"Writer":            reflect.TypeOf(&writer).Elem(),
	}
}

func initCompressGZip() {
	env.Packages["compress/gzip"] = map[string]reflect.Value{
		// define constants
		"NoCompression":      reflect.ValueOf(gzip.NoCompression),
		"BestSpeed":          reflect.ValueOf(gzip.BestSpeed),
		"BestCompression":    reflect.ValueOf(gzip.BestCompression),
		"DefaultCompression": reflect.ValueOf(gzip.DefaultCompression),
		"HuffmanOnly":        reflect.ValueOf(gzip.HuffmanOnly),

		// define variables
		"ErrHeader":   reflect.ValueOf(gzip.ErrHeader),
		"ErrChecksum": reflect.ValueOf(gzip.ErrChecksum),

		// define functions
		"NewWriter":      reflect.ValueOf(gzip.NewWriter),
		"NewWriterLevel": reflect.ValueOf(gzip.NewWriterLevel),
		"NewReader":      reflect.ValueOf(gzip.NewReader),
	}
	var (
		writer gzip.Writer
		header gzip.Header
		reader gzip.Reader
	)
	env.PackageTypes["compress/gzip"] = map[string]reflect.Type{
		"Writer": reflect.TypeOf(&writer).Elem(),
		"Header": reflect.TypeOf(&header).Elem(),
		"Reader": reflect.TypeOf(&reader).Elem(),
	}
}

func initCompressZlib() {
	env.Packages["compress/zlib"] = map[string]reflect.Value{
		// define constants
		"DefaultCompression": reflect.ValueOf(zlib.DefaultCompression),
		"HuffmanOnly":        reflect.ValueOf(zlib.HuffmanOnly),
		"NoCompression":      reflect.ValueOf(zlib.NoCompression),
		"BestSpeed":          reflect.ValueOf(zlib.BestSpeed),
		"BestCompression":    reflect.ValueOf(zlib.BestCompression),

		// define variables
		"ErrChecksum":   reflect.ValueOf(zlib.ErrChecksum),
		"ErrDictionary": reflect.ValueOf(zlib.ErrDictionary),
		"ErrHeader":     reflect.ValueOf(zlib.ErrHeader),

		// define functions
		"NewWriter":          reflect.ValueOf(zlib.NewWriter),
		"NewWriterLevel":     reflect.ValueOf(zlib.NewWriterLevel),
		"NewWriterLevelDict": reflect.ValueOf(zlib.NewWriterLevelDict),
		"NewReader":          reflect.ValueOf(zlib.NewReader),
		"NewReaderDict":      reflect.ValueOf(zlib.NewReaderDict),
	}
	var (
		writer   zlib.Writer
		resetter zlib.Resetter
	)
	env.PackageTypes["compress/zlib"] = map[string]reflect.Type{
		"Writer":   reflect.TypeOf(&writer).Elem(),
		"Resetter": reflect.TypeOf(&resetter).Elem(),
	}
}

func initContainerHeap() {
	env.Packages["container/heap"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
		"Init":   reflect.ValueOf(heap.Init),
		"Push":   reflect.ValueOf(heap.Push),
		"Pop":    reflect.ValueOf(heap.Pop),
		"Remove": reflect.ValueOf(heap.Remove),
		"Fix":    reflect.ValueOf(heap.Fix),
	}
	var (
		iface heap.Interface
	)
	env.PackageTypes["container/heap"] = map[string]reflect.Type{
		"Interface": reflect.TypeOf(&iface).Elem(),
	}
}

func initContainerList() {
	env.Packages["container/list"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
		"New": reflect.ValueOf(list.New),
	}
	var (
		element list.Element
		ll      list.List
	)
	env.PackageTypes["container/list"] = map[string]reflect.Type{
		"Element": reflect.TypeOf(&element).Elem(),
		"List":    reflect.TypeOf(&ll).Elem(),
	}
}

func initContainerRing() {
	env.Packages["container/ring"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
		"New": reflect.ValueOf(ring.New),
	}
	var (
		r ring.Ring
	)
	env.PackageTypes["container/ring"] = map[string]reflect.Type{
		"Ring": reflect.TypeOf(&r).Elem(),
	}
}

func initContext() {
	env.Packages["context"] = map[string]reflect.Value{
		// define constants

		// define variables
		"Canceled":         reflect.ValueOf(context.Canceled),
		"DeadlineExceeded": reflect.ValueOf(context.DeadlineExceeded),

		// define functions
		"WithDeadline": reflect.ValueOf(context.WithDeadline),
		"WithTimeout":  reflect.ValueOf(context.WithTimeout),
		"WithValue":    reflect.ValueOf(context.WithValue),
		"Background":   reflect.ValueOf(context.Background),
		"TODO":         reflect.ValueOf(context.TODO),
		"WithCancel":   reflect.ValueOf(context.WithCancel),
	}
	var (
		ctx        context.Context
		cancelFunc context.CancelFunc
	)
	env.PackageTypes["context"] = map[string]reflect.Type{
		"Context":    reflect.TypeOf(&ctx).Elem(),
		"CancelFunc": reflect.TypeOf(&cancelFunc).Elem(),
	}
}

func initCrypto() {
	env.Packages["crypto"] = map[string]reflect.Value{
		// define constants
		"MD5":         reflect.ValueOf(crypto.MD5),
		"SHA512":      reflect.ValueOf(crypto.SHA512),
		"MD5SHA1":     reflect.ValueOf(crypto.MD5SHA1),
		"SHA256":      reflect.ValueOf(crypto.SHA256),
		"SHA3_512":    reflect.ValueOf(crypto.SHA3_512),
		"BLAKE2b_512": reflect.ValueOf(crypto.BLAKE2b_512),
		"MD4":         reflect.ValueOf(crypto.MD4),
		"SHA384":      reflect.ValueOf(crypto.SHA384),
		"SHA3_224":    reflect.ValueOf(crypto.SHA3_224),
		"SHA3_256":    reflect.ValueOf(crypto.SHA3_256),
		"SHA3_384":    reflect.ValueOf(crypto.SHA3_384),
		"SHA512_224":  reflect.ValueOf(crypto.SHA512_224),
		"SHA512_256":  reflect.ValueOf(crypto.SHA512_256),
		"SHA1":        reflect.ValueOf(crypto.SHA1),
		"SHA224":      reflect.ValueOf(crypto.SHA224),
		"RIPEMD160":   reflect.ValueOf(crypto.RIPEMD160),
		"BLAKE2s_256": reflect.ValueOf(crypto.BLAKE2s_256),
		"BLAKE2b_256": reflect.ValueOf(crypto.BLAKE2b_256),
		"BLAKE2b_384": reflect.ValueOf(crypto.BLAKE2b_384),

		// define variables

		// define functions
		"RegisterHash": reflect.ValueOf(crypto.RegisterHash),
	}
	var (
		signer        crypto.Signer
		signerOpts    crypto.SignerOpts
		decrypter     crypto.Decrypter
		decrypterOpts crypto.DecrypterOpts
		h             crypto.Hash
		publicKey     crypto.PublicKey
		privateKey    crypto.PrivateKey
	)
	env.PackageTypes["crypto"] = map[string]reflect.Type{
		"Signer":        reflect.TypeOf(&signer).Elem(),
		"SignerOpts":    reflect.TypeOf(&signerOpts).Elem(),
		"Decrypter":     reflect.TypeOf(&decrypter).Elem(),
		"DecrypterOpts": reflect.TypeOf(&decrypterOpts).Elem(),
		"Hash":          reflect.TypeOf(&h).Elem(),
		"PublicKey":     reflect.TypeOf(&publicKey).Elem(),
		"PrivateKey":    reflect.TypeOf(&privateKey).Elem(),
	}
}

func initCryptoAES() {
	env.Packages["crypto/aes"] = map[string]reflect.Value{
		// define constants
		"BlockSize": reflect.ValueOf(aes.BlockSize),

		// define variables

		// define functions
		"NewCipher": reflect.ValueOf(aes.NewCipher),
	}
	var (
		keySizeError aes.KeySizeError
	)
	env.PackageTypes["crypto/aes"] = map[string]reflect.Type{
		"KeySizeError": reflect.TypeOf(&keySizeError).Elem(),
	}
}

func initCryptoCipher() {
	env.Packages["crypto/cipher"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
		"NewOFB":              reflect.ValueOf(cipher.NewOFB),
		"NewCBCDecrypter":     reflect.ValueOf(cipher.NewCBCDecrypter),
		"NewCFBEncrypter":     reflect.ValueOf(cipher.NewCFBEncrypter),
		"NewCTR":              reflect.ValueOf(cipher.NewCTR),
		"NewCFBDecrypter":     reflect.ValueOf(cipher.NewCFBDecrypter),
		"NewGCM":              reflect.ValueOf(cipher.NewGCM),
		"NewGCMWithNonceSize": reflect.ValueOf(cipher.NewGCMWithNonceSize),
		"NewGCMWithTagSize":   reflect.ValueOf(cipher.NewGCMWithTagSize),
		"NewCBCEncrypter":     reflect.ValueOf(cipher.NewCBCEncrypter),
	}
	var (
		streamReader cipher.StreamReader
		streamWriter cipher.StreamWriter
		aEAD         cipher.AEAD
		block        cipher.Block
		stream       cipher.Stream
		blockMode    cipher.BlockMode
	)
	env.PackageTypes["crypto/cipher"] = map[string]reflect.Type{
		"StreamReader": reflect.TypeOf(&streamReader).Elem(),
		"StreamWriter": reflect.TypeOf(&streamWriter).Elem(),
		"AEAD":         reflect.TypeOf(&aEAD).Elem(),
		"Block":        reflect.TypeOf(&block).Elem(),
		"Stream":       reflect.TypeOf(&stream).Elem(),
		"BlockMode":    reflect.TypeOf(&blockMode).Elem(),
	}
}

func initCryptoDES() {
	env.Packages["crypto/des"] = map[string]reflect.Value{
		// define constants
		"BlockSize": reflect.ValueOf(des.BlockSize),

		// define variables

		// define functions
		"NewCipher":          reflect.ValueOf(des.NewCipher),
		"NewTripleDESCipher": reflect.ValueOf(des.NewTripleDESCipher),
	}
	var (
		keySizeError des.KeySizeError
	)
	env.PackageTypes["crypto/des"] = map[string]reflect.Type{
		"KeySizeError": reflect.TypeOf(&keySizeError).Elem(),
	}
}

func initCryptoDSA() {
	env.Packages["crypto/dsa"] = map[string]reflect.Value{
		// define constants
		"L2048N256": reflect.ValueOf(dsa.L2048N256),
		"L3072N256": reflect.ValueOf(dsa.L3072N256),
		"L1024N160": reflect.ValueOf(dsa.L1024N160),
		"L2048N224": reflect.ValueOf(dsa.L2048N224),

		// define variables
		"ErrInvalidPublicKey": reflect.ValueOf(dsa.ErrInvalidPublicKey),

		// define functions
		"GenerateParameters": reflect.ValueOf(dsa.GenerateParameters),
		"GenerateKey":        reflect.ValueOf(dsa.GenerateKey),
		"Sign":               reflect.ValueOf(dsa.Sign),
		"Verify":             reflect.ValueOf(dsa.Verify),
	}
	var (
		privateKey     dsa.PrivateKey
		parameterSizes dsa.ParameterSizes
		parameters     dsa.Parameters
		publicKey      dsa.PublicKey
	)
	env.PackageTypes["crypto/dsa"] = map[string]reflect.Type{
		"PrivateKey":     reflect.TypeOf(&privateKey).Elem(),
		"ParameterSizes": reflect.TypeOf(&parameterSizes).Elem(),
		"Parameters":     reflect.TypeOf(&parameters).Elem(),
		"PublicKey":      reflect.TypeOf(&publicKey).Elem(),
	}
}

func initCryptoECDSA() {
	env.Packages["crypto/ecdsa"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
		"VerifyASN1":  reflect.ValueOf(ecdsa.VerifyASN1),
		"GenerateKey": reflect.ValueOf(ecdsa.GenerateKey),
		"Sign":        reflect.ValueOf(ecdsa.Sign),
		"SignASN1":    reflect.ValueOf(ecdsa.SignASN1),
		"Verify":      reflect.ValueOf(ecdsa.Verify),
	}
	var (
		publicKey  ecdsa.PublicKey
		privateKey ecdsa.PrivateKey
	)
	env.PackageTypes["crypto/ecdsa"] = map[string]reflect.Type{
		"PublicKey":  reflect.TypeOf(&publicKey).Elem(),
		"PrivateKey": reflect.TypeOf(&privateKey).Elem(),
	}
}

func initCryptoEd25519() {
	env.Packages["crypto/ed25519"] = map[string]reflect.Value{
		// define constants
		"PrivateKeySize": reflect.ValueOf(ed25519.PrivateKeySize),
		"SignatureSize":  reflect.ValueOf(ed25519.SignatureSize),
		"SeedSize":       reflect.ValueOf(ed25519.SeedSize),
		"PublicKeySize":  reflect.ValueOf(ed25519.PublicKeySize),

		// define variables

		// define functions
		"GenerateKey":    reflect.ValueOf(ed25519.GenerateKey),
		"NewKeyFromSeed": reflect.ValueOf(ed25519.NewKeyFromSeed),
		"Sign":           reflect.ValueOf(ed25519.Sign),
		"Verify":         reflect.ValueOf(ed25519.Verify),
	}
	var (
		publicKey  ed25519.PublicKey
		privateKey ed25519.PrivateKey
	)
	env.PackageTypes["crypto/ed25519"] = map[string]reflect.Type{
		"PublicKey":  reflect.TypeOf(&publicKey).Elem(),
		"PrivateKey": reflect.TypeOf(&privateKey).Elem(),
	}
}

func initCryptoElliptic() {
	env.Packages["crypto/elliptic"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
		"P384":                reflect.ValueOf(elliptic.P384),
		"MarshalCompressed":   reflect.ValueOf(elliptic.MarshalCompressed),
		"Unmarshal":           reflect.ValueOf(elliptic.Unmarshal),
		"UnmarshalCompressed": reflect.ValueOf(elliptic.UnmarshalCompressed),
		"P256":                reflect.ValueOf(elliptic.P256),
		"GenerateKey":         reflect.ValueOf(elliptic.GenerateKey),
		"Marshal":             reflect.ValueOf(elliptic.Marshal),
		"P521":                reflect.ValueOf(elliptic.P521),
		"P224":                reflect.ValueOf(elliptic.P224),
	}
	var (
		curve       elliptic.Curve
		curveParams elliptic.CurveParams
	)
	env.PackageTypes["crypto/elliptic"] = map[string]reflect.Type{
		"Curve":       reflect.TypeOf(&curve).Elem(),
		"CurveParams": reflect.TypeOf(&curveParams).Elem(),
	}
}

func initCryptoHMAC() {
	env.Packages["crypto/hmac"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
		"New":   reflect.ValueOf(hmac.New),
		"Equal": reflect.ValueOf(hmac.Equal),
	}
	var ()
	env.PackageTypes["crypto/hmac"] = map[string]reflect.Type{}
}

func initCryptoMD5() {
	env.Packages["crypto/md5"] = map[string]reflect.Value{
		// define constants
		"Size":      reflect.ValueOf(md5.Size),
		"BlockSize": reflect.ValueOf(md5.BlockSize),

		// define variables

		// define functions
		"New": reflect.ValueOf(md5.New),
		"Sum": reflect.ValueOf(md5.Sum),
	}
	var ()
	env.PackageTypes["crypto/md5"] = map[string]reflect.Type{}
}

func initCryptoRC4() {
	env.Packages["crypto/rc4"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
		"NewCipher": reflect.ValueOf(rc4.NewCipher),
	}
	var (
		cip          rc4.Cipher
		keySizeError rc4.KeySizeError
	)
	env.PackageTypes["crypto/rc4"] = map[string]reflect.Type{
		"Cipher":       reflect.TypeOf(&cip).Elem(),
		"KeySizeError": reflect.TypeOf(&keySizeError).Elem(),
	}
}

func initCryptoRSA() {
	env.Packages["crypto/rsa"] = map[string]reflect.Value{
		// define constants
		"PSSSaltLengthAuto":       reflect.ValueOf(rsa.PSSSaltLengthAuto),
		"PSSSaltLengthEqualsHash": reflect.ValueOf(rsa.PSSSaltLengthEqualsHash),

		// define variables
		"ErrMessageTooLong": reflect.ValueOf(rsa.ErrMessageTooLong),
		"ErrDecryption":     reflect.ValueOf(rsa.ErrDecryption),
		"ErrVerification":   reflect.ValueOf(rsa.ErrVerification),

		// define functions
		"DecryptOAEP":               reflect.ValueOf(rsa.DecryptOAEP),
		"EncryptPKCS1v15":           reflect.ValueOf(rsa.EncryptPKCS1v15),
		"SignPKCS1v15":              reflect.ValueOf(rsa.SignPKCS1v15),
		"VerifyPKCS1v15":            reflect.ValueOf(rsa.VerifyPKCS1v15),
		"SignPSS":                   reflect.ValueOf(rsa.SignPSS),
		"VerifyPSS":                 reflect.ValueOf(rsa.VerifyPSS),
		"GenerateKey":               reflect.ValueOf(rsa.GenerateKey),
		"GenerateMultiPrimeKey":     reflect.ValueOf(rsa.GenerateMultiPrimeKey),
		"EncryptOAEP":               reflect.ValueOf(rsa.EncryptOAEP),
		"DecryptPKCS1v15":           reflect.ValueOf(rsa.DecryptPKCS1v15),
		"DecryptPKCS1v15SessionKey": reflect.ValueOf(rsa.DecryptPKCS1v15SessionKey),
	}
	var (
		precomputedValues      rsa.PrecomputedValues
		cRTValue               rsa.CRTValue
		pKCS1v15DecryptOptions rsa.PKCS1v15DecryptOptions
		pSSOptions             rsa.PSSOptions
		publicKey              rsa.PublicKey
		oAEPOptions            rsa.OAEPOptions
		privateKey             rsa.PrivateKey
	)
	env.PackageTypes["crypto/rsa"] = map[string]reflect.Type{
		"PrecomputedValues":      reflect.TypeOf(&precomputedValues).Elem(),
		"CRTValue":               reflect.TypeOf(&cRTValue).Elem(),
		"PKCS1v15DecryptOptions": reflect.TypeOf(&pKCS1v15DecryptOptions).Elem(),
		"PSSOptions":             reflect.TypeOf(&pSSOptions).Elem(),
		"PublicKey":              reflect.TypeOf(&publicKey).Elem(),
		"OAEPOptions":            reflect.TypeOf(&oAEPOptions).Elem(),
		"PrivateKey":             reflect.TypeOf(&privateKey).Elem(),
	}
}

func initCryptoSHA1() {
	env.Packages["crypto/sha1"] = map[string]reflect.Value{
		// define constants
		"Size":      reflect.ValueOf(sha1.Size),
		"BlockSize": reflect.ValueOf(sha1.BlockSize),

		// define variables

		// define functions
		"New": reflect.ValueOf(sha1.New),
		"Sum": reflect.ValueOf(sha1.Sum),
	}
	var ()
	env.PackageTypes["crypto/sha1"] = map[string]reflect.Type{}
}

func initCryptoSHA256() {
	env.Packages["crypto/sha256"] = map[string]reflect.Value{
		// define constants
		"Size":      reflect.ValueOf(sha256.Size),
		"Size224":   reflect.ValueOf(sha256.Size224),
		"BlockSize": reflect.ValueOf(sha256.BlockSize),

		// define variables

		// define functions
		"New":    reflect.ValueOf(sha256.New),
		"New224": reflect.ValueOf(sha256.New224),
		"Sum256": reflect.ValueOf(sha256.Sum256),
		"Sum224": reflect.ValueOf(sha256.Sum224),
	}
	var ()
	env.PackageTypes["crypto/sha256"] = map[string]reflect.Type{}
}

func initCryptoSHA512() {
	env.Packages["crypto/sha512"] = map[string]reflect.Value{
		// define constants
		"Size256":   reflect.ValueOf(sha512.Size256),
		"Size384":   reflect.ValueOf(sha512.Size384),
		"BlockSize": reflect.ValueOf(sha512.BlockSize),
		"Size":      reflect.ValueOf(sha512.Size),
		"Size224":   reflect.ValueOf(sha512.Size224),

		// define variables

		// define functions
		"New384":     reflect.ValueOf(sha512.New384),
		"Sum512":     reflect.ValueOf(sha512.Sum512),
		"Sum384":     reflect.ValueOf(sha512.Sum384),
		"Sum512_224": reflect.ValueOf(sha512.Sum512_224),
		"Sum512_256": reflect.ValueOf(sha512.Sum512_256),
		"New":        reflect.ValueOf(sha512.New),
		"New512_224": reflect.ValueOf(sha512.New512_224),
		"New512_256": reflect.ValueOf(sha512.New512_256),
	}
	var ()
	env.PackageTypes["crypto/sha512"] = map[string]reflect.Type{}
}

func initCryptoSubtle() {
	env.Packages["crypto/subtle"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
		"ConstantTimeEq":       reflect.ValueOf(subtle.ConstantTimeEq),
		"ConstantTimeCopy":     reflect.ValueOf(subtle.ConstantTimeCopy),
		"ConstantTimeLessOrEq": reflect.ValueOf(subtle.ConstantTimeLessOrEq),
		"ConstantTimeCompare":  reflect.ValueOf(subtle.ConstantTimeCompare),
		"ConstantTimeSelect":   reflect.ValueOf(subtle.ConstantTimeSelect),
		"ConstantTimeByteEq":   reflect.ValueOf(subtle.ConstantTimeByteEq),
	}
	var ()
	env.PackageTypes["crypto/subtle"] = map[string]reflect.Type{}
}

func initCryptoTLS() {
	env.Packages["crypto/tls"] = map[string]reflect.Value{
		// define constants
		"ECDSAWithP384AndSHA384":                        reflect.ValueOf(tls.ECDSAWithP384AndSHA384),
		"TLS_FALLBACK_SCSV":                             reflect.ValueOf(tls.TLS_FALLBACK_SCSV),
		"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305":          reflect.ValueOf(tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305),
		"Ed25519":                                       reflect.ValueOf(tls.Ed25519),
		"TLS_RSA_WITH_RC4_128_SHA":                      reflect.ValueOf(tls.TLS_RSA_WITH_RC4_128_SHA),
		"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256":         reflect.ValueOf(tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256),
		"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256": reflect.ValueOf(tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256),
		"ECDSAWithSHA1":                                 reflect.ValueOf(tls.ECDSAWithSHA1),
		"RenegotiateFreelyAsClient":                     reflect.ValueOf(tls.RenegotiateFreelyAsClient),
		"TLS_RSA_WITH_AES_128_CBC_SHA":                  reflect.ValueOf(tls.TLS_RSA_WITH_AES_128_CBC_SHA),
		"CurveP521":                                     reflect.ValueOf(tls.CurveP521),
		"X25519":                                        reflect.ValueOf(tls.X25519),
		"NoClientCert":                                  reflect.ValueOf(tls.NoClientCert),
		"PSSWithSHA256":                                 reflect.ValueOf(tls.PSSWithSHA256),
		"PKCS1WithSHA1":                                 reflect.ValueOf(tls.PKCS1WithSHA1),
		"TLS_RSA_WITH_AES_256_GCM_SHA384":               reflect.ValueOf(tls.TLS_RSA_WITH_AES_256_GCM_SHA384),
		"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256":       reflect.ValueOf(tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256),
		"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256":   reflect.ValueOf(tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256),
		"TLS_ECDHE_ECDSA_WITH_RC4_128_SHA":              reflect.ValueOf(tls.TLS_ECDHE_ECDSA_WITH_RC4_128_SHA),
		"RenegotiateNever":                              reflect.ValueOf(tls.RenegotiateNever),
		"CurveP384":                                     reflect.ValueOf(tls.CurveP384),
		"PSSWithSHA512":                                 reflect.ValueOf(tls.PSSWithSHA512),
		"CurveP256":                                     reflect.ValueOf(tls.CurveP256),
		"ECDSAWithP521AndSHA512":                        reflect.ValueOf(tls.ECDSAWithP521AndSHA512),
		"VersionTLS13":                                  reflect.ValueOf(tls.VersionTLS13),
		"PSSWithSHA384":                                 reflect.ValueOf(tls.PSSWithSHA384),
		"ECDSAWithP256AndSHA256":                        reflect.ValueOf(tls.ECDSAWithP256AndSHA256),
		"TLS_RSA_WITH_3DES_EDE_CBC_SHA":                 reflect.ValueOf(tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA),
		"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA":            reflect.ValueOf(tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA),
		"VersionTLS11":                                  reflect.ValueOf(tls.VersionTLS11),
		"RequireAndVerifyClientCert":                    reflect.ValueOf(tls.RequireAndVerifyClientCert),
		"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256":       reflect.ValueOf(tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256),
		"PKCS1WithSHA256":                               reflect.ValueOf(tls.PKCS1WithSHA256),
		"TLS_RSA_WITH_AES_256_CBC_SHA":                  reflect.ValueOf(tls.TLS_RSA_WITH_AES_256_CBC_SHA),
		"TLS_RSA_WITH_AES_128_GCM_SHA256":               reflect.ValueOf(tls.TLS_RSA_WITH_AES_128_GCM_SHA256),
		"TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA":            reflect.ValueOf(tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA),
		"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256":         reflect.ValueOf(tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256),
		"VerifyClientCertIfGiven":                       reflect.ValueOf(tls.VerifyClientCertIfGiven),
		"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384":       reflect.ValueOf(tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384),
		"TLS_AES_128_GCM_SHA256":                        reflect.ValueOf(tls.TLS_AES_128_GCM_SHA256),
		"PKCS1WithSHA384":                               reflect.ValueOf(tls.PKCS1WithSHA384),
		"TLS_RSA_WITH_AES_128_CBC_SHA256":               reflect.ValueOf(tls.TLS_RSA_WITH_AES_128_CBC_SHA256),
		"TLS_ECDHE_RSA_WITH_RC4_128_SHA":                reflect.ValueOf(tls.TLS_ECDHE_RSA_WITH_RC4_128_SHA),
		"TLS_CHACHA20_POLY1305_SHA256":                  reflect.ValueOf(tls.TLS_CHACHA20_POLY1305_SHA256),
		"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305":        reflect.ValueOf(tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305),
		"TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA":          reflect.ValueOf(tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA),
		"TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA":           reflect.ValueOf(tls.TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA),
		"VersionTLS10":                                  reflect.ValueOf(tls.VersionTLS10),
		"VersionTLS12":                                  reflect.ValueOf(tls.VersionTLS12),
		"RequestClientCert":                             reflect.ValueOf(tls.RequestClientCert),
		"PKCS1WithSHA512":                               reflect.ValueOf(tls.PKCS1WithSHA512),
		"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384":         reflect.ValueOf(tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384),
		"RequireAnyClientCert":                          reflect.ValueOf(tls.RequireAnyClientCert),
		"RenegotiateOnceAsClient":                       reflect.ValueOf(tls.RenegotiateOnceAsClient),
		"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA":          reflect.ValueOf(tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA),
		"TLS_AES_256_GCM_SHA384":                        reflect.ValueOf(tls.TLS_AES_256_GCM_SHA384),

		// define variables

		// define functions
		"NewLRUClientSessionCache": reflect.ValueOf(tls.NewLRUClientSessionCache),
		"CipherSuiteName":          reflect.ValueOf(tls.CipherSuiteName),
		"DialWithDialer":           reflect.ValueOf(tls.DialWithDialer),
		"LoadX509KeyPair":          reflect.ValueOf(tls.LoadX509KeyPair),
		"X509KeyPair":              reflect.ValueOf(tls.X509KeyPair),
		"Listen":                   reflect.ValueOf(tls.Listen),
		"Dial":                     reflect.ValueOf(tls.Dial),
		"CipherSuites":             reflect.ValueOf(tls.CipherSuites),
		"InsecureCipherSuites":     reflect.ValueOf(tls.InsecureCipherSuites),
		"Server":                   reflect.ValueOf(tls.Server),
		"Client":                   reflect.ValueOf(tls.Client),
		"NewListener":              reflect.ValueOf(tls.NewListener),
	}
	var (
		clientSessionCache     tls.ClientSessionCache
		signatureScheme        tls.SignatureScheme
		renegotiationSupport   tls.RenegotiationSupport
		certificate            tls.Certificate
		dialer                 tls.Dialer
		clientAuthType         tls.ClientAuthType
		clientSessionState     tls.ClientSessionState
		cipherSuite            tls.CipherSuite
		curveID                tls.CurveID
		connectionState        tls.ConnectionState
		certificateRequestInfo tls.CertificateRequestInfo
		recordHeaderError      tls.RecordHeaderError
		clientHelloInfo        tls.ClientHelloInfo
		config                 tls.Config
		conn                   tls.Conn
	)
	env.PackageTypes["crypto/tls"] = map[string]reflect.Type{
		"ClientSessionCache":     reflect.TypeOf(&clientSessionCache).Elem(),
		"SignatureScheme":        reflect.TypeOf(&signatureScheme).Elem(),
		"RenegotiationSupport":   reflect.TypeOf(&renegotiationSupport).Elem(),
		"Certificate":            reflect.TypeOf(&certificate).Elem(),
		"Dialer":                 reflect.TypeOf(&dialer).Elem(),
		"ClientAuthType":         reflect.TypeOf(&clientAuthType).Elem(),
		"ClientSessionState":     reflect.TypeOf(&clientSessionState).Elem(),
		"CipherSuite":            reflect.TypeOf(&cipherSuite).Elem(),
		"CurveID":                reflect.TypeOf(&curveID).Elem(),
		"ConnectionState":        reflect.TypeOf(&connectionState).Elem(),
		"CertificateRequestInfo": reflect.TypeOf(&certificateRequestInfo).Elem(),
		"RecordHeaderError":      reflect.TypeOf(&recordHeaderError).Elem(),
		"ClientHelloInfo":        reflect.TypeOf(&clientHelloInfo).Elem(),
		"Config":                 reflect.TypeOf(&config).Elem(),
		"Conn":                   reflect.TypeOf(&conn).Elem(),
	}
}

func initCryptoX509() {
	env.Packages["crypto/x509"] = map[string]reflect.Value{
		// define constants
		"TooManyIntermediates":                      reflect.ValueOf(x509.TooManyIntermediates),
		"RSA":                                       reflect.ValueOf(x509.RSA),
		"KeyUsageKeyEncipherment":                   reflect.ValueOf(x509.KeyUsageKeyEncipherment),
		"KeyUsageEncipherOnly":                      reflect.ValueOf(x509.KeyUsageEncipherOnly),
		"ExtKeyUsageMicrosoftServerGatedCrypto":     reflect.ValueOf(x509.ExtKeyUsageMicrosoftServerGatedCrypto),
		"ECDSAWithSHA512":                           reflect.ValueOf(x509.ECDSAWithSHA512),
		"DSA":                                       reflect.ValueOf(x509.DSA),
		"Ed25519":                                   reflect.ValueOf(x509.Ed25519),
		"ExtKeyUsageIPSECUser":                      reflect.ValueOf(x509.ExtKeyUsageIPSECUser),
		"ECDSA":                                     reflect.ValueOf(x509.ECDSA),
		"KeyUsageContentCommitment":                 reflect.ValueOf(x509.KeyUsageContentCommitment),
		"ExtKeyUsageIPSECEndSystem":                 reflect.ValueOf(x509.ExtKeyUsageIPSECEndSystem),
		"NameConstraintsWithoutSANs":                reflect.ValueOf(x509.NameConstraintsWithoutSANs),
		"ExtKeyUsageNetscapeServerGatedCrypto":      reflect.ValueOf(x509.ExtKeyUsageNetscapeServerGatedCrypto),
		"ExtKeyUsageMicrosoftKernelCodeSigning":     reflect.ValueOf(x509.ExtKeyUsageMicrosoftKernelCodeSigning),
		"UnconstrainedName":                         reflect.ValueOf(x509.UnconstrainedName),
		"ExtKeyUsageOCSPSigning":                    reflect.ValueOf(x509.ExtKeyUsageOCSPSigning),
		"SHA384WithRSA":                             reflect.ValueOf(x509.SHA384WithRSA),
		"ECDSAWithSHA256":                           reflect.ValueOf(x509.ECDSAWithSHA256),
		"SHA512WithRSAPSS":                          reflect.ValueOf(x509.SHA512WithRSAPSS),
		"ExtKeyUsageAny":                            reflect.ValueOf(x509.ExtKeyUsageAny),
		"KeyUsageKeyAgreement":                      reflect.ValueOf(x509.KeyUsageKeyAgreement),
		"TooManyConstraints":                        reflect.ValueOf(x509.TooManyConstraints),
		"DSAWithSHA256":                             reflect.ValueOf(x509.DSAWithSHA256),
		"MD2WithRSA":                                reflect.ValueOf(x509.MD2WithRSA),
		"DSAWithSHA1":                               reflect.ValueOf(x509.DSAWithSHA1),
		"ECDSAWithSHA1":                             reflect.ValueOf(x509.ECDSAWithSHA1),
		"PEMCipherDES":                              reflect.ValueOf(x509.PEMCipherDES),
		"KeyUsageCRLSign":                           reflect.ValueOf(x509.KeyUsageCRLSign),
		"ExtKeyUsageClientAuth":                     reflect.ValueOf(x509.ExtKeyUsageClientAuth),
		"NameMismatch":                              reflect.ValueOf(x509.NameMismatch),
		"PEMCipherAES192":                           reflect.ValueOf(x509.PEMCipherAES192),
		"SHA256WithRSAPSS":                          reflect.ValueOf(x509.SHA256WithRSAPSS),
		"SHA384WithRSAPSS":                          reflect.ValueOf(x509.SHA384WithRSAPSS),
		"PureEd25519":                               reflect.ValueOf(x509.PureEd25519),
		"Expired":                                   reflect.ValueOf(x509.Expired),
		"CANotAuthorizedForThisName":                reflect.ValueOf(x509.CANotAuthorizedForThisName),
		"IncompatibleUsage":                         reflect.ValueOf(x509.IncompatibleUsage),
		"SHA512WithRSA":                             reflect.ValueOf(x509.SHA512WithRSA),
		"UnknownPublicKeyAlgorithm":                 reflect.ValueOf(x509.UnknownPublicKeyAlgorithm),
		"KeyUsageDataEncipherment":                  reflect.ValueOf(x509.KeyUsageDataEncipherment),
		"ExtKeyUsageTimeStamping":                   reflect.ValueOf(x509.ExtKeyUsageTimeStamping),
		"SHA1WithRSA":                               reflect.ValueOf(x509.SHA1WithRSA),
		"KeyUsageCertSign":                          reflect.ValueOf(x509.KeyUsageCertSign),
		"ExtKeyUsageMicrosoftCommercialCodeSigning": reflect.ValueOf(x509.ExtKeyUsageMicrosoftCommercialCodeSigning),
		"PEMCipherAES256":                           reflect.ValueOf(x509.PEMCipherAES256),
		"MD5WithRSA":                                reflect.ValueOf(x509.MD5WithRSA),
		"KeyUsageDecipherOnly":                      reflect.ValueOf(x509.KeyUsageDecipherOnly),
		"ExtKeyUsageEmailProtection":                reflect.ValueOf(x509.ExtKeyUsageEmailProtection),
		"ExtKeyUsageIPSECTunnel":                    reflect.ValueOf(x509.ExtKeyUsageIPSECTunnel),
		"UnknownSignatureAlgorithm":                 reflect.ValueOf(x509.UnknownSignatureAlgorithm),
		"SHA256WithRSA":                             reflect.ValueOf(x509.SHA256WithRSA),
		"PEMCipher3DES":                             reflect.ValueOf(x509.PEMCipher3DES),
		"PEMCipherAES128":                           reflect.ValueOf(x509.PEMCipherAES128),
		"ECDSAWithSHA384":                           reflect.ValueOf(x509.ECDSAWithSHA384),
		"ExtKeyUsageCodeSigning":                    reflect.ValueOf(x509.ExtKeyUsageCodeSigning),
		"NotAuthorizedToSign":                       reflect.ValueOf(x509.NotAuthorizedToSign),
		"CANotAuthorizedForExtKeyUsage":             reflect.ValueOf(x509.CANotAuthorizedForExtKeyUsage),
		"KeyUsageDigitalSignature":                  reflect.ValueOf(x509.KeyUsageDigitalSignature),
		"ExtKeyUsageServerAuth":                     reflect.ValueOf(x509.ExtKeyUsageServerAuth),

		// define variables
		"ErrUnsupportedAlgorithm": reflect.ValueOf(x509.ErrUnsupportedAlgorithm),
		"IncorrectPasswordError":  reflect.ValueOf(x509.IncorrectPasswordError),

		// define functions
		"MarshalPKCS1PrivateKey":   reflect.ValueOf(x509.MarshalPKCS1PrivateKey),
		"ParseDERCRL":              reflect.ValueOf(x509.ParseDERCRL),
		"ParseECPrivateKey":        reflect.ValueOf(x509.ParseECPrivateKey),
		"IsEncryptedPEMBlock":      reflect.ValueOf(x509.IsEncryptedPEMBlock),
		"DecryptPEMBlock":          reflect.ValueOf(x509.DecryptPEMBlock),
		"ParseCertificates":        reflect.ValueOf(x509.ParseCertificates),
		"CreateCertificate":        reflect.ValueOf(x509.CreateCertificate),
		"ParseCRL":                 reflect.ValueOf(x509.ParseCRL),
		"EncryptPEMBlock":          reflect.ValueOf(x509.EncryptPEMBlock),
		"NewCertPool":              reflect.ValueOf(x509.NewCertPool),
		"MarshalPKIXPublicKey":     reflect.ValueOf(x509.MarshalPKIXPublicKey),
		"ParseCertificate":         reflect.ValueOf(x509.ParseCertificate),
		"MarshalECPrivateKey":      reflect.ValueOf(x509.MarshalECPrivateKey),
		"SystemCertPool":           reflect.ValueOf(x509.SystemCertPool),
		"ParsePKCS1PrivateKey":     reflect.ValueOf(x509.ParsePKCS1PrivateKey),
		"ParsePKCS1PublicKey":      reflect.ValueOf(x509.ParsePKCS1PublicKey),
		"MarshalPKCS1PublicKey":    reflect.ValueOf(x509.MarshalPKCS1PublicKey),
		"ParsePKCS8PrivateKey":     reflect.ValueOf(x509.ParsePKCS8PrivateKey),
		"MarshalPKCS8PrivateKey":   reflect.ValueOf(x509.MarshalPKCS8PrivateKey),
		"ParsePKIXPublicKey":       reflect.ValueOf(x509.ParsePKIXPublicKey),
		"CreateCertificateRequest": reflect.ValueOf(x509.CreateCertificateRequest),
		"ParseCertificateRequest":  reflect.ValueOf(x509.ParseCertificateRequest),
		"CreateRevocationList":     reflect.ValueOf(x509.CreateRevocationList),
	}
	var (
		certPool                   x509.CertPool
		signatureAlgorithm         x509.SignatureAlgorithm
		extKeyUsage                x509.ExtKeyUsage
		certificate                x509.Certificate
		constraintViolationError   x509.ConstraintViolationError
		unhandledCriticalExtension x509.UnhandledCriticalExtension
		certificateInvalidError    x509.CertificateInvalidError
		verifyOptions              x509.VerifyOptions
		publicKeyAlgorithm         x509.PublicKeyAlgorithm
		hostnameError              x509.HostnameError
		pEMCipher                  x509.PEMCipher
		insecureAlgorithmError     x509.InsecureAlgorithmError
		certificateRequest         x509.CertificateRequest
		unknownAuthorityError      x509.UnknownAuthorityError
		keyUsage                   x509.KeyUsage
		revocationList             x509.RevocationList
		invalidReason              x509.InvalidReason
		systemRootsError           x509.SystemRootsError
	)
	env.PackageTypes["crypto/x509"] = map[string]reflect.Type{
		"CertPool":                   reflect.TypeOf(&certPool).Elem(),
		"SignatureAlgorithm":         reflect.TypeOf(&signatureAlgorithm).Elem(),
		"ExtKeyUsage":                reflect.TypeOf(&extKeyUsage).Elem(),
		"Certificate":                reflect.TypeOf(&certificate).Elem(),
		"ConstraintViolationError":   reflect.TypeOf(&constraintViolationError).Elem(),
		"UnhandledCriticalExtension": reflect.TypeOf(&unhandledCriticalExtension).Elem(),
		"CertificateInvalidError":    reflect.TypeOf(&certificateInvalidError).Elem(),
		"VerifyOptions":              reflect.TypeOf(&verifyOptions).Elem(),
		"PublicKeyAlgorithm":         reflect.TypeOf(&publicKeyAlgorithm).Elem(),
		"HostnameError":              reflect.TypeOf(&hostnameError).Elem(),
		"PEMCipher":                  reflect.TypeOf(&pEMCipher).Elem(),
		"InsecureAlgorithmError":     reflect.TypeOf(&insecureAlgorithmError).Elem(),
		"CertificateRequest":         reflect.TypeOf(&certificateRequest).Elem(),
		"UnknownAuthorityError":      reflect.TypeOf(&unknownAuthorityError).Elem(),
		"KeyUsage":                   reflect.TypeOf(&keyUsage).Elem(),
		"RevocationList":             reflect.TypeOf(&revocationList).Elem(),
		"InvalidReason":              reflect.TypeOf(&invalidReason).Elem(),
		"SystemRootsError":           reflect.TypeOf(&systemRootsError).Elem(),
	}
}

func initCryptoX509PKIX() {
	env.Packages["crypto/x509/pkix"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
	}
	var (
		attributeTypeAndValue        pkix.AttributeTypeAndValue
		extension                    pkix.Extension
		tBSCertificateList           pkix.TBSCertificateList
		revokedCertificate           pkix.RevokedCertificate
		algorithmIdentifier          pkix.AlgorithmIdentifier
		relativeDistinguishedNameSET pkix.RelativeDistinguishedNameSET
		name                         pkix.Name
		certificateList              pkix.CertificateList
		rDNSequence                  pkix.RDNSequence
		attributeTypeAndValueSET     pkix.AttributeTypeAndValueSET
	)
	env.PackageTypes["crypto/x509/pkix"] = map[string]reflect.Type{
		"AttributeTypeAndValue":        reflect.TypeOf(&attributeTypeAndValue).Elem(),
		"Extension":                    reflect.TypeOf(&extension).Elem(),
		"TBSCertificateList":           reflect.TypeOf(&tBSCertificateList).Elem(),
		"RevokedCertificate":           reflect.TypeOf(&revokedCertificate).Elem(),
		"AlgorithmIdentifier":          reflect.TypeOf(&algorithmIdentifier).Elem(),
		"RelativeDistinguishedNameSET": reflect.TypeOf(&relativeDistinguishedNameSET).Elem(),
		"Name":                         reflect.TypeOf(&name).Elem(),
		"CertificateList":              reflect.TypeOf(&certificateList).Elem(),
		"RDNSequence":                  reflect.TypeOf(&rDNSequence).Elem(),
		"AttributeTypeAndValueSET":     reflect.TypeOf(&attributeTypeAndValueSET).Elem(),
	}
}

func initEncoding() {
	env.Packages["encoding"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
	}
	var (
		binaryMarshaler   encoding.BinaryMarshaler
		binaryUnmarshaler encoding.BinaryUnmarshaler
		textMarshaler     encoding.TextMarshaler
		textUnmarshaler   encoding.TextUnmarshaler
	)
	env.PackageTypes["encoding"] = map[string]reflect.Type{
		"BinaryMarshaler":   reflect.TypeOf(&binaryMarshaler).Elem(),
		"BinaryUnmarshaler": reflect.TypeOf(&binaryUnmarshaler).Elem(),
		"TextMarshaler":     reflect.TypeOf(&textMarshaler).Elem(),
		"TextUnmarshaler":   reflect.TypeOf(&textUnmarshaler).Elem(),
	}
}

func initEncodingBase64() {
	env.Packages["encoding/base64"] = map[string]reflect.Value{
		// define constants
		"StdPadding": reflect.ValueOf(base64.StdPadding),
		"NoPadding":  reflect.ValueOf(base64.NoPadding),

		// define variables
		"StdEncoding":    reflect.ValueOf(base64.StdEncoding),
		"URLEncoding":    reflect.ValueOf(base64.URLEncoding),
		"RawStdEncoding": reflect.ValueOf(base64.RawStdEncoding),
		"RawURLEncoding": reflect.ValueOf(base64.RawURLEncoding),

		// define functions
		"NewDecoder":  reflect.ValueOf(base64.NewDecoder),
		"NewEncoding": reflect.ValueOf(base64.NewEncoding),
		"NewEncoder":  reflect.ValueOf(base64.NewEncoder),
	}
	var (
		enc               base64.Encoding
		corruptInputError base64.CorruptInputError
	)
	env.PackageTypes["encoding/base64"] = map[string]reflect.Type{
		"Encoding":          reflect.TypeOf(&enc).Elem(),
		"CorruptInputError": reflect.TypeOf(&corruptInputError).Elem(),
	}
}

func initEncodingCSV() {
	env.Packages["encoding/csv"] = map[string]reflect.Value{
		// define constants

		// define variables
		"ErrBareQuote":  reflect.ValueOf(csv.ErrBareQuote),
		"ErrQuote":      reflect.ValueOf(csv.ErrQuote),
		"ErrFieldCount": reflect.ValueOf(csv.ErrFieldCount),

		// define functions
		"NewReader": reflect.ValueOf(csv.NewReader),
		"NewWriter": reflect.ValueOf(csv.NewWriter),
	}
	var (
		parseError csv.ParseError
		reader     csv.Reader
		writer     csv.Writer
	)
	env.PackageTypes["encoding/csv"] = map[string]reflect.Type{
		"ParseError": reflect.TypeOf(&parseError).Elem(),
		"Reader":     reflect.TypeOf(&reader).Elem(),
		"Writer":     reflect.TypeOf(&writer).Elem(),
	}
}

func initEncodingHex() {
	env.Packages["encoding/hex"] = map[string]reflect.Value{
		// define constants

		// define variables
		"ErrLength": reflect.ValueOf(hex.ErrLength),

		// define functions
		"Dumper":         reflect.ValueOf(hex.Dumper),
		"EncodedLen":     reflect.ValueOf(hex.EncodedLen),
		"Encode":         reflect.ValueOf(hex.Encode),
		"EncodeToString": reflect.ValueOf(hex.EncodeToString),
		"DecodeString":   reflect.ValueOf(hex.DecodeString),
		"Dump":           reflect.ValueOf(hex.Dump),
		"NewEncoder":     reflect.ValueOf(hex.NewEncoder),
		"NewDecoder":     reflect.ValueOf(hex.NewDecoder),
		"DecodedLen":     reflect.ValueOf(hex.DecodedLen),
		"Decode":         reflect.ValueOf(hex.Decode),
	}
	var (
		invalidByteError hex.InvalidByteError
	)
	env.PackageTypes["encoding/hex"] = map[string]reflect.Type{
		"InvalidByteError": reflect.TypeOf(&invalidByteError).Elem(),
	}
}

func initEncodingJSON() {
	env.Packages["encoding/json"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
		"NewDecoder":    reflect.ValueOf(json.NewDecoder),
		"NewEncoder":    reflect.ValueOf(json.NewEncoder),
		"Marshal":       reflect.ValueOf(json.Marshal),
		"MarshalIndent": reflect.ValueOf(json.MarshalIndent),
		"HTMLEscape":    reflect.ValueOf(json.HTMLEscape),
		"Valid":         reflect.ValueOf(json.Valid),
		"Unmarshal":     reflect.ValueOf(json.Unmarshal),
		"Compact":       reflect.ValueOf(json.Compact),
		"Indent":        reflect.ValueOf(json.Indent),
	}
	var (
		syntaxError           json.SyntaxError
		encoder               json.Encoder
		token                 json.Token
		unmarshalTypeError    json.UnmarshalTypeError
		decoder               json.Decoder
		rawMessage            json.RawMessage
		delim                 json.Delim
		number                json.Number
		marshalerError        json.MarshalerError
		unsupportedTypeError  json.UnsupportedTypeError
		unsupportedValueError json.UnsupportedValueError
		unmarshaler           json.Unmarshaler
		invalidUnmarshalError json.InvalidUnmarshalError
		marshaler             json.Marshaler
	)
	env.PackageTypes["encoding/json"] = map[string]reflect.Type{
		"SyntaxError":           reflect.TypeOf(&syntaxError).Elem(),
		"Encoder":               reflect.TypeOf(&encoder).Elem(),
		"Token":                 reflect.TypeOf(&token).Elem(),
		"UnmarshalTypeError":    reflect.TypeOf(&unmarshalTypeError).Elem(),
		"Decoder":               reflect.TypeOf(&decoder).Elem(),
		"RawMessage":            reflect.TypeOf(&rawMessage).Elem(),
		"Delim":                 reflect.TypeOf(&delim).Elem(),
		"Number":                reflect.TypeOf(&number).Elem(),
		"MarshalerError":        reflect.TypeOf(&marshalerError).Elem(),
		"UnsupportedTypeError":  reflect.TypeOf(&unsupportedTypeError).Elem(),
		"UnsupportedValueError": reflect.TypeOf(&unsupportedValueError).Elem(),
		"Unmarshaler":           reflect.TypeOf(&unmarshaler).Elem(),
		"InvalidUnmarshalError": reflect.TypeOf(&invalidUnmarshalError).Elem(),
		"Marshaler":             reflect.TypeOf(&marshaler).Elem(),
	}
}

func initEncodingPEM() {
	env.Packages["encoding/pem"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
		"EncodeToMemory": reflect.ValueOf(pem.EncodeToMemory),
		"Decode":         reflect.ValueOf(pem.Decode),
		"Encode":         reflect.ValueOf(pem.Encode),
	}
	var (
		block pem.Block
	)
	env.PackageTypes["encoding/pem"] = map[string]reflect.Type{
		"Block": reflect.TypeOf(&block).Elem(),
	}
}

func initEncodingXML() {
	env.Packages["encoding/xml"] = map[string]reflect.Value{
		// define constants
		"Header": reflect.ValueOf(xml.Header),

		// define variables
		"HTMLEntity":    reflect.ValueOf(xml.HTMLEntity),
		"HTMLAutoClose": reflect.ValueOf(xml.HTMLAutoClose),

		// define functions
		"MarshalIndent":   reflect.ValueOf(xml.MarshalIndent),
		"NewEncoder":      reflect.ValueOf(xml.NewEncoder),
		"Unmarshal":       reflect.ValueOf(xml.Unmarshal),
		"EscapeText":      reflect.ValueOf(xml.EscapeText),
		"Escape":          reflect.ValueOf(xml.Escape),
		"Marshal":         reflect.ValueOf(xml.Marshal),
		"CopyToken":       reflect.ValueOf(xml.CopyToken),
		"NewDecoder":      reflect.ValueOf(xml.NewDecoder),
		"NewTokenDecoder": reflect.ValueOf(xml.NewTokenDecoder),
	}
	var (
		encoder              xml.Encoder
		tagPathError         xml.TagPathError
		token                xml.Token
		decoder              xml.Decoder
		marshalerAttr        xml.MarshalerAttr
		unmarshalerAttr      xml.UnmarshalerAttr
		comment              xml.Comment
		procInst             xml.ProcInst
		unmarshaler          xml.Unmarshaler
		unmarshalError       xml.UnmarshalError
		endElement           xml.EndElement
		marshaler            xml.Marshaler
		syntaxError          xml.SyntaxError
		name                 xml.Name
		attr                 xml.Attr
		startElement         xml.StartElement
		charData             xml.CharData
		directive            xml.Directive
		tokenReader          xml.TokenReader
		unsupportedTypeError xml.UnsupportedTypeError
	)
	env.PackageTypes["encoding/xml"] = map[string]reflect.Type{
		"Encoder":              reflect.TypeOf(&encoder).Elem(),
		"TagPathError":         reflect.TypeOf(&tagPathError).Elem(),
		"Token":                reflect.TypeOf(&token).Elem(),
		"Decoder":              reflect.TypeOf(&decoder).Elem(),
		"MarshalerAttr":        reflect.TypeOf(&marshalerAttr).Elem(),
		"UnmarshalerAttr":      reflect.TypeOf(&unmarshalerAttr).Elem(),
		"Comment":              reflect.TypeOf(&comment).Elem(),
		"ProcInst":             reflect.TypeOf(&procInst).Elem(),
		"Unmarshaler":          reflect.TypeOf(&unmarshaler).Elem(),
		"UnmarshalError":       reflect.TypeOf(&unmarshalError).Elem(),
		"EndElement":           reflect.TypeOf(&endElement).Elem(),
		"Marshaler":            reflect.TypeOf(&marshaler).Elem(),
		"SyntaxError":          reflect.TypeOf(&syntaxError).Elem(),
		"Name":                 reflect.TypeOf(&name).Elem(),
		"Attr":                 reflect.TypeOf(&attr).Elem(),
		"StartElement":         reflect.TypeOf(&startElement).Elem(),
		"CharData":             reflect.TypeOf(&charData).Elem(),
		"Directive":            reflect.TypeOf(&directive).Elem(),
		"TokenReader":          reflect.TypeOf(&tokenReader).Elem(),
		"UnsupportedTypeError": reflect.TypeOf(&unsupportedTypeError).Elem(),
	}
}

func initFMT() {
	env.Packages["fmt"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
		"Scanln":   reflect.ValueOf(fmt.Scanln),
		"Fscan":    reflect.ValueOf(fmt.Fscan),
		"Printf":   reflect.ValueOf(fmt.Printf),
		"Sprintf":  reflect.ValueOf(fmt.Sprintf),
		"Fprintln": reflect.ValueOf(fmt.Fprintln),
		"Println":  reflect.ValueOf(fmt.Println),
		"Fscanf":   reflect.ValueOf(fmt.Fscanf),
		"Errorf":   reflect.ValueOf(fmt.Errorf),
		"Fprint":   reflect.ValueOf(fmt.Fprint),
		"Sprintln": reflect.ValueOf(fmt.Sprintln),
		"Fscanln":  reflect.ValueOf(fmt.Fscanln),
		"Fprintf":  reflect.ValueOf(fmt.Fprintf),
		"Sprint":   reflect.ValueOf(fmt.Sprint),
		"Sscanln":  reflect.ValueOf(fmt.Sscanln),
		"Sscanf":   reflect.ValueOf(fmt.Sscanf),
		"Print":    reflect.ValueOf(fmt.Print),
		"Scan":     reflect.ValueOf(fmt.Scan),
		"Scanf":    reflect.ValueOf(fmt.Scanf),
		"Sscan":    reflect.ValueOf(fmt.Sscan),
	}
	var (
		state      fmt.State
		formatter  fmt.Formatter
		stringer   fmt.Stringer
		goStringer fmt.GoStringer
		scanState  fmt.ScanState
		scanner    fmt.Scanner
	)
	env.PackageTypes["fmt"] = map[string]reflect.Type{
		"State":      reflect.TypeOf(&state).Elem(),
		"Formatter":  reflect.TypeOf(&formatter).Elem(),
		"Stringer":   reflect.TypeOf(&stringer).Elem(),
		"GoStringer": reflect.TypeOf(&goStringer).Elem(),
		"ScanState":  reflect.TypeOf(&scanState).Elem(),
		"Scanner":    reflect.TypeOf(&scanner).Elem(),
	}
}

func initHash() {
	env.Packages["hash"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
	}
	var (
		h      hash.Hash
		hash32 hash.Hash32
		hash64 hash.Hash64
	)
	env.PackageTypes["hash"] = map[string]reflect.Type{
		"Hash":   reflect.TypeOf(&h).Elem(),
		"Hash32": reflect.TypeOf(&hash32).Elem(),
		"Hash64": reflect.TypeOf(&hash64).Elem(),
	}
}

func initHashCRC32() {
	env.Packages["hash/crc32"] = map[string]reflect.Value{
		// define constants
		"Size":       reflect.ValueOf(crc32.Size),
		"IEEE":       reflect.ValueOf(crc32.IEEE),
		"Castagnoli": reflect.ValueOf(crc32.Castagnoli),
		"Koopman":    reflect.ValueOf(crc32.Koopman),

		// define variables
		"IEEETable": reflect.ValueOf(crc32.IEEETable),

		// define functions
		"Checksum":     reflect.ValueOf(crc32.Checksum),
		"ChecksumIEEE": reflect.ValueOf(crc32.ChecksumIEEE),
		"MakeTable":    reflect.ValueOf(crc32.MakeTable),
		"New":          reflect.ValueOf(crc32.New),
		"NewIEEE":      reflect.ValueOf(crc32.NewIEEE),
		"Update":       reflect.ValueOf(crc32.Update),
	}
	var (
		table crc32.Table
	)
	env.PackageTypes["hash/crc32"] = map[string]reflect.Type{
		"Table": reflect.TypeOf(&table).Elem(),
	}
}

func initHashCRC64() {
	env.Packages["hash/crc64"] = map[string]reflect.Value{
		// define constants
		"Size": reflect.ValueOf(crc64.Size),
		"ISO":  reflect.ValueOf(uint64(crc64.ISO)),
		"ECMA": reflect.ValueOf(uint64(crc64.ECMA)),

		// define variables

		// define functions
		"Checksum":  reflect.ValueOf(crc64.Checksum),
		"MakeTable": reflect.ValueOf(crc64.MakeTable),
		"New":       reflect.ValueOf(crc64.New),
		"Update":    reflect.ValueOf(crc64.Update),
	}
	var (
		table crc64.Table
	)
	env.PackageTypes["hash/crc64"] = map[string]reflect.Type{
		"Table": reflect.TypeOf(&table).Elem(),
	}
}

func initIO() {
	env.Packages["io"] = map[string]reflect.Value{
		// define constants
		"SeekStart":   reflect.ValueOf(io.SeekStart),
		"SeekCurrent": reflect.ValueOf(io.SeekCurrent),
		"SeekEnd":     reflect.ValueOf(io.SeekEnd),

		// define variables
		"ErrShortWrite":    reflect.ValueOf(io.ErrShortWrite),
		"ErrShortBuffer":   reflect.ValueOf(io.ErrShortBuffer),
		"EOF":              reflect.ValueOf(io.EOF),
		"ErrUnexpectedEOF": reflect.ValueOf(io.ErrUnexpectedEOF),
		"ErrNoProgress":    reflect.ValueOf(io.ErrNoProgress),
		"ErrClosedPipe":    reflect.ValueOf(io.ErrClosedPipe),

		// define functions
		"Pipe":             reflect.ValueOf(io.Pipe),
		"ReadFull":         reflect.ValueOf(io.ReadFull),
		"Copy":             reflect.ValueOf(io.Copy),
		"LimitReader":      reflect.ValueOf(io.LimitReader),
		"NewSectionReader": reflect.ValueOf(io.NewSectionReader),
		"MultiReader":      reflect.ValueOf(io.MultiReader),
		"MultiWriter":      reflect.ValueOf(io.MultiWriter),
		"WriteString":      reflect.ValueOf(io.WriteString),
		"ReadAtLeast":      reflect.ValueOf(io.ReadAtLeast),
		"CopyN":            reflect.ValueOf(io.CopyN),
		"CopyBuffer":       reflect.ValueOf(io.CopyBuffer),
		"TeeReader":        reflect.ValueOf(io.TeeReader),
	}
	var (
		sectionReader   io.SectionReader
		readWriter      io.ReadWriter
		readCloser      io.ReadCloser
		writerAt        io.WriterAt
		byteReader      io.ByteReader
		stringWriter    io.StringWriter
		limitedReader   io.LimitedReader
		runeReader      io.RuneReader
		pipeReader      io.PipeReader
		writer          io.Writer
		seeker          io.Seeker
		writeSeeker     io.WriteSeeker
		readWriteSeeker io.ReadWriteSeeker
		readerAt        io.ReaderAt
		byteWriter      io.ByteWriter
		pipeWriter      io.PipeWriter
		reader          io.Reader
		writeCloser     io.WriteCloser
		readWriteCloser io.ReadWriteCloser
		byteScanner     io.ByteScanner
		runeScanner     io.RuneScanner
		closer          io.Closer
		readSeeker      io.ReadSeeker
		readerFrom      io.ReaderFrom
		writerTo        io.WriterTo
	)
	env.PackageTypes["io"] = map[string]reflect.Type{
		"SectionReader":   reflect.TypeOf(&sectionReader).Elem(),
		"ReadWriter":      reflect.TypeOf(&readWriter).Elem(),
		"ReadCloser":      reflect.TypeOf(&readCloser).Elem(),
		"WriterAt":        reflect.TypeOf(&writerAt).Elem(),
		"ByteReader":      reflect.TypeOf(&byteReader).Elem(),
		"StringWriter":    reflect.TypeOf(&stringWriter).Elem(),
		"LimitedReader":   reflect.TypeOf(&limitedReader).Elem(),
		"RuneReader":      reflect.TypeOf(&runeReader).Elem(),
		"PipeReader":      reflect.TypeOf(&pipeReader).Elem(),
		"Writer":          reflect.TypeOf(&writer).Elem(),
		"Seeker":          reflect.TypeOf(&seeker).Elem(),
		"WriteSeeker":     reflect.TypeOf(&writeSeeker).Elem(),
		"ReadWriteSeeker": reflect.TypeOf(&readWriteSeeker).Elem(),
		"ReaderAt":        reflect.TypeOf(&readerAt).Elem(),
		"ByteWriter":      reflect.TypeOf(&byteWriter).Elem(),
		"PipeWriter":      reflect.TypeOf(&pipeWriter).Elem(),
		"Reader":          reflect.TypeOf(&reader).Elem(),
		"WriteCloser":     reflect.TypeOf(&writeCloser).Elem(),
		"ReadWriteCloser": reflect.TypeOf(&readWriteCloser).Elem(),
		"ByteScanner":     reflect.TypeOf(&byteScanner).Elem(),
		"RuneScanner":     reflect.TypeOf(&runeScanner).Elem(),
		"Closer":          reflect.TypeOf(&closer).Elem(),
		"ReadSeeker":      reflect.TypeOf(&readSeeker).Elem(),
		"ReaderFrom":      reflect.TypeOf(&readerFrom).Elem(),
		"WriterTo":        reflect.TypeOf(&writerTo).Elem(),
	}
}

func initIOioutil() {
	env.Packages["io/ioutil"] = map[string]reflect.Value{
		// define constants

		// define variables
		"Discard": reflect.ValueOf(ioutil.Discard),

		// define functions
		"ReadDir":   reflect.ValueOf(ioutil.ReadDir),
		"NopCloser": reflect.ValueOf(ioutil.NopCloser),
		"TempFile":  reflect.ValueOf(ioutil.TempFile),
		"TempDir":   reflect.ValueOf(ioutil.TempDir),
		"ReadAll":   reflect.ValueOf(ioutil.ReadAll),
		"ReadFile":  reflect.ValueOf(ioutil.ReadFile),
		"WriteFile": reflect.ValueOf(ioutil.WriteFile),
	}
	var ()
	env.PackageTypes["io/ioutil"] = map[string]reflect.Type{}
}

func initMath() {
	env.Packages["math"] = map[string]reflect.Value{
		// define constants
		"E":                      reflect.ValueOf(math.E),
		"Log2E":                  reflect.ValueOf(math.Log2E),
		"Ln10":                   reflect.ValueOf(math.Ln10),
		"MaxFloat64":             reflect.ValueOf(math.MaxFloat64),
		"MinInt8":                reflect.ValueOf(math.MinInt8),
		"MaxInt32":               reflect.ValueOf(math.MaxInt32),
		"MinInt32":               reflect.ValueOf(math.MinInt32),
		"SqrtE":                  reflect.ValueOf(math.SqrtE),
		"MaxInt64":               reflect.ValueOf(math.MaxInt64),
		"MaxUint32":              reflect.ValueOf(math.MaxUint32),
		"Pi":                     reflect.ValueOf(math.Pi),
		"SqrtPi":                 reflect.ValueOf(math.SqrtPi),
		"MaxInt16":               reflect.ValueOf(math.MaxInt16),
		"MaxInt8":                reflect.ValueOf(math.MaxInt8),
		"Sqrt2":                  reflect.ValueOf(math.Sqrt2),
		"Log10E":                 reflect.ValueOf(math.Log10E),
		"SmallestNonzeroFloat64": reflect.ValueOf(math.SmallestNonzeroFloat64),
		"MaxUint16":              reflect.ValueOf(math.MaxUint16),
		"Phi":                    reflect.ValueOf(math.Phi),
		"MaxFloat32":             reflect.ValueOf(math.MaxFloat32),
		"SmallestNonzeroFloat32": reflect.ValueOf(math.SmallestNonzeroFloat32),
		"MinInt16":               reflect.ValueOf(math.MinInt16),
		"MaxUint8":               reflect.ValueOf(math.MaxUint8),
		"Ln2":                    reflect.ValueOf(math.Ln2),
		"MaxUint64":              reflect.ValueOf(uint64(math.MaxUint64)),
		"SqrtPhi":                reflect.ValueOf(math.SqrtPhi),
		"MinInt64":               reflect.ValueOf(math.MinInt64),

		// define variables

		// define functions
		"Cbrt":            reflect.ValueOf(math.Cbrt),
		"Cosh":            reflect.ValueOf(math.Cosh),
		"Float32frombits": reflect.ValueOf(math.Float32frombits),
		"Yn":              reflect.ValueOf(math.Yn),
		"Acos":            reflect.ValueOf(math.Acos),
		"Cos":             reflect.ValueOf(math.Cos),
		"Erf":             reflect.ValueOf(math.Erf),
		"Atan2":           reflect.ValueOf(math.Atan2),
		"Remainder":       reflect.ValueOf(math.Remainder),
		"Max":             reflect.ValueOf(math.Max),
		"NaN":             reflect.ValueOf(math.NaN),
		"Gamma":           reflect.ValueOf(math.Gamma),
		"Signbit":         reflect.ValueOf(math.Signbit),
		"Ldexp":           reflect.ValueOf(math.Ldexp),
		"Copysign":        reflect.ValueOf(math.Copysign),
		"Sincos":          reflect.ValueOf(math.Sincos),
		"Asin":            reflect.ValueOf(math.Asin),
		"FMA":             reflect.ValueOf(math.FMA),
		"Log1p":           reflect.ValueOf(math.Log1p),
		"Acosh":           reflect.ValueOf(math.Acosh),
		"IsInf":           reflect.ValueOf(math.IsInf),
		"Y1":              reflect.ValueOf(math.Y1),
		"Y0":              reflect.ValueOf(math.Y0),
		"Pow":             reflect.ValueOf(math.Pow),
		"Float64frombits": reflect.ValueOf(math.Float64frombits),
		"Tanh":            reflect.ValueOf(math.Tanh),
		"Round":           reflect.ValueOf(math.Round),
		"Lgamma":          reflect.ValueOf(math.Lgamma),
		"Logb":            reflect.ValueOf(math.Logb),
		"Exp2":            reflect.ValueOf(math.Exp2),
		"Sin":             reflect.ValueOf(math.Sin),
		"Tan":             reflect.ValueOf(math.Tan),
		"Erfc":            reflect.ValueOf(math.Erfc),
		"J0":              reflect.ValueOf(math.J0),
		"Hypot":           reflect.ValueOf(math.Hypot),
		"Jn":              reflect.ValueOf(math.Jn),
		"Trunc":           reflect.ValueOf(math.Trunc),
		"Log10":           reflect.ValueOf(math.Log10),
		"Frexp":           reflect.ValueOf(math.Frexp),
		"Exp":             reflect.ValueOf(math.Exp),
		"Sinh":            reflect.ValueOf(math.Sinh),
		"Float32bits":     reflect.ValueOf(math.Float32bits),
		"Log":             reflect.ValueOf(math.Log),
		"Nextafter":       reflect.ValueOf(math.Nextafter),
		"Nextafter32":     reflect.ValueOf(math.Nextafter32),
		"Min":             reflect.ValueOf(math.Min),
		"Erfcinv":         reflect.ValueOf(math.Erfcinv),
		"Floor":           reflect.ValueOf(math.Floor),
		"Atan":            reflect.ValueOf(math.Atan),
		"J1":              reflect.ValueOf(math.J1),
		"Sqrt":            reflect.ValueOf(math.Sqrt),
		"Mod":             reflect.ValueOf(math.Mod),
		"Ilogb":           reflect.ValueOf(math.Ilogb),
		"Expm1":           reflect.ValueOf(math.Expm1),
		"Atanh":           reflect.ValueOf(math.Atanh),
		"Inf":             reflect.ValueOf(math.Inf),
		"IsNaN":           reflect.ValueOf(math.IsNaN),
		"Erfinv":          reflect.ValueOf(math.Erfinv),
		"Log2":            reflect.ValueOf(math.Log2),
		"RoundToEven":     reflect.ValueOf(math.RoundToEven),
		"Dim":             reflect.ValueOf(math.Dim),
		"Pow10":           reflect.ValueOf(math.Pow10),
		"Float64bits":     reflect.ValueOf(math.Float64bits),
		"Modf":            reflect.ValueOf(math.Modf),
		"Ceil":            reflect.ValueOf(math.Ceil),
		"Abs":             reflect.ValueOf(math.Abs),
		"Asinh":           reflect.ValueOf(math.Asinh),
	}
	var ()
	env.PackageTypes["math"] = map[string]reflect.Type{}
}

func initMathBig() {
	env.Packages["math/big"] = map[string]reflect.Value{
		// define constants
		"ToPositiveInf": reflect.ValueOf(big.ToPositiveInf),
		"Below":         reflect.ValueOf(big.Below),
		"MaxBase":       reflect.ValueOf(big.MaxBase),
		"MaxPrec":       reflect.ValueOf(big.MaxPrec),
		"ToNearestEven": reflect.ValueOf(big.ToNearestEven),
		"ToNearestAway": reflect.ValueOf(big.ToNearestAway),
		"AwayFromZero":  reflect.ValueOf(big.AwayFromZero),
		"Exact":         reflect.ValueOf(big.Exact),
		"Above":         reflect.ValueOf(big.Above),
		"MaxExp":        reflect.ValueOf(big.MaxExp),
		"MinExp":        reflect.ValueOf(big.MinExp),
		"ToZero":        reflect.ValueOf(big.ToZero),
		"ToNegativeInf": reflect.ValueOf(big.ToNegativeInf),

		// define variables

		// define functions
		"NewInt":     reflect.ValueOf(big.NewInt),
		"Jacobi":     reflect.ValueOf(big.Jacobi),
		"ParseFloat": reflect.ValueOf(big.ParseFloat),
		"NewFloat":   reflect.ValueOf(big.NewFloat),
		"NewRat":     reflect.ValueOf(big.NewRat),
	}
	var (
		errNaN       big.ErrNaN
		roundingMode big.RoundingMode
		accuracy     big.Accuracy
		rat          big.Rat
		i            big.Int
		word         big.Word
		float        big.Float
	)
	env.PackageTypes["math/big"] = map[string]reflect.Type{
		"ErrNaN":       reflect.TypeOf(&errNaN).Elem(),
		"RoundingMode": reflect.TypeOf(&roundingMode).Elem(),
		"Accuracy":     reflect.TypeOf(&accuracy).Elem(),
		"Rat":          reflect.TypeOf(&rat).Elem(),
		"Int":          reflect.TypeOf(&i).Elem(),
		"Word":         reflect.TypeOf(&word).Elem(),
		"Float":        reflect.TypeOf(&float).Elem(),
	}
}

func initMathRand() {
	env.Packages["math/rand"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
		"New":         reflect.ValueOf(rand.New),
		"Float64":     reflect.ValueOf(rand.Float64),
		"Shuffle":     reflect.ValueOf(rand.Shuffle),
		"ExpFloat64":  reflect.ValueOf(rand.ExpFloat64),
		"NewSource":   reflect.ValueOf(rand.NewSource),
		"Seed":        reflect.ValueOf(rand.Seed),
		"Int31":       reflect.ValueOf(rand.Int31),
		"Perm":        reflect.ValueOf(rand.Perm),
		"Int63":       reflect.ValueOf(rand.Int63),
		"Uint32":      reflect.ValueOf(rand.Uint32),
		"Uint64":      reflect.ValueOf(rand.Uint64),
		"Intn":        reflect.ValueOf(rand.Intn),
		"Read":        reflect.ValueOf(rand.Read),
		"NormFloat64": reflect.ValueOf(rand.NormFloat64),
		"NewZipf":     reflect.ValueOf(rand.NewZipf),
		"Int":         reflect.ValueOf(rand.Int),
		"Int63n":      reflect.ValueOf(rand.Int63n),
		"Int31n":      reflect.ValueOf(rand.Int31n),
		"Float32":     reflect.ValueOf(rand.Float32),
	}
	var (
		source   rand.Source
		source64 rand.Source64
		r        rand.Rand
		zipf     rand.Zipf
	)
	env.PackageTypes["math/rand"] = map[string]reflect.Type{
		"Source":   reflect.TypeOf(&source).Elem(),
		"Source64": reflect.TypeOf(&source64).Elem(),
		"Rand":     reflect.TypeOf(&r).Elem(),
		"Zipf":     reflect.TypeOf(&zipf).Elem(),
	}
}

func initNet() {
	env.Packages["net"] = map[string]reflect.Value{
		// define constants
		"FlagUp":           reflect.ValueOf(net.FlagUp),
		"FlagBroadcast":    reflect.ValueOf(net.FlagBroadcast),
		"FlagLoopback":     reflect.ValueOf(net.FlagLoopback),
		"FlagPointToPoint": reflect.ValueOf(net.FlagPointToPoint),
		"FlagMulticast":    reflect.ValueOf(net.FlagMulticast),
		"IPv4len":          reflect.ValueOf(net.IPv4len),
		"IPv6len":          reflect.ValueOf(net.IPv6len),

		// define variables
		"ErrWriteToConnected":        reflect.ValueOf(net.ErrWriteToConnected),
		"IPv4zero":                   reflect.ValueOf(net.IPv4zero),
		"IPv6interfacelocalallnodes": reflect.ValueOf(net.IPv6interfacelocalallnodes),
		"IPv6linklocalallrouters":    reflect.ValueOf(net.IPv6linklocalallrouters),
		"IPv6linklocalallnodes":      reflect.ValueOf(net.IPv6linklocalallnodes),
		"DefaultResolver":            reflect.ValueOf(net.DefaultResolver),
		"IPv4bcast":                  reflect.ValueOf(net.IPv4bcast),
		"IPv4allsys":                 reflect.ValueOf(net.IPv4allsys),
		"IPv4allrouter":              reflect.ValueOf(net.IPv4allrouter),
		"IPv6zero":                   reflect.ValueOf(net.IPv6zero),
		"IPv6unspecified":            reflect.ValueOf(net.IPv6unspecified),
		"IPv6loopback":               reflect.ValueOf(net.IPv6loopback),

		// define functions
		"FileListener":       reflect.ValueOf(net.FileListener),
		"ListenUnix":         reflect.ValueOf(net.ListenUnix),
		"DialTCP":            reflect.ValueOf(net.DialTCP),
		"ListenIP":           reflect.ValueOf(net.ListenIP),
		"InterfaceByName":    reflect.ValueOf(net.InterfaceByName),
		"JoinHostPort":       reflect.ValueOf(net.JoinHostPort),
		"ResolveUnixAddr":    reflect.ValueOf(net.ResolveUnixAddr),
		"DialUDP":            reflect.ValueOf(net.DialUDP),
		"LookupCNAME":        reflect.ValueOf(net.LookupCNAME),
		"ResolveIPAddr":      reflect.ValueOf(net.ResolveIPAddr),
		"Interfaces":         reflect.ValueOf(net.Interfaces),
		"LookupAddr":         reflect.ValueOf(net.LookupAddr),
		"FileConn":           reflect.ValueOf(net.FileConn),
		"FilePacketConn":     reflect.ValueOf(net.FilePacketConn),
		"Listen":             reflect.ValueOf(net.Listen),
		"ListenPacket":       reflect.ValueOf(net.ListenPacket),
		"ResolveUDPAddr":     reflect.ValueOf(net.ResolveUDPAddr),
		"LookupPort":         reflect.ValueOf(net.LookupPort),
		"LookupSRV":          reflect.ValueOf(net.LookupSRV),
		"DialIP":             reflect.ValueOf(net.DialIP),
		"InterfaceByIndex":   reflect.ValueOf(net.InterfaceByIndex),
		"IPv4":               reflect.ValueOf(net.IPv4),
		"IPv4Mask":           reflect.ValueOf(net.IPv4Mask),
		"DialTimeout":        reflect.ValueOf(net.DialTimeout),
		"LookupIP":           reflect.ValueOf(net.LookupIP),
		"LookupMX":           reflect.ValueOf(net.LookupMX),
		"LookupNS":           reflect.ValueOf(net.LookupNS),
		"Pipe":               reflect.ValueOf(net.Pipe),
		"ParseIP":            reflect.ValueOf(net.ParseIP),
		"ListenUDP":          reflect.ValueOf(net.ListenUDP),
		"ListenTCP":          reflect.ValueOf(net.ListenTCP),
		"LookupHost":         reflect.ValueOf(net.LookupHost),
		"ParseCIDR":          reflect.ValueOf(net.ParseCIDR),
		"ParseMAC":           reflect.ValueOf(net.ParseMAC),
		"DialUnix":           reflect.ValueOf(net.DialUnix),
		"ListenUnixgram":     reflect.ValueOf(net.ListenUnixgram),
		"LookupTXT":          reflect.ValueOf(net.LookupTXT),
		"CIDRMask":           reflect.ValueOf(net.CIDRMask),
		"SplitHostPort":      reflect.ValueOf(net.SplitHostPort),
		"Dial":               reflect.ValueOf(net.Dial),
		"ListenMulticastUDP": reflect.ValueOf(net.ListenMulticastUDP),
		"ResolveTCPAddr":     reflect.ValueOf(net.ResolveTCPAddr),
		"InterfaceAddrs":     reflect.ValueOf(net.InterfaceAddrs),
	}
	var (
		err                 net.Error
		dNSConfigError      net.DNSConfigError
		flags               net.Flags
		iPAddr              net.IPAddr
		iP                  net.IP
		packetConn          net.PacketConn
		buffers             net.Buffers
		uDPAddr             net.UDPAddr
		tCPAddr             net.TCPAddr
		nS                  net.NS
		tCPListener         net.TCPListener
		resolver            net.Resolver
		conn                net.Conn
		opError             net.OpError
		parseError          net.ParseError
		invalidAddrError    net.InvalidAddrError
		iPConn              net.IPConn
		iPMask              net.IPMask
		iPNet               net.IPNet
		mX                  net.MX
		dialer              net.Dialer
		addr                net.Addr
		tCPConn             net.TCPConn
		dNSError            net.DNSError
		hardwareAddr        net.HardwareAddr
		listenConfig        net.ListenConfig
		addrError           net.AddrError
		unknownNetworkError net.UnknownNetworkError
		unixAddr            net.UnixAddr
		uDPConn             net.UDPConn
		iface               net.Interface
		sRV                 net.SRV
		listener            net.Listener
		unixConn            net.UnixConn
		unixListener        net.UnixListener
	)
	env.PackageTypes["net"] = map[string]reflect.Type{
		"Error":               reflect.TypeOf(&err).Elem(),
		"DNSConfigError":      reflect.TypeOf(&dNSConfigError).Elem(),
		"Flags":               reflect.TypeOf(&flags).Elem(),
		"IPAddr":              reflect.TypeOf(&iPAddr).Elem(),
		"IP":                  reflect.TypeOf(&iP).Elem(),
		"PacketConn":          reflect.TypeOf(&packetConn).Elem(),
		"Buffers":             reflect.TypeOf(&buffers).Elem(),
		"UDPAddr":             reflect.TypeOf(&uDPAddr).Elem(),
		"TCPAddr":             reflect.TypeOf(&tCPAddr).Elem(),
		"NS":                  reflect.TypeOf(&nS).Elem(),
		"TCPListener":         reflect.TypeOf(&tCPListener).Elem(),
		"Resolver":            reflect.TypeOf(&resolver).Elem(),
		"Conn":                reflect.TypeOf(&conn).Elem(),
		"OpError":             reflect.TypeOf(&opError).Elem(),
		"ParseError":          reflect.TypeOf(&parseError).Elem(),
		"InvalidAddrError":    reflect.TypeOf(&invalidAddrError).Elem(),
		"IPConn":              reflect.TypeOf(&iPConn).Elem(),
		"IPMask":              reflect.TypeOf(&iPMask).Elem(),
		"IPNet":               reflect.TypeOf(&iPNet).Elem(),
		"MX":                  reflect.TypeOf(&mX).Elem(),
		"Dialer":              reflect.TypeOf(&dialer).Elem(),
		"Addr":                reflect.TypeOf(&addr).Elem(),
		"TCPConn":             reflect.TypeOf(&tCPConn).Elem(),
		"DNSError":            reflect.TypeOf(&dNSError).Elem(),
		"HardwareAddr":        reflect.TypeOf(&hardwareAddr).Elem(),
		"ListenConfig":        reflect.TypeOf(&listenConfig).Elem(),
		"AddrError":           reflect.TypeOf(&addrError).Elem(),
		"UnknownNetworkError": reflect.TypeOf(&unknownNetworkError).Elem(),
		"UnixAddr":            reflect.TypeOf(&unixAddr).Elem(),
		"UDPConn":             reflect.TypeOf(&uDPConn).Elem(),
		"Interface":           reflect.TypeOf(&iface).Elem(),
		"SRV":                 reflect.TypeOf(&sRV).Elem(),
		"Listener":            reflect.TypeOf(&listener).Elem(),
		"UnixConn":            reflect.TypeOf(&unixConn).Elem(),
		"UnixListener":        reflect.TypeOf(&unixListener).Elem(),
	}
}

func initNetHTTP() {
	env.Packages["net/http"] = map[string]reflect.Value{
		// define constants
		"StatusRequestTimeout":                reflect.ValueOf(http.StatusRequestTimeout),
		"StatusVariantAlsoNegotiates":         reflect.ValueOf(http.StatusVariantAlsoNegotiates),
		"StatusLoopDetected":                  reflect.ValueOf(http.StatusLoopDetected),
		"StateHijacked":                       reflect.ValueOf(http.StateHijacked),
		"StatusEarlyHints":                    reflect.ValueOf(http.StatusEarlyHints),
		"StatusNonAuthoritativeInfo":          reflect.ValueOf(http.StatusNonAuthoritativeInfo),
		"MethodGet":                           reflect.ValueOf(http.MethodGet),
		"StateActive":                         reflect.ValueOf(http.StateActive),
		"StatusNotExtended":                   reflect.ValueOf(http.StatusNotExtended),
		"StatusHTTPVersionNotSupported":       reflect.ValueOf(http.StatusHTTPVersionNotSupported),
		"StateClosed":                         reflect.ValueOf(http.StateClosed),
		"StatusTooEarly":                      reflect.ValueOf(http.StatusTooEarly),
		"StatusServiceUnavailable":            reflect.ValueOf(http.StatusServiceUnavailable),
		"StateIdle":                           reflect.ValueOf(http.StateIdle),
		"StatusPreconditionFailed":            reflect.ValueOf(http.StatusPreconditionFailed),
		"StatusMisdirectedRequest":            reflect.ValueOf(http.StatusMisdirectedRequest),
		"StatusPreconditionRequired":          reflect.ValueOf(http.StatusPreconditionRequired),
		"StatusCreated":                       reflect.ValueOf(http.StatusCreated),
		"StatusIMUsed":                        reflect.ValueOf(http.StatusIMUsed),
		"StatusMultipleChoices":               reflect.ValueOf(http.StatusMultipleChoices),
		"StatusPermanentRedirect":             reflect.ValueOf(http.StatusPermanentRedirect),
		"StatusBadRequest":                    reflect.ValueOf(http.StatusBadRequest),
		"StatusForbidden":                     reflect.ValueOf(http.StatusForbidden),
		"StatusLengthRequired":                reflect.ValueOf(http.StatusLengthRequired),
		"StatusLocked":                        reflect.ValueOf(http.StatusLocked),
		"StatusOK":                            reflect.ValueOf(http.StatusOK),
		"MethodPut":                           reflect.ValueOf(http.MethodPut),
		"MethodTrace":                         reflect.ValueOf(http.MethodTrace),
		"StatusNetworkAuthenticationRequired": reflect.ValueOf(http.StatusNetworkAuthenticationRequired),
		"StatusMultiStatus":                   reflect.ValueOf(http.StatusMultiStatus),
		"StatusNotModified":                   reflect.ValueOf(http.StatusNotModified),
		"StatusTemporaryRedirect":             reflect.ValueOf(http.StatusTemporaryRedirect),
		"StatusMethodNotAllowed":              reflect.ValueOf(http.StatusMethodNotAllowed),
		"StatusUpgradeRequired":               reflect.ValueOf(http.StatusUpgradeRequired),
		"StatusRequestHeaderFieldsTooLarge":   reflect.ValueOf(http.StatusRequestHeaderFieldsTooLarge),
		"StatusInternalServerError":           reflect.ValueOf(http.StatusInternalServerError),
		"SameSiteDefaultMode":                 reflect.ValueOf(http.SameSiteDefaultMode),
		"MethodOptions":                       reflect.ValueOf(http.MethodOptions),
		"StatusBadGateway":                    reflect.ValueOf(http.StatusBadGateway),
		"StatusUnauthorized":                  reflect.ValueOf(http.StatusUnauthorized),
		"StatusNotImplemented":                reflect.ValueOf(http.StatusNotImplemented),
		"MethodPatch":                         reflect.ValueOf(http.MethodPatch),
		"StateNew":                            reflect.ValueOf(http.StateNew),
		"StatusFound":                         reflect.ValueOf(http.StatusFound),
		"StatusTeapot":                        reflect.ValueOf(http.StatusTeapot),
		"StatusInsufficientStorage":           reflect.ValueOf(http.StatusInsufficientStorage),
		"MethodDelete":                        reflect.ValueOf(http.MethodDelete),
		"SameSiteLaxMode":                     reflect.ValueOf(http.SameSiteLaxMode),
		"StatusAccepted":                      reflect.ValueOf(http.StatusAccepted),
		"StatusAlreadyReported":               reflect.ValueOf(http.StatusAlreadyReported),
		"StatusNotAcceptable":                 reflect.ValueOf(http.StatusNotAcceptable),
		"StatusConflict":                      reflect.ValueOf(http.StatusConflict),
		"StatusGone":                          reflect.ValueOf(http.StatusGone),
		"StatusRequestURITooLong":             reflect.ValueOf(http.StatusRequestURITooLong),
		"StatusUnsupportedMediaType":          reflect.ValueOf(http.StatusUnsupportedMediaType),
		"StatusContinue":                      reflect.ValueOf(http.StatusContinue),
		"StatusRequestEntityTooLarge":         reflect.ValueOf(http.StatusRequestEntityTooLarge),
		"StatusUnavailableForLegalReasons":    reflect.ValueOf(http.StatusUnavailableForLegalReasons),
		"StatusGatewayTimeout":                reflect.ValueOf(http.StatusGatewayTimeout),
		"StatusPaymentRequired":               reflect.ValueOf(http.StatusPaymentRequired),
		"StatusSeeOther":                      reflect.ValueOf(http.StatusSeeOther),
		"StatusNotFound":                      reflect.ValueOf(http.StatusNotFound),
		"StatusExpectationFailed":             reflect.ValueOf(http.StatusExpectationFailed),
		"MethodConnect":                       reflect.ValueOf(http.MethodConnect),
		"SameSiteStrictMode":                  reflect.ValueOf(http.SameSiteStrictMode),
		"StatusUnprocessableEntity":           reflect.ValueOf(http.StatusUnprocessableEntity),
		"StatusFailedDependency":              reflect.ValueOf(http.StatusFailedDependency),
		"StatusTooManyRequests":               reflect.ValueOf(http.StatusTooManyRequests),
		"DefaultMaxIdleConnsPerHost":          reflect.ValueOf(http.DefaultMaxIdleConnsPerHost),
		"MethodPost":                          reflect.ValueOf(http.MethodPost),
		"StatusPartialContent":                reflect.ValueOf(http.StatusPartialContent),
		"DefaultMaxHeaderBytes":               reflect.ValueOf(http.DefaultMaxHeaderBytes),
		"StatusMovedPermanently":              reflect.ValueOf(http.StatusMovedPermanently),
		"StatusRequestedRangeNotSatisfiable":  reflect.ValueOf(http.StatusRequestedRangeNotSatisfiable),
		"TrailerPrefix":                       reflect.ValueOf(http.TrailerPrefix),
		"TimeFormat":                          reflect.ValueOf(http.TimeFormat),
		"StatusProcessing":                    reflect.ValueOf(http.StatusProcessing),
		"StatusUseProxy":                      reflect.ValueOf(http.StatusUseProxy),
		"MethodHead":                          reflect.ValueOf(http.MethodHead),
		"SameSiteNoneMode":                    reflect.ValueOf(http.SameSiteNoneMode),
		"StatusNoContent":                     reflect.ValueOf(http.StatusNoContent),
		"StatusResetContent":                  reflect.ValueOf(http.StatusResetContent),
		"StatusProxyAuthRequired":             reflect.ValueOf(http.StatusProxyAuthRequired),
		"StatusSwitchingProtocols":            reflect.ValueOf(http.StatusSwitchingProtocols),

		// define variables
		"ErrNotMultipart":       reflect.ValueOf(http.ErrNotMultipart),
		"ErrNoCookie":           reflect.ValueOf(http.ErrNoCookie),
		"ServerContextKey":      reflect.ValueOf(http.ServerContextKey),
		"DefaultServeMux":       reflect.ValueOf(http.DefaultServeMux),
		"ErrMissingFile":        reflect.ValueOf(http.ErrMissingFile),
		"ErrNotSupported":       reflect.ValueOf(http.ErrNotSupported),
		"ErrMissingBoundary":    reflect.ValueOf(http.ErrMissingBoundary),
		"ErrNoLocation":         reflect.ValueOf(http.ErrNoLocation),
		"ErrLineTooLong":        reflect.ValueOf(http.ErrLineTooLong),
		"NoBody":                reflect.ValueOf(http.NoBody),
		"ErrAbortHandler":       reflect.ValueOf(http.ErrAbortHandler),
		"ErrUseLastResponse":    reflect.ValueOf(http.ErrUseLastResponse),
		"ErrContentLength":      reflect.ValueOf(http.ErrContentLength),
		"LocalAddrContextKey":   reflect.ValueOf(http.LocalAddrContextKey),
		"ErrServerClosed":       reflect.ValueOf(http.ErrServerClosed),
		"ErrHandlerTimeout":     reflect.ValueOf(http.ErrHandlerTimeout),
		"DefaultClient":         reflect.ValueOf(http.DefaultClient),
		"ErrBodyReadAfterClose": reflect.ValueOf(http.ErrBodyReadAfterClose),
		"ErrBodyNotAllowed":     reflect.ValueOf(http.ErrBodyNotAllowed),
		"ErrHijacked":           reflect.ValueOf(http.ErrHijacked),
		"DefaultTransport":      reflect.ValueOf(http.DefaultTransport),
		"ErrSkipAltProtocol":    reflect.ValueOf(http.ErrSkipAltProtocol),

		// define functions
		"MaxBytesReader":        reflect.ValueOf(http.MaxBytesReader),
		"ReadResponse":          reflect.ValueOf(http.ReadResponse),
		"ListenAndServe":        reflect.ValueOf(http.ListenAndServe),
		"ListenAndServeTLS":     reflect.ValueOf(http.ListenAndServeTLS),
		"StatusText":            reflect.ValueOf(http.StatusText),
		"NewFileTransport":      reflect.ValueOf(http.NewFileTransport),
		"Post":                  reflect.ValueOf(http.Post),
		"Error":                 reflect.ValueOf(http.Error),
		"Redirect":              reflect.ValueOf(http.Redirect),
		"RedirectHandler":       reflect.ValueOf(http.RedirectHandler),
		"Get":                   reflect.ValueOf(http.Get),
		"ParseTime":             reflect.ValueOf(http.ParseTime),
		"ServeTLS":              reflect.ValueOf(http.ServeTLS),
		"Head":                  reflect.ValueOf(http.Head),
		"ServeContent":          reflect.ValueOf(http.ServeContent),
		"NewRequest":            reflect.ValueOf(http.NewRequest),
		"Handle":                reflect.ValueOf(http.Handle),
		"HandleFunc":            reflect.ValueOf(http.HandleFunc),
		"ServeFile":             reflect.ValueOf(http.ServeFile),
		"ParseHTTPVersion":      reflect.ValueOf(http.ParseHTTPVersion),
		"NotFoundHandler":       reflect.ValueOf(http.NotFoundHandler),
		"Serve":                 reflect.ValueOf(http.Serve),
		"TimeoutHandler":        reflect.ValueOf(http.TimeoutHandler),
		"NewRequestWithContext": reflect.ValueOf(http.NewRequestWithContext),
		"ProxyURL":              reflect.ValueOf(http.ProxyURL),
		"FileServer":            reflect.ValueOf(http.FileServer),
		"ReadRequest":           reflect.ValueOf(http.ReadRequest),
		"SetCookie":             reflect.ValueOf(http.SetCookie),
		"CanonicalHeaderKey":    reflect.ValueOf(http.CanonicalHeaderKey),
		"PostForm":              reflect.ValueOf(http.PostForm),
		"ProxyFromEnvironment":  reflect.ValueOf(http.ProxyFromEnvironment),
		"NotFound":              reflect.ValueOf(http.NotFound),
		"StripPrefix":           reflect.ValueOf(http.StripPrefix),
		"NewServeMux":           reflect.ValueOf(http.NewServeMux),
		"DetectContentType":     reflect.ValueOf(http.DetectContentType),
	}
	var (
		header         http.Header
		hijacker       http.Hijacker
		handlerFunc    http.HandlerFunc
		transport      http.Transport
		response       http.Response
		sameSite       http.SameSite
		responseWriter http.ResponseWriter
		flusher        http.Flusher
		serveMux       http.ServeMux
		client         http.Client
		roundTripper   http.RoundTripper
		fileSystem     http.FileSystem
		server         http.Server
		dir            http.Dir
		file           http.File
		request        http.Request
		cookie         http.Cookie
		handler        http.Handler
		connState      http.ConnState
		cookieJar      http.CookieJar
		pushOptions    http.PushOptions
		pusher         http.Pusher
	)
	env.PackageTypes["net/http"] = map[string]reflect.Type{
		"Header":         reflect.TypeOf(&header).Elem(),
		"Hijacker":       reflect.TypeOf(&hijacker).Elem(),
		"HandlerFunc":    reflect.TypeOf(&handlerFunc).Elem(),
		"Transport":      reflect.TypeOf(&transport).Elem(),
		"Response":       reflect.TypeOf(&response).Elem(),
		"SameSite":       reflect.TypeOf(&sameSite).Elem(),
		"ResponseWriter": reflect.TypeOf(&responseWriter).Elem(),
		"Flusher":        reflect.TypeOf(&flusher).Elem(),
		"ServeMux":       reflect.TypeOf(&serveMux).Elem(),
		"Client":         reflect.TypeOf(&client).Elem(),
		"RoundTripper":   reflect.TypeOf(&roundTripper).Elem(),
		"FileSystem":     reflect.TypeOf(&fileSystem).Elem(),
		"Server":         reflect.TypeOf(&server).Elem(),
		"Dir":            reflect.TypeOf(&dir).Elem(),
		"File":           reflect.TypeOf(&file).Elem(),
		"Request":        reflect.TypeOf(&request).Elem(),
		"Cookie":         reflect.TypeOf(&cookie).Elem(),
		"Handler":        reflect.TypeOf(&handler).Elem(),
		"ConnState":      reflect.TypeOf(&connState).Elem(),
		"CookieJar":      reflect.TypeOf(&cookieJar).Elem(),
		"PushOptions":    reflect.TypeOf(&pushOptions).Elem(),
		"Pusher":         reflect.TypeOf(&pusher).Elem(),
	}
}

func initNetHTTPCookieJar() {
	env.Packages["net/http/cookiejar"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
		"New": reflect.ValueOf(cookiejar.New),
	}
	var (
		jar              cookiejar.Jar
		publicSuffixList cookiejar.PublicSuffixList
		options          cookiejar.Options
	)
	env.PackageTypes["net/http/cookiejar"] = map[string]reflect.Type{
		"Jar":              reflect.TypeOf(&jar).Elem(),
		"PublicSuffixList": reflect.TypeOf(&publicSuffixList).Elem(),
		"Options":          reflect.TypeOf(&options).Elem(),
	}
}

func initNetURL() {
	env.Packages["net/url"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
		"PathUnescape":    reflect.ValueOf(url.PathUnescape),
		"PathEscape":      reflect.ValueOf(url.PathEscape),
		"UserPassword":    reflect.ValueOf(url.UserPassword),
		"Parse":           reflect.ValueOf(url.Parse),
		"ParseQuery":      reflect.ValueOf(url.ParseQuery),
		"QueryUnescape":   reflect.ValueOf(url.QueryUnescape),
		"QueryEscape":     reflect.ValueOf(url.QueryEscape),
		"User":            reflect.ValueOf(url.User),
		"ParseRequestURI": reflect.ValueOf(url.ParseRequestURI),
	}
	var (
		userinfo         url.Userinfo
		values           url.Values
		err              url.Error
		escapeError      url.EscapeError
		invalidHostError url.InvalidHostError
		uRL              url.URL
	)
	env.PackageTypes["net/url"] = map[string]reflect.Type{
		"Userinfo":         reflect.TypeOf(&userinfo).Elem(),
		"Values":           reflect.TypeOf(&values).Elem(),
		"Error":            reflect.TypeOf(&err).Elem(),
		"EscapeError":      reflect.TypeOf(&escapeError).Elem(),
		"InvalidHostError": reflect.TypeOf(&invalidHostError).Elem(),
		"URL":              reflect.TypeOf(&uRL).Elem(),
	}
}

func initOS() {
	env.Packages["os"] = map[string]reflect.Value{
		// define constants
		"PathSeparator":     reflect.ValueOf(os.PathSeparator),
		"O_WRONLY":          reflect.ValueOf(os.O_WRONLY),
		"O_APPEND":          reflect.ValueOf(os.O_APPEND),
		"O_CREATE":          reflect.ValueOf(os.O_CREATE),
		"O_SYNC":            reflect.ValueOf(os.O_SYNC),
		"ModeAppend":        reflect.ValueOf(os.ModeAppend),
		"ModeTemporary":     reflect.ValueOf(os.ModeTemporary),
		"ModeSymlink":       reflect.ValueOf(os.ModeSymlink),
		"ModeDevice":        reflect.ValueOf(os.ModeDevice),
		"ModeSetgid":        reflect.ValueOf(os.ModeSetgid),
		"PathListSeparator": reflect.ValueOf(os.PathListSeparator),
		"O_RDONLY":          reflect.ValueOf(os.O_RDONLY),
		"ModeSocket":        reflect.ValueOf(os.ModeSocket),
		"ModeIrregular":     reflect.ValueOf(os.ModeIrregular),
		"ModePerm":          reflect.ValueOf(os.ModePerm),
		"O_EXCL":            reflect.ValueOf(os.O_EXCL),
		"DevNull":           reflect.ValueOf(os.DevNull),
		"ModeDir":           reflect.ValueOf(os.ModeDir),
		"ModeCharDevice":    reflect.ValueOf(os.ModeCharDevice),
		"ModeType":          reflect.ValueOf(os.ModeType),
		"O_RDWR":            reflect.ValueOf(os.O_RDWR),
		"O_TRUNC":           reflect.ValueOf(os.O_TRUNC),
		"ModeExclusive":     reflect.ValueOf(os.ModeExclusive),
		"ModeNamedPipe":     reflect.ValueOf(os.ModeNamedPipe),
		"ModeSetuid":        reflect.ValueOf(os.ModeSetuid),
		"ModeSticky":        reflect.ValueOf(os.ModeSticky),

		// define variables
		"Stdout":              reflect.ValueOf(os.Stdout),
		"Interrupt":           reflect.ValueOf(os.Interrupt),
		"ErrInvalid":          reflect.ValueOf(os.ErrInvalid),
		"ErrPermission":       reflect.ValueOf(os.ErrPermission),
		"ErrNotExist":         reflect.ValueOf(os.ErrNotExist),
		"ErrNoDeadline":       reflect.ValueOf(os.ErrNoDeadline),
		"ErrDeadlineExceeded": reflect.ValueOf(os.ErrDeadlineExceeded),
		"Args":                reflect.ValueOf(os.Args),
		"ErrExist":            reflect.ValueOf(os.ErrExist),
		"ErrClosed":           reflect.ValueOf(os.ErrClosed),
		"Stdin":               reflect.ValueOf(os.Stdin),
		"Stderr":              reflect.ValueOf(os.Stderr),
		"Kill":                reflect.ValueOf(os.Kill),

		// define functions
		"NewSyscallError": reflect.ValueOf(os.NewSyscallError),
		"Getgid":          reflect.ValueOf(os.Getgid),
		"Getegid":         reflect.ValueOf(os.Getegid),
		"Exit":            reflect.ValueOf(os.Exit),
		"Chdir":           reflect.ValueOf(os.Chdir),
		"Unsetenv":        reflect.ValueOf(os.Unsetenv),
		"Create":          reflect.ValueOf(os.Create),
		"MkdirAll":        reflect.ValueOf(os.MkdirAll),
		"Environ":         reflect.ValueOf(os.Environ),
		"UserConfigDir":   reflect.ValueOf(os.UserConfigDir),
		"Getenv":          reflect.ValueOf(os.Getenv),
		"Lstat":           reflect.ValueOf(os.Lstat),
		"Symlink":         reflect.ValueOf(os.Symlink),
		"LookupEnv":       reflect.ValueOf(os.LookupEnv),
		"IsExist":         reflect.ValueOf(os.IsExist),
		"IsNotExist":      reflect.ValueOf(os.IsNotExist),
		"Getppid":         reflect.ValueOf(os.Getppid),
		"SameFile":        reflect.ValueOf(os.SameFile),
		"IsPathSeparator": reflect.ValueOf(os.IsPathSeparator),
		"UserCacheDir":    reflect.ValueOf(os.UserCacheDir),
		"UserHomeDir":     reflect.ValueOf(os.UserHomeDir),
		"NewFile":         reflect.ValueOf(os.NewFile),
		"Geteuid":         reflect.ValueOf(os.Geteuid),
		"Chmod":           reflect.ValueOf(os.Chmod),
		"Stat":            reflect.ValueOf(os.Stat),
		"Expand":          reflect.ValueOf(os.Expand),
		"Clearenv":        reflect.ValueOf(os.Clearenv),
		"Lchown":          reflect.ValueOf(os.Lchown),
		"Getpid":          reflect.ValueOf(os.Getpid),
		"Executable":      reflect.ValueOf(os.Executable),
		"Getpagesize":     reflect.ValueOf(os.Getpagesize),
		"Remove":          reflect.ValueOf(os.Remove),
		"Chown":           reflect.ValueOf(os.Chown),
		"RemoveAll":       reflect.ValueOf(os.RemoveAll),
		"StartProcess":    reflect.ValueOf(os.StartProcess),
		"Getgroups":       reflect.ValueOf(os.Getgroups),
		"Mkdir":           reflect.ValueOf(os.Mkdir),
		"Truncate":        reflect.ValueOf(os.Truncate),
		"Rename":          reflect.ValueOf(os.Rename),
		"Setenv":          reflect.ValueOf(os.Setenv),
		"Chtimes":         reflect.ValueOf(os.Chtimes),
		"Hostname":        reflect.ValueOf(os.Hostname),
		"Getuid":          reflect.ValueOf(os.Getuid),
		"Open":            reflect.ValueOf(os.Open),
		"OpenFile":        reflect.ValueOf(os.OpenFile),
		"Link":            reflect.ValueOf(os.Link),
		"Readlink":        reflect.ValueOf(os.Readlink),
		"IsPermission":    reflect.ValueOf(os.IsPermission),
		"IsTimeout":       reflect.ValueOf(os.IsTimeout),
		"Pipe":            reflect.ValueOf(os.Pipe),
		"TempDir":         reflect.ValueOf(os.TempDir),
		"ExpandEnv":       reflect.ValueOf(os.ExpandEnv),
		"Getwd":           reflect.ValueOf(os.Getwd),
		"FindProcess":     reflect.ValueOf(os.FindProcess),
	}
	var (
		processState os.ProcessState
		process      os.Process
		sig          os.Signal
		file         os.File
		fileInfo     os.FileInfo
		pathError    os.PathError
		syscallError os.SyscallError
		linkError    os.LinkError
		fileMode     os.FileMode
		procAttr     os.ProcAttr
	)
	env.PackageTypes["os"] = map[string]reflect.Type{
		"ProcessState": reflect.TypeOf(&processState).Elem(),
		"Process":      reflect.TypeOf(&process).Elem(),
		"Signal":       reflect.TypeOf(&sig).Elem(),
		"File":         reflect.TypeOf(&file).Elem(),
		"FileInfo":     reflect.TypeOf(&fileInfo).Elem(),
		"PathError":    reflect.TypeOf(&pathError).Elem(),
		"SyscallError": reflect.TypeOf(&syscallError).Elem(),
		"LinkError":    reflect.TypeOf(&linkError).Elem(),
		"FileMode":     reflect.TypeOf(&fileMode).Elem(),
		"ProcAttr":     reflect.TypeOf(&procAttr).Elem(),
	}
}

func initOSExec() {
	env.Packages["os/exec"] = map[string]reflect.Value{
		// define constants

		// define variables
		"ErrNotFound": reflect.ValueOf(exec.ErrNotFound),

		// define functions
		"CommandContext": reflect.ValueOf(exec.CommandContext),
		"LookPath":       reflect.ValueOf(exec.LookPath),
		"Command":        reflect.ValueOf(exec.Command),
	}
	var (
		exitError exec.ExitError
		err       exec.Error
		cmd       exec.Cmd
	)
	env.PackageTypes["os/exec"] = map[string]reflect.Type{
		"ExitError": reflect.TypeOf(&exitError).Elem(),
		"Error":     reflect.TypeOf(&err).Elem(),
		"Cmd":       reflect.TypeOf(&cmd).Elem(),
	}
}

func initOSSignal() {
	env.Packages["os/signal"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
		"Notify":  reflect.ValueOf(signal.Notify),
		"Reset":   reflect.ValueOf(signal.Reset),
		"Stop":    reflect.ValueOf(signal.Stop),
		"Ignore":  reflect.ValueOf(signal.Ignore),
		"Ignored": reflect.ValueOf(signal.Ignored),
	}
	var ()
	env.PackageTypes["os/signal"] = map[string]reflect.Type{}
}

func initOSUser() {
	env.Packages["os/user"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
		"LookupId":      reflect.ValueOf(user.LookupId),
		"LookupGroup":   reflect.ValueOf(user.LookupGroup),
		"LookupGroupId": reflect.ValueOf(user.LookupGroupId),
		"Current":       reflect.ValueOf(user.Current),
		"Lookup":        reflect.ValueOf(user.Lookup),
	}
	var (
		usr                 user.User
		group               user.Group
		unknownUserIDError  user.UnknownUserIdError
		unknownUserError    user.UnknownUserError
		unknownGroupIDError user.UnknownGroupIdError
		unknownGroupError   user.UnknownGroupError
	)
	env.PackageTypes["os/user"] = map[string]reflect.Type{
		"User":                reflect.TypeOf(&usr).Elem(),
		"Group":               reflect.TypeOf(&group).Elem(),
		"UnknownUserIdError":  reflect.TypeOf(&unknownUserIDError).Elem(),
		"UnknownUserError":    reflect.TypeOf(&unknownUserError).Elem(),
		"UnknownGroupIdError": reflect.TypeOf(&unknownGroupIDError).Elem(),
		"UnknownGroupError":   reflect.TypeOf(&unknownGroupError).Elem(),
	}
}

func initPath() {
	env.Packages["path"] = map[string]reflect.Value{
		// define constants

		// define variables
		"ErrBadPattern": reflect.ValueOf(path.ErrBadPattern),

		// define functions
		"Clean": reflect.ValueOf(path.Clean),
		"Split": reflect.ValueOf(path.Split),
		"Join":  reflect.ValueOf(path.Join),
		"Ext":   reflect.ValueOf(path.Ext),
		"Base":  reflect.ValueOf(path.Base),
		"IsAbs": reflect.ValueOf(path.IsAbs),
		"Dir":   reflect.ValueOf(path.Dir),
		"Match": reflect.ValueOf(path.Match),
	}
	var ()
	env.PackageTypes["path"] = map[string]reflect.Type{}
}

func initPathFilepath() {
	env.Packages["path/filepath"] = map[string]reflect.Value{
		// define constants
		"Separator":     reflect.ValueOf(filepath.Separator),
		"ListSeparator": reflect.ValueOf(filepath.ListSeparator),

		// define variables
		"SkipDir":       reflect.ValueOf(filepath.SkipDir),
		"ErrBadPattern": reflect.ValueOf(filepath.ErrBadPattern),

		// define functions
		"Split":        reflect.ValueOf(filepath.Split),
		"Abs":          reflect.ValueOf(filepath.Abs),
		"IsAbs":        reflect.ValueOf(filepath.IsAbs),
		"Match":        reflect.ValueOf(filepath.Match),
		"ToSlash":      reflect.ValueOf(filepath.ToSlash),
		"FromSlash":    reflect.ValueOf(filepath.FromSlash),
		"SplitList":    reflect.ValueOf(filepath.SplitList),
		"Rel":          reflect.ValueOf(filepath.Rel),
		"Walk":         reflect.ValueOf(filepath.Walk),
		"VolumeName":   reflect.ValueOf(filepath.VolumeName),
		"Clean":        reflect.ValueOf(filepath.Clean),
		"Dir":          reflect.ValueOf(filepath.Dir),
		"Glob":         reflect.ValueOf(filepath.Glob),
		"Base":         reflect.ValueOf(filepath.Base),
		"Ext":          reflect.ValueOf(filepath.Ext),
		"EvalSymlinks": reflect.ValueOf(filepath.EvalSymlinks),
		"Join":         reflect.ValueOf(filepath.Join),
	}
	var (
		walkFunc filepath.WalkFunc
	)
	env.PackageTypes["path/filepath"] = map[string]reflect.Type{
		"WalkFunc": reflect.TypeOf(&walkFunc).Elem(),
	}
}

func initRegexp() {
	env.Packages["regexp"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
		"MatchString":      reflect.ValueOf(regexp.MatchString),
		"Match":            reflect.ValueOf(regexp.Match),
		"QuoteMeta":        reflect.ValueOf(regexp.QuoteMeta),
		"Compile":          reflect.ValueOf(regexp.Compile),
		"CompilePOSIX":     reflect.ValueOf(regexp.CompilePOSIX),
		"MustCompile":      reflect.ValueOf(regexp.MustCompile),
		"MustCompilePOSIX": reflect.ValueOf(regexp.MustCompilePOSIX),
		"MatchReader":      reflect.ValueOf(regexp.MatchReader),
	}
	var (
		reg regexp.Regexp
	)
	env.PackageTypes["regexp"] = map[string]reflect.Type{
		"Regexp": reflect.TypeOf(&reg).Elem(),
	}
}

func initSort() {
	env.Packages["sort"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
		"SliceStable":       reflect.ValueOf(sort.SliceStable),
		"Strings":           reflect.ValueOf(sort.Strings),
		"Float64sAreSorted": reflect.ValueOf(sort.Float64sAreSorted),
		"IsSorted":          reflect.ValueOf(sort.IsSorted),
		"StringsAreSorted":  reflect.ValueOf(sort.StringsAreSorted),
		"Stable":            reflect.ValueOf(sort.Stable),
		"Search":            reflect.ValueOf(sort.Search),
		"SearchInts":        reflect.ValueOf(sort.SearchInts),
		"Sort":              reflect.ValueOf(sort.Sort),
		"Reverse":           reflect.ValueOf(sort.Reverse),
		"IntsAreSorted":     reflect.ValueOf(sort.IntsAreSorted),
		"SearchFloat64s":    reflect.ValueOf(sort.SearchFloat64s),
		"SearchStrings":     reflect.ValueOf(sort.SearchStrings),
		"Slice":             reflect.ValueOf(sort.Slice),
		"SliceIsSorted":     reflect.ValueOf(sort.SliceIsSorted),
		"Ints":              reflect.ValueOf(sort.Ints),
		"Float64s":          reflect.ValueOf(sort.Float64s),
	}
	var (
		float64Slice sort.Float64Slice
		stringSlice  sort.StringSlice
		iface        sort.Interface
		intSlice     sort.IntSlice
	)
	env.PackageTypes["sort"] = map[string]reflect.Type{
		"Float64Slice": reflect.TypeOf(&float64Slice).Elem(),
		"StringSlice":  reflect.TypeOf(&stringSlice).Elem(),
		"Interface":    reflect.TypeOf(&iface).Elem(),
		"IntSlice":     reflect.TypeOf(&intSlice).Elem(),
	}
}

func initStrconv() {
	env.Packages["strconv"] = map[string]reflect.Value{
		// define constants
		"IntSize": reflect.ValueOf(strconv.IntSize),

		// define variables
		"ErrRange":  reflect.ValueOf(strconv.ErrRange),
		"ErrSyntax": reflect.ValueOf(strconv.ErrSyntax),

		// define functions
		"AppendQuote":              reflect.ValueOf(strconv.AppendQuote),
		"QuoteToASCII":             reflect.ValueOf(strconv.QuoteToASCII),
		"QuoteRuneToGraphic":       reflect.ValueOf(strconv.QuoteRuneToGraphic),
		"IsPrint":                  reflect.ValueOf(strconv.IsPrint),
		"Atoi":                     reflect.ValueOf(strconv.Atoi),
		"FormatComplex":            reflect.ValueOf(strconv.FormatComplex),
		"AppendInt":                reflect.ValueOf(strconv.AppendInt),
		"AppendUint":               reflect.ValueOf(strconv.AppendUint),
		"AppendQuoteRuneToASCII":   reflect.ValueOf(strconv.AppendQuoteRuneToASCII),
		"UnquoteChar":              reflect.ValueOf(strconv.UnquoteChar),
		"QuoteRune":                reflect.ValueOf(strconv.QuoteRune),
		"Unquote":                  reflect.ValueOf(strconv.Unquote),
		"IsGraphic":                reflect.ValueOf(strconv.IsGraphic),
		"ParseInt":                 reflect.ValueOf(strconv.ParseInt),
		"AppendBool":               reflect.ValueOf(strconv.AppendBool),
		"FormatFloat":              reflect.ValueOf(strconv.FormatFloat),
		"ParseComplex":             reflect.ValueOf(strconv.ParseComplex),
		"ParseUint":                reflect.ValueOf(strconv.ParseUint),
		"QuoteToGraphic":           reflect.ValueOf(strconv.QuoteToGraphic),
		"AppendQuoteToGraphic":     reflect.ValueOf(strconv.AppendQuoteToGraphic),
		"QuoteRuneToASCII":         reflect.ValueOf(strconv.QuoteRuneToASCII),
		"Quote":                    reflect.ValueOf(strconv.Quote),
		"AppendQuoteToASCII":       reflect.ValueOf(strconv.AppendQuoteToASCII),
		"AppendQuoteRune":          reflect.ValueOf(strconv.AppendQuoteRune),
		"CanBackquote":             reflect.ValueOf(strconv.CanBackquote),
		"AppendFloat":              reflect.ValueOf(strconv.AppendFloat),
		"ParseBool":                reflect.ValueOf(strconv.ParseBool),
		"FormatBool":               reflect.ValueOf(strconv.FormatBool),
		"AppendQuoteRuneToGraphic": reflect.ValueOf(strconv.AppendQuoteRuneToGraphic),
		"FormatUint":               reflect.ValueOf(strconv.FormatUint),
		"FormatInt":                reflect.ValueOf(strconv.FormatInt),
		"Itoa":                     reflect.ValueOf(strconv.Itoa),
		"ParseFloat":               reflect.ValueOf(strconv.ParseFloat),
	}
	var (
		numError strconv.NumError
	)
	env.PackageTypes["strconv"] = map[string]reflect.Type{
		"NumError": reflect.TypeOf(&numError).Elem(),
	}
}

func initStrings() {
	env.Packages["strings"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
		"Index":          reflect.ValueOf(strings.Index),
		"Compare":        reflect.ValueOf(strings.Compare),
		"Contains":       reflect.ValueOf(strings.Contains),
		"SplitAfter":     reflect.ValueOf(strings.SplitAfter),
		"Join":           reflect.ValueOf(strings.Join),
		"LastIndexFunc":  reflect.ValueOf(strings.LastIndexFunc),
		"TrimPrefix":     reflect.ValueOf(strings.TrimPrefix),
		"ReplaceAll":     reflect.ValueOf(strings.ReplaceAll),
		"Count":          reflect.ValueOf(strings.Count),
		"LastIndexAny":   reflect.ValueOf(strings.LastIndexAny),
		"Map":            reflect.ValueOf(strings.Map),
		"Repeat":         reflect.ValueOf(strings.Repeat),
		"ToLowerSpecial": reflect.ValueOf(strings.ToLowerSpecial),
		"NewReplacer":    reflect.ValueOf(strings.NewReplacer),
		"ContainsRune":   reflect.ValueOf(strings.ContainsRune),
		"IndexAny":       reflect.ValueOf(strings.IndexAny),
		"Trim":           reflect.ValueOf(strings.Trim),
		"NewReader":      reflect.ValueOf(strings.NewReader),
		"TrimSuffix":     reflect.ValueOf(strings.TrimSuffix),
		"IndexRune":      reflect.ValueOf(strings.IndexRune),
		"LastIndexByte":  reflect.ValueOf(strings.LastIndexByte),
		"SplitAfterN":    reflect.ValueOf(strings.SplitAfterN),
		"HasPrefix":      reflect.ValueOf(strings.HasPrefix),
		"TrimLeftFunc":   reflect.ValueOf(strings.TrimLeftFunc),
		"TrimRight":      reflect.ValueOf(strings.TrimRight),
		"ToTitle":        reflect.ValueOf(strings.ToTitle),
		"Title":          reflect.ValueOf(strings.Title),
		"TrimFunc":       reflect.ValueOf(strings.TrimFunc),
		"IndexFunc":      reflect.ValueOf(strings.IndexFunc),
		"Replace":        reflect.ValueOf(strings.Replace),
		"Fields":         reflect.ValueOf(strings.Fields),
		"ToUpperSpecial": reflect.ValueOf(strings.ToUpperSpecial),
		"ToValidUTF8":    reflect.ValueOf(strings.ToValidUTF8),
		"ContainsAny":    reflect.ValueOf(strings.ContainsAny),
		"LastIndex":      reflect.ValueOf(strings.LastIndex),
		"SplitN":         reflect.ValueOf(strings.SplitN),
		"HasSuffix":      reflect.ValueOf(strings.HasSuffix),
		"ToUpper":        reflect.ValueOf(strings.ToUpper),
		"ToLower":        reflect.ValueOf(strings.ToLower),
		"TrimRightFunc":  reflect.ValueOf(strings.TrimRightFunc),
		"EqualFold":      reflect.ValueOf(strings.EqualFold),
		"IndexByte":      reflect.ValueOf(strings.IndexByte),
		"Split":          reflect.ValueOf(strings.Split),
		"FieldsFunc":     reflect.ValueOf(strings.FieldsFunc),
		"ToTitleSpecial": reflect.ValueOf(strings.ToTitleSpecial),
		"TrimLeft":       reflect.ValueOf(strings.TrimLeft),
		"TrimSpace":      reflect.ValueOf(strings.TrimSpace),
	}
	var (
		replacer strings.Replacer
		builder  strings.Builder
		reader   strings.Reader
	)
	env.PackageTypes["strings"] = map[string]reflect.Type{
		"Replacer": reflect.TypeOf(&replacer).Elem(),
		"Builder":  reflect.TypeOf(&builder).Elem(),
		"Reader":   reflect.TypeOf(&reader).Elem(),
	}
}

func initSync() {
	env.Packages["sync"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
		"NewCond": reflect.ValueOf(sync.NewCond),
	}
	var (
		cond      sync.Cond
		once      sync.Once
		pool      sync.Pool
		m         sync.Map
		mutex     sync.Mutex
		locker    sync.Locker
		waitGroup sync.WaitGroup
		rWMutex   sync.RWMutex
	)
	env.PackageTypes["sync"] = map[string]reflect.Type{
		"Cond":      reflect.TypeOf(&cond).Elem(),
		"Once":      reflect.TypeOf(&once).Elem(),
		"Pool":      reflect.TypeOf(&pool).Elem(),
		"Map":       reflect.TypeOf(&m).Elem(),
		"Mutex":     reflect.TypeOf(&mutex).Elem(),
		"Locker":    reflect.TypeOf(&locker).Elem(),
		"WaitGroup": reflect.TypeOf(&waitGroup).Elem(),
		"RWMutex":   reflect.TypeOf(&rWMutex).Elem(),
	}
}

func initSyncAtomic() {
	env.Packages["sync/atomic"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
		"CompareAndSwapUint32":  reflect.ValueOf(atomic.CompareAndSwapUint32),
		"LoadUintptr":           reflect.ValueOf(atomic.LoadUintptr),
		"StoreUintptr":          reflect.ValueOf(atomic.StoreUintptr),
		"CompareAndSwapUint64":  reflect.ValueOf(atomic.CompareAndSwapUint64),
		"AddInt32":              reflect.ValueOf(atomic.AddInt32),
		"LoadUint64":            reflect.ValueOf(atomic.LoadUint64),
		"StoreUint64":           reflect.ValueOf(atomic.StoreUint64),
		"CompareAndSwapInt32":   reflect.ValueOf(atomic.CompareAndSwapInt32),
		"CompareAndSwapUintptr": reflect.ValueOf(atomic.CompareAndSwapUintptr),
		"StoreInt64":            reflect.ValueOf(atomic.StoreInt64),
		"SwapUint32":            reflect.ValueOf(atomic.SwapUint32),
		"CompareAndSwapInt64":   reflect.ValueOf(atomic.CompareAndSwapInt64),
		"StoreInt32":            reflect.ValueOf(atomic.StoreInt32),
		"LoadInt32":             reflect.ValueOf(atomic.LoadInt32),
		"StorePointer":          reflect.ValueOf(atomic.StorePointer),
		"AddInt64":              reflect.ValueOf(atomic.AddInt64),
		"AddUint64":             reflect.ValueOf(atomic.AddUint64),
		"AddUintptr":            reflect.ValueOf(atomic.AddUintptr),
		"SwapInt64":             reflect.ValueOf(atomic.SwapInt64),
		"LoadPointer":           reflect.ValueOf(atomic.LoadPointer),
		"LoadUint32":            reflect.ValueOf(atomic.LoadUint32),
		"SwapInt32":             reflect.ValueOf(atomic.SwapInt32),
		"SwapPointer":           reflect.ValueOf(atomic.SwapPointer),
		"LoadInt64":             reflect.ValueOf(atomic.LoadInt64),
		"AddUint32":             reflect.ValueOf(atomic.AddUint32),
		"StoreUint32":           reflect.ValueOf(atomic.StoreUint32),
		"SwapUint64":            reflect.ValueOf(atomic.SwapUint64),
		"SwapUintptr":           reflect.ValueOf(atomic.SwapUintptr),
		"CompareAndSwapPointer": reflect.ValueOf(atomic.CompareAndSwapPointer),
	}
	var (
		value atomic.Value
	)
	env.PackageTypes["sync/atomic"] = map[string]reflect.Type{
		"Value": reflect.TypeOf(&value).Elem(),
	}
}

func initTime() {
	env.Packages["time"] = map[string]reflect.Value{
		// define constants
		"Sunday":      reflect.ValueOf(time.Sunday),
		"Nanosecond":  reflect.ValueOf(time.Nanosecond),
		"Microsecond": reflect.ValueOf(time.Microsecond),
		"Millisecond": reflect.ValueOf(time.Millisecond),
		"RFC3339":     reflect.ValueOf(time.RFC3339),
		"StampMicro":  reflect.ValueOf(time.StampMicro),
		"November":    reflect.ValueOf(time.November),
		"Wednesday":   reflect.ValueOf(time.Wednesday),
		"Second":      reflect.ValueOf(time.Second),
		"RFC822Z":     reflect.ValueOf(time.RFC822Z),
		"UnixDate":    reflect.ValueOf(time.UnixDate),
		"RubyDate":    reflect.ValueOf(time.RubyDate),
		"RFC1123":     reflect.ValueOf(time.RFC1123),
		"RFC1123Z":    reflect.ValueOf(time.RFC1123Z),
		"RFC3339Nano": reflect.ValueOf(time.RFC3339Nano),
		"StampMilli":  reflect.ValueOf(time.StampMilli),
		"January":     reflect.ValueOf(time.January),
		"May":         reflect.ValueOf(time.May),
		"September":   reflect.ValueOf(time.September),
		"Monday":      reflect.ValueOf(time.Monday),
		"Minute":      reflect.ValueOf(time.Minute),
		"June":        reflect.ValueOf(time.June),
		"October":     reflect.ValueOf(time.October),
		"Tuesday":     reflect.ValueOf(time.Tuesday),
		"Thursday":    reflect.ValueOf(time.Thursday),
		"Hour":        reflect.ValueOf(time.Hour),
		"RFC850":      reflect.ValueOf(time.RFC850),
		"Kitchen":     reflect.ValueOf(time.Kitchen),
		"February":    reflect.ValueOf(time.February),
		"April":       reflect.ValueOf(time.April),
		"Saturday":    reflect.ValueOf(time.Saturday),
		"Stamp":       reflect.ValueOf(time.Stamp),
		"March":       reflect.ValueOf(time.March),
		"July":        reflect.ValueOf(time.July),
		"August":      reflect.ValueOf(time.August),
		"December":    reflect.ValueOf(time.December),
		"Friday":      reflect.ValueOf(time.Friday),
		"ANSIC":       reflect.ValueOf(time.ANSIC),
		"RFC822":      reflect.ValueOf(time.RFC822),
		"StampNano":   reflect.ValueOf(time.StampNano),

		// define variables
		"UTC":   reflect.ValueOf(time.UTC),
		"Local": reflect.ValueOf(time.Local),

		// define functions
		"ParseInLocation":        reflect.ValueOf(time.ParseInLocation),
		"NewTicker":              reflect.ValueOf(time.NewTicker),
		"Tick":                   reflect.ValueOf(time.Tick),
		"Unix":                   reflect.ValueOf(time.Unix),
		"FixedZone":              reflect.ValueOf(time.FixedZone),
		"LoadLocationFromTZData": reflect.ValueOf(time.LoadLocationFromTZData),
		"ParseDuration":          reflect.ValueOf(time.ParseDuration),
		"NewTimer":               reflect.ValueOf(time.NewTimer),
		"AfterFunc":              reflect.ValueOf(time.AfterFunc),
		"Since":                  reflect.ValueOf(time.Since),
		"Until":                  reflect.ValueOf(time.Until),
		"Now":                    reflect.ValueOf(time.Now),
		"Date":                   reflect.ValueOf(time.Date),
		"Parse":                  reflect.ValueOf(time.Parse),
		"After":                  reflect.ValueOf(time.After),
		"LoadLocation":           reflect.ValueOf(time.LoadLocation),
		"Sleep":                  reflect.ValueOf(time.Sleep),
	}
	var (
		weekday    time.Weekday
		duration   time.Duration
		location   time.Location
		parseError time.ParseError
		timer      time.Timer
		ticker     time.Ticker
		t          time.Time
		month      time.Month
	)
	env.PackageTypes["time"] = map[string]reflect.Type{
		"Weekday":    reflect.TypeOf(&weekday).Elem(),
		"Duration":   reflect.TypeOf(&duration).Elem(),
		"Location":   reflect.TypeOf(&location).Elem(),
		"ParseError": reflect.TypeOf(&parseError).Elem(),
		"Timer":      reflect.TypeOf(&timer).Elem(),
		"Ticker":     reflect.TypeOf(&ticker).Elem(),
		"Time":       reflect.TypeOf(&t).Elem(),
		"Month":      reflect.TypeOf(&month).Elem(),
	}
}

func initUnicode() {
	env.Packages["unicode"] = map[string]reflect.Value{
		// define constants
		"MaxRune":         reflect.ValueOf(unicode.MaxRune),
		"MaxASCII":        reflect.ValueOf(unicode.MaxASCII),
		"UpperCase":       reflect.ValueOf(unicode.UpperCase),
		"TitleCase":       reflect.ValueOf(unicode.TitleCase),
		"MaxCase":         reflect.ValueOf(unicode.MaxCase),
		"UpperLower":      reflect.ValueOf(unicode.UpperLower),
		"Version":         reflect.ValueOf(unicode.Version),
		"ReplacementChar": reflect.ValueOf(unicode.ReplacementChar),
		"MaxLatin1":       reflect.ValueOf(unicode.MaxLatin1),
		"LowerCase":       reflect.ValueOf(unicode.LowerCase),

		// define variables
		"Miao":                               reflect.ValueOf(unicode.Miao),
		"Phoenician":                         reflect.ValueOf(unicode.Phoenician),
		"Egyptian_Hieroglyphs":               reflect.ValueOf(unicode.Egyptian_Hieroglyphs),
		"Old_Turkic":                         reflect.ValueOf(unicode.Old_Turkic),
		"Thaana":                             reflect.ValueOf(unicode.Thaana),
		"Warang_Citi":                        reflect.ValueOf(unicode.Warang_Citi),
		"Imperial_Aramaic":                   reflect.ValueOf(unicode.Imperial_Aramaic),
		"Kannada":                            reflect.ValueOf(unicode.Kannada),
		"Saurashtra":                         reflect.ValueOf(unicode.Saurashtra),
		"Rejang":                             reflect.ValueOf(unicode.Rejang),
		"Wancho":                             reflect.ValueOf(unicode.Wancho),
		"Nl":                                 reflect.ValueOf(unicode.Nl),
		"Yi":                                 reflect.ValueOf(unicode.Yi),
		"Makasar":                            reflect.ValueOf(unicode.Makasar),
		"Samaritan":                          reflect.ValueOf(unicode.Samaritan),
		"Shavian":                            reflect.ValueOf(unicode.Shavian),
		"Tai_Viet":                           reflect.ValueOf(unicode.Tai_Viet),
		"Pe":                                 reflect.ValueOf(unicode.Pe),
		"Pi":                                 reflect.ValueOf(unicode.Pi),
		"Coptic":                             reflect.ValueOf(unicode.Coptic),
		"Lao":                                reflect.ValueOf(unicode.Lao),
		"Newa":                               reflect.ValueOf(unicode.Newa),
		"Extender":                           reflect.ValueOf(unicode.Extender),
		"Co":                                 reflect.ValueOf(unicode.Co),
		"Caucasian_Albanian":                 reflect.ValueOf(unicode.Caucasian_Albanian),
		"Pc":                                 reflect.ValueOf(unicode.Pc),
		"Cuneiform":                          reflect.ValueOf(unicode.Cuneiform),
		"Hanifi_Rohingya":                    reflect.ValueOf(unicode.Hanifi_Rohingya),
		"Mandaic":                            reflect.ValueOf(unicode.Mandaic),
		"Bidi_Control":                       reflect.ValueOf(unicode.Bidi_Control),
		"Syloti_Nagri":                       reflect.ValueOf(unicode.Syloti_Nagri),
		"Ideographic":                        reflect.ValueOf(unicode.Ideographic),
		"Other_ID_Start":                     reflect.ValueOf(unicode.Other_ID_Start),
		"L":                                  reflect.ValueOf(unicode.L),
		"Canadian_Aboriginal":                reflect.ValueOf(unicode.Canadian_Aboriginal),
		"Inscriptional_Parthian":             reflect.ValueOf(unicode.Inscriptional_Parthian),
		"Khudawadi":                          reflect.ValueOf(unicode.Khudawadi),
		"Nushu":                              reflect.ValueOf(unicode.Nushu),
		"SignWriting":                        reflect.ValueOf(unicode.SignWriting),
		"Bengali":                            reflect.ValueOf(unicode.Bengali),
		"Cham":                               reflect.ValueOf(unicode.Cham),
		"Deseret":                            reflect.ValueOf(unicode.Deseret),
		"Ogham":                              reflect.ValueOf(unicode.Ogham),
		"Unified_Ideograph":                  reflect.ValueOf(unicode.Unified_Ideograph),
		"Sundanese":                          reflect.ValueOf(unicode.Sundanese),
		"IDS_Trinary_Operator":               reflect.ValueOf(unicode.IDS_Trinary_Operator),
		"Mn":                                 reflect.ValueOf(unicode.Mn),
		"Zp":                                 reflect.ValueOf(unicode.Zp),
		"Cyrillic":                           reflect.ValueOf(unicode.Cyrillic),
		"Javanese":                           reflect.ValueOf(unicode.Javanese),
		"Linear_A":                           reflect.ValueOf(unicode.Linear_A),
		"Manichaean":                         reflect.ValueOf(unicode.Manichaean),
		"FoldCategory":                       reflect.ValueOf(unicode.FoldCategory),
		"Ps":                                 reflect.ValueOf(unicode.Ps),
		"Gurmukhi":                           reflect.ValueOf(unicode.Gurmukhi),
		"Linear_B":                           reflect.ValueOf(unicode.Linear_B),
		"Elbasan":                            reflect.ValueOf(unicode.Elbasan),
		"Medefaidrin":                        reflect.ValueOf(unicode.Medefaidrin),
		"Meroitic_Cursive":                   reflect.ValueOf(unicode.Meroitic_Cursive),
		"Siddham":                            reflect.ValueOf(unicode.Siddham),
		"Soyombo":                            reflect.ValueOf(unicode.Soyombo),
		"Other_Math":                         reflect.ValueOf(unicode.Other_Math),
		"N":                                  reflect.ValueOf(unicode.N),
		"Inscriptional_Pahlavi":              reflect.ValueOf(unicode.Inscriptional_Pahlavi),
		"Malayalam":                          reflect.ValueOf(unicode.Malayalam),
		"Old_Italic":                         reflect.ValueOf(unicode.Old_Italic),
		"Chakma":                             reflect.ValueOf(unicode.Chakma),
		"Mende_Kikakui":                      reflect.ValueOf(unicode.Mende_Kikakui),
		"Zanabazar_Square":                   reflect.ValueOf(unicode.Zanabazar_Square),
		"Lower":                              reflect.ValueOf(unicode.Lower),
		"Lycian":                             reflect.ValueOf(unicode.Lycian),
		"Ol_Chiki":                           reflect.ValueOf(unicode.Ol_Chiki),
		"Diacritic":                          reflect.ValueOf(unicode.Diacritic),
		"Quotation_Mark":                     reflect.ValueOf(unicode.Quotation_Mark),
		"Lu":                                 reflect.ValueOf(unicode.Lu),
		"Anatolian_Hieroglyphs":              reflect.ValueOf(unicode.Anatolian_Hieroglyphs),
		"Avestan":                            reflect.ValueOf(unicode.Avestan),
		"Braille":                            reflect.ValueOf(unicode.Braille),
		"Limbu":                              reflect.ValueOf(unicode.Limbu),
		"Other_Uppercase":                    reflect.ValueOf(unicode.Other_Uppercase),
		"Mc":                                 reflect.ValueOf(unicode.Mc),
		"Zl":                                 reflect.ValueOf(unicode.Zl),
		"Old_Hungarian":                      reflect.ValueOf(unicode.Old_Hungarian),
		"Palmyrene":                          reflect.ValueOf(unicode.Palmyrene),
		"Properties":                         reflect.ValueOf(unicode.Properties),
		"Zs":                                 reflect.ValueOf(unicode.Zs),
		"Bamum":                              reflect.ValueOf(unicode.Bamum),
		"Cherokee":                           reflect.ValueOf(unicode.Cherokee),
		"Hebrew":                             reflect.ValueOf(unicode.Hebrew),
		"Katakana":                           reflect.ValueOf(unicode.Katakana),
		"Lepcha":                             reflect.ValueOf(unicode.Lepcha),
		"Logical_Order_Exception":            reflect.ValueOf(unicode.Logical_Order_Exception),
		"Other_Grapheme_Extend":              reflect.ValueOf(unicode.Other_Grapheme_Extend),
		"AzeriCase":                          reflect.ValueOf(unicode.AzeriCase),
		"Gothic":                             reflect.ValueOf(unicode.Gothic),
		"Gujarati":                           reflect.ValueOf(unicode.Gujarati),
		"Old_Permic":                         reflect.ValueOf(unicode.Old_Permic),
		"Nyiakeng_Puachue_Hmong":             reflect.ValueOf(unicode.Nyiakeng_Puachue_Hmong),
		"Pattern_White_Space":                reflect.ValueOf(unicode.Pattern_White_Space),
		"Soft_Dotted":                        reflect.ValueOf(unicode.Soft_Dotted),
		"CaseRanges":                         reflect.ValueOf(unicode.CaseRanges),
		"Cs":                                 reflect.ValueOf(unicode.Cs),
		"No":                                 reflect.ValueOf(unicode.No),
		"Devanagari":                         reflect.ValueOf(unicode.Devanagari),
		"Georgian":                           reflect.ValueOf(unicode.Georgian),
		"Tibetan":                            reflect.ValueOf(unicode.Tibetan),
		"Other_Default_Ignorable_Code_Point": reflect.ValueOf(unicode.Other_Default_Ignorable_Code_Point),
		"Grantha":                            reflect.ValueOf(unicode.Grantha),
		"Lydian":                             reflect.ValueOf(unicode.Lydian),
		"Nd":                                 reflect.ValueOf(unicode.Nd),
		"C":                                  reflect.ValueOf(unicode.C),
		"Title":                              reflect.ValueOf(unicode.Title),
		"Buhid":                              reflect.ValueOf(unicode.Buhid),
		"Gunjala_Gondi":                      reflect.ValueOf(unicode.Gunjala_Gondi),
		"Osmanya":                            reflect.ValueOf(unicode.Osmanya),
		"Syriac":                             reflect.ValueOf(unicode.Syriac),
		"Tangut":                             reflect.ValueOf(unicode.Tangut),
		"Letter":                             reflect.ValueOf(unicode.Letter),
		"Mark":                               reflect.ValueOf(unicode.Mark),
		"Join_Control":                       reflect.ValueOf(unicode.Join_Control),
		"Sm":                                 reflect.ValueOf(unicode.Sm),
		"Duployan":                           reflect.ValueOf(unicode.Duployan),
		"Nandinagari":                        reflect.ValueOf(unicode.Nandinagari),
		"Takri":                              reflect.ValueOf(unicode.Takri),
		"Telugu":                             reflect.ValueOf(unicode.Telugu),
		"New_Tai_Lue":                        reflect.ValueOf(unicode.New_Tai_Lue),
		"Masaram_Gondi":                      reflect.ValueOf(unicode.Masaram_Gondi),
		"Phags_Pa":                           reflect.ValueOf(unicode.Phags_Pa),
		"ASCII_Hex_Digit":                    reflect.ValueOf(unicode.ASCII_Hex_Digit),
		"Ethiopic":                           reflect.ValueOf(unicode.Ethiopic),
		"Inherited":                          reflect.ValueOf(unicode.Inherited),
		"Khojki":                             reflect.ValueOf(unicode.Khojki),
		"Sharada":                            reflect.ValueOf(unicode.Sharada),
		"Terminal_Punctuation":               reflect.ValueOf(unicode.Terminal_Punctuation),
		"Pd":                                 reflect.ValueOf(unicode.Pd),
		"Lisu":                               reflect.ValueOf(unicode.Lisu),
		"Meetei_Mayek":                       reflect.ValueOf(unicode.Meetei_Mayek),
		"Old_Sogdian":                        reflect.ValueOf(unicode.Old_Sogdian),
		"Pau_Cin_Hau":                        reflect.ValueOf(unicode.Pau_Cin_Hau),
		"Runic":                              reflect.ValueOf(unicode.Runic),
		"Regional_Indicator":                 reflect.ValueOf(unicode.Regional_Indicator),
		"Other":                              reflect.ValueOf(unicode.Other),
		"Pf":                                 reflect.ValueOf(unicode.Pf),
		"Bhaiksuki":                          reflect.ValueOf(unicode.Bhaiksuki),
		"Khmer":                              reflect.ValueOf(unicode.Khmer),
		"Tamil":                              reflect.ValueOf(unicode.Tamil),
		"Hex_Digit":                          reflect.ValueOf(unicode.Hex_Digit),
		"Categories":                         reflect.ValueOf(unicode.Categories),
		"Nko":                                reflect.ValueOf(unicode.Nko),
		"Vai":                                reflect.ValueOf(unicode.Vai),
		"Carian":                             reflect.ValueOf(unicode.Carian),
		"Greek":                              reflect.ValueOf(unicode.Greek),
		"Multani":                            reflect.ValueOf(unicode.Multani),
		"Osage":                              reflect.ValueOf(unicode.Osage),
		"M":                                  reflect.ValueOf(unicode.M),
		"Han":                                reflect.ValueOf(unicode.Han),
		"Deprecated":                         reflect.ValueOf(unicode.Deprecated),
		"Thai":                               reflect.ValueOf(unicode.Thai),
		"Other_Lowercase":                    reflect.ValueOf(unicode.Other_Lowercase),
		"Prepended_Concatenation_Mark":       reflect.ValueOf(unicode.Prepended_Concatenation_Mark),
		"Tifinagh":                           reflect.ValueOf(unicode.Tifinagh),
		"FoldScript":                         reflect.ValueOf(unicode.FoldScript),
		"Digit":                              reflect.ValueOf(unicode.Digit),
		"Po":                                 reflect.ValueOf(unicode.Po),
		"Upper":                              reflect.ValueOf(unicode.Upper),
		"Kharoshthi":                         reflect.ValueOf(unicode.Kharoshthi),
		"Psalter_Pahlavi":                    reflect.ValueOf(unicode.Psalter_Pahlavi),
		"Sinhala":                            reflect.ValueOf(unicode.Sinhala),
		"Punct":                              reflect.ValueOf(unicode.Punct),
		"Armenian":                           reflect.ValueOf(unicode.Armenian),
		"Elymaic":                            reflect.ValueOf(unicode.Elymaic),
		"TurkishCase":                        reflect.ValueOf(unicode.TurkishCase),
		"Hatran":                             reflect.ValueOf(unicode.Hatran),
		"Tagbanwa":                           reflect.ValueOf(unicode.Tagbanwa),
		"Radical":                            reflect.ValueOf(unicode.Radical),
		"STerm":                              reflect.ValueOf(unicode.STerm),
		"Sora_Sompeng":                       reflect.ValueOf(unicode.Sora_Sompeng),
		"Tagalog":                            reflect.ValueOf(unicode.Tagalog),
		"Dogra":                              reflect.ValueOf(unicode.Dogra),
		"Marchen":                            reflect.ValueOf(unicode.Marchen),
		"Tai_Le":                             reflect.ValueOf(unicode.Tai_Le),
		"Tirhuta":                            reflect.ValueOf(unicode.Tirhuta),
		"Ll":                                 reflect.ValueOf(unicode.Ll),
		"Sk":                                 reflect.ValueOf(unicode.Sk),
		"Symbol":                             reflect.ValueOf(unicode.Symbol),
		"Mro":                                reflect.ValueOf(unicode.Mro),
		"IDS_Binary_Operator":                reflect.ValueOf(unicode.IDS_Binary_Operator),
		"GraphicRanges":                      reflect.ValueOf(unicode.GraphicRanges),
		"Me":                                 reflect.ValueOf(unicode.Me),
		"Scripts":                            reflect.ValueOf(unicode.Scripts),
		"Bopomofo":                           reflect.ValueOf(unicode.Bopomofo),
		"Latin":                              reflect.ValueOf(unicode.Latin),
		"Hyphen":                             reflect.ValueOf(unicode.Hyphen),
		"Noncharacter_Code_Point":            reflect.ValueOf(unicode.Noncharacter_Code_Point),
		"Space":                              reflect.ValueOf(unicode.Space),
		"Lt":                                 reflect.ValueOf(unicode.Lt),
		"Adlam":                              reflect.ValueOf(unicode.Adlam),
		"Hiragana":                           reflect.ValueOf(unicode.Hiragana),
		"Mongolian":                          reflect.ValueOf(unicode.Mongolian),
		"Cc":                                 reflect.ValueOf(unicode.Cc),
		"P":                                  reflect.ValueOf(unicode.P),
		"Common":                             reflect.ValueOf(unicode.Common),
		"Ugaritic":                           reflect.ValueOf(unicode.Ugaritic),
		"White_Space":                        reflect.ValueOf(unicode.White_Space),
		"Pahawh_Hmong":                       reflect.ValueOf(unicode.Pahawh_Hmong),
		"Sogdian":                            reflect.ValueOf(unicode.Sogdian),
		"Lo":                                 reflect.ValueOf(unicode.Lo),
		"Number":                             reflect.ValueOf(unicode.Number),
		"Hangul":                             reflect.ValueOf(unicode.Hangul),
		"Old_North_Arabian":                  reflect.ValueOf(unicode.Old_North_Arabian),
		"Variation_Selector":                 reflect.ValueOf(unicode.Variation_Selector),
		"Sc":                                 reflect.ValueOf(unicode.Sc),
		"Kaithi":                             reflect.ValueOf(unicode.Kaithi),
		"Batak":                              reflect.ValueOf(unicode.Batak),
		"Glagolitic":                         reflect.ValueOf(unicode.Glagolitic),
		"Myanmar":                            reflect.ValueOf(unicode.Myanmar),
		"Other_ID_Continue":                  reflect.ValueOf(unicode.Other_ID_Continue),
		"Sentence_Terminal":                  reflect.ValueOf(unicode.Sentence_Terminal),
		"Z":                                  reflect.ValueOf(unicode.Z),
		"Hanunoo":                            reflect.ValueOf(unicode.Hanunoo),
		"Old_South_Arabian":                  reflect.ValueOf(unicode.Old_South_Arabian),
		"Cf":                                 reflect.ValueOf(unicode.Cf),
		"Lm":                                 reflect.ValueOf(unicode.Lm),
		"So":                                 reflect.ValueOf(unicode.So),
		"Cypriot":                            reflect.ValueOf(unicode.Cypriot),
		"Kayah_Li":                           reflect.ValueOf(unicode.Kayah_Li),
		"Nabataean":                          reflect.ValueOf(unicode.Nabataean),
		"S":                                  reflect.ValueOf(unicode.S),
		"Dash":                               reflect.ValueOf(unicode.Dash),
		"Buginese":                           reflect.ValueOf(unicode.Buginese),
		"Oriya":                              reflect.ValueOf(unicode.Oriya),
		"Bassa_Vah":                          reflect.ValueOf(unicode.Bassa_Vah),
		"Other_Alphabetic":                   reflect.ValueOf(unicode.Other_Alphabetic),
		"Pattern_Syntax":                     reflect.ValueOf(unicode.Pattern_Syntax),
		"Brahmi":                             reflect.ValueOf(unicode.Brahmi),
		"Meroitic_Hieroglyphs":               reflect.ValueOf(unicode.Meroitic_Hieroglyphs),
		"Modi":                               reflect.ValueOf(unicode.Modi),
		"Old_Persian":                        reflect.ValueOf(unicode.Old_Persian),
		"Tai_Tham":                           reflect.ValueOf(unicode.Tai_Tham),
		"PrintRanges":                        reflect.ValueOf(unicode.PrintRanges),
		"Ahom":                               reflect.ValueOf(unicode.Ahom),
		"Arabic":                             reflect.ValueOf(unicode.Arabic),
		"Balinese":                           reflect.ValueOf(unicode.Balinese),
		"Mahajani":                           reflect.ValueOf(unicode.Mahajani),

		// define functions
		"IsGraphic":  reflect.ValueOf(unicode.IsGraphic),
		"IsPrint":    reflect.ValueOf(unicode.IsPrint),
		"IsNumber":   reflect.ValueOf(unicode.IsNumber),
		"IsSpace":    reflect.ValueOf(unicode.IsSpace),
		"IsTitle":    reflect.ValueOf(unicode.IsTitle),
		"ToTitle":    reflect.ValueOf(unicode.ToTitle),
		"IsMark":     reflect.ValueOf(unicode.IsMark),
		"IsSymbol":   reflect.ValueOf(unicode.IsSymbol),
		"IsLower":    reflect.ValueOf(unicode.IsLower),
		"To":         reflect.ValueOf(unicode.To),
		"ToUpper":    reflect.ValueOf(unicode.ToUpper),
		"In":         reflect.ValueOf(unicode.In),
		"IsControl":  reflect.ValueOf(unicode.IsControl),
		"IsLetter":   reflect.ValueOf(unicode.IsLetter),
		"Is":         reflect.ValueOf(unicode.Is),
		"ToLower":    reflect.ValueOf(unicode.ToLower),
		"IsDigit":    reflect.ValueOf(unicode.IsDigit),
		"IsOneOf":    reflect.ValueOf(unicode.IsOneOf),
		"IsPunct":    reflect.ValueOf(unicode.IsPunct),
		"IsUpper":    reflect.ValueOf(unicode.IsUpper),
		"SimpleFold": reflect.ValueOf(unicode.SimpleFold),
	}
	var (
		rangeTable  unicode.RangeTable
		range16     unicode.Range16
		range32     unicode.Range32
		caseRange   unicode.CaseRange
		specialCase unicode.SpecialCase
	)
	env.PackageTypes["unicode"] = map[string]reflect.Type{
		"RangeTable":  reflect.TypeOf(&rangeTable).Elem(),
		"Range16":     reflect.TypeOf(&range16).Elem(),
		"Range32":     reflect.TypeOf(&range32).Elem(),
		"CaseRange":   reflect.TypeOf(&caseRange).Elem(),
		"SpecialCase": reflect.TypeOf(&specialCase).Elem(),
	}
}

func initUnicodeUTF8() {
	env.Packages["unicode/utf8"] = map[string]reflect.Value{
		// define constants
		"RuneError": reflect.ValueOf(utf8.RuneError),
		"RuneSelf":  reflect.ValueOf(utf8.RuneSelf),
		"MaxRune":   reflect.ValueOf(utf8.MaxRune),
		"UTFMax":    reflect.ValueOf(utf8.UTFMax),

		// define variables

		// define functions
		"RuneCountInString":      reflect.ValueOf(utf8.RuneCountInString),
		"ValidString":            reflect.ValueOf(utf8.ValidString),
		"ValidRune":              reflect.ValueOf(utf8.ValidRune),
		"RuneLen":                reflect.ValueOf(utf8.RuneLen),
		"DecodeRuneInString":     reflect.ValueOf(utf8.DecodeRuneInString),
		"DecodeLastRuneInString": reflect.ValueOf(utf8.DecodeLastRuneInString),
		"RuneStart":              reflect.ValueOf(utf8.RuneStart),
		"DecodeRune":             reflect.ValueOf(utf8.DecodeRune),
		"DecodeLastRune":         reflect.ValueOf(utf8.DecodeLastRune),
		"RuneCount":              reflect.ValueOf(utf8.RuneCount),
		"FullRuneInString":       reflect.ValueOf(utf8.FullRuneInString),
		"EncodeRune":             reflect.ValueOf(utf8.EncodeRune),
		"Valid":                  reflect.ValueOf(utf8.Valid),
		"FullRune":               reflect.ValueOf(utf8.FullRune),
	}
	var ()
	env.PackageTypes["unicode/utf8"] = map[string]reflect.Type{}
}

func initUnicodeUTF16() {
	env.Packages["unicode/utf16"] = map[string]reflect.Value{
		// define constants

		// define variables

		// define functions
		"Decode":      reflect.ValueOf(utf16.Decode),
		"IsSurrogate": reflect.ValueOf(utf16.IsSurrogate),
		"DecodeRune":  reflect.ValueOf(utf16.DecodeRune),
		"EncodeRune":  reflect.ValueOf(utf16.EncodeRune),
		"Encode":      reflect.ValueOf(utf16.Encode),
	}
	var ()
	env.PackageTypes["unicode/utf16"] = map[string]reflect.Type{}
}
