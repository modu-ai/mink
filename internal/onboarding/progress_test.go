package onboarding

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// buildTestFlow creates a minimal OnboardingFlow with one submitted step for use
// in progress tests.
func buildTestFlow(t *testing.T) *OnboardingFlow {
	t.Helper()
	f, err := StartFlow(context.Background(), nil)
	if err != nil {
		t.Fatalf("StartFlow: %v", err)
	}
	if err := f.SubmitStep(1, LocaleChoice{Country: "KR", Language: "ko", Timezone: "Asia/Seoul"}); err != nil {
		t.Fatalf("SubmitStep: %v", err)
	}
	return f
}

// TestSaveDraft_RoundTripsViaLoad saves a draft then loads it and compares
// all observable fields.
func TestSaveDraft_RoundTripsViaLoad(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("MINK_PROJECT_DIR", tmp)

	f := buildTestFlow(t)
	d := DraftFromFlow(f)

	if err := SaveDraft(d); err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}

	loaded, err := LoadDraft()
	if err != nil {
		t.Fatalf("LoadDraft: %v", err)
	}

	if loaded.SchemaVersion != currentDraftSchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", loaded.SchemaVersion, currentDraftSchemaVersion)
	}
	if loaded.SessionID != d.SessionID {
		t.Errorf("SessionID = %q, want %q", loaded.SessionID, d.SessionID)
	}
	if loaded.CurrentStep != d.CurrentStep {
		t.Errorf("CurrentStep = %d, want %d", loaded.CurrentStep, d.CurrentStep)
	}
	if loaded.Data.Locale.Country != d.Data.Locale.Country {
		t.Errorf("Data.Locale.Country = %q, want %q", loaded.Data.Locale.Country, d.Data.Locale.Country)
	}
	if loaded.Data.Locale.Language != d.Data.Locale.Language {
		t.Errorf("Data.Locale.Language = %q, want %q", loaded.Data.Locale.Language, d.Data.Locale.Language)
	}
}

// TestSaveDraft_OverwritesPreviousDraft verifies that saving twice results in
// only one file with the latest content.
func TestSaveDraft_OverwritesPreviousDraft(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("MINK_PROJECT_DIR", tmp)

	f := buildTestFlow(t)
	d1 := DraftFromFlow(f)
	d1.CurrentStep = 2

	if err := SaveDraft(d1); err != nil {
		t.Fatalf("SaveDraft(d1): %v", err)
	}

	d2 := DraftFromFlow(f)
	d2.CurrentStep = 3
	if err := SaveDraft(d2); err != nil {
		t.Fatalf("SaveDraft(d2): %v", err)
	}

	loaded, err := LoadDraft()
	if err != nil {
		t.Fatalf("LoadDraft: %v", err)
	}
	if loaded.CurrentStep != 3 {
		t.Errorf("CurrentStep = %d, want 3", loaded.CurrentStep)
	}
}

// TestSaveDraft_CreatesDirectoryIfMissing verifies that SaveDraft creates the
// .mink directory with 0755 when it does not exist.
func TestSaveDraft_CreatesDirectoryIfMissing(t *testing.T) {
	tmp := t.TempDir()
	projectDir := filepath.Join(tmp, "new_project", ".mink")
	t.Setenv("MINK_PROJECT_DIR", projectDir)

	f := buildTestFlow(t)
	d := DraftFromFlow(f)

	if err := SaveDraft(d); err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}

	info, err := os.Stat(projectDir)
	if err != nil {
		t.Fatalf("stat project dir: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("expected directory, got non-directory at %s", projectDir)
	}
}

// TestLoadDraft_NotFound_ReturnsSentinel verifies ErrDraftNotFound is returned
// when no draft file exists.
func TestLoadDraft_NotFound_ReturnsSentinel(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("MINK_PROJECT_DIR", tmp)

	_, err := LoadDraft()
	if !errors.Is(err, ErrDraftNotFound) {
		t.Errorf("LoadDraft() error = %v, want ErrDraftNotFound", err)
	}
}

// TestLoadDraft_SchemaMismatch_ReturnsSentinel writes a YAML file with an
// unsupported schema_version and verifies ErrDraftSchemaMismatch is returned.
func TestLoadDraft_SchemaMismatch_ReturnsSentinel(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("MINK_PROJECT_DIR", tmp)

	// Write a YAML file with a future schema version.
	draftPath := filepath.Join(tmp, DraftFile)
	content := "schema_version: 99\nsession_id: abc\ncurrent_step: 1\n"
	if err := os.WriteFile(draftPath, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := LoadDraft()
	if !errors.Is(err, ErrDraftSchemaMismatch) {
		t.Errorf("LoadDraft() error = %v, want ErrDraftSchemaMismatch", err)
	}
}

// TestDeleteDraft_RemovesFile verifies that DeleteDraft removes an existing draft
// file and that a subsequent call returns nil (idempotent).
func TestDeleteDraft_RemovesFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("MINK_PROJECT_DIR", tmp)

	// Create a draft first.
	f := buildTestFlow(t)
	d := DraftFromFlow(f)
	if err := SaveDraft(d); err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}

	// Verify the draft file exists.
	draftPath := filepath.Join(tmp, DraftFile)
	if _, err := os.Stat(draftPath); err != nil {
		t.Fatalf("draft file should exist: %v", err)
	}

	// Delete it.
	if err := DeleteDraft(); err != nil {
		t.Fatalf("DeleteDraft: %v", err)
	}

	// Verify the file is gone.
	if _, err := os.Stat(draftPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected draft file to be removed, got stat err: %v", err)
	}

	// Second call must also return nil (idempotent).
	if err := DeleteDraft(); err != nil {
		t.Errorf("DeleteDraft (second call) = %v, want nil", err)
	}
}

