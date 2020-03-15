package guid

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"project/internal/convert"
	"project/internal/random"
	"project/internal/xpanic"
)

// +------------+-------------+----------+------------------+------------+
// | hash(part) | PID(hashed) |  random  | timestamp(int64) | ID(uint64) |
// +------------+-------------+----------+------------------+------------+
// |  8 bytes   |   4 bytes   |  8 bytes |      8 bytes     |  4 bytes   |
// +------------+-------------+----------+------------------+------------+
// head = hash + PID

// Size is the generated GUID size.
const Size int = 8 + 4 + 8 + 8 + 4

// GUID is the generated GUID
type GUID [Size]byte

// Write is used to copy []byte to guid.
func (guid *GUID) Write(b []byte) error {
	if len(b) != Size {
		return errors.New("invalid byte slice size")
	}
	copy(guid[:], b)
	return nil
}

// Print is used to print GUID with prefix.
//
// GUID: BF0AF7928C30AA6B1027DE8D6789F09202262591000000005E6C65F8002AD680
func (guid *GUID) Print() string {
	// 6 = len("GUID: ")
	dst := make([]byte, Size*2+6)
	copy(dst, "GUID: ")
	hex.Encode(dst[6:], guid[:])
	return strings.ToUpper(string(dst))
}

// Hex is used to encode GUID to a hex string.
//
// BF0AF7928C30AA6B1027DE8D6789F09202262591000000005E6C65F8002AD680
func (guid *GUID) Hex() string {
	dst := make([]byte, Size*2) // add a "\n"
	hex.Encode(dst, guid[:])
	return strings.ToUpper(string(dst))
}

// Timestamp is used to get timestamp in the GUID.
func (guid *GUID) Timestamp() int64 {
	return int64(binary.BigEndian.Uint64(guid[20:28]))
}

// MarshalJSON is used to implement JSON Marshaler interface.
func (guid GUID) MarshalJSON() ([]byte, error) {
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
	now       func() time.Time
	rand      *random.Rand
	head      []byte // hash + PID
	id        uint32 // self add
	guidQueue chan *GUID
	rwm       sync.RWMutex

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
	// calculate head (8+4 PID)
	hash := sha256.New()
	for i := 0; i < 4096; i++ {
		hash.Write(g.rand.Bytes(64))
	}
	g.head = make([]byte, 0, 8)
	g.head = append(g.head, hash.Sum(nil)[:8]...)
	hash.Write(convert.Int64ToBytes(int64(os.Getpid())))
	g.head = append(g.head, hash.Sum(nil)[:4]...)
	// random ID
	for i := 0; i < 5; i++ {
		g.id += uint32(g.rand.Int(1048576))
	}
	g.wg.Add(1)
	go g.generate()
	return &g
}

// Get is used to get a GUID, if generator closed, Get will return zero.
func (g *Generator) Get() *GUID {
	guid := <-g.guidQueue
	if guid == nil {
		return new(GUID)
	}
	g.rwm.RLock()
	defer g.rwm.RUnlock()
	if g.now == nil {
		return new(GUID)
	}
	binary.BigEndian.PutUint64(guid[20:28], uint64(g.now().Unix()))
	return guid
}

// Close is used to close generator.
func (g *Generator) Close() {
	g.closeOnce.Do(func() {
		close(g.stopSignal)
		g.wg.Wait()
		g.rwm.Lock()
		defer g.rwm.Unlock()
		g.now = nil
	})
}

func (g *Generator) generate() {
	defer func() {
		if r := recover(); r != nil {
			log.Println(xpanic.Print(r, "Generator.generate"))
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
		copy(guid[12:20], g.rand.Bytes(8))
		// reserve timestamp guid[20:28]
		binary.BigEndian.PutUint32(guid[28:32], g.id)
		select {
		case g.guidQueue <- &guid:
			g.id += uint32(g.rand.Int(1024))
		case <-g.stopSignal:
			return
		}
	}
}
