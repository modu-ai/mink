//go:build windows

package locale

import (
	"context"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// detectFromOSAPIs performs Windows-specific locale detection.
//
// Uses GetUserDefaultLocaleName via syscall.NewLazySystemDLL (pure Go, no CGO).
// research.md §2.3 cites this as the standard method.
//
// TODO: Add GetUserPreferredUILanguages for secondary_language extraction in a follow-up PR.
func detectFromOSAPIs(_ context.Context) (country, lang string, err error) {
	// getUserLocaleName is injected in tests; in production it calls the Windows API.
	name, apiErr := getUserLocaleName()
	if apiErr != nil {
		return "", "", ErrNoOSLocale
	}

	// Windows returns locale names like "ko-KR", "en-US", "ja-JP".
	// Normalise hyphens to underscores for parseLocaleString.
	normalized := strings.ReplaceAll(name, "-", "_")

	c, l, ok := parseLocaleString(normalized)
	if !ok {
		return "", "", ErrNoOSLocale
	}
	return c, l, nil
}

// getUserLocaleName is the injectable indirection for GetUserDefaultLocaleName.
// Tests substitute a fake; production calls the Win32 API via syscall.
//
// @MX:WARN: [AUTO] unsafe.Pointer passed via uintptr to proc.Call; GC may relocate buf mid-syscall.
// @MX:REASON: unsafe.Pointer requires runtime.KeepAlive to prevent GC relocation mid-syscall.
var getUserLocaleName = func() (string, error) {
	// kernel32.GetUserDefaultLocaleName is exposed via golang.org/x/sys/windows
	// (syscall.NewLazySystemDLL does not exist on the stdlib side).
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	proc := kernel32.NewProc("GetUserDefaultLocaleName")
	if err := proc.Find(); err != nil {
		return "", err
	}

	const maxLen = 85 // LOCALE_NAME_MAX_LENGTH
	buf := make([]uint16, maxLen)

	r, _, e := proc.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(maxLen),
	)
	// Keep buf alive until after proc.Call returns to prevent GC from relocating
	// the backing array while the Windows API is writing into it.
	runtime.KeepAlive(buf)
	if r == 0 {
		if e != nil && e.(syscall.Errno) != 0 {
			return "", e
		}
		return "", ErrNoOSLocale
	}

	return syscall.UTF16ToString(buf), nil
}
