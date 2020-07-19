package donut

import (
	"bytes"
	"encoding/binary"
	"io"

	"github.com/google/uuid"
)

// about entropy level
const (
	EntropyNone = 1 // don't use any entropy
	// EntropyRandom  = 2 // use random names
	EntropyDefault = 3 // use random names + symmetric encryption
)

// about maru
const (
	maruMaxStr = 64
	maruBlkLen = 16
	// maruHashLen = 8
	maruIVLen = 8
)

// about config, module and module
const (
	// maxParam      = 8 // maximum number of parameters passed to method
	maxName = 256
	// maxDLL        = 8 // maximum number of DLL supported by instance
	maxURL        = 256
	maxModuleName = 8
	signatureLen  = 8 // 64-bit string to verify decryption ok
	// versionLen    = 32
	domainLen = 8
	// runtimeNET4   = "v4.0.30319"
)

// about DLL
const (
	ntDLL    = "ntdll.dll"
	kernel32 = "kernel32.dll"
	shell32  = "shell32.dll"
	// advAPI32 = "advapi32.dll"
	// crypt32  = "crypt32.dll"
	msCoreE  = "mscoree.dll"
	ole32    = "ole32.dll"
	oleaut32 = "oleaut32.dll"
	winiNet  = "wininet.dll"
	// comBase  = "combase.dll"
	// user32   = "user32.dll"
	// shLwAPI  = "shlwapi.dll"
)

// Arch - CPU architecture type (32, 64, or 32+64)
type Arch int

const (
	// X32 - 32bit
	X32 Arch = iota
	// X64 - 64 bit
	X64
	// X84 - 32+64 bit
	X84
)

// ModuleType is input module type
type ModuleType int

// about module types
const (
	ModuleNETDLL ModuleType = 1 // .NET DLL. Requires class and method
	ModuleNETEXE            = 2 // .NET EXE. Executes Main if no class and method provided
	ModuleDLL               = 3 // Unmanaged DLL, function is optional
	ModuleEXE               = 4 // Unmanaged EXE
	ModuleVBS               = 5 // VBScript
	ModuleJS                = 6 // JavaScript or JScript
	ModuleXSL               = 7 // XSL with JavaScript/JScript or VBScript embedded
)

// InstanceType is input instance type
type InstanceType int

// about instance type
const (
	InstancePIC InstanceType = 1 // Self-contained
	InstanceURL              = 2 // Download from remote server
)

// Config contains configuration about donut.
type Config struct {
	Arch       Arch
	Type       ModuleType
	InstType   InstanceType
	Parameters string // separated by , or ;

	Entropy    uint32
	DotNetMode bool

	// new in 0.9.3
	Thread   uint32
	Compress uint32
	Unicode  uint32
	OEP      uint64
	ExitOpt  uint32
	Format   uint32

	Domain  string // .NET stuff
	Class   string
	Method  string // Used by Native DLL and .NET DLL
	Runtime string
	Bypass  int

	Module     *Module
	ModuleName string
	URL        string
	ModuleMac  uint64
	ModuleData *bytes.Buffer

	inst    *Instance
	instLen uint32
}

// Module is the donut module.
type Module struct {
	ModType  uint32 // EXE, DLL, JS, VBS, XSL
	Thread   uint32 // run entrypoint of unmanaged EXE as a thread
	Compress uint32 // indicates engine used for compression

	Runtime [maxName]byte // runtime version for .NET EXE/DLL (donut max name = 256)
	Domain  [maxName]byte // domain name to use for .NET EXE/DLL
	Cls     [maxName]byte // name of class and optional namespace for .NET EXE/DLL
	Method  [maxName]byte // name of method to invoke for .NET DLL or api for unmanaged DLL
	Param   [maxName]byte // string parameters for DLL/EXE (donut max param = 8)

	Unicode uint32             // convert command line to unicode for unmanaged DLL function
	Sig     [signatureLen]byte // random string to verify decryption
	Mac     uint64             // to verify decryption was ok
	ZLen    uint32             // compressed size of EXE/DLL/JS/VBS file
	Len     uint32             // size of EXE/DLL/XSL/JS/VBS file
	Data    [4]byte            // data of EXE/DLL/XSL/JS/VBS file
}

