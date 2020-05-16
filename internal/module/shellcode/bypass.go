package shellcode

import (
	"bytes"
	"context"
	"runtime"
	"time"

	"project/internal/random"
	"project/internal/xpanic"
)

const scheduleCount = 16384

func schedule(ctx context.Context, ch chan []byte) {
	defer func() {
		if r := recover(); r != nil {
			xpanic.Log(r, "schedule")
		}
	}()
	rand := random.NewRand()
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

// bypass is used to create a lot of goroutine to call "select"
// that can split syscall to random threads to call.
func bypass() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rand := random.NewRand()
	// must > n* (n in schedule)
	bChan := make(chan []byte, 5120)
	n := 8 + rand.Int(8)
	for i := 0; i < n; i++ {
		go schedule(ctx, bChan)
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