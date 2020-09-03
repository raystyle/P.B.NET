package kiwi

import (
	"fmt"
	"testing"
)

// this test is used to pass golangci-lint
func TestPrintUnusedVersion(t *testing.T) {
	fmt.Println(buildWin10v1511)
	fmt.Println(buildWin10v1607)
	fmt.Println(buildWin10v1709)
	fmt.Println(buildWin10v1909)
	fmt.Println(buildWin10v2004)

	fmt.Println(buildMinWinXP)
}
