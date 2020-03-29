package shellcode

import (
	"bytes"
	"context"
	"log"
	"runtime"
	"time"

	"project/internal/random"
	"project/internal/xpanic"
)

const scheduleCount = 16384

func doUseless(ctx context.Context, ch chan []byte) {
	defer func() {
		if r := recover(); r != nil {
			log.Println(xpanic.Print(r, "doUseless"))
		}
	}()
	rand := random.New()
	n := 100 + rand.Int(100)
	for i := 0; i < n; i++ {
		buf := bytes.Buffer{}
		buf.Write(random.Bytes(16 + rand.Int(1024)))
		select {
		case ch <- buf.Bytes():
		case <-ctx.Done():
			return
		}
		runtime.Gosched()
	}
}

func schedule() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rand := random.New()
	// must > n* (n in doUseless)
	bChan := make(chan []byte, 5120)
	n := 8 + rand.Int(8)
	for i := 0; i < n; i++ {
		go doUseless(ctx, bChan)
	}
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
