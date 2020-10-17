package msfrpc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"

	"project/internal/logger"
	"project/internal/security"
	"project/internal/xpanic"
	"project/internal/xsync"
)

// about errors
var (
	ErrIOManagerClosed   = fmt.Errorf("io manager is closed")
	ErrAnotherUserLocked = fmt.Errorf("another user lock it")
)

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

// close will not call wait group.
func (reader *ioReader) close() error {
	return reader.rc.Close()
}

// IOObject contains under io object, io Reader and io status about io object.
// io status contains the status about the IO(console, shell and meterpreter).
// must use token for write data to io object, usually the admin can unlock it force.
type IOObject struct {
	object   io.Writer // *Console, *Shell and *Meterpreter
	reader   *ioReader
	now      func() time.Time
	onLock   func(token string)
	onUnlock func(token string)

	// status
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

// ForceUnlock is used to clean locker force, usually
// only admin cal call it or occur some error.
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
		return ErrAnotherUserLocked
	}
	_, err := obj.object.Write(data)
	return err
}

// Clean is used to clean buffer in the under io reader.
func (obj *IOObject) Clean(token string) error {
	if !obj.checkToken(token) {
		return ErrAnotherUserLocked
	}
	obj.reader.Clean()
	return nil
}

// Close is used to close the under io reader.
func (obj *IOObject) Close() error {
	return obj.reader.Close()
}

