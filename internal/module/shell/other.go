// +build !windows
// +build !linux

package shell

// Shell ...
func Shell(command string) ([]byte, error) {
	return nil, nil
}

func sendInterruptSignal(cmd *exec.Cmd) error {
	return cmd.Process.Signal(os.Interrupt)
}
