package msfrpc

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"project/internal/logger"
	"project/internal/security"
	"project/internal/xpanic"
	"project/internal/xsync"
)

// ErrIOManagerClosed is a error that to tell caller IOManager is closed.
var ErrIOManagerClosed = errors.New("io manager is closed")

// ioReader is used to wrap Console, Shell and Meterpreter.
// different reader(user) can get the same data, when new data read,
// it will call onRead() for notice user that can get new data.
type ioReader struct {
	logger  logger.Logger
	rc      io.ReadCloser
	onRead  func()
	onClose func()

	// store history output
	buf *bytes.Buffer
	rwm sync.RWMutex

	wg sync.WaitGroup
}

func newIOReader(lg logger.Logger, rc io.ReadCloser, onRead, onClose func()) *ioReader {
	return &ioReader{
		logger:  lg,
		rc:      rc,
		onRead:  onRead,
		onClose: onClose,
		buf:     bytes.NewBuffer(make([]byte, 0, 64)),
	}
}

// ReadLoop is used to start read data loop.
func (reader *ioReader) ReadLoop() {
	reader.wg.Add(1)
	go reader.readLoop()
}

func (reader *ioReader) readLoop() {
	defer reader.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			buf := xpanic.Print(r, "ioReader.readLoop")
			reader.logger.Println(logger.Fatal, "msfrpc-io reader", buf)
			// restart readLoop
			time.Sleep(time.Second)
			reader.wg.Add(1)
			go reader.readLoop()
			return
		}
		reader.onClose()
		// prevent cycle reference
		reader.onRead = nil
		reader.onClose = nil
	}()
	var (
		n   int
		err error
	)
	buf := make([]byte, 4096)
	for {
		n, err = reader.rc.Read(buf)
		if err != nil {
			return
		}
		reader.write(buf[:n])
		reader.onRead()
	}
}

func (reader *ioReader) write(b []byte) {
	reader.rwm.Lock()
	defer reader.rwm.Unlock()
	reader.buf.Write(b)
}

// Read is used to read buffer data [start:].
func (reader *ioReader) Read(start int) []byte {
	if start < 0 {
		start = 0
	}
	reader.rwm.RLock()
	defer reader.rwm.RUnlock()
	l := reader.buf.Len()
	if start >= l {
		return nil
	}
	b := make([]byte, l-start)
	copy(b, reader.buf.Bytes()[start:])
	return b
}

// Clean is used to clean buffer data.
func (reader *ioReader) Clean() {
	reader.rwm.Lock()
	defer reader.rwm.Unlock()
	// clean buffer data at once
	security.CoverBytes(reader.buf.Bytes())
	// alloc a new buffer
	reader.buf = bytes.NewBuffer(make([]byte, 0, 64))
}

// Close is used to close io reader, it will also close under ReadCloser.
func (reader *ioReader) Close() error {
	err := reader.rc.Close()
	reader.wg.Wait()
	return err
}

// IOObject contains under io object, io Reader and io status about io object.
// io status contains the status about the IO(console, shell and meterpreter).
// must use token for write data to io object, usually the admin can unlock it force.
type IOObject struct {
	object   io.Writer // *Console, *Shell and *Meterpreter
	reader   *ioReader
	onLock   func(token string)
	onUnlock func(token string)

	// status
	now    func() time.Time
	locker string // user token
	lockAt time.Time
	rwm    sync.RWMutex
}

// ToConsole is used to convert *IOObject to a *Console.
func (obj *IOObject) ToConsole() *Console {
	return obj.object.(*Console)
}

// ToShell is used to convert *IOObject to a *Shell.
func (obj *IOObject) ToShell() *Shell {
	return obj.object.(*Shell)
}

// ToMeterpreter is used to convert *IOObject to a *Meterpreter.
func (obj *IOObject) ToMeterpreter() *Meterpreter {
	return obj.object.(*Meterpreter)
}

