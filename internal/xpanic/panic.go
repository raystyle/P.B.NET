package xpanic

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"runtime"
)

const depth = 32

func Print(panic interface{}, title string) *bytes.Buffer {
	b := &bytes.Buffer{}
	b.WriteString(title)
	b.WriteString(":\n\n")
	_, _ = fmt.Fprintln(b, panic)
	printStack(b)
	return b
}

func Error(panic interface{}, title string) error {
	return errors.New(Print(panic, title).String())
}

// from github.com/pkg/errors

func printStack(b *bytes.Buffer) {
	b.WriteString("\n")
	var pcs [depth]uintptr
	n := runtime.Callers(3, pcs[:])
	for _, pc := range pcs[2:n] {
		f := frame(pc)
		// write source file
		pc := f.pc()
		fn := runtime.FuncForPC(pc)
		if fn == nil {
			_, _ = io.WriteString(b, "unknown")
		} else {
			file, _ := fn.FileLine(pc)
			_, _ = fmt.Fprintf(b, "%s\n\t%s", fn.Name(), file)
		}
		_, _ = io.WriteString(b, ":")
		_, _ = fmt.Fprintf(b, "%d\n", f.line())
	}
}

// Frame represents a program counter inside a stack frame.
type frame uintptr

// pc returns the program counter for this frame;
// multiple frames may have the same PC value.
func (f frame) pc() uintptr { return uintptr(f) - 1 }

// line returns the line number of source code of the
// function for this Frame's pc.
func (f frame) line() int {
	fn := runtime.FuncForPC(f.pc())
	if fn == nil {
		return 0
	}
	_, line := fn.FileLine(f.pc())
	return line
}
