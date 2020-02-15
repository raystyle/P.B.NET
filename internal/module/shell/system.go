package shell

import (
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/pkg/errors"
)

// System is a interactive system shell.
type System struct {
	// input pipe
	iPr *os.File
	iPw *os.File

	// output pipe
	oPr *os.File
	oPw *os.File

	// multi writer, record user input
	input io.Writer

	// shell process
	cmd *exec.Cmd

	closeOnce sync.Once
}

// NewSystem is used to create a interactive system shell.
// path is the executable file path.
func NewSystem(path string, args []string, dir string) (*System, error) {
	system := System{}
	iPr, iPw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	system.iPr = iPr
	system.iPw = iPw
	oPr, oPw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	system.oPr = oPr
	system.oPw = oPw
	// must copy
	system.input = io.MultiWriter(system.iPw, system.oPw)
	cmd := createCommand(path, args)
	cmd.Dir = dir
	cmd.Stdin = system.iPr
	cmd.Stdout = system.oPw
	cmd.Stderr = system.oPw
	err = cmd.Start()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create system shell")
	}
	system.cmd = cmd
	return &system, nil
}

// Read is used to read session output data.
func (system *System) Read(data []byte) (int, error) {
	return system.oPr.Read(data)
}

// Write is used to write user input data.
func (system *System) Write(data []byte) (int, error) {
	return system.input.Write(data)
}

// Close is used to close session, if this session is running a program,
// it will be kill at the same time.
func (system *System) Close() error {
	system.closeOnce.Do(func() {
		_ = system.cmd.Process.Kill()
		_ = system.cmd.Process.Release()
		_ = system.iPr.Close()
		_ = system.iPw.Close()
		_ = system.oPr.Close()
		_ = system.oPw.Close()
	})
	return nil
}

// Interrupt is used to send interrupt signal to opened process.
func (system *System) Interrupt() error {
	return sendInterruptSignal(system.cmd)
}
