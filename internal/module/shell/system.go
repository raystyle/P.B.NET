package shell

import (
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

	cmd *exec.Cmd

	closeOnce sync.Once
}

// NewSystem is used to create a interactive system shell.
// path is the executable file path.
func NewSystem(path string, args []string, dir string) (*System, error) {
	system := System{}
	system.iPr, system.iPw, _ = os.Pipe()
	system.oPr, system.oPw, _ = os.Pipe()
	cmd := createCommand(path, args)
	cmd.Dir = dir
	cmd.Stdin = system.iPr
	cmd.Stdout = system.oPw
	cmd.Stderr = system.oPw
	err := cmd.Start()
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
	return system.iPw.Write(data)
}

// Close is used to close session, if this session is running a program,
// it will be kill at the same time.
func (system *System) Close() error {
	var errStr string
	system.closeOnce.Do(func() {
		err := system.cmd.Process.Kill()
		if err != nil {
			errStr += err.Error()
		}
		err = system.cmd.Process.Release()
		if err != nil {
			errStr += err.Error()
		}
		_ = system.iPr.Close()
		_ = system.iPw.Close()
		_ = system.oPr.Close()
		_ = system.oPw.Close()
	})
	if errStr != "" {
		return errors.New(errStr)
	}
	return nil
}
