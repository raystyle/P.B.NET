package protocol

import (
	"bytes"
	"time"

	"project/internal/convert"
	"project/internal/xnet"
)

var (
	// follow command.go
	ERR_NULL_MESSAGE    = []byte{0xFF}
	ERR_TOO_BIG_MESSAGE = []byte{0xFE}
)

// handler receive message = message type(4 byte) + message
func Handle_Message(conn *xnet.Conn, handler func([]byte)) {
	const (
		buffer_size = 4096

		// if data buffer size > this new buffer
		max_buffer_size  = 4 * buffer_size
		max_message_size = 64 * 1048576 // 64 MB

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
				data = bytes.NewBuffer(make([]byte, buffer_size))
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
					handler(ERR_NULL_MESSAGE)
					return
				}
				if body_size > max_message_size {
					handler(ERR_TOO_BIG_MESSAGE)
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
