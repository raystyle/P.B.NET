package msfrpc

import (
	"bytes"
	"io"
	"sync"
	"time"

	"project/internal/logger"
	"project/internal/xpanic"
)

// IOStatus contains the status about the IO(console, shell and meterpreter).
// must use token to operate IO, except with Force. usually the admin can call it.
type IOStatus struct {
	readToken  string
	writeToken string

	rwm sync.RWMutex
}

// IOManager is used to manage Console IO, Shell session IO and Meterpreter session IO.
// It can lock IO instance for one user can use it, other user can read it with parallel reader.
//
// It can create IO instance that only one user can read or write, other user can only
// destroy it(Console IO) or kill session(Shell or Meterpreter).
type IOManager struct {
	ctx *MSFRPC

	// key = console id
	consoles map[string]*Console
	// key = shell session id
	shells map[uint64]*Shell
	// key = meterpreter session id
	meterpreters map[uint64]*Meterpreter

	rwm sync.RWMutex
}

// NewIOManager is used to create a new IO manager.
func (msf *MSFRPC) NewIOManager() *IOManager {
	return &IOManager{
		ctx:          msf,
		consoles:     make(map[string]*Console),
		shells:       make(map[uint64]*Shell),
		meterpreters: make(map[uint64]*Meterpreter),
	}
}

// NewConsole is used to create a new console, All user can read ro write.
func (iom *IOManager) NewConsole() {

}

// NewConsoleAndLockWrite is used to create a new console and lock write.
// Only the creator can write it.
func (iom *IOManager) NewConsoleAndLockWrite() {

}

// NewConsoleAndLockRW is used to create a new console and lock read and write.
// Only the creator can read and write it.
func (iom *IOManager) NewConsoleAndLockRW() {

}

// NewConsoleWithID is used to create a new console with id, All user can read ro write.
func (iom *IOManager) NewConsoleWithID() {
	// TODO check is exist
}

// NewConsoleWithIDAndLockWrite is used to create a new console with id and lock write.
// Only the creator can write it.
func (iom *IOManager) NewConsoleWithIDAndLockWrite() {
	// TODO check is exist
}

// NewConsoleWithIDAndLockRW is used to create a new console with id and lock read and write.
// Only the creator can read and write it.
func (iom *IOManager) NewConsoleWithIDAndLockRW() {
	// TODO check is exist
}

// ConsoleLockWrite is used to lock write for console that only one user
// can write to this console.
func (iom *IOManager) ConsoleLockWrite() {

}

// ConsoleUnlockWrite is used to unlock write for console that all user
// can write to this console.
func (iom *IOManager) ConsoleUnlockWrite() {

}

// ConsoleLockRW is used to lock read and write for console that only one user
// can read or write to this console. (single mode)
func (iom *IOManager) ConsoleLockRW() {

}

// ConsoleUnLockRW is used to unlock read and write for console that all user
// can read or write to this console. (common mode)
func (iom *IOManager) ConsoleUnLockRW() {

}

// ConsoleForceUnlockWrite is used to unlock write for console that all user
// can write to this console, it will not check the token.
func (iom *IOManager) ConsoleForceUnlockWrite() {

}

// ConsoleForceUnLockRW is used to unlock write for console that all user
// can write to this console, it will not check the token.
func (iom *IOManager) ConsoleForceUnLockRW() {

}

// ConsoleRead is used to read data from console, it will check token.
func (iom *IOManager) ConsoleRead() {

}

// ConsoleWrite is used to write data to console, it will check token.
func (iom *IOManager) ConsoleWrite() {

}

// ConsoleSessionDetach is used to detach session in console, it will check token.
func (iom *IOManager) ConsoleSessionDetach() {

}

// ConsoleSessionKill is used to kill session in console, it will check token.
func (iom *IOManager) ConsoleSessionKill() {

}

// parallelReader is used to wrap Console, Shell and Meterpreter.
// different reader can get the same data.
type parallelReader struct {
	rc     io.ReadCloser
	logger logger.Logger
	onRead func()

	// store history output
	buf bytes.Buffer
	rwm sync.RWMutex
}

func newParallelReader(rc io.ReadCloser, logger logger.Logger, onRead func()) *parallelReader {
	reader := parallelReader{
		rc:     rc,
		logger: logger,
		onRead: onRead,
	}
	go reader.readLoop()
	return &reader
}

func (pr *parallelReader) readLoop() {
	defer func() {
		if r := recover(); r != nil {
			b := xpanic.Print(r, "parallelReader.readLoop")
			pr.logger.Println(logger.Fatal, "parallelReader", b)
			// restart readLoop
			time.Sleep(time.Second)
			go pr.readLoop()
		}
	}()
	var (
		n   int
		err error
	)
	buf := make([]byte, 4096)
	for {
		n, err = pr.rc.Read(buf)
		if err != nil {
			return
		}
		pr.writeToBuffer(buf[:n])
		pr.onRead()
	}
}

func (pr *parallelReader) writeToBuffer(b []byte) {
	pr.rwm.Lock()
	defer pr.rwm.Unlock()
	pr.buf.Write(b)
}

// Bytes is used to get buffer data.
func (pr *parallelReader) Bytes(start int) []byte {
	if start < 0 {
		start = 0
	}
	pr.rwm.RLock()
	defer pr.rwm.RUnlock()
	l := pr.buf.Len()
	if start > l {
		return nil
	}
	b := make([]byte, l-start)
	copy(b, pr.buf.Bytes()[start:])
	return b
}

// Close is used to close parallel reader.
func (pr *parallelReader) Close() error {
	return pr.rc.Close()
}
