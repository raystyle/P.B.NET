package protocol

import (
	"bytes"
	"errors"
	"runtime"
	"time"

	"project/internal/convert"
	"project/internal/xnet"
)

var (
	// message id is uint16 < 65536
	SLOT_SIZE  = 16 * runtime.NumCPU()
	MAX_MSG_ID = SLOT_SIZE - 1
)

type Slot struct {
	Available chan struct{}
	Reply     chan []byte
	Timer     *time.Timer // receive reply timeout
}

const (
	// follow command.go
	ERR_NULL_MSG    uint8 = 0xFF
	ERR_TOO_BIG_MSG uint8 = 0xFE
	SEND_TIMEOUT          = time.Minute
	RECV_TIMEOUT          = 2 * time.Minute
)

var (
	ERR_INVALID_MSG_SIZE         = errors.New("invalid message size")
	ERR_RECV_NULL_MSG            = errors.New("receive null message")
	ERR_RECV_TOO_BIG_MSG         = errors.New("receive too big message")
	ERR_RECV_UNKNOWN_CMD         = errors.New("receive unknown command")
	ERR_RECV_INVALID_MSG_ID_SIZE = errors.New("receive invalid message id size")
	ERR_RECV_INVALID_MSG_ID      = errors.New("receive invalid message id")
	ERR_RECV_INVALID_REPLY       = errors.New("receive invalid reply")
	ERR_RECV_INVALID_TEST_MSG    = errors.New("receive invalid test message")
	ERR_CONN_CLOSED              = errors.New("connection closed")
	ERR_RECV_TIMEOUT             = errors.New("receive reply timeout")
)

var (
	err_null_msg    = []byte{ERR_NULL_MSG}
	err_too_big_msg = []byte{ERR_TOO_BIG_MSG}
)

// msg_handler receive message = message type(4 byte) + message
func Handle_Conn(conn *xnet.Conn, msg_handler func([]byte), close func()) {
	const (
		buf_size = 4096

		// if data buffer size > this new buffer
		max_buf_size = 4 * buf_size
		max_msg_size = 16 * 1048576 // 64 MB

		// client send heartbeat in 0-60 s
		heartbeat = 120 * time.Second
	)
	buffer := make([]byte, buf_size)
	data := bytes.NewBuffer(make([]byte, 0, buf_size))
	body_size := 0
	flush_and_write := func() {
		// if Grow not NewBuffer
		if body_size == 0 {
			leftover := data.Bytes()
			if data.Cap() > max_buf_size {
				data = bytes.NewBuffer(make([]byte, 0, buf_size))
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
			close()
			return
		}
		data.Write(buffer[:n])
		l := data.Len()
		for {
			if l < xnet.HEADER_SIZE {
				break
			}
			if body_size == 0 { // avoid duplicate calculations
				body_size = int(convert.Bytes_Uint32(data.Next(xnet.HEADER_SIZE)))
				if body_size == 0 {
					msg_handler(err_null_msg)
					return
				}
				if body_size > max_msg_size {
					msg_handler(err_too_big_msg)
					return
				}
			}
			l = data.Len()
			if l < body_size {
				break
			}
			msg_handler(data.Next(body_size))
			body_size = 0
			l = data.Len()
		}
		flush_and_write()
	}
}
