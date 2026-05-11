package journal

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ExportManager provides data-portability and deletion APIs for a single user's journal.
// All operations are strictly user-scoped (WHERE user_id = ? in SQL). AC-010
type ExportManager struct {
	storage *Storage
	auditor *journalAuditWriter
}

// NewExportManager creates an ExportManager using the provided Storage.
func NewExportManager(storage *Storage, auditor *journalAuditWriter) *ExportManager {
	return &ExportManager{storage: storage, auditor: auditor}
}

// exportPayload is the JSON envelope returned by ExportAll.
type exportPayload struct {
	UserIDHash string         `json:"user_id_hash"`
	ExportedAt string         `json:"exported_at"`
	EntryCount int            `json:"entry_count"`
	Entries    []*StoredEntry `json:"entries"`
}

// ExportAll returns all journal entries for userID serialised as JSON.
// Only entries matching userID are included (strict SQL filter). AC-010
//
// @MX:ANCHOR: [AUTO] Data export API; must enforce strict user isolation
// @MX:REASON: Called by export tests, CLI, and privacy compliance flows — fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-JOURNAL-001 REQ-016, AC-010
func (m *ExportManager) ExportAll(ctx context.Context, userID string) ([]byte, error) {
	if userID == "" {
		return nil, ErrInvalidUserID
	}

	entries, err := m.storage.ExportAll(ctx, userID)
	if err != nil {
		m.auditor.emitOperation("export", userID, "err")
		return nil, fmt.Errorf("export: %w", err)
	}

	payload := exportPayload{
		UserIDHash: hashUserID(userID),
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		EntryCount: len(entries),
		Entries:    entries,
	}
	if payload.Entries == nil {
		payload.Entries = []*StoredEntry{}
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal export: %w", err)
	}

	m.auditor.emitOperation("export", userID, "ok")
	return data, nil
}

// DeleteAll permanently removes all journal entries for userID.
// This is a hard delete — no recovery is possible. AC-011
func (m *ExportManager) DeleteAll(ctx context.Context, userID string) error {
	if userID == "" {
		return ErrInvalidUserID
	}

	if err := m.storage.DeleteAll(ctx, userID); err != nil {
		m.auditor.emitOperation("delete_all", userID, "err")
		return fmt.Errorf("delete_all: %w", err)
	}

	m.auditor.emitOperation("delete_all", userID, "ok")
	return nil
}

// DeleteByDateRange permanently removes entries for userID within [from, to] inclusive.
func (m *ExportManager) DeleteByDateRange(ctx context.Context, userID string, from, to time.Time) error {
	if userID == "" {
		return ErrInvalidUserID
	}

	if err := m.storage.DeleteByDateRange(ctx, userID, from, to); err != nil {
		m.auditor.emitOperation("delete_range", userID, "err")
		return fmt.Errorf("delete_range: %w", err)
	}

	m.auditor.emitOperation("delete_range", userID, "ok")
	return nil
}

// OptOut processes the user's opt-out request.
// If deleteData is true, all journal entries are permanently deleted.
// If deleteData is false, the opt-out is recorded but data is preserved.
func (m *ExportManager) OptOut(ctx context.Context, userID string, deleteData bool) error {
	if userID == "" {
		return ErrInvalidUserID
	}

	if deleteData {
		if err := m.storage.DeleteAll(ctx, userID); err != nil {
			m.auditor.emitOperation("opt_out", userID, "err")
			return fmt.Errorf("opt_out delete: %w", err)
		}
	}

	m.auditor.emitOperation("opt_out", userID, "ok")
	return nil
}