// close will not call wait group, IOManager will call it.
func (obj *IOObject) close() error {
	return obj.reader.close()
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
		interval = minReadInterval
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

func (mgr *IOManager) trackShell(shell *IOObject, add bool) bool {
	id := shell.ToShell().id
	mgr.rwm.Lock()
	defer mgr.rwm.Unlock()
	if add {
		if mgr.shuttingDown() {
			return false
		}
		mgr.shells[id] = shell
		mgr.counter.Add(1)
	} else {
		delete(mgr.shells, id)
		mgr.counter.Done()
	}
	return true
}

func (mgr *IOManager) trackMeterpreter(meterpreter *IOObject, add bool) bool {
	id := meterpreter.ToMeterpreter().id
	mgr.rwm.Lock()
	defer mgr.rwm.Unlock()
	if add {
		if mgr.shuttingDown() {
			return false
		}
		mgr.meterpreters[id] = meterpreter
		mgr.counter.Add(1)
	} else {
		delete(mgr.meterpreters, id)
		mgr.counter.Done()
	}
	return true
}

// Consoles is used to get consoles that IOManager has attached.
func (mgr *IOManager) Consoles() map[string]*IOObject {
	mgr.rwm.RLock()
	defer mgr.rwm.RUnlock()
	consoles := make(map[string]*IOObject, len(mgr.consoles))
	for id, console := range mgr.consoles {
		consoles[id] = console
	}
	return consoles
}

// Shells is used to get shells that IOManager has attached.
func (mgr *IOManager) Shells() map[uint64]*IOObject {
	mgr.rwm.RLock()
	defer mgr.rwm.RUnlock()
	shells := make(map[uint64]*IOObject, len(mgr.shells))
	for id, shell := range mgr.shells {
		shells[id] = shell
	}
	return shells
}

// Meterpreters is used to get meterpreters that IOManager has attached.
func (mgr *IOManager) Meterpreters() map[uint64]*IOObject {
	mgr.rwm.RLock()
	defer mgr.rwm.RUnlock()
	meterpreters := make(map[uint64]*IOObject, len(mgr.meterpreters))
	for id, meterpreter := range mgr.meterpreters {
		meterpreters[id] = meterpreter
	}
	return meterpreters
}

// GetConsole is used to get console by ID.
func (mgr *IOManager) GetConsole(id string) (*IOObject, error) {
	mgr.rwm.RLock()
	defer mgr.rwm.RUnlock()
	if console, ok := mgr.consoles[id]; ok {
		return console, nil
	}
	return nil, errors.Errorf("console %s is not exist", id)
}

// GetShell is used to get shell by ID.
func (mgr *IOManager) GetShell(id uint64) (*IOObject, error) {
	mgr.rwm.RLock()
	defer mgr.rwm.RUnlock()
	if shell, ok := mgr.shells[id]; ok {
		return shell, nil
	}
	return nil, errors.Errorf("shell %d is not exist", id)
}

// GetConsole is used to get console by ID.
func (mgr *IOManager) GetMeterpreter(id uint64) (*IOObject, error) {
	mgr.rwm.RLock()
	defer mgr.rwm.RUnlock()
	if meterpreter, ok := mgr.meterpreters[id]; ok {
		return meterpreter, nil
	}
	return nil, errors.Errorf("meterpreter %d is not exist", id)
}

// NewConsole is used to create a new console and wrap it to IOObject,
// all users can read or write. It will create a new under console.
func (mgr *IOManager) NewConsole(ctx context.Context, workspace string) (*IOObject, error) {
	return mgr.NewConsoleWithLocker(ctx, workspace, "")
}

// NewConsoleWithLocker is used to create a new console and lock it.
// Only the creator can write it. It will create a new under console.
func (mgr *IOManager) NewConsoleWithLocker(ctx context.Context, workspace, token string) (*IOObject, error) {
	if mgr.shuttingDown() {
		return nil, ErrIOManagerClosed
	}
	console, err := mgr.ctx.NewConsole(ctx, workspace, mgr.interval)
	if err != nil {
		return nil, err
	}
	return mgr.createConsoleIOObject(console, token)
}

// NewConsoleWithID is used to wrap an existing console and wrap it to IOObject,
// all users can read or write. It will not create a new under console.
// usually it used to wrap an idle console.
func (mgr *IOManager) NewConsoleWithID(ctx context.Context, id string) (*IOObject, error) {
	return mgr.NewConsoleWithIDAndLocker(ctx, id, "")
}

// NewConsoleWithIDAndLocker is used to create a new console with id and lock write.
// Only the creator can write it. It will not create a new under console.
func (mgr *IOManager) NewConsoleWithIDAndLocker(ctx context.Context, id, token string) (*IOObject, error) {
	if mgr.shuttingDown() {
		return nil, ErrIOManagerClosed
	}
	err := mgr.checkConsoleID(ctx, id)
	if err != nil {
		return nil, err
	}
	console := mgr.ctx.NewConsoleWithID(id, mgr.interval)
	return mgr.createConsoleIOObject(console, token)
}

func (mgr *IOManager) createConsoleIOObject(console *Console, token string) (*IOObject, error) {
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
	obj.reader = newIOReader(mgr.ctx.logger, console, onRead, onClose)
	onLock := func(token string) {
		mgr.handlers.OnConsoleLocked(console.id, token)
	}
	onUnlock := func(token string) {
		mgr.handlers.OnConsoleUnlocked(console.id, token)
	}
	obj.onLock = onLock
	obj.onUnlock = onUnlock
	obj.Lock(token)
	// must track first.
	if !mgr.trackConsole(obj, true) {
		// if track failed must deference reader,
		// otherwise it will occur cycle reference
		obj.reader = nil
		return nil, ErrIOManagerClosed
	}
	obj.reader.ReadLoop()
	return obj, nil
}

func (mgr *IOManager) checkConsoleID(ctx context.Context, id string) error {
	consoles, err := mgr.ctx.ConsoleList(ctx)
	if err != nil {
		return err
	}
	for _, console := range consoles {
		if console.ID == id {
			return nil
		}
	}
	return errors.Errorf("console %s is not exist", id)
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

}

// NewShellAndLockWrite is used to create a new shell with IO status and lock write.
// Only the creator can write it.
func (mgr *IOManager) NewShellAndLockWrite() {

}

// NewShellAndLockRW is used to create a new shell and lock read and write.
// Only the creator can read and write it.
func (mgr *IOManager) NewShellAndLockRW() {

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

}

// NewMeterpreterAndLockWrite is used to create a new meterpreter with IO status and lock write.
// Only the creator can write it.
func (mgr *IOManager) NewMeterpreterAndLockWrite() {

}

// NewMeterpreterAndLockRW is used to create a new meterpreter and lock read and write.
// Only the creator can read and write it.
func (mgr *IOManager) NewMeterpreterAndLockRW() {

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
	err := mgr.close()
	mgr.counter.Wait()
	return err
}

func (mgr *IOManager) close() error {
	atomic.StoreInt32(&mgr.inShutdown, 1)
	var err error
	mgr.rwm.Lock()
	defer mgr.rwm.Unlock()
	// close all consoles
	for id, console := range mgr.consoles {
		e := console.close()
		if e != nil && err == nil {
			err = e
		}
		console.reader = nil
		delete(mgr.consoles, id)
	}
	// close all shells
	for id, shell := range mgr.shells {
		e := shell.close()
		if e != nil && err == nil {
			err = e
		}
		shell.reader = nil
		delete(mgr.shells, id)
	}
	// close all meterpreters
	for id, meterpreter := range mgr.meterpreters {
		e := meterpreter.close()
		if e != nil && err == nil {
			err = e
		}
		meterpreter.reader = nil
		delete(mgr.meterpreters, id)
	}
	return err
}
