package protocol

import (
	"bytes"
	"errors"
	"math"
	"net"
	"runtime"
	"time"

	"project/internal/convert"
	"project/internal/xnet"
)

// about connection
const (
	MaxMsgSize  = 2 * 1048576 // 2MB
	SendTimeout = time.Minute
	RecvTimeout = 2 * SendTimeout // wait heartbeat send time

	// don't change
	MsgLenSize    = 4
	MsgCMDSize    = 1
	MsgIDSize     = 2
	MsgHeaderSize = MsgLenSize + MsgCMDSize + MsgIDSize
)

// errors
var (
	ErrTooBigMsg            = errors.New("too big message")
	ErrInvalidMsgSize       = errors.New("invalid message size")
	ErrRecvNullMsg          = errors.New("receive null message")
	ErrRecvTooBigMsg        = errors.New("receive too big message")
	ErrRecvUnknownCMD       = errors.New("receive unknown command")
	ErrRecvInvalidMsgIDSize = errors.New("receive invalid message id size")
	ErrRecvInvalidMsgID     = errors.New("receive invalid message id")
	ErrRecvInvalidReplyID   = errors.New("receive invalid reply id")
	ErrConnClosed           = errors.New("connection closed")
	ErrRecvTimeout          = errors.New("receive reply timeout")
)

// it will change about the number of the current system
var (
	SlotSize int // message id is uint16 < 65536
	MaxMsgID int // check invalid message id
)

func init() {
	SlotSize = int(uint16(math.Abs(float64(runtime.NumCPU()))) + 64)
	MaxMsgID = SlotSize - 1
}

// Slot is used to handle message async
type Slot struct {
	Available chan struct{}
	Reply     chan []byte
	Timer     *time.Timer // receive reply timeout
}

var (
	errNullMsg   = []byte{ErrCMDRecvNullMsg}
	errTooBigMsg = []byte{ErrCMDTooBigMsg}
)

// HandleConn is used to handle message,
// msgHandler receive message = cmd(1 byte) + other data
func HandleConn(conn net.Conn, msgHandler func([]byte)) {
	const (
		// if data buffer bufSize > this, new buffer
		bufSize    = 4096
		maxBufSize = 4 * bufSize

		// 2048 for cmd msgID GUID Hash...
		maxPayloadSize = MaxMsgSize + 2048

		// client send heartbeat
		heartbeat = 120 * time.Second
	)
	buffer := make([]byte, bufSize)
	data := bytes.NewBuffer(make([]byte, 0, bufSize))
	bodySize := 0
	flushAndWrite := func() {
		// if Grow not NewBuffer
		if bodySize == 0 {
			leftover := data.Bytes()
			if data.Cap() > maxBufSize {
				data = bytes.NewBuffer(make([]byte, 0, bufSize))
			} else {
				data.Reset() // for set b.off = 0
			}
			data.Write(leftover)
		}
	}
	for {
		_ = conn.SetReadDeadline(time.Now().Add(heartbeat))
		n, err := conn.Read(buffer)
		if err != nil {
			return
		}
		data.Write(buffer[:n])
		l := data.Len()
		for {
			if l < xnet.DataSize {
				break
			}
			if bodySize == 0 { // avoid duplicate calculations
				bodySize = int(convert.BytesToUint32(data.Next(xnet.DataSize)))
				if bodySize == 0 {
					msgHandler(errNullMsg)
					return
				}
				if bodySize > maxPayloadSize {
					msgHandler(errTooBigMsg)
					return
				}
			}
			l = data.Len()
			if l < bodySize {
				break
			}
			msgHandler(data.Next(bodySize))
			bodySize = 0
			l = data.Len()
		}
		flushAndWrite()
	}
}
