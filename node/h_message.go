package node

import (
	"bytes"
	"time"

	"project/internal/convert"
	"project/internal/xnet"
)

func v1_handle_message(conn *xnet.Conn, handler func([]byte)) {
	const (
		buffer_size = 4096

		// if data buffer size > this new buffer
		max_buffer_size = 4 * buffer_size

		// client send heartbeat in 0-60 s
		heartbeat = 60 * time.Second
	)
	data := bytes.NewBuffer(make([]byte, buffer_size))
	buffer := make([]byte, buffer_size)
	body_size := 0

	if data.Cap() > max_buffer_size {
		data = bytes.NewBuffer(make([]byte, buffer_size))
	}

	for {
		_ = conn.SetReadDeadline(time.Now().Add(heartbeat))
		n, err := conn.Read(buffer)
		if err != nil {
			return
		}
		data.Write(buffer[:n])
		l := data.Len()
		if l < xnet.HEADER_SIZE {
			continue
		}
		if body_size == 0 { // avoid duplicate calculations
			body_size = int(convert.Bytes_Uint32(data.Bytes()[:xnet.HEADER_SIZE]))

		}
		if l-xnet.HEADER_SIZE >= body_size {

		}

	}

}