// Lock is used to lock io object write.
func (obj *IOObject) Lock(token string) bool {
	obj.rwm.Lock()
	defer obj.rwm.Unlock()
	if obj.locker == "" {
		obj.locker = token
		obj.lockAt = obj.now()
		obj.onLock(token)
		return true
	}
	return false
}

// Unlock is used to unlock io object write.
func (obj *IOObject) Unlock(token string) bool {
	obj.rwm.Lock()
	defer obj.rwm.Unlock()
	if obj.locker == "" {
		return true
	}
	if obj.locker == token {
		obj.locker = ""
		obj.onUnlock(token)
		return true
	}
	return false
}

// ForceUnlock is used to clean locker force, usually only admin can call it.
func (obj *IOObject) ForceUnlock(token string) {
	obj.rwm.Lock()
	defer obj.rwm.Unlock()
	obj.locker = ""
	obj.onUnlock(token)
}

// Locker is used to return locker token for find lock user.
func (obj *IOObject) Locker() string {
	obj.rwm.RLock()
	defer obj.rwm.RUnlock()
	return obj.locker
}

// LockAt is used to get the time about lock object.
func (obj *IOObject) LockAt() time.Time {
	obj.rwm.RLock()
	defer obj.rwm.RUnlock()
	return obj.lockAt
}

// checkToken is used to check is user that lock this object.
func (obj *IOObject) checkToken(token string) bool {
	locker := obj.Locker()
	if locker != "" && token != locker {
		return false
	}
	return true
}

// Read is used to read data from the under io reader.
func (obj *IOObject) Read(start int) []byte {
	return obj.reader.Read(start)
}

// Write is used to write data to the under io object,
// if token is correct, it will failed.
func (obj *IOObject) Write(token string, data []byte) error {
	if !obj.checkToken(token) {
		return errors.New("another user lock it")
	}
	_, err := obj.object.Write(data)
	return err
}

// Clean is used to clean buffer in the under io reader.
func (obj *IOObject) Clean(token string) error {
	if !obj.checkToken(token) {
		return errors.New("another user lock it")
	}
	obj.reader.Clean()
	return nil
}

// Close is used to close the under io reader.
func (obj *IOObject) Close() error {
	return obj.reader.Close()
}

// IOEventHandlers contains callbacks about io objects events.
type IOEventHandlers struct {
	OnConsoleRead     func(id string)
	OnConsoleClosed   func(id string)
	OnConsoleLocked   func(id, token string)
	OnConsoleUnlocked func(id, token string)

	OnShellRead     func(id uint64)
	OnShellClosed   func(id uint64)
	OnShellLocked   func(id uint64, token string)
	OnShellUnlocked func(id uint64, token string)

	OnMeterpreterRead     func(id uint64)
	OnMeterpreterClosed   func(id uint64)
	OnMeterpreterLocked   func(id uint64, token string)
	OnMeterpreterUnlocked func(id uint64, token string)
}

// IOManager is used to manage Console IO, Shell session IO and Meterpreter session IO.
// It can lock IO instance for only one user can write data to it, other user can read it
// with io reader, other user can only destroy it(Console) or kill session(Shell or Meterpreter).
type IOManager struct {
	ctx      *Client
	handlers *IOEventHandlers
	interval time.Duration
	now      func() time.Time

	consoles     map[string]*IOObject // key = console id
	shells       map[uint64]*IOObject // key = shell session id
	meterpreters map[uint64]*IOObject // key = meterpreter session id
	inShutdown   int32
	rwm          sync.RWMutex

	// io object counter
	counter xsync.Counter
}

// IOManagerOptions contains options about io manager.
type IOManagerOptions struct {
	Interval time.Duration `toml:"interval"`

	// inner usage
	Now func() time.Time `toml:"-" msgpack:"-"`
}

