package virtualconn

import (
	"bytes"

	"project/internal/guid"
)

func testGenerateConn() *Conn {
	localGUID := guid.GUID{}
	copy(localGUID[:], bytes.Repeat([]byte{1}, guid.Size))
	localPort := uint32(1999)
	remoteGUID := guid.GUID{}
	copy(remoteGUID[:], bytes.Repeat([]byte{2}, guid.Size))
	remotePort := uint32(20160507)
	return NewConn(nil, nil, &localGUID, localPort, &remoteGUID, remotePort)
}
