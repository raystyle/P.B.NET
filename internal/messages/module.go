package messages

// ExecuteShellCode is used to execute shellcode
type ExecuteShellCode struct {
	Method    string
	ShellCode []byte
}
