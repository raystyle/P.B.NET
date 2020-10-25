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
	ErrInvalidLockToken  = fmt.Errorf("invalid lock token")
)

// ioReader is used to wrap Console, Shell and Meterpreter.
// different reader(user) can get the same data, when new data read,
// it will call onRead() for notice user that can get new data.
type ioReader struct {
	logger  logger.Logger
	rc      io.ReadCloser
	onRead  func()
	onClean func()
	onClose func()

	// store history output
	buf *bytes.Buffer
	rwm sync.RWMutex

	wg sync.WaitGroup
}

func newIOReader(lg logger.Logger, rc io.ReadCloser, onRead, onClean, onClose func()) *ioReader {
	return &ioReader{
		logger:  lg,
		rc:      rc,
		onRead:  onRead,
		onClean: onClean,
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
		// must use lock
		reader.rwm.Lock()
		defer reader.rwm.Unlock()
		reader.onClean = nil
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

// Read is used to read buffer data with offset [offset:].
func (reader *ioReader) Read(offset int) []byte {
	const maxChunkSize = 32 * 1024
	if offset < 0 {
		offset = 0
	}
	reader.rwm.RLock()
	defer reader.rwm.RUnlock()
	l := reader.buf.Len()
	if offset >= l {
		return nil
	}
	size := l - offset
	if size > maxChunkSize {
		size = maxChunkSize
	}
	b := make([]byte, size)
	copy(b, reader.buf.Bytes()[offset:offset+size])
	return b
}

// Clean is used to clean buffer data.
func (reader *ioReader) Clean() {
	cleanFn := reader.clean()
	if cleanFn != nil {
		cleanFn()
	}
}

func (reader *ioReader) clean() func() {
	reader.rwm.Lock()
	defer reader.rwm.Unlock()
	// clean buffer data at once
	security.CoverBytes(reader.buf.Bytes())
	// alloc a new buffer
	reader.buf = bytes.NewBuffer(make([]byte, 0, 64))
	// must use lock or may be call nil onClean
	return reader.onClean
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
	if obj.locker == token {
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

// CheckToken is used to check is user that lock this object.
func (obj *IOObject) CheckToken(token string) bool {
	locker := obj.Locker()
	if locker != "" && token != locker {
		return false
	}
	return true
}

// Read is used to read data from the under io reader.
func (obj *IOObject) Read(offset int) []byte {
	return obj.reader.Read(offset)
}

// Write is used to write data to the under io object,
// if token is correct, it will failed.
func (obj *IOObject) Write(token string, data []byte) error {
	if !obj.CheckToken(token) {
		return ErrAnotherUserLocked
	}
	_, err := obj.object.Write(data)
	return err
}

// Clean is used to clean buffer in the under io reader.
func (obj *IOObject) Clean(token string) error {
	if !obj.CheckToken(token) {
		return ErrAnotherUserLocked
	}
	obj.reader.Clean()
	return nil
}

// Close is used to close the under io reader.
func (obj *IOObject) Close(token string) error {
	if !obj.CheckToken(token) {
		return ErrAnotherUserLocked
	}
	return obj.reader.Close()
}

// close will not call wait group, IOManager will call it.
func (obj *IOObject) close() error {
	return obj.reader.close()
}

// IOEventHandlers contains callbacks about io objects events.
type IOEventHandlers struct {
	OnConsoleRead     func(id string)
	OnConsoleClean    func(id string)
	OnConsoleClosed   func(id string)
	OnConsoleLocked   func(id, token string)
	OnConsoleUnlocked func(id, token string)

	OnShellRead     func(id uint64)
	OnShellClean    func(id uint64)
	OnShellClosed   func(id uint64)
	OnShellLocked   func(id uint64, token string)
	OnShellUnlocked func(id uint64, token string)

	OnMeterpreterRead     func(id uint64)
	OnMeterpreterClean    func(id uint64)
	OnMeterpreterClosed   func(id uint64)
	OnMeterpreterLocked   func(id uint64, token string)
	OnMeterpreterUnlocked func(id uint64, token string)
}

// IOManagerOptions contains options about io manager.
type IOManagerOptions struct {
	// Interval is the IO object read loop interval
	Interval time.Duration `toml:"interval"`

	// Now is used to set lock at time [inner usage]
	Now func() time.Time `toml:"-" msgpack:"-"`
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

	counter xsync.Counter // io object counter
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

func (mgr *IOManager) logf(lv logger.Level, format string, log ...interface{}) {
	if mgr.shuttingDown() {
		return
	}
	mgr.ctx.logger.Printf(lv, "msfrpc-io manager", format, log...)
}

func (mgr *IOManager) log(lv logger.Level, log ...interface{}) {
	if mgr.shuttingDown() {
		return
	}
	mgr.ctx.logger.Println(lv, "msfrpc-io manager", log...)
}

func (mgr *IOManager) trackConsole(console *IOObject, add bool) error {
	id := console.ToConsole().ID()
	mgr.rwm.Lock()
	defer mgr.rwm.Unlock()
	if add {
		if mgr.shuttingDown() {
			return ErrIOManagerClosed
		}
		if _, ok := mgr.consoles[id]; ok {
			return errors.Errorf("console %s is already being tracked", id)
		}
		mgr.consoles[id] = console
		mgr.counter.Add(1)
	} else {
		delete(mgr.consoles, id)
		mgr.counter.Done()
	}
	return nil
}

func (mgr *IOManager) trackShell(shell *IOObject, add bool) error {
	id := shell.ToShell().ID()
	mgr.rwm.Lock()
	defer mgr.rwm.Unlock()
	if add {
		if mgr.shuttingDown() {
			return ErrIOManagerClosed
		}
		if _, ok := mgr.shells[id]; ok {
			return errors.Errorf("shell session %d is already being tracked", id)
		}
		mgr.shells[id] = shell
		mgr.counter.Add(1)
	} else {
		delete(mgr.shells, id)
		mgr.counter.Done()
	}
	return nil
}

func (mgr *IOManager) trackMeterpreter(meterpreter *IOObject, add bool) error {
	id := meterpreter.ToMeterpreter().ID()
	mgr.rwm.Lock()
	defer mgr.rwm.Unlock()
	if add {
		if mgr.shuttingDown() {
			return ErrIOManagerClosed
		}
		if _, ok := mgr.meterpreters[id]; ok {
			return errors.Errorf("meterpreter session %d is already being tracked", id)
		}
		mgr.meterpreters[id] = meterpreter
		mgr.counter.Add(1)
	} else {
		delete(mgr.meterpreters, id)
		mgr.counter.Done()
	}
	return nil
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

// Shells is used to get shell sessions that IOManager has attached.
func (mgr *IOManager) Shells() map[uint64]*IOObject {
	mgr.rwm.RLock()
	defer mgr.rwm.RUnlock()
	shells := make(map[uint64]*IOObject, len(mgr.shells))
	for id, shell := range mgr.shells {
		shells[id] = shell
	}
	return shells
}

// Meterpreters is used to get meterpreter sessions that IOManager has attached.
func (mgr *IOManager) Meterpreters() map[uint64]*IOObject {
	mgr.rwm.RLock()
	defer mgr.rwm.RUnlock()
	meterpreters := make(map[uint64]*IOObject, len(mgr.meterpreters))
	for id, meterpreter := range mgr.meterpreters {
		meterpreters[id] = meterpreter
	}
	return meterpreters
}

// GetConsole is used to get console by id.
func (mgr *IOManager) GetConsole(id string) (*IOObject, error) {
	mgr.rwm.RLock()
	defer mgr.rwm.RUnlock()
	if console, ok := mgr.consoles[id]; ok {
		return console, nil
	}
	return nil, errors.Errorf("console %s is not exist", id)
}

// GetShell is used to get shell session by id.
func (mgr *IOManager) GetShell(id uint64) (*IOObject, error) {
	mgr.rwm.RLock()
	defer mgr.rwm.RUnlock()
	if shell, ok := mgr.shells[id]; ok {
		return shell, nil
	}
	return nil, errors.Errorf("shell session %d is not exist", id)
}

// GetMeterpreter is used to get meterpreter session by id.
func (mgr *IOManager) GetMeterpreter(id uint64) (*IOObject, error) {
	mgr.rwm.RLock()
	defer mgr.rwm.RUnlock()
	if meterpreter, ok := mgr.meterpreters[id]; ok {
		return meterpreter, nil
	}
	return nil, errors.Errorf("meterpreter session %d is not exist", id)
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

func (mgr *IOManager) checkSessionID(ctx context.Context, typ string, id uint64) error {
	sessions, err := mgr.ctx.SessionList(ctx)
	if err != nil {
		return err
	}
	for sid, session := range sessions {
		if session.Type == typ && sid == id {
			return nil
		}
	}
	return errors.Errorf("%s session %d is not exist", typ, id)
}

// ------------------------------------------about console-----------------------------------------

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
	var ok bool
	defer func() {
		// if track failed, close created console.
		if ok {
			return
		}
		err := console.Destroy()
		if err != nil {
			id := console.ID()
			mgr.logf(logger.Warning, "appear error when close created console %s: %s", id, err)
		}
	}()
	obj, err := mgr.createConsoleIOObject(console, token)
	if err != nil {
		return nil, err
	}
	ok = true
	return obj, nil
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
	var ok bool
	defer func() {
		// if track failed, close console(not destroy under console).
		if ok {
			return
		}
		err := console.Close()
		if err != nil {
			mgr.logf(logger.Warning, "appear error when close console %s: %s", id, err)
		}
	}()
	obj, err := mgr.createConsoleIOObject(console, token)
	if err != nil {
		return nil, err
	}
	ok = true
	return obj, nil
}

func (mgr *IOManager) createConsoleIOObject(console *Console, token string) (*IOObject, error) {
	id := console.ID()
	obj := &IOObject{
		object: console,
		now:    mgr.now,
	}
	onRead := func() {
		mgr.handlers.OnConsoleRead(id)
	}
	onClean := func() {
		mgr.handlers.OnConsoleClean(id)
	}
	onClose := func() {
		mgr.handlers.OnConsoleClosed(id)
		_ = mgr.trackConsole(obj, false)
	}
	obj.reader = newIOReader(mgr.ctx.logger, console, onRead, onClean, onClose)
	onLock := func(token string) {
		mgr.handlers.OnConsoleLocked(id, token)
	}
	onUnlock := func(token string) {
		mgr.handlers.OnConsoleUnlocked(id, token)
	}
	obj.onLock = onLock
	obj.onUnlock = onUnlock
	if token != "" {
		obj.locker = token
		obj.lockAt = mgr.now()
	}
	// must track first.
	err := mgr.trackConsole(obj, true)
	if err != nil {
		// if track failed must deference reader,
		// otherwise it will occur cycle reference
		obj.reader = nil
		return nil, err
	}
	obj.reader.ReadLoop()
	return obj, nil
}

// ConsoleLock is used to lock console that only one user
// can operate this console, other user can only read.
func (mgr *IOManager) ConsoleLock(id, token string) error {
	console, err := mgr.GetConsole(id)
	if err != nil {
		return err
	}
	if console.Lock(token) {
		return nil
	}
	return ErrAnotherUserLocked
}

// ConsoleUnlock is used to unlock console.
func (mgr *IOManager) ConsoleUnlock(id, token string) error {
	console, err := mgr.GetConsole(id)
	if err != nil {
		return err
	}
	if console.Unlock(token) {
		return nil
	}
	return ErrInvalidLockToken
}

// ConsoleForceUnlock is used to force unlock console, it will not check the token.
func (mgr *IOManager) ConsoleForceUnlock(id, token string) error {
	console, err := mgr.GetConsole(id)
	if err != nil {
		return err
	}
	console.ForceUnlock(token)
	return nil
}

// ConsoleRead is used to read data from console.
func (mgr *IOManager) ConsoleRead(id string, offset int) ([]byte, error) {
	console, err := mgr.GetConsole(id)
	if err != nil {
		return nil, err
	}
	return console.Read(offset), nil
}

// ConsoleWrite is used to write data to console.
func (mgr *IOManager) ConsoleWrite(id, token string, data []byte) error {
	console, err := mgr.GetConsole(id)
	if err != nil {
		return err
	}
	return console.Write(token, data)
}

// ConsoleClean is used to clean buffer in under reader.
func (mgr *IOManager) ConsoleClean(id, token string) error {
	console, err := mgr.GetConsole(id)
	if err != nil {
		return err
	}
	return console.Clean(token)
}

// ConsoleClose is used to close console io object, it will not destroy the under console.
func (mgr *IOManager) ConsoleClose(id, token string) error {
	console, err := mgr.GetConsole(id)
	if err != nil {
		return err
	}
	return console.Close(token)
}

// ConsoleDetach is used to detach current console.
func (mgr *IOManager) ConsoleDetach(ctx context.Context, id, token string) error {
	console, err := mgr.GetConsole(id)
	if err != nil {
		return err
	}
	if !console.CheckToken(token) {
		return ErrAnotherUserLocked
	}
	return console.ToConsole().Detach(ctx)
}

// ConsoleInterrupt is used to send interrupt to current console.
func (mgr *IOManager) ConsoleInterrupt(ctx context.Context, id, token string) error {
	console, err := mgr.GetConsole(id)
	if err != nil {
		return err
	}
	if !console.CheckToken(token) {
		return ErrAnotherUserLocked
	}
	return console.ToConsole().Interrupt(ctx)
}

// ConsoleDestroy is used to destroy the under console, it will close io object first.
func (mgr *IOManager) ConsoleDestroy(id, token string) error {
	console, err := mgr.GetConsole(id)
	if err != nil {
		return err
	}
	err = console.Close(token)
	if err != nil {
		return err
	}
	return console.ToConsole().Destroy()
}

// -------------------------------------------about shell------------------------------------------

// NewShell is used to wrap an existing shell session and wrap it to IOObject.
func (mgr *IOManager) NewShell(ctx context.Context, id uint64) (*IOObject, error) {
	return mgr.NewShellWithLocker(ctx, id, "")
}

// NewShellWithLocker is used to wrap an existing shell session and wrap it to IOObject with locker.
func (mgr *IOManager) NewShellWithLocker(ctx context.Context, id uint64, token string) (*IOObject, error) {
	if mgr.shuttingDown() {
		return nil, ErrIOManagerClosed
	}
	err := mgr.checkSessionID(ctx, "shell", id)
	if err != nil {
		return nil, err
	}
	shell := mgr.ctx.NewShell(id, mgr.interval)
	var ok bool
	defer func() {
		// if track failed, close shell(not close under shell session)
		if ok {
			return
		}
		err := shell.Close()
		if err != nil {
			mgr.logf(logger.Warning, "appear error when close shell session %d: %s", id, err)
		}
	}()
	obj := &IOObject{
		object: shell,
		now:    mgr.now,
	}
	onRead := func() {
		mgr.handlers.OnShellRead(id)
	}
	onClean := func() {
		mgr.handlers.OnShellClean(id)
	}
	onClose := func() {
		mgr.handlers.OnShellClosed(id)
		_ = mgr.trackShell(obj, false)
	}
	obj.reader = newIOReader(mgr.ctx.logger, shell, onRead, onClean, onClose)
	onLock := func(token string) {
		mgr.handlers.OnShellLocked(id, token)
	}
	onUnlock := func(token string) {
		mgr.handlers.OnShellUnlocked(id, token)
	}
	obj.onLock = onLock
	obj.onUnlock = onUnlock
	if token != "" {
		obj.locker = token
		obj.lockAt = mgr.now()
	}
	// must track first.
	err = mgr.trackShell(obj, true)
	if err != nil {
		// if track failed must deference reader,
		// otherwise it will occur cycle reference
		obj.reader = nil
		return nil, err
	}
	obj.reader.ReadLoop()
	ok = true
	return obj, nil
}

// ShellLock is used to lock shell that only one user
// can operate this shell, other user can only read.
func (mgr *IOManager) ShellLock(id uint64, token string) error {
	shell, err := mgr.GetShell(id)
	if err != nil {
		return err
	}
	if shell.Lock(token) {
		return nil
	}
	return ErrAnotherUserLocked
}

// ShellUnlock is used to unlock shell.
func (mgr *IOManager) ShellUnlock(id uint64, token string) error {
	shell, err := mgr.GetShell(id)
	if err != nil {
		return err
	}
	if shell.Unlock(token) {
		return nil
	}
	return ErrInvalidLockToken
}

// ShellForceUnlock is used to force unlock shell, it will not check the token.
func (mgr *IOManager) ShellForceUnlock(id uint64, token string) error {
	shell, err := mgr.GetShell(id)
	if err != nil {
		return err
	}
	shell.ForceUnlock(token)
	return nil
}

// ShellRead is used to read data from shell.
func (mgr *IOManager) ShellRead(id uint64, offset int) ([]byte, error) {
	shell, err := mgr.GetShell(id)
	if err != nil {
		return nil, err
	}
	return shell.Read(offset), nil
}

// ShellWrite is used to write data to shell.
func (mgr *IOManager) ShellWrite(id uint64, token string, data []byte) error {
	shell, err := mgr.GetShell(id)
	if err != nil {
		return err
	}
	return shell.Write(token, data)
}

// ShellClean is used to clean buffer in under reader.
func (mgr *IOManager) ShellClean(id uint64, token string) error {
	shell, err := mgr.GetShell(id)
	if err != nil {
		return err
	}
	return shell.Clean(token)
}

// ShellClose is used to close shell io object, it will not stop the under session.
func (mgr *IOManager) ShellClose(id uint64, token string) error {
	shell, err := mgr.GetShell(id)
	if err != nil {
		return err
	}
	return shell.Close(token)
}

// ShellCompatibleModules is used to return a list of Post modules that compatible.
func (mgr *IOManager) ShellCompatibleModules(ctx context.Context, id uint64) ([]string, error) {
	shell, err := mgr.GetShell(id)
	if err != nil {
		return nil, err
	}
	return shell.ToShell().CompatibleModules(ctx)
}

// ShellStop is used to stop the under shell session, it will close io object first.
func (mgr *IOManager) ShellStop(id uint64, token string) error {
	shell, err := mgr.GetShell(id)
	if err != nil {
		return err
	}
	err = shell.Close(token)
	if err != nil {
		return err
	}
	return shell.ToShell().Stop()
}

// ----------------------------------------about meterpreter---------------------------------------

// NewMeterpreter is used to wrap an existing meterpreter session and wrap it to IOObject.
func (mgr *IOManager) NewMeterpreter(ctx context.Context, id uint64) (*IOObject, error) {
	return mgr.NewMeterpreterWithLocker(ctx, id, "")
}

// NewMeterpreterWithLocker is used to wrap an existing meterpreter session and wrap it to IOObject with locker.
func (mgr *IOManager) NewMeterpreterWithLocker(ctx context.Context, id uint64, token string) (*IOObject, error) {
	if mgr.shuttingDown() {
		return nil, ErrIOManagerClosed
	}
	err := mgr.checkSessionID(ctx, "meterpreter", id)
	if err != nil {
		return nil, err
	}
	meterpreter := mgr.ctx.NewMeterpreter(id, mgr.interval)
	var ok bool
	defer func() {
		// if track failed, close meterpreter(not close under meterpreter session)
		if ok {
			return
		}
		err := meterpreter.Close()
		if err != nil {
			mgr.logf(logger.Warning, "appear error when close meterpreter session %d: %s", id, err)
		}
	}()
	obj := &IOObject{
		object: meterpreter,
		now:    mgr.now,
	}
	onRead := func() {
		mgr.handlers.OnMeterpreterRead(id)
	}
	onClean := func() {
		mgr.handlers.OnMeterpreterClean(id)
	}
	onClose := func() {
		mgr.handlers.OnMeterpreterClosed(id)
		_ = mgr.trackMeterpreter(obj, false)
	}
	obj.reader = newIOReader(mgr.ctx.logger, meterpreter, onRead, onClean, onClose)
	onLock := func(token string) {
		mgr.handlers.OnMeterpreterLocked(id, token)
	}
	onUnlock := func(token string) {
		mgr.handlers.OnMeterpreterUnlocked(id, token)
	}
	obj.onLock = onLock
	obj.onUnlock = onUnlock
	if token != "" {
		obj.locker = token
		obj.lockAt = mgr.now()
	}
	// must track first.
	err = mgr.trackMeterpreter(obj, true)
	if err != nil {
		// if track failed must deference reader,
		// otherwise it will occur cycle reference
		obj.reader = nil
		return nil, err
	}
	obj.reader.ReadLoop()
	ok = true
	return obj, nil
}

// MeterpreterLock is used to lock meterpreter that only one user
// can operate this meterpreter, other user can only read.
func (mgr *IOManager) MeterpreterLock(id uint64, token string) error {
	meterpreter, err := mgr.GetMeterpreter(id)
	if err != nil {
		return err
	}
	if meterpreter.Lock(token) {
		return nil
	}
	return ErrAnotherUserLocked
}

// MeterpreterUnlock is used to unlock meterpreter.
func (mgr *IOManager) MeterpreterUnlock(id uint64, token string) error {
	meterpreter, err := mgr.GetMeterpreter(id)
	if err != nil {
		return err
	}
	if meterpreter.Unlock(token) {
		return nil
	}
	return ErrInvalidLockToken
}

// MeterpreterForceUnlock is used to force unlock meterpreter, it will not check the token.
func (mgr *IOManager) MeterpreterForceUnlock(id uint64, token string) error {
	meterpreter, err := mgr.GetMeterpreter(id)
	if err != nil {
		return err
	}
	meterpreter.ForceUnlock(token)
	return nil
}

// MeterpreterRead is used to read data from meterpreter.
func (mgr *IOManager) MeterpreterRead(id uint64, offset int) ([]byte, error) {
	meterpreter, err := mgr.GetMeterpreter(id)
	if err != nil {
		return nil, err
	}
	return meterpreter.Read(offset), nil
}

// MeterpreterWrite is used to write data to meterpreter.
func (mgr *IOManager) MeterpreterWrite(id uint64, token string, data []byte) error {
	meterpreter, err := mgr.GetMeterpreter(id)
	if err != nil {
		return err
	}
	return meterpreter.Write(token, data)
}

// MeterpreterClean is used to clean buffer in under reader.
func (mgr *IOManager) MeterpreterClean(id uint64, token string) error {
	meterpreter, err := mgr.GetMeterpreter(id)
	if err != nil {
		return err
	}
	return meterpreter.Clean(token)
}

// MeterpreterClose is used to close meterpreter io object, it will not stop the under session.
func (mgr *IOManager) MeterpreterClose(id uint64, token string) error {
	meterpreter, err := mgr.GetMeterpreter(id)
	if err != nil {
		return err
	}
	return meterpreter.Close(token)
}

// MeterpreterDetach is used to detach meterpreter session.
func (mgr *IOManager) MeterpreterDetach(ctx context.Context, id uint64, token string) error {
	meterpreter, err := mgr.GetMeterpreter(id)
	if err != nil {
		return err
	}
	if !meterpreter.CheckToken(token) {
		return ErrAnotherUserLocked
	}
	return meterpreter.ToMeterpreter().Detach(ctx)
}

// MeterpreterInterrupt is used to interrupt meterpreter session.
func (mgr *IOManager) MeterpreterInterrupt(ctx context.Context, id uint64, token string) error {
	meterpreter, err := mgr.GetMeterpreter(id)
	if err != nil {
		return err
	}
	if !meterpreter.CheckToken(token) {
		return ErrAnotherUserLocked
	}
	return meterpreter.ToMeterpreter().Interrupt(ctx)
}

// MeterpreterRunSingle is used to run single command.
func (mgr *IOManager) MeterpreterRunSingle(ctx context.Context, id uint64, token, cmd string) error {
	meterpreter, err := mgr.GetMeterpreter(id)
	if err != nil {
		return err
	}
	if !meterpreter.CheckToken(token) {
		return ErrAnotherUserLocked
	}
	return meterpreter.ToMeterpreter().RunSingle(ctx, cmd)
}

// MeterpreterCompatibleModules is used to return a list of Post modules that compatible.
func (mgr *IOManager) MeterpreterCompatibleModules(ctx context.Context, id uint64) ([]string, error) {
	meterpreter, err := mgr.GetMeterpreter(id)
	if err != nil {
		return nil, err
	}
	return meterpreter.ToMeterpreter().CompatibleModules(ctx)
}

// MeterpreterStop is used to stop the under meterpreter session, it will close io object first.
func (mgr *IOManager) MeterpreterStop(id uint64, token string) error {
	meterpreter, err := mgr.GetMeterpreter(id)
	if err != nil {
		return err
	}
	err = meterpreter.Close(token)
	if err != nil {
		return err
	}
	return meterpreter.ToMeterpreter().Stop()
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
	// close all console objects
	for id, console := range mgr.consoles {
		e := console.close()
		if e != nil && err == nil {
			err = e
		}
		delete(mgr.consoles, id)
	}
	// close all shell objects
	for id, shell := range mgr.shells {
		e := shell.close()
		if e != nil && err == nil {
			err = e
		}
		delete(mgr.shells, id)
	}
	// close all meterpreter objects
	for id, meterpreter := range mgr.meterpreters {
		e := meterpreter.close()
		if e != nil && err == nil {
			err = e
		}
		delete(mgr.meterpreters, id)
	}
	return err
}
