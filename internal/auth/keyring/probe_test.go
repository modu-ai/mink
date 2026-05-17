package keyring_test

import (
	"testing"

	"github.com/modu-ai/mink/internal/auth/keyring"
)

// TestProbeNoPanic asserts that Probe() returns without panicking on any
// platform.  In CI the keyring may or may not be available; the test only
// verifies the function contract (consistent return values), not the result.
//
// AC-CR-020 (partial): the full keyring-unavailable scenario is verified in
// the integration test (backend_integration_test.go).
func TestProbeNoPanic(t *testing.T) {
	available, reason := keyring.Probe()

	// (available=true, reason!="") would be internally inconsistent.
	if available && reason != "" {
		t.Errorf("Probe() returned available=true with non-empty reason %q", reason)
	}
	// (available=false, reason=="") would not help callers diagnose the issue.
	if !available && reason == "" {
		t.Errorf("Probe() returned available=false with empty reason")
	}
}
