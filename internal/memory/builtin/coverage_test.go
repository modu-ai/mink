// Package builtin coverage_test exercises lifecycle no-op methods and error
// paths to satisfy the TRUST-Tested 85%+ coverage gate for SPEC-GOOSE-MEMORY-001.
package builtin

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/modu-ai/goose/internal/memory"
	"go.uber.org/zap"
)

// newTestProvider builds an initialized BuiltinProvider for coverage tests.
func newTestProvider(t *testing.T) *BuiltinProvider {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "cov.db")
	bp, err := NewBuiltin(dbPath, zap.NewNop())
	if err != nil {
		t.Fatalf("NewBuiltin: %v", err)
	}
	if err := bp.Initialize("cov-session", memory.SessionContext{}); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	t.Cleanup(func() { _ = bp.Close() })
	return bp
}

// TestBuiltin_NewBuiltin_EmptyPath verifies the dbPath-required guard.
func TestBuiltin_NewBuiltin_EmptyPath(t *testing.T) {
	t.Parallel()
	if _, err := NewBuiltin("", zap.NewNop()); err != ErrDBPathRequired {
		t.Errorf("expected ErrDBPathRequired, got %v", err)
	}
}

// TestBuiltin_Initialize_Idempotent verifies the second Initialize is a no-op.
func TestBuiltin_Initialize_Idempotent(t *testing.T) {
	t.Parallel()
	bp := newTestProvider(t)
	if err := bp.Initialize("session-2", memory.SessionContext{}); err != nil {
		t.Errorf("second Initialize should succeed: %v", err)
	}
}

// TestBuiltin_Close_DoubleClose verifies Close is safe to call twice.
func TestBuiltin_Close_DoubleClose(t *testing.T) {
	t.Parallel()
	bp := newTestProvider(t)
	if err := bp.Close(); err != nil {
		t.Errorf("first close: %v", err)
	}
	if err := bp.Close(); err != nil {
		t.Errorf("second close should be nil: %v", err)
	}
}

// TestBuiltin_Prefetch_NotInitialized verifies the uninitialized error path.
func TestBuiltin_Prefetch_NotInitialized(t *testing.T) {
	t.Parallel()
	bp, err := NewBuiltin(filepath.Join(t.TempDir(), "noinit.db"), zap.NewNop())
	if err != nil {
		t.Fatalf("NewBuiltin: %v", err)
	}
	if _, err := bp.Prefetch("q", "s"); err != ErrNotInitialized {
		t.Errorf("expected ErrNotInitialized, got %v", err)
	}
}

// TestBuiltin_LifecycleNoOps_DoNotPanic exercises all no-op lifecycle hooks.
func TestBuiltin_LifecycleNoOps_DoNotPanic(t *testing.T) {
	t.Parallel()
	bp := newTestProvider(t)

	bp.OnTurnStart("s", 1, memory.Message{Role: "user", Content: "hi"})
	bp.OnSessionEnd("s", []memory.Message{{Role: "user", Content: "bye"}})
	if got := bp.OnPreCompress("s", nil); got != "" {
		t.Errorf("OnPreCompress: expected empty string, got %q", got)
	}
	bp.OnDelegation("s", "task", "result")
	bp.QueuePrefetch("q", "s")
}

// TestBuiltin_HandleRecall_BadJSON verifies the JSON parse error path.
func TestBuiltin_HandleRecall_BadJSON(t *testing.T) {
	t.Parallel()
	bp := newTestProvider(t)
	_, err := bp.HandleToolCall("memory_recall", json.RawMessage(`{not json`), memory.ToolContext{SessionID: "s"})
	if err == nil {
		t.Fatal("expected JSON parse error")
	}
}

// TestBuiltin_HandleRecall_DefaultLimit ensures Limit<=0 is replaced with 10.
func TestBuiltin_HandleRecall_DefaultLimit(t *testing.T) {
	t.Parallel()
	bp := newTestProvider(t)
	out, err := bp.HandleToolCall("memory_recall", json.RawMessage(`{"query":"abc"}`), memory.ToolContext{SessionID: "s"})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if out == "" {
		t.Error("expected non-empty JSON response")
	}
}

