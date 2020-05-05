package xpanic

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"runtime"
)

const maxDepth = 32

// Print is used to print panic and stack to a *bytes.Buffer.
func Print(panic interface{}, title string) *bytes.Buffer {
	b := &bytes.Buffer{}
	b.WriteString(title)
	b.WriteString(":\n")
	_, _ = fmt.Fprintln(b, panic)
	b.WriteString("\n")
	PrintStack(b, 4) // skip about defer
	return b
}

// Error is used to print panic and stack to a *bytes.Buffer buf and return an error.
func Error(panic interface{}, title string) error {
	return errors.New(Print(panic, title).String())
}

// PrintStack is used to print current stack to a *bytes.Buffer.
func PrintStack(b *bytes.Buffer, skip int) {
	defer func() {
		if r := recover(); r != nil {
			b.WriteString("\nfailed to print stack\n")
		}
	}()
	if skip > maxDepth {
		skip = 0
	}
	var pcs [maxDepth]uintptr
	n := runtime.Callers(skip, pcs[:])
	// skip unnecessary pc
	for _, pc := range pcs[2 : n-2] {
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

// frame represents a program counter inside a stack frame.
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
