// +build windows

package api

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/convert"
)

func TestFileTime(t *testing.T) {
	now := time.Now()
	ft := GetFileTime(now)

	fmt.Println(ft.Time())
	fmt.Println(ft.Unix())

	require.True(t, convert.AbsInt64(ft.Time().Unix()-now.Unix()) < 5)
}
