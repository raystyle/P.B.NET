package shell

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"project/internal/xpanic"
)

// Welcome to terminal [version 1.0.0]
//
// C:\Windows\System32>

// Terminal is a interactive platform-independent system shell.
type Terminal struct {
	// input pipe
	iPr *os.File
	iPw *os.File

	// output pipe
	oPr *os.File
	oPw *os.File

	// multi writer, record user input
	input io.Writer

	// status
	cd  string
	env []string

	ctx       context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once
	wg        sync.WaitGroup
}

// NewTerminal is used to create a platform-independent system shell.
func NewTerminal() (*Terminal, error) {
	cd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	terminal := Terminal{
		cd:  cd,
		env: os.Environ(),
	}
	iPr, iPw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	terminal.iPr = iPr
	terminal.iPw = iPw
	oPr, oPw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	terminal.oPr = oPr
	terminal.oPw = oPw
	// must copy
	terminal.input = io.MultiWriter(terminal.iPw, terminal.oPw)
	terminal.ctx, terminal.cancel = context.WithCancel(context.Background())
	terminal.wg.Add(1)
	go terminal.readInputLoop()
	return &terminal, nil
}

// Read is used to read terminal output data.
func (t *Terminal) Read(data []byte) (int, error) {
	return t.oPr.Read(data)
}

// Write is used to write user input data.
func (t *Terminal) Write(data []byte) (int, error) {
	return t.input.Write(data)
}

// Close is used to close terminal, if this terminal is running a program,
// it will be kill at the same time.
func (t *Terminal) Close() error {
	t.close()
	t.wg.Wait()
	return nil
}

// Interrupt is used to send interrupt signal to opened process.
func (t *Terminal) Interrupt() error {
	return nil
}

func (t *Terminal) close() {
	t.closeOnce.Do(func() {
		t.cancel()
		_ = t.iPr.Close()
		_ = t.iPw.Close()
		_ = t.oPr.Close()
		_ = t.oPw.Close()
	})
}

// readInputLoop is used to read user input command and run.
func (t *Terminal) readInputLoop() {
	defer func() {
		if r := recover(); r != nil {
			_, _ = xpanic.Print(r, "Terminal.readInputLoop").WriteTo(t.oPw)
			// restart
			time.Sleep(time.Second)
			go t.readInputLoop()
		} else {
			t.wg.Done()
		}
	}()

	// print hello
	hello := []byte("Welcome to terminal [version 1.0.0]\n\n")
	_, _ = t.oPw.Write(hello)
	t.printCurrentDirectory()
	var (
		run bool
		cmd *exec.Cmd
	)
	scanner := bufio.NewScanner(t.iPr)
	for scanner.Scan() {
		if !run {
			input := scanner.Text()
			// check is internal command
			commandLine := strings.SplitN(trimPrefixSpace(input), " ", 2)
			command := commandLine[0]
			var args []string
			if len(commandLine) == 2 {
				args = CommandLineToArgv(commandLine[1])
			}
			if t.executeInternalCommand(command, args) {
				t.printCurrentDirectory()
				continue
			}

			cmd = exec.CommandContext(t.ctx, "")
			// program output
			cmd.Stderr = t.oPw
			cmd.Stdout = t.oPw
			cmd.Dir = t.cd
			// copy environment variable
			env := make([]string, len(t.env))
			copy(env, t.env)
			cmd.Env = env

			run = true
		} else {

		}
	}
}

// trimPrefixSpace is used to remove space in prefix
// "  a" -> "a"
func trimPrefixSpace(s string) string {
	r := []rune(s)
	l := len(r)
	space, _ := utf8.DecodeRuneInString(" ")
	for i := 0; i < l; i++ {
		if r[i] != space {
			return string(r[i:])
		}
	}
	return ""
}

func (t *Terminal) printCurrentDirectory() {
	line := []byte(t.cd + ">")
	_, _ = t.oPw.Write(line)
}

func (t *Terminal) executeInternalCommand(cmd string, args []string) bool {
	switch cmd {
	case "":
		// no input
	case "cd": // change current path
		t.changeDirectory(args)
	case "set": // set environment variable

	case "dir":

	case "exit":
		t.close()
	default:
		return false
	}
	return true
}

func (t *Terminal) changeDirectory(args []string) {
	if len(args) == 0 {
		return
	}
	err := os.Chdir(args[0])
	if err != nil {
		_, _ = fmt.Fprintf(t.oPw, "%s\n\n", err)
		return
	}
	path, err := os.Getwd()
	if err != nil {
		_, _ = fmt.Fprintf(t.oPw, "%s\n\n", err)
		return
	}
	t.cd = path
	// print empty line
	_, _ = fmt.Fprintln(t.oPw)
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
