// +build windows

package kiwi

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/testsuite"
)

func TestKiwi(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	kiwi, err := NewKiwi(logger.Test)
	require.NoError(t, err)

	err = kiwi.EnableDebugPrivilege()
	require.NoError(t, err)

	creds, err := kiwi.GetAllCredential()
	require.NoError(t, err)

	fmt.Println(creds)

	kiwi.Close()

	testsuite.IsDestroyed(t, kiwi)
}