func writeField(w *bytes.Buffer, _ string, i interface{}) {
	_ = binary.Write(w, binary.LittleEndian, i)

}

func (mod *Module) writeTo(w *bytes.Buffer) {
	writeField(w, "ModType", mod.ModType)
	writeField(w, "Thread", mod.Thread)
	writeField(w, "Compress", mod.Compress)

	writeField(w, "Runtime", mod.Runtime)
	writeField(w, "Domain", mod.Domain)
	writeField(w, "CLS", mod.Cls)
	writeField(w, "Method", mod.Method)
	writeField(w, "Param", mod.Param)

	writeField(w, "Unicode", mod.Unicode)
	w.Write(mod.Sig[:signatureLen])
	writeField(w, "Mac", mod.Mac)
	writeField(w, "Zlen", mod.ZLen)
	writeField(w, "Len", mod.Len)
}

// Instance is the donut instance.
type Instance struct {
	Len uint32 // total size of instance

	// Key  DonutCrypt // decrypts instance (32 bytes total = 16+16)
	KeyMk  [cipherKeyLen]byte   // master key
	KeyCtr [cipherBlockLen]byte // counter + nonce

	Iv   uint64     // the 64-bit initial value for maru hash
	Hash [64]uint64 // holds up to 64 api hashes/addrs {api}

	ExitOpt uint32 // call RtlExitUserProcess to terminate the host process
	Entropy uint32 // indicates entropy option
	OEP     uint64 // original entrypoint

	// everything from here is encrypted
	APICount uint32        // the 64-bit hashes of API required for instance to work
	DLLNames [maxName]byte // a list of DLL strings to load, separated by semi-colon

	DataName   [8]byte  // ".data"
	KernelBase [12]byte // "kernelBase"
	AMSI       [8]byte  // "amsi"
	Clr        [4]byte  // clr
	WLDP       [8]byte  // wldp

	CmdSyms [maxName]byte // symbols related to command line
	ExitAPI [maxName]byte // exit-related API

	Bypass         uint32   // indicates behaviour of bypassing AMSI/WLDP
	WldpQuery      [32]byte // WldpQueryDynamicCodeTrust
	WldpIsApproved [32]byte // WldpIsClassInApprovedList
	AmsiInit       [16]byte // AmsiInitialize
	AmsiScanBuf    [16]byte // AmsiScanBuffer
	AmsiScanStr    [16]byte // AmsiScanString

	WScript    [8]byte  // WScript
	WScriptEXE [12]byte // wscript.exe

	XIIDIUnknown  uuid.UUID
	XIIDIDispatch uuid.UUID

	//  GUID required to load .NET assemblies
	XCLSIDCLRMetaHost    uuid.UUID
	XIIDICLRMetaHost     uuid.UUID
	XIIDICLRRuntimeInfo  uuid.UUID
	XCLSIDCorRuntimeHost uuid.UUID
	XIIDICorRuntimeHost  uuid.UUID
	XIIDAppDomain        uuid.UUID

	//  GUID required to run VBS and JS files
	XCLSIDScriptLanguage        uuid.UUID // vbs or js
	XIIDIHost                   uuid.UUID // wscript object
	XIIDIActiveScript           uuid.UUID // engine
	XIIDIActiveScriptSite       uuid.UUID // implementation
	XIIDIActiveScriptSiteWindow uuid.UUID // basic GUI stuff
	XIIDIActiveScriptParse32    uuid.UUID // parser
	XIIDIActiveScriptParse64    uuid.UUID

	Type uint32 // InstancePIC or InstanceURL

	URL [maxURL]byte // staging server hosting donut module
	Req [8]byte      // just a buffer for "GET"

	Sig [maxName]byte // string to hash
	Mac uint64        // to verify decryption ok

	ModKeyMk  [cipherKeyLen]byte   // master key
	ModKeyCtr [cipherBlockLen]byte // counter + nonce

	ModuleLen uint64 // total size of module
}

