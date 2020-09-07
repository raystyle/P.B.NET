package privilege

import (
	"testing"
)

func TestGenerateRtlEnableDisable(t *testing.T) {
	for _, item := range [...]*struct {
		privilege string
		comment   string
	}{
		{"SECreateToken", "create token"},
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
