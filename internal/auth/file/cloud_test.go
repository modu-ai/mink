// Package file — unit tests for WarnIfCloudSynced (T-009).
//
// Table-driven tests covering at least 6 path scenarios: 3 that should warn
// and 3 that should not.
//
// SPEC: SPEC-MINK-AUTH-CREDENTIAL-001 (T-009, AC-CR-022 partial)
package file

import (
	"testing"
)

// TestWarnIfCloudSynced verifies the cloud-sync path detection logic.
// Matching is case-insensitive on all platforms — see WarnIfCloudSynced
// documentation for rationale (R4 risk uniformity).
func TestWarnIfCloudSynced(t *testing.T) {
	type testCase struct {
		name    string
		path    string
		wantMsg bool // true = expect a non-empty warning
	}

	tests := []testCase{
		// ---- should warn -------------------------------------------------
		{
			name:    "iCloud Drive on macOS",
			path:    "/Users/alice/iCloud Drive/Documents/.mink/auth/credentials.json",
			wantMsg: true,
		},
		{
			name:    "OneDrive",
			path:    "/Users/alice/OneDrive/.mink/auth/credentials.json",
			wantMsg: true,
		},
		{
			name:    "Dropbox",
			path:    "/Users/alice/Dropbox/.mink/auth/credentials.json",
			wantMsg: true,
		},
		{
			name:    "Google Drive",
			path:    "/Users/alice/Google Drive/.mink/auth/credentials.json",
			wantMsg: true,
		},
		{
			name:    "Box",
			path:    "/Users/alice/Box/.mink/auth/credentials.json",
			wantMsg: true,
		},

		// ---- should NOT warn ---------------------------------------------
		{
			name:    "default home dir path",
			path:    "/home/alice/.mink/auth/credentials.json",
			wantMsg: false,
		},
		{
			name:    "macOS default home",
			path:    "/Users/alice/.mink/auth/credentials.json",
			wantMsg: false,
		},
		{
			name:    "Documents folder (not cloud)",
			path:    "/home/alice/Documents/.mink/auth/credentials.json",
			wantMsg: false,
		},
		{
			name:    "root path only",
			path:    "/credentials.json",
			wantMsg: false,
		},
	}

	// Case-insensitive matching applies on all platforms — verify with a
	// lowercase-named Dropbox folder typically created by the Linux client.
	tests = append(tests, testCase{
		name:    "lowercase dropbox folder (linux client)",
		path:    "/home/alice/dropbox/.mink/auth/credentials.json",
		wantMsg: true,
	})

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg := WarnIfCloudSynced(tc.path)
			if tc.wantMsg && msg == "" {
				t.Errorf("WarnIfCloudSynced(%q): expected non-empty warning, got empty", tc.path)
			}
			if !tc.wantMsg && msg != "" {
				t.Errorf("WarnIfCloudSynced(%q): expected empty warning, got %q", tc.path, msg)
			}
		})
	}
}
