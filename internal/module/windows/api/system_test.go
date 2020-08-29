package api

import (
	"fmt"
	"testing"

	"github.com/davecgh/go-spew/spew"
)

func TestGetVersionNumber(t *testing.T) {
	major, minor, build := GetVersionNumber()
	fmt.Println("major:", major, "minor:", minor, "build:", build)
}

func TestGetVersion(t *testing.T) {
	info := GetVersion()
	spew.Dump(info)
}
