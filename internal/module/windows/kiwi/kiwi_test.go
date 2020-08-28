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

	kiwi := NewKiwi(logger.Test)

	creds, err := kiwi.GetAllCredential()
	require.NoError(t, err)

	fmt.Println(creds)

	testsuite.IsDestroyed(t, kiwi)
}
