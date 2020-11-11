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
			originCaption := windows.UTF16PtrToString(caption)

			fmt.Println(originText, originCaption)

			hookedText := fmt.Sprintf("origin: %s, hooked!", originText)
			hookedCaption := fmt.Sprintf("origin: %s, hooked!", originCaption)
			hookedTextPtr, err := windows.UTF16PtrFromString(hookedText)
			require.NoError(t, err)
			hookedCaptionPtr, err := windows.UTF16PtrFromString(hookedCaption)
			require.NoError(t, err)

			// call original function
			ret, _, _ := guard.Original.Call(
				uintptr(hwnd),
				uintptr(unsafe.Pointer(hookedTextPtr)),
				uintptr(unsafe.Pointer(hookedCaptionPtr)),
				uintptr(uType))
			require.Equal(t, uintptr(1), ret)
			return 1234
		}
		guard, err = NewInlineHookByName("user32.dll", "MessageBoxW", true, hookFn)
		require.NoError(t, err)

		proc := windows.NewLazySystemDLL("user32.dll").NewProc("MessageBoxW")

		textPtr, err := windows.UTF16PtrFromString("text")
		require.NoError(t, err)
		captionPtr, err := windows.UTF16PtrFromString("caption")
		require.NoError(t, err)

		ret, _, _ := proc.Call(0, uintptr(unsafe.Pointer(textPtr)), uintptr(unsafe.Pointer(captionPtr)), 1)
		require.Equal(t, uintptr(1234), ret)
	})

	t.Run("CryptProtectMemory", func(t *testing.T) {
		hookFn := func() uintptr {
			return 1
		}
		guard, err := NewInlineHookByName("crypt32.dll", "CryptProtectMemory", true, hookFn)
		require.NoError(t, err)
		fmt.Println(guard)
	})

}