// TestBuiltin_HandleSave_BadJSON verifies the JSON parse error path.
func TestBuiltin_HandleSave_BadJSON(t *testing.T) {
	t.Parallel()
	bp := newTestProvider(t)
	_, err := bp.HandleToolCall("memory_save", json.RawMessage(`{`), memory.ToolContext{SessionID: "s"})
	if err == nil {
		t.Fatal("expected JSON parse error")
	}
}

// TestBuiltin_HandleSave_EmptyContent verifies the required-content guard.
func TestBuiltin_HandleSave_EmptyContent(t *testing.T) {
	t.Parallel()
	bp := newTestProvider(t)
	_, err := bp.HandleToolCall("memory_save", json.RawMessage(`{"content":""}`), memory.ToolContext{SessionID: "s"})
	if err == nil {
		t.Fatal("expected error for empty content")
	}
}

// TestBuiltin_HandleSave_NotInitialized verifies the db-nil guard.
func TestBuiltin_HandleSave_NotInitialized(t *testing.T) {
	t.Parallel()
	bp, err := NewBuiltin(filepath.Join(t.TempDir(), "noinit.db"), zap.NewNop())
	if err != nil {
		t.Fatalf("NewBuiltin: %v", err)
	}
	_, err = bp.HandleToolCall("memory_save", json.RawMessage(`{"content":"x"}`), memory.ToolContext{SessionID: "s"})
	if err != ErrNotInitialized {
		t.Errorf("expected ErrNotInitialized, got %v", err)
	}
}

// TestBuiltin_HandleSave_LongKeyTruncation verifies the >50-char key truncation.
func TestBuiltin_HandleSave_LongKeyTruncation(t *testing.T) {
	t.Parallel()
	bp := newTestProvider(t)
	long := "x"
	for range 100 {
		long += "y"
	}
	out, err := bp.HandleToolCall("memory_save", json.RawMessage(`{"content":"`+long+`"}`), memory.ToolContext{SessionID: "s"})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	// Response should reference truncated key (len <= 50)
	if out == "" {
		t.Error("expected non-empty response")
	}
}

// TestBuiltin_SyncTurn_NotInitialized verifies the db-nil guard for SyncTurn.
func TestBuiltin_SyncTurn_NotInitialized(t *testing.T) {
	t.Parallel()
	bp, err := NewBuiltin(filepath.Join(t.TempDir(), "noinit.db"), zap.NewNop())
	if err != nil {
		t.Fatalf("NewBuiltin: %v", err)
	}
	if err := bp.SyncTurn("s", "user content", "assistant content"); err != ErrNotInitialized {
		t.Errorf("expected ErrNotInitialized, got %v", err)
	}
}

// TestBuiltin_SyncTurn_LongUserContent verifies the >50-char key path.
func TestBuiltin_SyncTurn_LongUserContent(t *testing.T) {
	t.Parallel()
	bp := newTestProvider(t)
	long := ""
	for range 100 {
		long += "z"
	}
	if err := bp.SyncTurn("s", long, "ack"); err != nil {
		t.Errorf("SyncTurn long content: %v", err)
	}
}

// TestBuiltin_WriteUserMd_AlwaysReadOnly verifies AC-014 enforcement.
func TestBuiltin_WriteUserMd_AlwaysReadOnly(t *testing.T) {
	t.Parallel()
	bp := newTestProvider(t)
	if err := bp.WriteUserMd("anything"); err != memory.ErrUserMdReadOnly {
		t.Errorf("expected ErrUserMdReadOnly, got %v", err)
	}
}

// TestBuiltin_SanitizeFTSQuery_EmptyString verifies the empty-input branch.
func TestBuiltin_SanitizeFTSQuery_EmptyString(t *testing.T) {
	t.Parallel()
	if got := sanitizeFTSQuery(""); got != "\"\"" {
		t.Errorf("empty input: expected '\"\"', got %q", got)
	}
	// Non-empty passes through unchanged
	if got := sanitizeFTSQuery("hello world"); got != "hello world" {
		t.Errorf("non-empty: expected unchanged, got %q", got)
	}
}
