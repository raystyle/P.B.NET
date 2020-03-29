package meterpreter

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/module/shellcode"
)

const (
	testNetwork = "tcp"
	testAddress = "127.0.0.1:8990"
)

func TestReverseTCP(t *testing.T) {
	err := ReverseTCP(testNetwork, testAddress, shellcode.MethodCreateThread)
	require.NoError(t, err)
}
