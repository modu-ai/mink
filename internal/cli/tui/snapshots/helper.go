// Package snapshots provides golden-file snapshot testing helpers for the TUI.
//
// Usage:
//
//	func TestFoo(t *testing.T) {
//	    snapshots.SetupAsciiTermenv()
//	    got := renderSomething()
//	    snapshots.RequireSnapshot(t, "foo", got)
//	}
//
// Regenerate golden files:
//
//	go test ./internal/cli/tui/... -run TestFoo -update
package snapshots

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// updateSnapshots reports whether golden-file regeneration is requested.
// It checks both the -update flag (if already registered, e.g., by
// charmbracelet/x/exp/golden) and the UPDATE_SNAPSHOTS environment variable.
// This avoids a "flag redefined" panic when both packages are imported in the
// same test binary.
func updateSnapshots() bool {
	if f := flag.Lookup("update"); f != nil && f.Value.String() == "true" {
		return true
	}
	return os.Getenv("UPDATE_SNAPSHOTS") == "1"
}

// SetupAsciiTermenv forces ASCII color profile for deterministic snapshots.
// Must be called before any lipgloss rendering in test.
//
// @MX:NOTE Snapshot determinism — forces termenv.Ascii, called before any lipgloss render. REQ-CLITUI-001
func SetupAsciiTermenv() {
	lipgloss.SetColorProfile(termenv.Ascii)
}

// FixedClock returns a function that always returns the given time.
// Use with tea.Program time injection for deterministic time in snapshots.
//
// @MX:NOTE Fixed time for deterministic snapshot rendering. REQ-CLITUI-001
func FixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

// goldenPath returns the path to the golden file for the given test name.
// Files are stored in testdata/snapshots/ relative to the callers' package directory.
func goldenPath(name string) string {
	return filepath.Join("testdata", "snapshots", name+".golden")
}

// RequireSnapshot compares output against a golden file.
// If the -update flag is set, the golden file is regenerated.
// Fails the test if the output does not match the stored golden file.
//
// @MX:NOTE Golden file comparison; -update flag regenerates. REQ-CLITUI-001
func RequireSnapshot(t *testing.T, name string, got []byte) {
	t.Helper()

	path := goldenPath(name)

	if updateSnapshots() {
		writeGolden(t, path, got)
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			t.Fatalf("golden file %q not found; run with -update to create it", path)
		}
		t.Fatalf("failed to read golden file %q: %v", path, err)
	}

	if string(got) != string(want) {
		t.Fatalf("snapshot mismatch for %q\n--- want ---\n%s\n--- got ---\n%s", name, want, got)
	}
}

// WriteGolden is exported for use in test setup where the golden file must
// be written before RequireSnapshot is called in the same test run.
// Callers should use RequireSnapshot with -update in normal workflows.
func WriteGolden(t *testing.T, name string, content []byte) {
	t.Helper()
	writeGolden(t, goldenPath(name), content)
}

// writeGolden writes content to path, creating parent directories as needed.
func writeGolden(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("failed to create golden directory: %v", err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("failed to write golden file %q: %v", path, err)
	}
}
