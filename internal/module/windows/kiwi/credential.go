package kiwi

import (
	"project/internal/module/windows/api"
)

// reference:
// https://github.com/gentilkiwi/mimikatz/blob/master/mimikatz/modules/sekurlsa/globals_sekurlsa.h

type genericPrimaryCredential struct {
	Username api.LSAUnicodeString
	Domain   api.LSAUnicodeString
	Password api.LSAUnicodeString
}

// Credential contain all credential about session.
type Credential struct {
	Session *Session
	Wdigest *Wdigest
}
