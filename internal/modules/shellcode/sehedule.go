package shellcode

import (
	"bytes"
	"runtime"
	"time"

	"project/internal/random"
)

func doUseless(c chan []byte) {
	buf := bytes.Buffer{}
	rand := random.New()
	n := 100 + rand.Int(100)
	for i := 0; i < n; i++ {
		buf.Write(random.Bytes(16 + rand.Int(1024)))
		c <- buf.Bytes()
	}
}

func schedule() {
	rand := random.New()
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
