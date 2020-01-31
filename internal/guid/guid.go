package guid

import (
	"crypto/sha256"
	"os"
	"sync"
	"time"

	"project/internal/convert"
	"project/internal/random"
)

// Size is the generated GUID size
// GUID = head + 8 bytes(random) + timestamp + add id
// total size = 28 + 8 + 8 + 8 = 52 Bytes
const Size int = 52

// Generator is a custom GUID generator
type Generator struct {
	now        func() time.Time
	random     *random.Rand
	head       []byte
	id         uint64
	guidQueue  chan []byte
	closeOnce  sync.Once
	stopSignal chan struct{}
	wg         sync.WaitGroup
}

// New is used to create a GUID generator
// if now is nil, use time.Now
func New(size int, now func() time.Time) *Generator {
	g := Generator{
		stopSignal: make(chan struct{}),
	}
	if size < 1 {
		g.guidQueue = make(chan []byte, 1)
	} else {
		g.guidQueue = make(chan []byte, size)
	}
	if now != nil {
		g.now = now
	} else {
		g.now = time.Now
	}
	g.random = random.New()
	// calculate head
	hash := sha256.New()
	for i := 0; i < 4096; i++ {
		hash.Write(g.random.Bytes(64))
	}
	g.head = make([]byte, 0, 28)
	g.head = append(g.head, hash.Sum(nil)[:24]...)
	g.head = append(g.head, convert.Int32ToBytes(int32(os.Getpid()))...)
	g.wg.Add(1)
	go g.generate()
	return &g
}

// Get is used to get a GUID, if generator closed, Get will return nil
func (g *Generator) Get() []byte {
	guid := <-g.guidQueue
	if len(guid) != 0 { // chan not closed
		copy(guid[36:44], convert.Int64ToBytes(g.now().Unix()))
	}
	return guid
}

// Close is used to close generator
func (g *Generator) Close() {
	g.closeOnce.Do(func() {
		close(g.stopSignal)
		g.wg.Wait()
	})
}

func (g *Generator) generate() {
	defer func() {
		if r := recover(); r != nil {
			// restart generate
			time.Sleep(time.Second)
			go g.generate()
		} else {
			close(g.guidQueue)
			g.wg.Done()
		}
	}()
	for {
		guid := make([]byte, Size)
		copy(guid, g.head)
		copy(guid[28:36], g.random.Bytes(8))
		// reserve timestamp
		copy(guid[44:52], convert.Uint64ToBytes(g.id))
		select {
		case <-g.stopSignal:
			return
		case g.guidQueue <- guid:
			g.id++
		}
	}
}
