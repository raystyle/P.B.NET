// +build windows

package kiwi

import (
	"fmt"
	"testing"
	"time"

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

	creds, err := kiwi.GetAllCredential()
	require.NoError(t, err)

	fmt.Println(creds)

	creds, err = kiwi.GetAllCredential()
	require.NoError(t, err)

	fmt.Println(creds)

	err = kiwi.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, kiwi)
}

func TestKiwi_GetAllCredentialWait(t *testing.T) {
	time.Sleep(10 * time.Second)

	TestKiwi_GetAllCredential(t)
}
