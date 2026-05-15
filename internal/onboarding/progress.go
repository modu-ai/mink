// Package onboarding — progress.go implements draft persistence and security event
// logging for the MINK onboarding wizard.
//
// Draft lifecycle:
//   - SaveDraft: atomically writes the current wizard state to
//     <project>/.mink/onboarding-draft.yaml (0644). Creates the .mink directory
//     (0755) if it does not exist.
//   - LoadDraft: reads the draft and validates SchemaVersion.
//   - DeleteDraft: removes the file; idempotent (nil on second call).
//   - DraftFromFlow: snapshot helper converting an OnboardingFlow to a Draft.
//
// Security event logging (REQ-OB-017):
//   - LogSecurityEvent: appends a single JSON line to
//     <project>/.mink/security-events.log (0640). Never truncates.
//
// SPEC: SPEC-MINK-ONBOARDING-001 §6.0
// REQ: REQ-OB-011, REQ-OB-017
// AC: AC-OB-011, AC-OB-015
package onboarding

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// currentDraftSchemaVersion is the schema_version value written by SaveDraft.
// Increment this constant when the Draft struct layout changes in a
// backward-incompatible way; LoadDraft will reject older values.
const currentDraftSchemaVersion = 1

// Sentinel errors returned by progress functions.
var (
	// ErrDraftNotFound is returned by LoadDraft when the draft file does not
	// exist (e.g., the user has never paused the wizard, or DeleteDraft was called).
	ErrDraftNotFound = errors.New("onboarding draft not found")

	// ErrDraftSchemaMismatch is returned by LoadDraft when the on-disk
	// schema_version does not equal currentDraftSchemaVersion.
	ErrDraftSchemaMismatch = errors.New("onboarding draft schema version mismatch")
)

// Draft is the on-disk representation of a paused OnboardingFlow.
// It is intentionally a separate struct from OnboardingFlow to allow schema
// evolution without breaking in-flight drafts.
type Draft struct {
	// SchemaVersion guards against loading drafts written by an incompatible
	// version of this package. Current value: currentDraftSchemaVersion (1).
	SchemaVersion int `yaml:"schema_version"`

	// SessionID is copied from OnboardingFlow.SessionID.
	SessionID string `yaml:"session_id"`

	// CurrentStep is the step the user must complete next (copied from
	// OnboardingFlow.CurrentStep at snapshot time).
	CurrentStep int `yaml:"current_step"`

	// Data holds all collected wizard answers at snapshot time.
	Data OnboardingData `yaml:"data"`

	// StartedAt is when the wizard was first started (UTC).
	StartedAt time.Time `yaml:"started_at"`

	// UpdatedAt is when SaveDraft was called (UTC). Set by DraftFromFlow.
	UpdatedAt time.Time `yaml:"updated_at"`
}

// SaveDraft writes d atomically to <project>/.mink/onboarding-draft.yaml.
//
// Atomic guarantee: the YAML payload is first written to a temporary file
// ("<draft>.tmp.<pid>") and then renamed into place. A pre-existing draft is
// overwritten only when the rename succeeds.
//
// File permissions: 0644 (draft contains no secrets — API keys live in the OS
// keyring, not here).
// Directory permissions: 0755 (project workspace is non-secret).
//
// @MX:ANCHOR: [AUTO] SaveDraft is called by CLI init, Web UI HTTP handler, and
// the completion path — fan_in expected >= 3.
// @MX:REASON: Atomic write contract (tmp + rename) must not be weakened;
// any change to permissions or path logic affects all callers simultaneously.
// @MX:SPEC: SPEC-MINK-ONBOARDING-001 §6.0
func SaveDraft(d *Draft) error {
	draftPath, err := DraftPath()
	if err != nil {
		return fmt.Errorf("onboarding: resolve draft path: %w", err)
	}

	// Ensure parent directory exists.
	dir := filepath.Dir(draftPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("onboarding: create .mink dir: %w", err)
	}

	data, err := yaml.Marshal(d)
	if err != nil {
		return fmt.Errorf("onboarding: marshal draft: %w", err)
	}

	// Write to a temporary file next to the final path to ensure the rename
	// is atomic on POSIX systems (same filesystem).
	tmpPath := fmt.Sprintf("%s.tmp.%d", draftPath, os.Getpid())
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("onboarding: write draft tmp: %w", err)
	}

	// Rename is atomic on POSIX (same mount point guaranteed above).
	if err := os.Rename(tmpPath, draftPath); err != nil {
		// Best-effort cleanup of the tmp file.
		_ = os.Remove(tmpPath)
		return fmt.Errorf("onboarding: rename draft: %w", err)
	}
	return nil
}

