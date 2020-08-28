// +build windows

package netmon

import (
	"errors"
	"net"
	"reflect"
	"unsafe"

	"golang.org/x/sys/windows"

	"project/internal/convert"
)

// references:
//
// DLL information, get exported functions list:
// http://xpdll.nirsoft.net/iphlpapi_dll.html GetExtendedTcpTable and GetExtendedUdpTable
//
// parameters about exported function:
// https://docs.microsoft.com/en-us/windows/win32/api/iphlpapi/nf-iphlpapi-getextendedtcptable
// https://docs.microsoft.com/en-us/windows/win32/api/iphlpapi/nf-iphlpapi-getextendedudptable

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

var (
	modIphlpapi = windows.NewLazySystemDLL("iphlpapi.dll")

	procGetExtendedTCPTable = modIphlpapi.NewProc("GetExtendedTcpTable")
	procGetExtendedUDPTable = modIphlpapi.NewProc("GetExtendedUdpTable")
)

type netStat struct{}

func newNetstat() (netStat, error) {
	err := procGetExtendedTCPTable.Find()
	if err != nil {
		return netStat{}, err
	}
	err = procGetExtendedUDPTable.Find()
	if err != nil {
		return netStat{}, err
	}
	return netStat{}, nil
}

// #nosec
func (netStat) GetTCP4Conns() ([]*TCP4Conn, error) {
	buffer, err := getTCPTable(windows.AF_INET)
	if err != nil {
		return nil, err
	}
	table := (*tcp4Table)(unsafe.Pointer(&buffer[0]))
	h := &reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(&table.table)),
		Len:  int(table.n),
		Cap:  int(table.n),
	}
	rows := *(*[]tcp4Row)(unsafe.Pointer(h))
	l := len(rows)
	conns := make([]*TCP4Conn, l)
	for i := 0; i < l; i++ {
		conn := TCP4Conn{
			LocalAddr:  convert.LEUint32ToBytes(rows[i].localAddr),
			RemoteAddr: convert.LEUint32ToBytes(rows[i].remoteAddr),
			State:      uint8(rows[i].state),
			PID:        int64(rows[i].pid),
		}
		b := convert.LEUint32ToBytes(rows[i].localPort)[:2]
		conn.LocalPort = convert.BEBytesToUint16(b)
		b = convert.LEUint32ToBytes(rows[i].remotePort)[:2]
		conn.RemotePort = convert.BEBytesToUint16(b)
		conns[i] = &conn
	}
	return conns, nil
}

// #nosec
func (netStat) GetTCP6Conns() ([]*TCP6Conn, error) {
	buffer, err := getTCPTable(windows.AF_INET6)
	if err != nil {
		return nil, err
	}
	table := (*tcp6Table)(unsafe.Pointer(&buffer[0]))
	h := &reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(&table.table)),
		Len:  int(table.n),
		Cap:  int(table.n),
	}
	rows := *(*[]tcp6Row)(unsafe.Pointer(h))
	l := len(rows)
	conns := make([]*TCP6Conn, l)
	for i := 0; i < l; i++ {
		conn := TCP6Conn{
			LocalAddr:     convertUint32sToIPv6(rows[i].localAddr),
			LocalScopeID:  rows[i].localScopeID,
			RemoteAddr:    convertUint32sToIPv6(rows[i].remoteAddr),
			RemoteScopeID: rows[i].remoteScopeID,
			State:         uint8(rows[i].state),
			PID:           int64(rows[i].pid),
		}
		b := convert.LEUint32ToBytes(rows[i].localPort)[:2]
		conn.LocalPort = convert.BEBytesToUint16(b)
		b = convert.LEUint32ToBytes(rows[i].remotePort)[:2]
		conn.RemotePort = convert.BEBytesToUint16(b)
		conns[i] = &conn
	}
	return conns, nil
}

