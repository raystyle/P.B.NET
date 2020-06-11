package exec

import (
	"os/exec"
)

// Run is used to call program wait it until exit, get the output and the exit code.
func Run(name string, arg ...string) (output string, code int, err error) {
	cmd := exec.Command(name, arg...) // #nosec
	out, err := cmd.CombinedOutput()
	output = string(out)
	if err != nil {
		return
	}
	code = cmd.ProcessState.ExitCode()
	return
}