// NewIOManager is used to create a new IO manager.
func NewIOManager(client *Client, handlers *IOEventHandlers, opts *IOManagerOptions) *IOManager {
	if opts == nil {
		opts = new(IOManagerOptions)
	}
	interval := opts.Interval
	if interval < minReadInterval {
		interval = minWatchInterval
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &IOManager{
		ctx:          client,
		handlers:     handlers,
		interval:     interval,
		now:          now,
		consoles:     make(map[string]*IOObject),
		shells:       make(map[uint64]*IOObject),
		meterpreters: make(map[uint64]*IOObject),
	}
}

func (mgr *IOManager) shuttingDown() bool {
	return atomic.LoadInt32(&mgr.inShutdown) != 0
}

func (mgr *IOManager) trackConsole(console *IOObject, add bool) bool {
	id := console.ToConsole().id
	mgr.rwm.Lock()
	defer mgr.rwm.Unlock()
	if add {
		if mgr.shuttingDown() {
			return false
		}
		mgr.consoles[id] = console
		mgr.counter.Add(1)
	} else {
		delete(mgr.consoles, id)
		mgr.counter.Done()
	}
	return true
}

// NewConsole is used to create a new console with status, All users can read or write.
func (mgr *IOManager) NewConsole(ctx context.Context, workspace string) (string, error) {
	if mgr.shuttingDown() {
		return "", ErrIOManagerClosed
	}
	console, err := mgr.ctx.NewConsole(ctx, workspace, mgr.interval)
	if err != nil {
		return "", err
	}
	obj := &IOObject{
		object: console,
		now:    mgr.now,
	}
	onRead := func() {
		mgr.handlers.OnConsoleRead(console.id)
	}
	onClose := func() {
		mgr.handlers.OnConsoleClosed(console.id)
		mgr.trackConsole(obj, false)
	}
	obj.onLock = func(token string) {
		mgr.handlers.OnConsoleLocked(console.id, token)
	}
	obj.onUnlock = func(token string) {
		mgr.handlers.OnConsoleUnlocked(console.id, token)
	}
	reader := newIOReader(mgr.ctx.logger, console, onRead, onClose)
	// must track first.
	if !mgr.trackConsole(obj, true) {
		return "", ErrIOManagerClosed
	}
	// prevent cycle reference.
	obj.reader = reader
	reader.ReadLoop()
	return console.id, nil
}

// NewConsoleWithLocker is used to create a new console and lock it.
// Only the creator can write it. It will create a new under console.
func (mgr *IOManager) NewConsoleWithLocker() {

}

// NewConsoleWithID is used to create a new console, All user can read or write.
// It will not create a new under console.
func (mgr *IOManager) NewConsoleWithID() {
	// TODO check is exist
}

// NewConsoleWithIDAndLocker is used to create a new console with id and lock write.
// Only the creator can write it. It will not create a new under console.
func (mgr *IOManager) NewConsoleWithIDAndLocker() {
	// TODO check is exist
}

// ConsoleLock is used to lock write for console that only one user
// can write to this console.
func (mgr *IOManager) ConsoleLock() {

}

// ConsoleUnlock is used to unlock write for console that all user
// can write to this console.
func (mgr *IOManager) ConsoleUnlock() {

}

// ConsoleForceUnlock is used to unlock write for console that all user
// can write to this console, it will not check the token.
func (mgr *IOManager) ConsoleForceUnlock() {

}

// ConsoleRead is used to read data from console, it will check token.
func (mgr *IOManager) ConsoleRead() {

}

// ConsoleWrite is used to write data to console, it will check token.
func (mgr *IOManager) ConsoleWrite() {

}

// ConsoleSessionDetach is used to detach session in console, it will check token.
func (mgr *IOManager) ConsoleSessionDetach() {

}

// ConsoleSessionKill is used to kill session in console, it will check token.
func (mgr *IOManager) ConsoleSessionKill() {

}

// NewShell is used to create a new shell with IO status.
func (mgr *IOManager) NewShell() {
	// TODO check is exist
}

// NewShellAndLockWrite is used to create a new shell with IO status and lock write.
// Only the creator can write it.
func (mgr *IOManager) NewShellAndLockWrite() {
	// TODO check is exist
}

// NewShellAndLockRW is used to create a new shell and lock read and write.
// Only the creator can read and write it.
func (mgr *IOManager) NewShellAndLockRW() {
	// TODO check is exist
}

// ShellLockWrite is used to lock write for shell that only one user
// can write to this shell.
func (mgr *IOManager) ShellLockWrite() {

}

// ShellUnlockWrite is used to unlock write for shell that all user
// can write to this shell.
func (mgr *IOManager) ShellUnlockWrite() {

}

// ShellLockRW is used to lock read and write for shell that only one user
// can read or write to this shell. (single mode)
func (mgr *IOManager) ShellLockRW() {

}

// ShellUnLockRW is used to unlock read and write for shell that all user
// can read or write to this shell. (common mode)
func (mgr *IOManager) ShellUnLockRW() {

}

// ShellForceUnlockWrite is used to unlock write for shell that all user
// can write to this shell, it will not check the token.
func (mgr *IOManager) ShellForceUnlockWrite() {

}

// ShellForceUnLockRW is used to unlock write for shell that all user
// can write to this shell, it will not check the token.
func (mgr *IOManager) ShellForceUnLockRW() {

}

// ShellRead is used to read data from shell, it will check token.
func (mgr *IOManager) ShellRead() {

}

// ShellWrite is used to write data to shell, it will check token.
func (mgr *IOManager) ShellWrite() {

}

// NewMeterpreter is used to create a new meterpreter with IO status.
func (mgr *IOManager) NewMeterpreter() {
	// TODO check is exist
}

// NewMeterpreterAndLockWrite is used to create a new meterpreter with IO status and lock write.
// Only the creator can write it.
func (mgr *IOManager) NewMeterpreterAndLockWrite() {
	// TODO check is exist
}

// NewMeterpreterAndLockRW is used to create a new meterpreter and lock read and write.
// Only the creator can read and write it.
func (mgr *IOManager) NewMeterpreterAndLockRW() {
	// TODO check is exist
}

// MeterpreterLockWrite is used to lock write for meterpreter that only one user
// can write to this meterpreter.
func (mgr *IOManager) MeterpreterLockWrite() {

}

// MeterpreterUnlockWrite is used to unlock write for meterpreter that all user
// can write to this meterpreter.
func (mgr *IOManager) MeterpreterUnlockWrite() {

}

// MeterpreterLockRW is used to lock read and write for meterpreter that only one user
// can read or write to this meterpreter. (single mode)
func (mgr *IOManager) MeterpreterLockRW() {

}

// MeterpreterUnLockRW is used to unlock read and write for meterpreter that all user
// can read or write to this meterpreter. (common mode)
func (mgr *IOManager) MeterpreterUnLockRW() {

}

// MeterpreterForceUnlockWrite is used to unlock write for meterpreter that all user
// can write to this meterpreter, it will not check the token.
func (mgr *IOManager) MeterpreterForceUnlockWrite() {

}

// MeterpreterForceUnLockRW is used to unlock write for meterpreter that all user
// can write to this meterpreter, it will not check the token.
func (mgr *IOManager) MeterpreterForceUnLockRW() {

}

// MeterpreterRead is used to read data from meterpreter, it will check token.
func (mgr *IOManager) MeterpreterRead() {

}

// MeterpreterWrite is used to write data to meterpreter, it will check token.
func (mgr *IOManager) MeterpreterWrite() {

}

// Close is used to close IOManager, it will close all io objects.
func (mgr *IOManager) Close() error {
	atomic.StoreInt32(&mgr.inShutdown, 1)
	var err error
	mgr.rwm.Lock()
	defer mgr.rwm.Unlock()
	for id, console := range mgr.consoles {
		e := console.Close()
		if e != nil && err == nil {
			err = e
		}
		delete(mgr.consoles, id)
	}
	for id, shell := range mgr.shells {
		e := shell.Close()
		if e != nil && err == nil {
			err = e
		}
		delete(mgr.shells, id)
	}
	for id, meterpreter := range mgr.meterpreters {
		e := meterpreter.Close()
		if e != nil && err == nil {
			err = e
		}
		delete(mgr.meterpreters, id)
	}
	mgr.counter.Wait()
	return err
}