// #nosec
func getTCPTable(ulAf uint32) ([]byte, error) {
	const maxAttemptTimes = 64
	var (
		buffer    []byte
		pTCPTable *byte
		dwSize    uint32
	)
	for i := 0; i < maxAttemptTimes; i++ {
		ret, _, errno := procGetExtendedTCPTable.Call(
			uintptr(unsafe.Pointer(pTCPTable)),
			uintptr(unsafe.Pointer(&dwSize)),
			uintptr(1),         // order
			uintptr(ulAf),      // IPv4 or IPv6
			uintptr(5),         //  TCP_TABLE_OWNER_PID_ALL
			uintptr(uint32(0)), // reserved
		)
		if ret != windows.NO_ERROR {
			if windows.Errno(ret) == windows.ERROR_INSUFFICIENT_BUFFER {
				buffer = make([]byte, dwSize)
				pTCPTable = &buffer[0]
				continue
			}
			return nil, errno
		}
		return buffer, nil
	}
	return nil, errors.New("failed to get tcp table because reach maximum attempt times")
}

// #nosec
func (netStat) GetUDP4Conns() ([]*UDP4Conn, error) {
	buffer, err := getUDPTable(windows.AF_INET)
	if err != nil {
		return nil, err
	}
	table := (*udp4Table)(unsafe.Pointer(&buffer[0]))
	h := &reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(&table.table)),
		Len:  int(table.n),
		Cap:  int(table.n),
	}
	rows := *(*[]udp4Row)(unsafe.Pointer(h))
	l := len(rows)
	conns := make([]*UDP4Conn, l)
	for i := 0; i < l; i++ {
		conn := UDP4Conn{
			LocalAddr: convert.LEUint32ToBytes(rows[i].localAddr),
			PID:       int64(rows[i].pid),
		}
		b := convert.LEUint32ToBytes(rows[i].localPort)[:2]
		conn.LocalPort = convert.BEBytesToUint16(b)
		conns[i] = &conn
	}
	return conns, nil
}

// #nosec
func (netStat) GetUDP6Conns() ([]*UDP6Conn, error) {
	buffer, err := getUDPTable(windows.AF_INET6)
	if err != nil {
		return nil, err
	}
	table := (*udp6Table)(unsafe.Pointer(&buffer[0]))
	h := &reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(&table.table)),
		Len:  int(table.n),
		Cap:  int(table.n),
	}
	rows := *(*[]udp6Row)(unsafe.Pointer(h))
	l := len(rows)
	conns := make([]*UDP6Conn, l)
	for i := 0; i < l; i++ {
		conn := UDP6Conn{
			LocalAddr:    convertUint32sToIPv6(rows[i].localAddr),
			LocalScopeID: rows[i].localScopeID,
			PID:          int64(rows[i].pid),
		}
		b := convert.LEUint32ToBytes(rows[i].localPort)[:2]
		conn.LocalPort = convert.BEBytesToUint16(b)
		conns[i] = &conn
	}
	return conns, nil
}

// #nosec
func getUDPTable(ulAf uint32) ([]byte, error) {
	const maxAttemptTimes = 64
	var (
		buffer    []byte
		pUDPTable *byte
		dwSize    uint32
	)
	for i := 0; i < maxAttemptTimes; i++ {
		ret, _, errno := procGetExtendedUDPTable.Call(
			uintptr(unsafe.Pointer(pUDPTable)),
			uintptr(unsafe.Pointer(&dwSize)),
			uintptr(1),         // order
			uintptr(ulAf),      // IPv4 or IPv6
			uintptr(1),         // UDP_TABLE_OWNER_PID
			uintptr(uint32(0)), // reserved
		)
		if ret != windows.NO_ERROR {
			if windows.Errno(ret) == windows.ERROR_INSUFFICIENT_BUFFER {
				buffer = make([]byte, dwSize)
				pUDPTable = &buffer[0]
				continue
			}
			return nil, errno
		}
		return buffer, nil
	}
	return nil, errors.New("failed to get udp table because reach maximum attempt times")
}

func convertUint32sToIPv6(addr [4]uint32) net.IP {
	ip := make([]byte, 0, net.IPv6len)
	for i := 0; i < 4; i++ {
		ip = append(ip, convert.LEUint32ToBytes(addr[i])...)
	}
	return ip
}
