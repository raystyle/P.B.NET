package messages

// ExecuteShellCode is used to execute shellcode
type ExecuteShellCode struct {
	Method    string
	ShellCode []byte
}

// ExecuteShellCodeResponse is used to execute shellcode
type ExecuteShellCodeResponse struct {
	Success bool
	Error   string
}

// Shell ...
type Shell struct {
	Command string
}

// ShellOutput ...
type ShellOutput struct {
	Output []byte
}
