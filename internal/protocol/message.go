package protocol

import (
	"bytes"
	"time"

	"project/internal/convert"
	"project/internal/xnet"
)

const (
	// follow command.go
	ERR_NULL_MESSAGE    uint8 = 0xFF
	ERR_TOO_BIG_MESSAGE uint8 = 0xFE
	// message id is uint16 < 65536
	SLOT_SIZE = 256
)

var (
	err_null_message    = []byte{ERR_NULL_MESSAGE}
	err_too_big_message = []byte{ERR_TOO_BIG_MESSAGE}
)

type Slot struct {
	Available chan struct{}
	Reply     chan []byte
}

// handler receive message = message type(4 byte) + message
func Handle_Message(conn *xnet.Conn, handler func([]byte)) {
	const (
		buffer_size = 4096

		// if data buffer size > this new buffer
		max_buffer_size  = 4 * buffer_size
		max_message_size = 16 * 1048576 // 64 MB

		// client send heartbeat in 0-60 s
		heartbeat = 120 * time.Second
	)
	buffer := make([]byte, buffer_size)
	data := bytes.NewBuffer(make([]byte, 0, buffer_size))
	body_size := 0
	flush_and_write := func() {
		// if Grow not NewBuffer
		if body_size == 0 {
			leftover := data.Bytes()
			if data.Cap() > max_buffer_size {
				data = bytes.NewBuffer(make([]byte, 0, buffer_size))
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
			if l < xnet.HEADER_SIZE {
				break
			}
			if body_size == 0 { // avoid duplicate calculations
				body_size = int(convert.Bytes_Uint32(data.Next(xnet.HEADER_SIZE)))
				if body_size == 0 {
					handler(err_null_message)
					return
				}
				if body_size > max_message_size {
					handler(err_too_big_message)
					return
				}
			}
			l = data.Len()
			if l < body_size {
				break
			}
			handler(data.Next(body_size))
			body_size = 0
			l = data.Len()
		}
		flush_and_write()
	}
}
