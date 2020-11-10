// +build windows

package hook

import (
	"fmt"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

func TestNewInlineHookByName(t *testing.T) {
	var err error
	t.Run("MessageBoxW", func(t *testing.T) {
		var guard *PatchGuard
		hookFn := func(hwnd windows.Handle, text, caption *uint16, uType uint) int {
			originText := windows.UTF16PtrToString(text)
			originCaption := windows.UTF16PtrToString(text)

			hookedText := fmt.Sprintf("origin: %s, hooked!", originText)
			hookedCaption := fmt.Sprintf("origin: %s, hooked!", originCaption)
			hookedTextPtr, err := windows.UTF16PtrFromString(hookedText)
			require.NoError(t, err)
			hookedCaptionPtr, err := windows.UTF16PtrFromString(hookedCaption)
			require.NoError(t, err)

			_, _, _ = guard.Original.Call(
				uintptr(hwnd),
				uintptr(unsafe.Pointer(hookedTextPtr)),
				uintptr(unsafe.Pointer(hookedCaptionPtr)),
				uintptr(uType))
			return 1
		}
		guard, err = NewInlineHookByName("user32.dll", "MessageBoxW", true, hookFn)
		require.NoError(t, err)
	})

	t.Run("CryptProtectMemory", func(t *testing.T) {
		hookFn := func() uintptr {
			return 1
		}
		guard, err := NewInlineHookByName("crypt32.dll", "CryptProtectMemory", true, hookFn)
		require.NoError(t, err)
		fmt.Println(guard)
	})

	select {}
}
