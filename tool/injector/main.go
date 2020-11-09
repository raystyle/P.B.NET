// +build windows

package main

import (
	"encoding/hex"
	"flag"
	"io/ioutil"

	"project/internal/module/windows/injector"
	"project/internal/system"
)

func main() {
	var (
		pid    int
		format string
		input  string
		chunk  int
		wait   bool
		clean  bool
	)
	flag.IntVar(&pid, "p", 0, "target process id")
	flag.StringVar(&format, "f", "raw", "shellcode format")
	flag.StringVar(&input, "i", "", "input shellcode file path")
	flag.IntVar(&chunk, "chunk", 32, "shellcode maximum random chunk size")
	flag.BoolVar(&clean, "wait", true, "wait shellcode execute finish")
	flag.BoolVar(&clean, "clean", true, "clean shellcode after execute finish")
	flag.Parse()

	if pid == 0 {
		system.PrintError("input pid")
	}
	scData, err := ioutil.ReadFile(input) // #nosec
	system.CheckError(err)
	if format == "hex" {
		scData, err = hex.DecodeString(string(scData))
		system.CheckError(err)
	}

	err = injector.InjectShellcode(uint32(pid), scData, chunk, wait, clean)
	system.CheckError(err)
}
