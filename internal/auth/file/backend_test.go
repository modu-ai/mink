// Package file — unit tests for FileBackend.
//
// Tests cover:
//   - Store → Load → Delete → List round-trip via a tmp dir
//   - mode 0600 verified via os.Stat (POSIX only; Windows test skipped)
//   - Concurrent Store calls (race-detector clean)
//   - JSON round-trip with unknown kind (forward-compat)
//   - Idempotent Delete
//   - .gitignore assertion (AC-CR-026)
//
// SPEC: SPEC-MINK-AUTH-CREDENTIAL-001 (AC-CR-004, AC-CR-026, AC-CR-027, T-006)
package file

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/modu-ai/mink/internal/auth/credential"
)

// newTestBackend creates a Backend whose credentials file lives in a fresh
// temporary directory.  The returned cleanup function removes the dir.
func newTestBackend(t *testing.T) (*Backend, func()) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, ".mink", "auth", "credentials.json")
	b, err := NewBackend(WithPath(path))
	if err != nil {
		t.Fatalf("NewBackend: %v", err)
	}
	return b, func() { os.RemoveAll(dir) }
}

// TestStoreLoadDeleteListRoundTrip verifies the basic CRUD cycle.
func TestStoreLoadDeleteListRoundTrip(t *testing.T) {
	b, cleanup := newTestBackend(t)
	defer cleanup()

	cred := credential.APIKey{Value: "sk-ant-test-1234567890"}

	// Store
	if err := b.Store("anthropic", cred); err != nil {
		t.Fatalf("Store: %v", err)
	}

	// Load
	loaded, err := b.Load("anthropic")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	apiKey, ok := loaded.(credential.APIKey)
	if !ok {
		t.Fatalf("Load: expected APIKey, got %T", loaded)
	}
	if apiKey.Value != cred.Value {
		t.Errorf("Load: value mismatch: got %q, want %q", apiKey.Value, cred.Value)
	}

	// List
	ids, err := b.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if !slices.Contains(ids, "anthropic") {
		t.Errorf("List: expected 'anthropic' in %v", ids)
	}

	// Delete
	if err := b.Delete("anthropic"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Load after delete should return ErrNotFound
	_, err = b.Load("anthropic")
	if !credential.IsNotFound(err) {
		t.Errorf("Load after Delete: expected ErrNotFound, got %v", err)
	}
}

// TestStoreCreatesParentDir verifies that Store creates missing parent dirs
// with mode 0700.
func TestStoreCreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	// Point at a deeply nested path that doesn't exist yet.
	path := filepath.Join(dir, "a", "b", "c", "credentials.json")
	b, err := NewBackend(WithPath(path))
	if err != nil {
		t.Fatalf("NewBackend: %v", err)
	}

	if err := b.Store("deepseek", credential.APIKey{Value: "ds-key-xyz"}); err != nil {
		t.Fatalf("Store: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("credentials.json should exist: %v", err)
	}
}

// TestFileMode0600 verifies that the credentials file has mode 0600 on POSIX.
func TestFileMode0600(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mode 0600 check not applicable on Windows (NTFS ACL gap documented)")
	}

	b, cleanup := newTestBackend(t)
	defer cleanup()

	if err := b.Store("anthropic", credential.APIKey{Value: "sk-test-0600"}); err != nil {
		t.Fatalf("Store: %v", err)
	}

	info, err := os.Stat(b.path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Errorf("expected mode 0600, got %04o", got)
	}
}

// TestVerifyModeRejectsWrongPermission verifies that verifyMode returns an
// error when the file has the wrong permissions.
func TestVerifyModeRejectsWrongPermission(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("verifyMode is a no-op on Windows")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "bad_perm.json")

	// Write with 0644 — intentionally wrong.
	if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := verifyMode(path); err == nil {
		t.Error("verifyMode: expected error for mode 0644, got nil")
	}
}

// TestVerifyModeAcceptsCorrectPermission verifies that verifyMode returns nil
// for mode 0600.
func TestVerifyModeAcceptsCorrectPermission(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("verifyMode is a no-op on Windows")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "good_perm.json")

	if err := os.WriteFile(path, []byte("{}"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := verifyMode(path); err != nil {
		t.Errorf("verifyMode: unexpected error: %v", err)
	}
}

// TestConcurrentStore verifies that multiple goroutines can call Store
// concurrently without data races (race detector must pass).
func TestConcurrentStore(t *testing.T) {
	b, cleanup := newTestBackend(t)
	defer cleanup()

	providers := []string{"anthropic", "deepseek", "openai_gpt", "zai_glm"}
	var wg sync.WaitGroup

	for _, p := range providers {
		wg.Add(1)
		go func(provider string) {
			defer wg.Done()
			if err := b.Store(provider, credential.APIKey{Value: "key-" + provider}); err != nil {
				t.Errorf("concurrent Store(%s): %v", provider, err)
			}
		}(p)
	}
	wg.Wait()

	ids, err := b.List()
	if err != nil {
		t.Fatalf("List after concurrent Store: %v", err)
	}
	if len(ids) != len(providers) {
		t.Errorf("expected %d providers after concurrent Store, got %d: %v",
			len(providers), len(ids), ids)
	}
}

// TestJSONToleratesUnknownKind verifies forward-compat: a JSON file with an
// unrecognised kind value does not corrupt other entries.
func TestJSONToleratesUnknownKind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")

	// Write a file with a mix of known and unknown kinds.
	raw := `{
  "version": 1,
  "credentials": {
    "anthropic": {"kind": "api_key", "value": "sk-ant-test"},
    "future_provider": {"kind": "unknown_future_kind", "value": "some-val"}
  }
}`
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	b, err := NewBackend(WithPath(path))
	if err != nil {
		t.Fatalf("NewBackend: %v", err)
	}

	// The known provider should load cleanly.
	cred, err := b.Load("anthropic")
	if err != nil {
		t.Fatalf("Load anthropic: %v", err)
	}
	apiKey, ok := cred.(credential.APIKey)
	if !ok || apiKey.Value != "sk-ant-test" {
		t.Errorf("Load anthropic: unexpected result: %v", cred)
	}

	// The unknown kind should return ErrNotFound (skipped with warning to stderr).
	_, err = b.Load("future_provider")
	if !credential.IsNotFound(err) {
		t.Errorf("Load future_provider: expected ErrNotFound, got %v", err)
	}
}

