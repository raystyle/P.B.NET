package controller

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"

	"project/internal/bootstrap"
	"project/internal/logger"
	"project/internal/xpanic"
)

type boot struct {
	ctx        *CTRL
	clients    map[string]*bClient // key = mBoot.Tag
	clientsRWM sync.RWMutex
	inClose    int32
}

func newBoot(ctx *CTRL) *boot {
	return &boot{
		ctx:     ctx,
		clients: make(map[string]*bClient),
	}
}

func (boot *boot) Add(m *mBoot) error {
	if boot.isClosed() {
		return errors.New("boot is closed")
	}
	boot.clientsRWM.Lock()
	defer boot.clientsRWM.Unlock()
	// check exist
	if _, ok := boot.clients[m.Tag]; ok {
		err := errors.Errorf("boot %s is running", m.Tag)
		boot.ctx.Printf(logger.Info, "boot", "add boot %s failed: %s", m.Tag, err)
		return err
	}
	// load
	g := boot.ctx.global
	b, err := bootstrap.Load(m.Mode, []byte(m.Config), g.proxyPool, g.dnsClient)
	if err != nil {
		err = errors.Wrapf(err, "load boot %s failed", m.Tag)
		boot.ctx.Printf(logger.Info, "boot", "add boot %s failed: %s", m.Tag, err)
		return err
	}
	bc := bClient{
		ctx:        boot,
		tag:        m.Tag,
		interval:   time.Duration(m.Interval) * time.Second,
		logSrc:     "boot-" + m.Tag,
		bootstrap:  b,
		stopSignal: make(chan struct{}),
	}
	boot.clients[m.Tag] = &bc
	bc.Boot()
	boot.ctx.Printf(logger.Info, "boot", "add boot %s", m.Tag)
	return nil
}

func (boot *boot) Delete(tag string) error {
	if boot.isClosed() {
		return errors.New("boot is closed")
	}
	boot.clientsRWM.Lock()
	defer boot.clientsRWM.Unlock()
	if client, ok := boot.clients[tag]; !ok {
		return errors.Errorf("boot: %s doesn't exist", tag)
	} else {
		client.Stop()
		delete(boot.clients, tag)
		return nil
	}
}

func (boot *boot) Close() {
	atomic.StoreInt32(&boot.inClose, 1)
	boot.clientsRWM.Lock()
	defer boot.clientsRWM.Unlock()
	for tag, client := range boot.clients {
		client.Stop()
		delete(boot.clients, tag)
	}
}

func (boot *boot) isClosed() bool {
	return atomic.LoadInt32(&boot.inClose) != 0
}

// TODO logger

// boot client
type bClient struct {
	ctx        *boot
	tag        string
	interval   time.Duration
	logSrc     string
	bootstrap  bootstrap.Bootstrap
	once       sync.Once
	stopSignal chan struct{}
	wg         sync.WaitGroup
}

func (bc *bClient) Boot() {
	bc.wg.Add(1)
	go bc.bootLoop()
}

func (bc *bClient) bootLoop() {
	var err error
	defer func() {
		if r := recover(); r != nil {
			err = xpanic.Error("bClient.bootLoop() panic:", r)
			bc.ctx.ctx.Print(logger.Fatal, bc.logSrc, err)
		}
		bc.ctx.ctx.Printf(logger.Info, "boot", "boot %s stop", bc.tag)
		bc.wg.Done()
	}()
	resolve := func() {
		err = bc.resolve()
		if err != nil {
			bc.ctx.ctx.Println(logger.Warning, bc.logSrc, err)
		} else {
			// stop and delete self
			close(bc.stopSignal)
			bc.ctx.clientsRWM.Lock()
			delete(bc.ctx.clients, bc.tag)
			bc.ctx.clientsRWM.Unlock()
		}
	}
	resolve()
	for {
		select {
		case <-time.After(bc.interval):
			resolve()
		case <-bc.stopSignal:
			return
		}
	}
}

func (bc *bClient) resolve() (err error) {
	nodes, err := bc.bootstrap.Resolve()
	if err != nil {
		return
	}
	// add syncer

	nodes[0] = nil
	return
}

func (bc *bClient) Stop() {
	bc.once.Do(func() {
		close(bc.stopSignal)
		bc.wg.Wait()
	})
}
