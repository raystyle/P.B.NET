package guid

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"os"
	"sync"
	"time"

	"project/internal/convert"
	"project/internal/random"
)

// +------------+-------------+----------+------------------+------------+
// | hash(part) | PID(hashed) |  random  | timestamp(int64) | ID(uint64) |
// +------------+-------------+----------+------------------+------------+
// |  20 bytes  |   4 bytes   |  8 bytes |      8 bytes     |  8 bytes   |
// +------------+-------------+----------+------------------+------------+
// head = hash + PID

// Size is the generated GUID size
const Size int = 20 + 4 + 8 + 8 + 8

// GUID is the generated GUID
type GUID [Size]byte

// String is used to print GUID with prefix
//
// GUID: FD4960D3BE40D9CE66B02949E1E85B9082AA0016C39D3225
//       2228B5F0502D7F3D94F0000000005E35B700000000000000
func (guid GUID) String() string {
	// 13 = len("GUID:  ") + len("      ") + len("\n")
	dst := make([]byte, Size*2+13)
	copy(dst, "GUID: ")
	hex.Encode(dst[6:], guid[:Size/2])
	copy(dst[6+Size:], "\n      ")
	hex.Encode(dst[Size+13:], guid[Size/2:])
	return string(dst)
}

// Hex is used to encode GUID to a hex string
//
// FD4960D3BE40D9CE66B02949E1E85B9082AA0016C39D3225
// 2228B5F0502D7F3D94F0000000005E35B700000000000000
func (guid GUID) Hex() string {
	dst := make([]byte, Size*2+1) // add a "\n"
	hex.Encode(dst, guid[:Size/2])
	dst[Size] = 10 // "\n"
	hex.Encode(dst[Size+1:], guid[Size/2:])
	return string(dst)
}

// Timestamp is used to get timestamp in the GUID
func (guid GUID) Timestamp() int64 {
	return int64(binary.BigEndian.Uint64(guid[32:40]))
}

// Generator is a custom GUID generator
type Generator struct {
	now        func() time.Time
	random     *random.Rand
	head       []byte // hash + PID
	id         uint64 // self add
	guidQueue  chan *GUID
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
		g.guidQueue = make(chan *GUID, 1)
	} else {
		g.guidQueue = make(chan *GUID, size)
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
	g.head = make([]byte, 0, 24)
	g.head = append(g.head, hash.Sum(nil)[:20]...)
	hash.Write(convert.Int64ToBytes(int64(os.Getpid())))
	g.head = append(g.head, hash.Sum(nil)[:4]...)
	g.wg.Add(1)
	go g.generate()
	return &g
}

// Get is used to get a GUID, if generator closed, Get will return nil
func (g *Generator) Get() *GUID {
	guid := <-g.guidQueue
	copy(guid[32:40], convert.Int64ToBytes(g.now().Unix()))
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
		guid := GUID{}
		copy(guid[:], g.head)
		copy(guid[24:32], g.random.Bytes(8))
		// reserve timestamp
		copy(guid[40:48], convert.Uint64ToBytes(g.id))
		select {
		case <-g.stopSignal:
			return
		case g.guidQueue <- &guid:
			g.id++
		}
	}
}
