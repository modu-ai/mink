//go:build darwin

package locale

import (
	"context"
	"os/exec"
	"strings"
)

// detectFromOSAPIs performs macOS-specific locale detection.
//
// Calls `defaults read -g AppleLocale` once (research.md §2.2).
// CGO (CFLocaleCopyCurrent) is intentionally excluded to avoid complexity.
func detectFromOSAPIs(ctx context.Context) (country, lang string, err error) {
	// execCommand is the injectable indirection used in tests.
	out, cmdErr := execCommandFn(ctx, "defaults", "read", "-g", "AppleLocale")
	if cmdErr != nil {
		return "", "", ErrNoOSLocale
	}

	// AppleLocale format: "ko_KR@calendar=gregorian" or "ja_JP"
	val := strings.TrimSpace(out)
	if idx := strings.IndexByte(val, '@'); idx >= 0 {
		val = val[:idx]
	}

	c, l, ok := parseLocaleString(val)
	if !ok {
		return "", "", ErrNoOSLocale
	}
	return c, l, nil
}

// execCommandFn is the injectable indirection for exec.CommandContext calls.
// Tests substitute a fake that returns fixture strings without spawning subprocesses.
var execCommandFn = func(ctx context.Context, name string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, name, args...).Output()
	return string(out), err
}