func (inst *Instance) writeTo(w *bytes.Buffer) {
	// start := w.Len()
	writeField(w, "Len", inst.Len)
	writeField(w, "KeyMk", inst.KeyMk)
	writeField(w, "KeyCtr", inst.KeyCtr)
	for i := 0; i < 4; i++ { // padding to 8-byte alignment after 4 byte field
		w.WriteByte(0)
	}
	writeField(w, "Iv", inst.Iv)
	writeField(w, "Hash", inst.Hash)
	writeField(w, "ExitOpt", inst.ExitOpt)
	writeField(w, "Entropy", inst.Entropy)
	writeField(w, "OEP", inst.OEP)

	writeField(w, "ApiCount", inst.APICount)
	writeField(w, "DllNames", inst.DLLNames)

	writeField(w, "Dataname", inst.DataName)
	writeField(w, "Kernelbase", inst.KernelBase)
	writeField(w, "Amsi", inst.AMSI)
	writeField(w, "Clr", inst.Clr)
	writeField(w, "Wldp", inst.WLDP)

	writeField(w, "CmdSyms", inst.CmdSyms)
	writeField(w, "ExitApi", inst.ExitAPI)

	writeField(w, "Bypass", inst.Bypass)
	writeField(w, "WldpQuery", inst.WldpQuery)
	writeField(w, "WldpIsApproved", inst.WldpIsApproved)
	writeField(w, "AmsiInit", inst.AmsiInit)
	writeField(w, "AmsiScanBuf", inst.AmsiScanBuf)
	writeField(w, "AmsiScanStr", inst.AmsiScanStr)

	writeField(w, "Wscript", inst.WScript)
	writeField(w, "WscriptExe", inst.WScriptEXE)

	swapUUID(w, inst.XIIDIUnknown)
	swapUUID(w, inst.XIIDIDispatch)

	swapUUID(w, inst.XCLSIDCLRMetaHost)
	swapUUID(w, inst.XIIDICLRMetaHost)
	swapUUID(w, inst.XIIDICLRRuntimeInfo)
	swapUUID(w, inst.XCLSIDCorRuntimeHost)
	swapUUID(w, inst.XIIDICorRuntimeHost)
	swapUUID(w, inst.XIIDAppDomain)

	swapUUID(w, inst.XCLSIDScriptLanguage)
	swapUUID(w, inst.XIIDIHost)
	swapUUID(w, inst.XIIDIActiveScript)
	swapUUID(w, inst.XIIDIActiveScriptSite)
	swapUUID(w, inst.XIIDIActiveScriptSiteWindow)
	swapUUID(w, inst.XIIDIActiveScriptParse32)
	swapUUID(w, inst.XIIDIActiveScriptParse64)

	writeField(w, "Type", inst.Type)
	writeField(w, "Url", inst.URL)
	writeField(w, "Req", inst.Req)
	writeField(w, "Sig", inst.Sig)
	writeField(w, "Mac", inst.Mac)
	writeField(w, "ModKeyMk", inst.ModKeyMk)
	writeField(w, "ModKeCtr", inst.ModKeyCtr)
	writeField(w, "Mod_len", inst.ModuleLen)
}

type apiImport struct {
	Module string
	Name   string
}

