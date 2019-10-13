package guid

import (
	"bytes"
	"crypto/rand"
	"net"
	"os"
	"runtime"
	"strconv"
	"sync"
	"time"

	"project/internal/convert"
	"project/internal/crypto/sha256"
	"project/internal/random"
)

// head = sha256(ip + hostname + pid + random data)
// guid = head + 4 bytes(random) + timestamp + add id
// total 32 + 4 + 8 + 8 = 52 Bytes
const Size int = 52

type GUID struct {
	now        func() time.Time
	random     *random.Rand
	head       []byte
	id         uint64 // self add
	guidQueue  chan []byte
	closeOnce  sync.Once
	stopSignal chan struct{}
	wg         sync.WaitGroup
}

// if now is nil use time.Now
func New(size int, now func() time.Time) *GUID {
	g := &GUID{
		stopSignal: make(chan struct{}),
	}
	if size < 1 {
		g.guidQueue = make(chan []byte, 1)
	} else {
		g.guidQueue = make(chan []byte, size)
	}
	if now != nil { // <security>
		g.now = now
	} else {
		g.now = time.Now
	}
	g.random = random.New(g.now().Unix())
	// head
	ip := "0.0.0.0"
	addrs, err := net.InterfaceAddrs()
	if err == nil && addrs != nil {
		ip = addrs[0].String()
	}
	hostname := "unknown"
	h, err := os.Hostname()
	if err == nil {
		hostname = h
	}
	buffer := bytes.Buffer{}
	buffer.WriteString(ip)
	buffer.WriteString(hostname)
	buffer.WriteString(strconv.Itoa(os.Getpid()))
	// <security>
	randBytes := make([]byte, 64)
	_, err = rand.Reader.Read(randBytes)
	if err != nil {
		time.Sleep(2 * time.Second)
		randBytes = random.New(g.now().Unix()).Bytes(64)
	}
	buffer.Write(randBytes)
	g.head = sha256.Bytes(buffer.Bytes())
	g.wg.Add(1)
	go g.generateLoop()
	runtime.SetFinalizer(g, (*GUID).Close)
	return g
}

func (g *GUID) Get() []byte {
	guid := <-g.guidQueue
	// chan not closed
	if len(guid) != 0 {
		copy(guid[36:44], convert.Int64ToBytes(g.now().Unix()))
	}
	return guid
}

func (g *GUID) Close() {
	g.closeOnce.Do(func() {
		close(g.stopSignal)
		g.wg.Wait()
	})
}

func (g *GUID) generateLoop() {
	defer func() {
		if r := recover(); r != nil {
			// restart generateLoop
			time.Sleep(time.Second)
			go g.generateLoop()
		} else {
			close(g.guidQueue)
			g.wg.Done()
		}
	}()
	for {
		guid := make([]byte, Size)
		copy(guid, g.head)
		copy(guid[32:36], g.random.Bytes(4))
		// reserve timestamp
		copy(guid[44:52], convert.Uint64ToBytes(g.id))
		select {
		case <-g.stopSignal:
			return
		case g.guidQueue <- guid:
			g.id += 1
		}
	}
}