// TestDeleteIdempotent verifies that deleting a non-existent provider returns
// nil (ED-3).
func TestDeleteIdempotent(t *testing.T) {
	b, cleanup := newTestBackend(t)
	defer cleanup()

	// Delete on a provider that was never stored.
	if err := b.Delete("anthropic"); err != nil {
		t.Errorf("Delete (non-existent): expected nil, got %v", err)
	}

	// Delete the same provider twice after storing it.
	if err := b.Store("anthropic", credential.APIKey{Value: "sk-test"}); err != nil {
		t.Fatalf("Store: %v", err)
	}
	if err := b.Delete("anthropic"); err != nil {
		t.Fatalf("first Delete: %v", err)
	}
	if err := b.Delete("anthropic"); err != nil {
		t.Errorf("second Delete: expected nil, got %v", err)
	}
}

// TestLoadNotFound verifies ErrNotFound on a missing provider.
func TestLoadNotFound(t *testing.T) {
	b, cleanup := newTestBackend(t)
	defer cleanup()

	_, err := b.Load("nonexistent")
	if !credential.IsNotFound(err) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// TestLoadNotFoundEmptyFile verifies ErrNotFound when the file does not exist.
func TestLoadNotFoundEmptyFile(t *testing.T) {
	dir := t.TempDir()
	b, err := NewBackend(WithPath(filepath.Join(dir, "does_not_exist.json")))
	if err != nil {
		t.Fatalf("NewBackend: %v", err)
	}
	_, err = b.Load("anthropic")
	if !credential.IsNotFound(err) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// TestHealthPresent verifies that Health reports present + masked value.
func TestHealthPresent(t *testing.T) {
	b, cleanup := newTestBackend(t)
	defer cleanup()

	if err := b.Store("anthropic", credential.APIKey{Value: "sk-ant-1234567890"}); err != nil {
		t.Fatalf("Store: %v", err)
	}

	status, err := b.Health("anthropic")
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if !status.Present {
		t.Error("Health: expected Present=true")
	}
	if status.Backend != backendName {
		t.Errorf("Health: expected Backend=%q, got %q", backendName, status.Backend)
	}
	// Must not contain the full key value.
	if strings.Contains(status.MaskedLast4, "sk-ant-1234567890") {
		t.Errorf("Health: plaintext leaked in MaskedLast4: %q", status.MaskedLast4)
	}
}

// TestHealthAbsent verifies that Health reports absent when no entry exists.
func TestHealthAbsent(t *testing.T) {
	b, cleanup := newTestBackend(t)
	defer cleanup()

	status, err := b.Health("anthropic")
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if status.Present {
		t.Error("Health: expected Present=false")
	}
}

// TestGitignoreContainsMinkAuth verifies AC-CR-026: the project .gitignore
// includes a pattern that prevents ~/.mink/auth/ from being committed
// accidentally.
func TestGitignoreContainsMinkAuth(t *testing.T) {
	// Walk up from this file's directory to find the project root (.gitignore).
	// This test file lives at internal/auth/file/backend_test.go.
	// We find the absolute path of the current test binary's working dir, then
	// walk upward until we find a .gitignore file.
	var gitignorePath string
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		candidate := filepath.Join(dir, ".gitignore")
		if _, statErr := os.Stat(candidate); statErr == nil {
			gitignorePath = candidate
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find .gitignore by walking up from test working directory")
		}
		dir = parent
	}

	data, readErr := os.ReadFile(gitignorePath)
	if readErr != nil {
		t.Fatalf("read .gitignore at %q: %v", gitignorePath, readErr)
	}

	content := string(data)
	// AC-CR-026: pattern must cover **/.mink/auth/ or equivalent.
	patterns := []string{"**/.mink/auth/", ".mink/auth/", ".mink/"}
	for _, p := range patterns {
		if strings.Contains(content, p) {
			return // found a matching pattern
		}
	}
	t.Errorf(".gitignore does not contain any of %v — AC-CR-026 requires the mink auth dir to be excluded from version control", patterns)
}