var apiImports = []apiImport{
	{Module: kernel32, Name: "LoadLibraryA"}, // 0
	{Module: kernel32, Name: "GetProcAddress"},
	{Module: kernel32, Name: "GetModuleHandleA"},
	{Module: kernel32, Name: "VirtualAlloc"},
	{Module: kernel32, Name: "VirtualFree"},
	{Module: kernel32, Name: "VirtualQuery"}, // 5
	{Module: kernel32, Name: "VirtualProtect"},
	{Module: kernel32, Name: "Sleep"},
	{Module: kernel32, Name: "MultiByteToWideChar"},
	{Module: kernel32, Name: "GetUserDefaultLCID"},
	{Module: kernel32, Name: "WaitForSingleObject"}, // 10
	{Module: kernel32, Name: "CreateThread"},
	{Module: kernel32, Name: "GetThreadContext"},
	{Module: kernel32, Name: "GetCurrentThread"},
	{Module: kernel32, Name: "GetCommandLineA"},
	{Module: kernel32, Name: "GetCommandLineW"}, // 15

	{Module: shell32, Name: "CommandLineToArgvW"},

	{Module: oleaut32, Name: "SafeArrayCreate"},
	{Module: oleaut32, Name: "SafeArrayCreateVector"},
	{Module: oleaut32, Name: "SafeArrayPutElement"},
	{Module: oleaut32, Name: "SafeArrayDestroy"}, // 20
	{Module: oleaut32, Name: "SafeArrayGetLBound"},
	{Module: oleaut32, Name: "SafeArrayGetUBound"},
	{Module: oleaut32, Name: "SysAllocString"},
	{Module: oleaut32, Name: "SysFreeString"},
	{Module: oleaut32, Name: "LoadTypeLib"}, // 25

	{Module: winiNet, Name: "InternetCrackUrlA"},
	{Module: winiNet, Name: "InternetOpenA"},
	{Module: winiNet, Name: "InternetConnectA"},
	{Module: winiNet, Name: "InternetSetOptionA"},
	{Module: winiNet, Name: "InternetReadFile"}, // 30
	{Module: winiNet, Name: "InternetCloseHandle"},
	{Module: winiNet, Name: "HttpOpenRequestA"},
	{Module: winiNet, Name: "HttpSendRequestA"},
	{Module: winiNet, Name: "HttpQueryInfoA"},

	{Module: msCoreE, Name: "CorBindToRuntime"}, // 35
	{Module: msCoreE, Name: "CLRCreateInstance"},

	{Module: ole32, Name: "CoInitializeEx"},
	{Module: ole32, Name: "CoCreateInstance"},
	{Module: ole32, Name: "CoUninitialize"},

	{Module: ntDLL, Name: "RtlEqualUnicodeString"}, // 40
	{Module: ntDLL, Name: "RtlEqualString"},
	{Module: ntDLL, Name: "RtlUnicodeStringToAnsiString"},
	{Module: ntDLL, Name: "RtlInitUnicodeString"},
	{Module: ntDLL, Name: "RtlExitUserThread"},
	{Module: ntDLL, Name: "RtlExitUserProcess"}, // 45
	{Module: ntDLL, Name: "RtlCreateUnicodeString"},
	{Module: ntDLL, Name: "RtlGetCompressionWorkSpaceSize"},
	{Module: ntDLL, Name: "RtlDecompressBuffer"},
	{Module: ntDLL, Name: "NtContinue"},

	{Module: kernel32, Name: "AddVectoredExceptionHandler"}, // 50
	{Module: kernel32, Name: "RemoveVectoredExceptionHandler"},
}

