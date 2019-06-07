package guid

import (
	"bytes"
	"net"
	"os"
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
const SIZE int = 52

type Generator struct {
	now         func() time.Time
	random      *random.Generator
	head        []byte
	id          uint64
	guid_queue  chan []byte
	stop_signal chan struct{}
	wg          sync.WaitGroup
	close_once  sync.Once
}

// if now is nil use time.Now
func New(size int, now func() time.Time) *Generator {
	g := &Generator{
		random:      random.New(),
		stop_signal: make(chan struct{}, 1),
	}
	if size < 1 {
		g.guid_queue = make(chan []byte, 1)
	} else {
		g.guid_queue = make(chan []byte, size)
	}
	if now != nil {
		g.now = now
	} else {
		g.now = time.Now
	}
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
	buffer.Write(random.New().Bytes(64)) // <security>
	g.head = sha256.Bytes(buffer.Bytes())
	g.wg.Add(1)
	go g.generate_loop()
	return g
}

func (this *Generator) Get() []byte {
	guid := <-this.guid_queue
	// chan not closed
	if len(guid) != 0 {
		copy(guid[36:44], convert.Int64_Bytes(this.now().Unix()))
	}
	return guid
}

func (this *Generator) Close() {
	this.close_once.Do(func() {
		this.stop_signal <- struct{}{}
		// clean and prevent block generate_loop()
		for range this.guid_queue {
		}
		this.wg.Wait()
	})
}

func (this *Generator) generate_loop() {
	for {
		select {
		case <-this.stop_signal:
			close(this.stop_signal)
			close(this.guid_queue)
			this.wg.Done()
			return
		default:
			guid := make([]byte, SIZE)
			copy(guid, this.head)
			copy(guid[32:36], this.random.Bytes(4))
			// timestamp
			copy(guid[44:52], convert.Uint64_Bytes(this.id))
			this.guid_queue <- guid
			this.id += 1
		}
	}
}
