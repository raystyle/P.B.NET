package shell

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// Terminal is a interactive platform-independent shell.
type Terminal struct {
	// input pipe
	iPr *io.PipeReader
	iPw *io.PipeWriter

	// output pipe
	oPr *io.PipeReader
	oPw *io.PipeWriter

	// status
	cd  string
	env []string

	ctx       context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once
	wg        sync.WaitGroup
}

// NewTerminal is used to create a platform-independent shell.
func NewTerminal() *Terminal {
	cd, _ := os.Getwd()
	session := Terminal{
		cd:  cd,
		env: os.Environ(),
	}
	session.iPr, session.iPw = io.Pipe()
	session.oPr, session.oPw = io.Pipe()
	session.ctx, session.cancel = context.WithCancel(context.Background())
	session.wg.Add(1)
	go session.readInputLoop()
	return &session
}

// Read is used to read session output data.
func (session *Terminal) Read(data []byte) (int, error) {
	return session.oPr.Read(data)
}

// Write is used to write user input data.
func (session *Terminal) Write(data []byte) (int, error) {
	return session.iPw.Write(data)
}

// Close is used to close session, if this session is running a program,
// it will be kill at the same time.
func (session *Terminal) Close() error {
	session.close()
	session.wg.Wait()
	return nil
}

func (session *Terminal) close() {
	session.closeOnce.Do(func() {
		session.cancel()
		_ = session.iPr.Close()
		_ = session.iPw.Close()
		_ = session.oPr.Close()
		_ = session.oPw.Close()
	})
}

// readInputLoop is used to read user input command and run.
func (session *Terminal) readInputLoop() {
	// print hello
	hello := []byte("welcome to interactive shell session [version 1.0.0]\n\n")
	_, _ = session.oPw.Write(hello)
	session.printInputLine()
	var (
		run bool
		cmd *exec.Cmd
	)
	scanner := bufio.NewScanner(session.iPr)
	for scanner.Scan() {
		if !run {
			input := scanner.Text()
			// simple split
			args := strings.Split(input, " ")
			if len(args) == 0 {
				session.printInputLine()
				continue
			}

			// args := CommandLineToArgv(input)
			// empty line

			if session.executeInternalCommand(args[0], args[1:]) {
				continue
			}

			cmd = exec.CommandContext(session.ctx, "")
			// program output
			cmd.Stderr = session.oPw
			cmd.Stdout = session.oPw
			cmd.Dir = session.cd
			// copy environment variable
			env := make([]string, len(session.env))
			copy(env, session.env)
			cmd.Env = env

			run = true
		} else {

		}
	}
}

func (session *Terminal) printInputLine() {
	line := []byte(session.cd + ">")
	_, _ = session.oPw.Write(line)
}

func (session *Terminal) executeInternalCommand(cmd string, args []string) bool {
	switch cmd {
	case "cd": // change current path
		session.cd = args[0]
	case "set": // set environment variable

	case "exit":
		session.close()
	default:
		return false
	}
	return true
}

// CommandLineToArgv splits a command line into individual argument
// strings, following the Windows conventions documented
// at http://daviddeley.com/autohotkey/parameters/parameters.htm#WINARGV
func CommandLineToArgv(cmd string) []string {
	var args []string
	for len(cmd) > 0 {
		if cmd[0] == ' ' || cmd[0] == '\t' {
			cmd = cmd[1:]
			continue
		}
		var arg []byte
		arg, cmd = readNextArg(cmd)
		args = append(args, string(arg))
	}
	return args
}

// appendBSBytes appends n '\\' bytes to b and returns the resulting slice.
func appendBSBytes(b []byte, n int) []byte {
	for ; n > 0; n-- {
		b = append(b, '\\')
	}
	return b
}

// readNextArg splits command line string cmd into next
// argument and command line remainder.
func readNextArg(cmd string) (arg []byte, rest string) {
	var b []byte
	var inQuote bool
	var nSlash int
	for ; len(cmd) > 0; cmd = cmd[1:] {
		c := cmd[0]
		switch c {
		case ' ', '\t':
			if !inQuote {
				return appendBSBytes(b, nSlash), cmd[1:]
			}
		case '"':
			b = appendBSBytes(b, nSlash/2)
			if nSlash%2 == 0 {
				// use "Prior to 2008" rule from
				// http://daviddeley.com/autohotkey/parameters/parameters.htm
				// section 5.2 to deal with double double quotes
				if inQuote && len(cmd) > 1 && cmd[1] == '"' {
					b = append(b, c)
					cmd = cmd[1:]
				}
				inQuote = !inQuote
			} else {
				b = append(b, c)
			}
			nSlash = 0
			continue
		case '\\':
			nSlash++
			continue
		}
		b = appendBSBytes(b, nSlash)
		nSlash = 0
		b = append(b, c)
	}
	return appendBSBytes(b, nSlash), ""
}
