package xio

import (
	"context"
	"io"
)

// CopyWithContext is used to copy io steam with context.
// Usually it used to copy large file.
func CopyWithContext(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	return CopyBufferWithContext(ctx, dst, src, nil)
}

// CopyBufferWithContext is used to copy io steam with context, you can set buffer size.
// reference GOROOT/src/io/io.go copyBuffer()
func CopyBufferWithContext(ctx context.Context, dst io.Writer, src io.Reader, buf []byte) (int64, error) {
	if buf == nil {
		var size int64 = 32 * 1024
		if l, ok := src.(*io.LimitedReader); ok && size > l.N {
			if l.N < 1 {
				size = 1
			} else {
				size = l.N
			}
		}
		buf = make([]byte, size)
	}
	var (
		rn      int   // read
		re      error // read error
		wn      int   // write
		we      error // write error
		written int64
		err     error
	)
	for {
		// check is canceled
		select {
		case <-ctx.Done():
			return written, ctx.Err()
		default:
		}
		// copy
		rn, re = src.Read(buf)
		if rn > 0 {
			wn, we = dst.Write(buf[:rn])
			if wn > 0 {
				written += int64(wn)
			}
			if we != nil {
				err = we
				break
			}
			if rn != wn {
				err = io.ErrShortWrite
				break
			}
		}
		if re != nil {
			if re != io.EOF {
				err = re
			}
			break
		}
	}
	return written, err
}
