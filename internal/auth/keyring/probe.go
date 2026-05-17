// Package keyring — platform-aware availability probe (T-003).
//
// Probe() detects whether the OS keyring is reachable without storing any
// real credential.  It returns (true, "") when the keyring is available or
// the probe entry simply does not exist (expected for fresh installs), and
// (false, reason) when a structural error prevents keyring access.
//
// SPEC: SPEC-MINK-AUTH-CREDENTIAL-001 (SD-1, AC-CR-020)
package keyring

import (
	"errors"
	"os"
	"runtime"

	zKeyring "github.com/zalando/go-keyring"

	"github.com/modu-ai/mink/internal/auth/credential"
)

// Probe attempts a lightweight keyring read to determine availability.
//
// Platform-specific logic:
//   - Linux: first checks DBUS_SESSION_BUS_ADDRESS; if empty, the Secret
//     Service is definitely unreachable and the probe returns false immediately
//     without making a D-Bus call.
//   - macOS / Windows: directly attempts the probe Get call.
//
// A zKeyring.ErrNotFound result means the keyring is reachable (just empty)
// and is treated as available.  Any other error is treated as unavailable and
// credential.ErrKeyringUnavailable is the returned sentinel.
func Probe() (available bool, reason string) {
	// Linux-specific early check: if D-Bus is absent, Secret Service cannot
	// be reached regardless of what go-keyring reports.
	if runtime.GOOS == "linux" {
		if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
			return false, "DBUS_SESSION_BUS_ADDRESS is not set; Secret Service unavailable"
		}
	}

	// Attempt a Get on the probe service.  We use the timeout wrapper so that
	// headless-session hang is mitigated even during probing.
	_, err := callWithTimeout(func() (string, error) {
		return zKeyring.Get(serviceIdentifier("probe"), accountName)
	})
	if err == nil || errors.Is(err, zKeyring.ErrNotFound) {
		// ErrNotFound means keyring is accessible; entry just doesn't exist.
		return true, ""
	}
	// Check our wrapped sentinel too (set when the call timed out).
	if errors.Is(err, credential.ErrKeyringUnavailable) {
		return false, err.Error()
	}
	return false, "Secret Service unavailable: " + err.Error()
}
