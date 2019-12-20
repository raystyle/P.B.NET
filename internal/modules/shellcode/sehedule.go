package shellcode

import (
	"bytes"
	"runtime"
	"time"

	"project/internal/random"
)

func doUseless(c chan []byte) {
	rand := random.New()
	n := 100 + rand.Int(100)
	for i := 0; i < n; i++ {
		buf := bytes.Buffer{}
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
	timer := time.NewTimer(25 * time.Millisecond)
	defer timer.Stop()
read:
	for {
		timer.Reset(25 * time.Millisecond)
		select {
		case b := <-bChan:
			b[0] = byte(rand.Int64())
		case <-timer.C:
			break read
		}
	}
	time.Sleep(time.Millisecond * time.Duration(50+rand.Int(100)))
}
