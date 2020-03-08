// +build !windows
// +build !linux

package shell

// Shell is used to run one command with system shell.
func Shell(ctx context.Context, command string) ([]byte, error) {
	return nil, nil
}

func createCommand(path string, args []string) *exec.Cmd {
	if path == "" {
		path = "sh"
	}
	cmd := exec.Command(path, args...) // #nosec
	cmd.SysProcAttr = setSysProcAttr()
	return cmd
}

func setSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{}
}

func sendInterruptSignal(cmd *exec.Cmd) error {
	return cmd.Process.Signal(os.Interrupt)
}
