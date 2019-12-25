package protocol

import (
	"bytes"
	"errors"
	"net"
	"time"

	"project/internal/convert"
)

// about connection
const (
	MaxMsgSize  = 2 * 1048576 // 2MB
	SendTimeout = time.Minute
	RecvTimeout = 2 * SendTimeout // wait heartbeat send time
	SlotSize    = 1024
	MaxMsgID    = SlotSize - 1

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
	ErrRecvReplyTimeout     = errors.New("receive reply timeout")
	ErrConnClosed           = errors.New("connection closed")
)

// Slot is used to handle message async
type Slot struct {
	Available chan struct{}
	Reply     chan []byte
	Timer     *time.Timer // receive reply timeout
}

// NewSlot is used to create slot
func NewSlot() *Slot {
	s := Slot{
		Available: make(chan struct{}, 1),
		Reply:     make(chan []byte, 1),
		Timer:     time.NewTimer(RecvTimeout),
	}
	s.Available <- struct{}{}
	return &s
}

var (
	errNullMsg   = []byte{ConnErrRecvNullMsg}
	errTooBigMsg = []byte{ConnErrRecvTooBigMsg}
)

// HandleConn is used to handle message,
// msgHandler receive message = cmd(1 byte) + other data
func HandleConn(conn net.Conn, msgHandler func([]byte)) {
	const (
		// if data buffer bufSize > this, new buffer
		bufSize    = 4096
		maxBufSize = 4 * bufSize

		// 2048 for cmd msgID GUID Hash...
		maxMsgSize = MaxMsgSize + 2048

		// client send heartbeat
		heartbeatTimeout = 120 * time.Second
	)
	buffer := make([]byte, bufSize)
	data := bytes.NewBuffer(make([]byte, 0, bufSize))
	var (
		bodySize int
		n        int
		l        int
		err      error
	)
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
		_ = conn.SetReadDeadline(time.Now().Add(heartbeatTimeout))
		n, err = conn.Read(buffer)
		if err != nil {
			return
		}
		data.Write(buffer[:n])
		l = data.Len()
		for {
			if l < MsgLenSize {
				break
			}
			if bodySize == 0 { // avoid duplicate calculations
				bodySize = int(convert.BytesToUint32(data.Next(MsgLenSize)))
				if bodySize == 0 {
					msgHandler(errNullMsg)
					return
				}
				if bodySize > maxMsgSize {
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
