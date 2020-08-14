// +build windows

package netstat

import (
	"reflect"
	"unsafe"

	"golang.org/x/sys/windows"

	"project/internal/convert"
)

// reference:
// https://docs.microsoft.com/en-us/windows/win32/api/iphlpapi/nf-iphlpapi-getextendedtcptable

// IPv4
type tcp4Table struct {
	n     uint32
	table [1]tcp4Row
}

// IPv4 TCP connection
type tcp4Row struct {
	state      uint32
	localAddr  uint32
	localPort  uint32
	remoteAddr uint32
	remotePort uint32
	pid        uint32
}

// IPv6
type tcp6Table struct {
	n     uint32
	table [1]tcp6Row
}

// IPv6 TCP connection
type tcp6Row struct {
	localAddr     [4]uint32
	localScopeId  uint32
	localPort     uint32
	remoteAddr    [4]uint32
	remoteScopeId uint32
	remotePort    uint32
	state         uint32
	pid           uint32
}

type refresher struct {
	getExtendedTCPTable *windows.LazyProc
	getExtendedUDPTable *windows.LazyProc
}

func (ref *refresher) Refresh() ([]*Connection, error) {
	// TCP with IPv4
	buffer, err := ref.getTCPTable(windows.AF_INET)
	if err != nil {
		return nil, err
	}
	tcp4Table := (*tcp4Table)(unsafe.Pointer(&buffer[0]))
	h := &reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(&tcp4Table.table)),
		Len:  int(tcp4Table.n),
		Cap:  int(tcp4Table.n),
	}
	tcp4Rows := *(*[]tcp4Row)(unsafe.Pointer(h))
	// TCP with IPv6
	buffer, err = ref.getTCPTable(windows.AF_INET6)
	if err != nil {
		return nil, err
	}
	tcp6Table := (*tcp6Table)(unsafe.Pointer(&buffer[0]))
	h = &reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(&tcp6Table.table)),
		Len:  int(tcp6Table.n),
		Cap:  int(tcp6Table.n),
	}
	tcp6Rows := *(*[]tcp6Row)(unsafe.Pointer(h))

	tcp4RowsLen := len(tcp4Rows)
	tcp6RowsLen := len(tcp6Rows)
	conns := make([]*Connection, 0, tcp4RowsLen+tcp6RowsLen)
	// TCP with IPv4
	for i := 0; i < tcp4RowsLen; i++ {
		conn := Connection{
			Protocol:   ProtocolTCP,
			LocalAddr:  convert.LEUint32ToBytes(tcp4Rows[i].localAddr),
			RemoteAddr: convert.LEUint32ToBytes(tcp4Rows[i].remoteAddr),
			State:      uint8(tcp4Rows[i].state),
			PID:        int64(tcp4Rows[i].pid),
		}
		b := convert.LEUint32ToBytes(tcp4Rows[i].localPort)[:2]
		conn.LocalPort = convert.BytesToUint16(b)
		b = convert.LEUint32ToBytes(tcp4Rows[i].remotePort)[:2]
		conn.RemotePort = convert.BytesToUint16(b)
		conns = append(conns, &conn)
	}
	// TCP with IPv6
	return conns, nil
}

func newRefresher() (Refresher, error) {
	// load dll and find proc
	lazyDLL := windows.NewLazySystemDLL("iphlpapi.dll")
	err := lazyDLL.Load()
	if err != nil {
		return nil, err
	}
	getExtendedTCPTable := lazyDLL.NewProc("GetExtendedTcpTable")
	err = getExtendedTCPTable.Find()
	if err != nil {
		return nil, err
	}
	getExtendedUDPTable := lazyDLL.NewProc("GetExtendedUdpTable")
	err = getExtendedUDPTable.Find()
	if err != nil {
		return nil, err
	}
	ref := refresher{
		getExtendedTCPTable: getExtendedTCPTable,
		getExtendedUDPTable: getExtendedUDPTable,
	}
	return &ref, nil
}

func (ref *refresher) getTCPTable(ulAf uint32) ([]byte, error) {
	var buffer []byte
	var pTcpTable *byte
	var dwSize uint32
	for {
		ret, _, errno := ref.getExtendedTCPTable.Call(
			uintptr(unsafe.Pointer(pTcpTable)),
			uintptr(unsafe.Pointer(&dwSize)),
			uintptr(1), // order
			uintptr(ulAf),
			uintptr(5),         //  TCP_TABLE_OWNER_PID_ALL
			uintptr(uint32(0)), // reserved
		)
		if ret != windows.NO_ERROR {
			if windows.Errno(ret) == windows.ERROR_INSUFFICIENT_BUFFER {
				buffer = make([]byte, dwSize)
				pTcpTable = &buffer[0]
				continue
			}
			return nil, errno
		}
		return buffer, nil
	}
}
