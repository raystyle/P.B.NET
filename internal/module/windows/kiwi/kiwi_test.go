// +build windows

package kiwi

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/testsuite"
)

func TestKiwi_GetAllCredential(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	kiwi, err := NewKiwi(logger.Test)
	require.NoError(t, err)

	err = kiwi.EnableDebugPrivilege()
	require.NoError(t, err)

	// time.Sleep(10 * time.Second)

	creds, err := kiwi.GetAllCredential()
	require.NoError(t, err)

	fmt.Println(creds)

	creds, err = kiwi.GetAllCredential()
	require.NoError(t, err)

	err = kiwi.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, kiwi)
}
