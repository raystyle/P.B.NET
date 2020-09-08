// +build windows

package kiwi

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/testsuite"
)

func TestKiwi(t *testing.T) {

}

func testPrintCredential(creds []*Credential) {
	for _, cred := range creds {
		session := cred.Session
		fmt.Println("Domain:      ", session.Domain)
		fmt.Println("Username:    ", session.Username)
		fmt.Println("Logon server:", session.LogonServer)
		fmt.Println("SID:         ", session.SID)
		fmt.Println("  wdigest:")
		if cred.Wdigest != nil {
			wdigest := cred.Wdigest
			fmt.Println("    *Domain:  ", wdigest.Domain)
			fmt.Println("    *Username:", wdigest.Username)
			fmt.Println("    *Password:", wdigest.Password)
		}
		fmt.Println()
	}
}

func TestKiwi_GetAllCredential(t *testing.T) {
	go func() {
		for {
			runtime.GC()
			time.Sleep(10 * time.Millisecond)
		}
	}()
	time.Sleep(time.Second)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	kiwi, err := NewKiwi(logger.Test)
	require.NoError(t, err)

	err = kiwi.EnableDebugPrivilege()
	require.NoError(t, err)

	creds, err := kiwi.GetAllCredential()
	require.NoError(t, err)
	testPrintCredential(creds)

	creds, err = kiwi.GetAllCredential()
	require.NoError(t, err)
	testPrintCredential(creds)

	err = kiwi.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, kiwi)
}

func TestKiwi_GetAllCredentialWait(t *testing.T) {
	time.Sleep(10 * time.Second)

	TestKiwi_GetAllCredential(t)
}
