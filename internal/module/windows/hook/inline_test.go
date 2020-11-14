// +build windows

package hook

import (
	"fmt"
	"reflect"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

func TestNewInlineHookByName(t *testing.T) {
	var err error
	t.Run("MessageBoxTimeoutW", func(t *testing.T) {
		var pg *PatchGuard
		hookFn := func(hwnd windows.Handle, text, caption *uint16, uType uint, id uint32, timeout uint) int {
			// compare parameters
			originText := windows.UTF16PtrToString(text)
			originCaption := windows.UTF16PtrToString(caption)
			require.Equal(t, "text", originText)
			require.Equal(t, "caption", originCaption)
			require.Equal(t, uint(1), uType)
			require.Equal(t, uint32(0), id)
			require.Equal(t, uint(1000), timeout)

			// call original function
			hookedText := fmt.Sprintf("origin: %s, hooked!", originText)
			hookedCaption := fmt.Sprintf("origin: %s, hooked!", originCaption)
			hookedTextPtr, err := windows.UTF16PtrFromString(hookedText)
			require.NoError(t, err)
			hookedCaptionPtr, err := windows.UTF16PtrFromString(hookedCaption)
			require.NoError(t, err)
			ret, _, _ := pg.Original.Call(
				uintptr(hwnd), uintptr(unsafe.Pointer(hookedTextPtr)),
				uintptr(unsafe.Pointer(hookedCaptionPtr)), uintptr(uType), 0, 1000,
			)
			require.Equal(t, uintptr(32000), ret)

			// return fake return value
			return 1234
		}
		pg, err = NewInlineHookByName("user32.dll", "MessageBoxTimeoutW", true, hookFn)
		require.NoError(t, err)

		textPtr, err := windows.UTF16PtrFromString("text")
		require.NoError(t, err)
		captionPtr, err := windows.UTF16PtrFromString("caption")
		require.NoError(t, err)

		proc := windows.NewLazySystemDLL("user32.dll").NewProc("MessageBoxTimeoutW")
		ret, _, _ := proc.Call(
			0, uintptr(unsafe.Pointer(textPtr)), uintptr(unsafe.Pointer(captionPtr)), 1, 0, 1000,
		)
		require.Equal(t, uintptr(1234), ret)

		// after unpatch
		err = pg.UnPatch()
		require.NoError(t, err)

		ret, _, _ = proc.Call(
			0, uintptr(unsafe.Pointer(textPtr)), uintptr(unsafe.Pointer(captionPtr)), 1, 0, 1000,
		)
		require.Equal(t, uintptr(32000), ret)
	})

	t.Run("CryptProtectMemory", func(t *testing.T) {
		data := make([]byte, 16)
		for i := 0; i < len(data); i++ {
			data[i] = byte(i)
		}
		var pg *PatchGuard
		hookFn := func(address uintptr, size, flags uint) uintptr {
			// compare parameters
			var d []byte
			sh := (*reflect.SliceHeader)(unsafe.Pointer(&d))
			sh.Data = address
			sh.Len = 16
			sh.Cap = 16
			require.Equal(t, data, d)
			require.Equal(t, uint(16), size)
			require.Equal(t, uint(1), flags)

			// call original function
			ret, _, _ := pg.Original.Call(uintptr(unsafe.Pointer(&data[0])), 16, 1)
			require.Equal(t, uintptr(1), ret)

			// return fake return value
			return 1234
		}
		pg, err = NewInlineHookByName("crypt32.dll", "CryptProtectMemory", true, hookFn)
		require.NoError(t, err)

		proc := windows.NewLazySystemDLL("crypt32.dll").NewProc("CryptProtectMemory")
		ret, _, _ := proc.Call(uintptr(unsafe.Pointer(&data[0])), 16, 1)
		require.Equal(t, uintptr(1234), ret)

		// after unpatch
		err = pg.UnPatch()
		require.NoError(t, err)

		ret, _, _ = proc.Call(uintptr(unsafe.Pointer(&data[0])), 16, 1)
		require.Equal(t, uintptr(1), ret)
	})

	t.Run("", func(t *testing.T) {

	})
}