// LoadDraft reads <project>/.mink/onboarding-draft.yaml and returns the Draft.
//
// Returns ErrDraftNotFound when the file does not exist.
// Returns ErrDraftSchemaMismatch when SchemaVersion != currentDraftSchemaVersion.
func LoadDraft() (*Draft, error) {
	draftPath, err := DraftPath()
	if err != nil {
		return nil, fmt.Errorf("onboarding: resolve draft path: %w", err)
	}

	raw, err := os.ReadFile(draftPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrDraftNotFound
		}
		return nil, fmt.Errorf("onboarding: read draft: %w", err)
	}

	var d Draft
	if err := yaml.Unmarshal(raw, &d); err != nil {
		return nil, fmt.Errorf("onboarding: unmarshal draft: %w", err)
	}

	if d.SchemaVersion != currentDraftSchemaVersion {
		return nil, fmt.Errorf("%w: got %d, want %d",
			ErrDraftSchemaMismatch, d.SchemaVersion, currentDraftSchemaVersion)
	}

	return &d, nil
}

// DeleteDraft removes the draft file. Returns nil if the file does not exist
// (idempotent — safe to call after successful onboarding completion).
func DeleteDraft() error {
	draftPath, err := DraftPath()
	if err != nil {
		return fmt.Errorf("onboarding: resolve draft path: %w", err)
	}

	if err := os.Remove(draftPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("onboarding: delete draft: %w", err)
	}
	return nil
}

// DraftFromFlow converts f into a Draft snapshot suitable for SaveDraft.
// SchemaVersion is set to currentDraftSchemaVersion.
// UpdatedAt is set to time.Now().UTC().
func DraftFromFlow(f *OnboardingFlow) *Draft {
	return &Draft{
		SchemaVersion: currentDraftSchemaVersion,
		SessionID:     f.SessionID,
		CurrentStep:   f.CurrentStep,
		Data:          f.Data, // value copy — OnboardingData contains no pointers
		StartedAt:     f.StartedAt,
		UpdatedAt:     time.Now().UTC(),
	}
}

// securityEventRecord is the JSON structure written to security-events.log.
type securityEventRecord struct {
	Timestamp string `json:"timestamp"`
	Kind      string `json:"kind"`
	Detail    string `json:"detail"`
}

// LogSecurityEvent appends a single newline-delimited JSON event to
// <project>/.mink/security-events.log.
//
// Format per line: {"timestamp":"<RFC3339>","kind":"<kind>","detail":"<detail>"}
//
// File permissions: 0640 (security log is sensitive but not a secret key file).
// The file is never truncated — only O_APPEND writes are performed.
//
// Typical callers: validators.ValidatePersonaName on ErrNameInjection (AC-OB-015).
func LogSecurityEvent(kind string, detail string) error {
	logPath, err := SecurityEventsPath()
	if err != nil {
		return fmt.Errorf("onboarding: resolve security-events path: %w", err)
	}

	// Ensure parent directory exists.
	dir := filepath.Dir(logPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("onboarding: create .mink dir for security log: %w", err)
	}

	rec := securityEventRecord{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Kind:      kind,
		Detail:    detail,
	}
	line, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("onboarding: marshal security event: %w", err)
	}
	line = append(line, '\n')

	// Open with append-only flags — never truncate.
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		return fmt.Errorf("onboarding: open security-events log: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("onboarding: write security event: %w", err)
	}
	return nil
}
