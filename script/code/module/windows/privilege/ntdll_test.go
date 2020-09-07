package privilege

import (
	"testing"
)

func TestGenerateRtlEnableDisable(t *testing.T) {
	for _, item := range [...]*struct {
		privilege string
		comment   string
	}{
		{"SESecurity", "security"},
		{"SELoadDriver", "load driver"},
		{"SESystemTime", "system time"},
		{"SESystemProf", "system profile"},
		{"SEBackup", "backup"},
		{"SEShutdown", "shutdown"},
		{"SEDebug", "debug"},
		{"SESystemEnv", "system environment"},
		{"SERemoteShutdown", "remote shutdown"},
	} {
		generateRtlEnableDisable(item.privilege, item.comment)
	}
}

func TestGenerateTestRtlEnableDisable(t *testing.T) {
	for _, privilege := range []string{
		"SESecurity",
		"SELoadDriver",
		"SESystemTime",
		"SESystemProf",
		"SEBackup",
		"SEShutdown",
		"SEDebug",
		"SESystemEnv",
		"SERemoteShutdown",
	} {
		generateTestRtlEnableDisable(t, privilege)
	}
}
