package api

import (
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"
)

// references:
//
// DLL information, get exported functions list:
// http://xpdll.nirsoft.net/iphlpapi_dll.html GetExtendedTcpTable and GetExtendedUdpTable
//
// parameters about exported function:
// https://docs.microsoft.com/en-us/windows/win32/api/iphlpapi/nf-iphlpapi-getextendedtcptable
// https://docs.microsoft.com/en-us/windows/win32/api/iphlpapi/nf-iphlpapi-getextendedudptable

// TCP table class
const (
	TCPTableBasicListener uint32 = iota
	TCPTableBasicConnections
	TCPTableBasicAll
	TCPTableOwnerPIDListener
	TCPTableOwnerPIDConnections
	TCPTableOwnerPIDAll
	TCPTableOwnerModuleListener
	TCPTableOwnerModuleConnections
	TCPTableOwnerModuleAll
)

// UDP table class
const (
	UDPTableBasic uint32 = iota
	UDPTableOwnerPID
	UDPTableOwnerModule
)

var (
	modIphlpapi = windows.NewLazySystemDLL("iphlpapi.dll")

	procGetExtendedTCPTable = modIphlpapi.NewProc("GetExtendedTcpTable")
	procGetExtendedUDPTable = modIphlpapi.NewProc("GetExtendedUdpTable")
)

// TCP over IPv4
type tcp4Table struct {
	n     uint32
	table [1]tcp4Row
}

// TCP over IPv4 connection
type tcp4Row struct {
	state      uint32
	localAddr  uint32
	localPort  uint32
	remoteAddr uint32
	remotePort uint32
	pid        uint32
}

// TCP over IPv6
type tcp6Table struct {
	n     uint32
	table [1]tcp6Row
}

// TCP over IPv6 connection
type tcp6Row struct {
	localAddr     [4]uint32
	localScopeID  uint32
	localPort     uint32
	remoteAddr    [4]uint32
	remoteScopeID uint32
	remotePort    uint32
	state         uint32
	pid           uint32
}

// GetTCP4Conns is used to get TCP-over-IPv4 connections.
func GetTCP4Conns(ulAf uint32) {

}

func getTCPTable(ulAf, class uint32) ([]byte, error) {
	const maxAttemptTimes = 64
	var (
		buffer   []byte
		tcpTable *byte
		dwSize   uint32
	)
	for i := 0; i < maxAttemptTimes; i++ {
		ret, _, errno := procGetExtendedTCPTable.Call(
			uintptr(unsafe.Pointer(tcpTable)), uintptr(unsafe.Pointer(&dwSize)),
			uintptr(uint32(1)), uintptr(ulAf), uintptr(class), uintptr(uint32(0)),
		)
		if ret != windows.NO_ERROR {
			if windows.Errno(ret) == windows.ERROR_INSUFFICIENT_BUFFER {
				buffer = make([]byte, dwSize)
				tcpTable = &buffer[0]
				continue
			}
			return nil, errno
		}
		return buffer, nil
	}
	return nil, errors.New("failed to get tcp table because reach maximum attempt times")
}

// UDP over IPv4
type udp4Table struct {
	n     uint32
	table [1]udp4Row
}

// UDP over IPv4 connection
type udp4Row struct {
	localAddr uint32
	localPort uint32
	pid       uint32
}

// UDP over IPv6
type udp6Table struct {
	n     uint32
	table [1]udp6Row
}

// UDP over IPv6 connection
type udp6Row struct {
	localAddr    [4]uint32
	localScopeID uint32
	localPort    uint32
	pid          uint32
}