// required to load .NET assemblies
// the first 6 bytes of these were int32+int16, need to be swapped on write
var (
	xCLSIDCorRuntimeHost = uuid.UUID{
		0xcb, 0x2f, 0x67, 0x23, 0xab, 0x3a, 0x11, 0xd2,
		0x9c, 0x40, 0x00, 0xc0, 0x4f, 0xa3, 0x0a, 0x3e,
	}

	xIIDICorRuntimeHost = uuid.UUID{
		0xcb, 0x2f, 0x67, 0x22, 0xab, 0x3a, 0x11, 0xd2,
		0x9c, 0x40, 0x00, 0xc0, 0x4f, 0xa3, 0x0a, 0x3e,
	}

	xCLSIDCLRMetaHost = uuid.UUID{
		0x92, 0x80, 0x18, 0x8d, 0x0e, 0x8e, 0x48, 0x67,
		0xb3, 0x0c, 0x7f, 0xa8, 0x38, 0x84, 0xe8, 0xde,
	}

	xIIDICLRMetaHost = uuid.UUID{
		0xD3, 0x32, 0xDB, 0x9E, 0xB9, 0xB3, 0x41, 0x25,
		0x82, 0x07, 0xA1, 0x48, 0x84, 0xF5, 0x32, 0x16,
	}

	xIIDICLRRuntimeInfo = uuid.UUID{
		0xBD, 0x39, 0xD1, 0xD2, 0xBA, 0x2F, 0x48, 0x6a,
		0x89, 0xB0, 0xB4, 0xB0, 0xCB, 0x46, 0x68, 0x91,
	}

	xIIDAppDomain = uuid.UUID{
		0x05, 0xF6, 0x96, 0xDC, 0x2B, 0x29, 0x36, 0x63,
		0xAD, 0x8B, 0xC4, 0x38, 0x9C, 0xF2, 0xA7, 0x13,
	}

	// required to load VBS and JS files
	xIIDIUnknown = uuid.UUID{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46,
	}

	xIIDIDispatch = uuid.UUID{
		0x00, 0x02, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46,
	}

	xIIDIHost = uuid.UUID{
		0x91, 0xaf, 0xbd, 0x1b, 0x5f, 0xeb, 0x43, 0xf5,
		0xb0, 0x28, 0xe2, 0xca, 0x96, 0x06, 0x17, 0xec,
	}

	xIIDIActiveScript = uuid.UUID{
		0xbb, 0x1a, 0x2a, 0xe1, 0xa4, 0xf9, 0x11, 0xcf,
		0x8f, 0x20, 0x00, 0x80, 0x5f, 0x2c, 0xd0, 0x64,
	}

	xIIDIActiveScriptSite = uuid.UUID{
		0xdb, 0x01, 0xa1, 0xe3, 0xa4, 0x2b, 0x11, 0xcf,
		0x8f, 0x20, 0x00, 0x80, 0x5f, 0x2c, 0xd0, 0x64,
	}

	xIIDIActiveScriptSiteWindow = uuid.UUID{
		0xd1, 0x0f, 0x67, 0x61, 0x83, 0xe9, 0x11, 0xcf,
		0x8f, 0x20, 0x00, 0x80, 0x5f, 0x2c, 0xd0, 0x64,
	}

	xIIDIActiveScriptParse32 = uuid.UUID{
		0xbb, 0x1a, 0x2a, 0xe2, 0xa4, 0xf9, 0x11, 0xcf,
		0x8f, 0x20, 0x00, 0x80, 0x5f, 0x2c, 0xd0, 0x64,
	}

	xIIDIActiveScriptParse64 = uuid.UUID{
		0xc7, 0xef, 0x76, 0x58, 0xe1, 0xee, 0x48, 0x0e,
		0x97, 0xea, 0xd5, 0x2c, 0xb4, 0xd7, 0x6d, 0x17,
	}

	xCLSIDVBScript = uuid.UUID{
		0xB5, 0x4F, 0x37, 0x41, 0x5B, 0x07, 0x11, 0xcf,
		0xA4, 0xB0, 0x00, 0xAA, 0x00, 0x4A, 0x55, 0xE8,
	}

	xCLSIDJScript = uuid.UUID{
		0xF4, 0x14, 0xC2, 0x60, 0x6A, 0xC0, 0x11, 0xCF,
		0xB6, 0xD1, 0x00, 0xAA, 0x00, 0xBB, 0xBB, 0x58,
	}

	// required to load XSL files
	// xCLSID_DOMDocument30 = uuid.UUID{
	// 	0xf5, 0x07, 0x8f, 0x32, 0xc5, 0x51, 0x11, 0xd3,
	// 	0x89, 0xb9, 0x00, 0x00, 0xf8, 0x1f, 0xe2, 0x21,
	// }

	// xIID_IXMLDOMDocument = uuid.UUID{
	// 	0x29, 0x33, 0xBF, 0x81, 0x7B, 0x36, 0x11, 0xD2,
	// 	0xB2, 0x0E, 0x00, 0xC0, 0x4F, 0x98, 0x3E, 0x60,
	// }

	// xIID_IXMLDOMNode = uuid.UUID{
	// 	0x29, 0x33, 0xbf, 0x80, 0x7b, 0x36, 0x11, 0xd2,
	// 	0xb2, 0x0e, 0x00, 0xc0, 0x4f, 0x98, 0x3e, 0x60,
	// }
)

func swapUUID(w io.Writer, u uuid.UUID) {
	bu := new(bytes.Buffer)
	_ = binary.Write(bu, binary.LittleEndian, u)
	var a uint32
	var b, c uint16
	_ = binary.Read(bu, binary.BigEndian, &a)
	_ = binary.Read(bu, binary.BigEndian, &b)
	_ = binary.Read(bu, binary.BigEndian, &c)
	_ = binary.Write(w, binary.LittleEndian, a)
	_ = binary.Write(w, binary.LittleEndian, b)
	_ = binary.Write(w, binary.LittleEndian, c)
	_, _ = bu.WriteTo(w)
}