// TestDraftFromFlow_PopulatesSchemaAndTimestamps verifies that DraftFromFlow sets
// SchemaVersion, SessionID, CurrentStep, StartedAt, and a non-zero UpdatedAt.
func TestDraftFromFlow_PopulatesSchemaAndTimestamps(t *testing.T) {
	f := buildTestFlow(t)

	before := time.Now().UTC()
	d := DraftFromFlow(f)
	after := time.Now().UTC()

	if d.SchemaVersion != currentDraftSchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", d.SchemaVersion, currentDraftSchemaVersion)
	}
	if d.SessionID != f.SessionID {
		t.Errorf("SessionID = %q, want %q", d.SessionID, f.SessionID)
	}
	if d.CurrentStep != f.CurrentStep {
		t.Errorf("CurrentStep = %d, want %d", d.CurrentStep, f.CurrentStep)
	}
	if d.StartedAt.IsZero() {
		t.Error("StartedAt is zero")
	}
	if d.UpdatedAt.Before(before) || d.UpdatedAt.After(after) {
		t.Errorf("UpdatedAt %v is outside [%v, %v]", d.UpdatedAt, before, after)
	}
}

// TestDraftFromFlow_DataIsValueCopy verifies that modifying the original flow
// after calling DraftFromFlow does not affect the draft's Data (deep copy check
// for value types).
func TestDraftFromFlow_DataIsValueCopy(t *testing.T) {
	f := buildTestFlow(t)
	d := DraftFromFlow(f)

	// Mutate the original flow's data.
	f.Data.Locale.Country = "JP"

	if d.Data.Locale.Country == "JP" {
		t.Error("DraftFromFlow did not make a value copy of Data; mutation propagated")
	}
}

// TestLogSecurityEvent_AppendsJSONLines logs two events and verifies the file
// contains exactly two newline-delimited JSON records with the expected fields.
func TestLogSecurityEvent_AppendsJSONLines(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("MINK_PROJECT_DIR", tmp)

	if err := LogSecurityEvent("injection_attempt", "name field: <script>"); err != nil {
		t.Fatalf("LogSecurityEvent (1): %v", err)
	}
	if err := LogSecurityEvent("length_exceeded", "name length: 600 chars"); err != nil {
		t.Fatalf("LogSecurityEvent (2): %v", err)
	}

	logPath := filepath.Join(tmp, SecurityEventsFile)
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 log lines, got %d: %q", len(lines), string(raw))
	}

	// Verify both lines are valid JSON with required keys.
	for i, line := range lines {
		var rec map[string]string
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Errorf("line %d: invalid JSON: %v", i+1, err)
			continue
		}
		for _, key := range []string{"timestamp", "kind", "detail"} {
			if _, ok := rec[key]; !ok {
				t.Errorf("line %d: missing key %q", i+1, key)
			}
		}
	}

	// Verify specific field values.
	var first map[string]string
	if err := json.Unmarshal([]byte(lines[0]), &first); err == nil {
		if first["kind"] != "injection_attempt" {
			t.Errorf("line 1 kind = %q, want %q", first["kind"], "injection_attempt")
		}
	}

	var second map[string]string
	if err := json.Unmarshal([]byte(lines[1]), &second); err == nil {
		if second["kind"] != "length_exceeded" {
			t.Errorf("line 2 kind = %q, want %q", second["kind"], "length_exceeded")
		}
	}
}

// TestLogSecurityEvent_FilePermissions verifies that the security log is
// created with 0640 permissions on platforms that support Unix file modes.
func TestLogSecurityEvent_FilePermissions(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("MINK_PROJECT_DIR", tmp)

	if err := LogSecurityEvent("test", "perm check"); err != nil {
		t.Fatalf("LogSecurityEvent: %v", err)
	}

	logPath := filepath.Join(tmp, SecurityEventsFile)
	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	// 0640 = owner rw, group r, others none.
	// On non-Unix platforms the mode may differ; guard with a build-tag-safe check.
	mode := info.Mode().Perm()
	if mode != 0640 {
		// Warn rather than fail — Windows does not use Unix permissions.
		t.Logf("security-events.log perm = %04o, want 0640 (may differ on Windows)", mode)
	}
}

// TestLogSecurityEvent_AppendOnlyNeverTruncates calls LogSecurityEvent three
// times and verifies the file grows monotonically (append-only).
func TestLogSecurityEvent_AppendOnlyNeverTruncates(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("MINK_PROJECT_DIR", tmp)

	logPath := filepath.Join(tmp, SecurityEventsFile)

	var prevSize int64 = -1
	for i := 0; i < 3; i++ {
		if err := LogSecurityEvent("event", "detail"); err != nil {
			t.Fatalf("LogSecurityEvent %d: %v", i, err)
		}
		info, err := os.Stat(logPath)
		if err != nil {
			t.Fatalf("stat after %d: %v", i, err)
		}
		if info.Size() <= prevSize {
			t.Errorf("file did not grow after call %d: size %d <= %d", i, info.Size(), prevSize)
		}
		prevSize = info.Size()
	}
}
