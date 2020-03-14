package guid

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"os"
	"strings"
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

// Size is the generated GUID size.
const Size int = 20 + 4 + 8 + 8 + 8

// GUID is the generated GUID
type GUID [Size]byte

// Write is used to copy []byte to guid.
func (guid *GUID) Write(s []byte) error {
	if len(s) != Size {
		return errors.New("invalid byte slice size")
	}
	copy(guid[:], s)
	return nil
}

// Print is used to print GUID with prefix.
//
// GUID: FD4960D3BE40D9CE66B02949E1E85B9082AA0016C39D3225
//       2228B5F0502D7F3D94F0000000005E35B700000000000000
func (guid *GUID) Print() string {
	// 13 = len("GUID:  ") + len("      ") + len("\n")
	dst := make([]byte, Size*2+13)
	copy(dst, "GUID: ")
	hex.Encode(dst[6:], guid[:Size/2])
	copy(dst[6+Size:], "\n      ")
	hex.Encode(dst[Size+13:], guid[Size/2:])
	return strings.ToUpper(string(dst))
}

// Hex is used to encode GUID to a hex string.
//
// FD4960D3BE40D9CE66B02949E1E85B9082AA0016C39D3225
// 2228B5F0502D7F3D94F0000000005E35B700000000000000
func (guid *GUID) Hex() string {
	dst := make([]byte, Size*2+1) // add a "\n"
	hex.Encode(dst, guid[:Size/2])
	dst[Size] = 10 // "\n"
	hex.Encode(dst[Size+1:], guid[Size/2:])
	return strings.ToUpper(string(dst))
}

// Line is used to encode GUID to a hex string in one line.
func (guid *GUID) Line() string {
	dst := make([]byte, Size*2)
	hex.Encode(dst, guid[:])
	return strings.ToUpper(string(dst))
}

// Timestamp is used to get timestamp in the GUID.
func (guid *GUID) Timestamp() int64 {
	return int64(binary.BigEndian.Uint64(guid[32:40]))
}

// MarshalJSON is used to implement JSON Marshaler interface.
func (guid *GUID) MarshalJSON() ([]byte, error) {
	const quotation = 34 // ASCII
	dst := make([]byte, 2*Size+2)
	dst[0] = quotation
	hex.Encode(dst[1:], guid[:])
	dst[2*Size+1] = quotation
	return dst, nil
}

// UnmarshalJSON is used to implement JSON Unmarshaler interface.
func (guid *GUID) UnmarshalJSON(data []byte) error {
	if len(data) != 2*Size+2 {
		return errors.New("invalid size about guid")
	}
	_, err := hex.Decode(guid[:], data[1:2*Size+1])
	return err
}

// Generator is a custom GUID generator.
type Generator struct {
	now        func() time.Time
	rand       *random.Rand
	head       []byte // hash + PID
	id         uint64 // self add
	guidQueue  chan *GUID
	closeOnce  sync.Once
	stopSignal chan struct{}
	wg         sync.WaitGroup
}

// New is used to create a GUID generator.
// if now is nil, use time.Now.
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
	g.rand = random.New()
	// calculate head
	hash := sha256.New()
	for i := 0; i < 4096; i++ {
		hash.Write(g.rand.Bytes(64))
	}
	g.head = make([]byte, 0, 24)
	g.head = append(g.head, hash.Sum(nil)[:20]...)
	hash.Write(convert.Int64ToBytes(int64(os.Getpid())))
	g.head = append(g.head, hash.Sum(nil)[:4]...)
	// random ID
	for i := 0; i < 5; i++ {
		g.id += uint64(g.rand.Int(1048576))
	}
	g.wg.Add(1)
	go g.generate()
	return &g
}

// Get is used to get a GUID, if generator closed, Get will return nil.
func (g *Generator) Get() *GUID {
	guid := <-g.guidQueue
	copy(guid[32:40], convert.Int64ToBytes(g.now().Unix()))
	return guid
}

// Close is used to close generator.
func (g *Generator) Close() {
	g.closeOnce.Do(func() {
		close(g.stopSignal)
		g.wg.Wait()
		g.now = nil
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
		copy(guid[24:32], g.rand.Bytes(8))
		// reserve timestamp
		copy(guid[40:48], convert.Uint64ToBytes(g.id))
		select {
		case <-g.stopSignal:
			return
		case g.guidQueue <- &guid:
			g.id += uint64(g.rand.Int(1024))
		}
	}
}
