package msfrpc

import (
	"bytes"
	"context"
	"io"
	"sync"
	"time"

	"project/internal/logger"
	"project/internal/security"
	"project/internal/xpanic"
)

// IOReader is used to wrap Console, Shell and Meterpreter.
// different reader(user) can get the same data, when new data read,
// it will call onRead() for notice user that can get new data.
type IOReader struct {
	rc     io.ReadCloser
	logger logger.Logger
	onRead func()

	// store history output
	buf *bytes.Buffer
	rwm sync.RWMutex

	wg sync.WaitGroup
}

func newIOReader(rc io.ReadCloser, lg logger.Logger, onRead func()) *IOReader {
	reader := IOReader{
		rc:     rc,
		logger: lg,
		onRead: onRead,
		buf:    bytes.NewBuffer(make([]byte, 0, 64)),
	}
	reader.wg.Add(1)
	go reader.readLoop()
	return &reader
}

func (reader *IOReader) readLoop() {
	defer reader.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			buf := xpanic.Print(r, "IOReader.readLoop")
			reader.logger.Println(logger.Fatal, "msfrpc-IOReader", buf)
			// restart readLoop
			time.Sleep(time.Second)
			reader.wg.Add(1)
			go reader.readLoop()
		}
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

func (reader *IOReader) write(b []byte) {
	reader.rwm.Lock()
	defer reader.rwm.Unlock()
	reader.buf.Write(b)
}

// Read is used to read buffer data [start:].
func (reader *IOReader) Read(start int) []byte {
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
func (reader *IOReader) Clean() {
	reader.rwm.Lock()
	defer reader.rwm.Unlock()
	// clean buffer data at once
	security.CoverBytes(reader.buf.Bytes())
	// alloc a new buffer
	reader.buf = bytes.NewBuffer(make([]byte, 0, 64))
}

// Close is used to close io reader, it will also close under ReadCloser.
func (reader *IOReader) Close() error {
	err := reader.rc.Close()
	reader.wg.Wait()
	// prevent cycle reference
	reader.onRead = nil
	return err
}

// IOObject contains under IO Object, IO Reader and IO status about IO Object.
// IO status contains the status about the IO(console, shell and meterpreter).
// must use token for write data to IO object, usually the admin can unlock it force.
type IOObject struct {
	Object interface{} // *Console, *Shell and *Meterpreter
	Reader *IOReader
	now    func() time.Time

	// status
	locker string // user token
	lockAt time.Time
	rwm    sync.RWMutex
}

// ToConsole is used to convert *IOObject to a *Console.
func (obj *IOObject) ToConsole() *Console {
	return obj.Object.(*Console)
}

// ToShell is used to convert *IOObject to a *Shell.
func (obj *IOObject) ToShell() *Shell {
	return obj.Object.(*Shell)
}

// ToMeterpreter is used to convert *IOObject to a *Meterpreter.
func (obj *IOObject) ToMeterpreter() *Meterpreter {
	return obj.Object.(*Meterpreter)
}

// Lock is used to lock IO object write.
func (obj *IOObject) Lock(token string) bool {
	obj.rwm.Lock()
	defer obj.rwm.Unlock()
	if obj.locker == "" {
		obj.locker = token
		obj.lockAt = obj.now()
		return true
	}
	return false
}

// Unlock is used to unlock IO object write.
func (obj *IOObject) Unlock(token string) bool {
	obj.rwm.Lock()
	defer obj.rwm.Unlock()
	if obj.locker == "" {
		return true
	}
	if obj.locker == token {
		obj.locker = ""
		return true
	}
	return false
}

// ForceUnlock is used to clean locker force, usually only admin can call it.
func (obj *IOObject) ForceUnlock() {
	obj.rwm.Lock()
	defer obj.rwm.Unlock()
	obj.locker = ""
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

// DataArrive contains callbacks about IO objects read new data.
type DataArrive struct {
	OnConsole     func(id string)
	OnShell       func(id uint64)
	OnMeterpreter func(id uint64)
}

// IOManager is used to manage Console IO, Shell session IO and Meterpreter session IO.
// It can lock IO instance for one user can write data to it, other user can read it
// with IO reader, other user can only destroy it(Console) or kill session(Shell or Meterpreter).
type IOManager struct {
	ctx *Client
	da  *DataArrive
	now func() time.Time

	// key = console id
	consoles    map[string]*IOObject
	consolesRWM sync.RWMutex

	// key = shell session id
	shells    map[uint64]*IOObject
	shellsRWM sync.RWMutex

	// key = meterpreter session id
	meterpreters    map[uint64]*IOObject
	meterpretersRWM sync.RWMutex
}

// NewIOManager is used to create a new IO manager.
func NewIOManager(client *Client, da *DataArrive, now func() time.Time) *IOManager {
	if now == nil {
		now = time.Now
	}
	return &IOManager{
		ctx:          client,
		da:           da,
		now:          now,
		consoles:     make(map[string]*IOObject),
		shells:       make(map[uint64]*IOObject),
		meterpreters: make(map[uint64]*IOObject),
	}
}

// NewConsole is used to create a new console with status, All users can read or write.
func (mgr *IOManager) NewConsole(
	ctx context.Context,
	workspace string,
	interval time.Duration,
) (string, error) {
	console, err := mgr.ctx.NewConsole(ctx, workspace, interval)
	if err != nil {
		return "", err
	}
	reader := newIOReader(console, mgr.ctx.logger, nil)
	obj := IOObject{
		Object: console,
		Reader: reader,
		now:    mgr.now,
	}
	obj.now = nil
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

// Close is used to close IOManager..
func (mgr *IOManager) Close() error {
	var err error
	for id, obj := range mgr.consoles {
		e := obj.Reader.Close()
		if e != nil && err == nil {
			err = e
		}
		delete(mgr.consoles, id)
	}
	for id, obj := range mgr.shells {
		e := obj.Reader.Close()
		if e != nil && err == nil {
			err = e
		}
		delete(mgr.shells, id)
	}
	for id, obj := range mgr.meterpreters {
		e := obj.Reader.Close()
		if e != nil && err == nil {
			err = e
		}
		delete(mgr.meterpreters, id)
	}
	mgr.now = nil
	return nil
}
