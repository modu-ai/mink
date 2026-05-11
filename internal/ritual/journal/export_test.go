package journal

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestExporter creates an ExportManager backed by a fresh test storage.
func newTestExporter(t *testing.T) (*ExportManager, *Storage) {
	t.Helper()
	s := newTestStorage(t)
	return NewExportManager(s, newJournalAuditWriter(nil)), s
}

// insertEntries inserts n entries for userID into storage.
func insertEntries(t *testing.T, s *Storage, userID string, n int) {
	t.Helper()
	ctx := context.Background()
	base := time.Now()
	for i := range n {
		e := sampleEntry(userID, base.AddDate(0, 0, i))
		require.NoError(t, s.Insert(ctx, e))
	}
}

// TestExport_UserFiltered verifies that ExportAll only returns the requesting user's data.
// AC-010
func TestExport_UserFiltered(t *testing.T) {
	t.Parallel()
	mgr, s := newTestExporter(t)

	insertEntries(t, s, "u1", 5)
	insertEntries(t, s, "u2", 3)

	data, err := mgr.ExportAll(context.Background(), "u1")
	require.NoError(t, err)

	var payload struct {
		EntryCount int `json:"entry_count"`
		Entries    []struct {
			UserID string `json:"user_id"`
		} `json:"entries"`
	}
	require.NoError(t, json.Unmarshal(data, &payload))

	assert.Equal(t, 5, payload.EntryCount, "export must return exactly 5 entries for u1")
	for _, entry := range payload.Entries {
		assert.Equal(t, "u1", entry.UserID, "all exported entries must belong to u1")
	}
}

// TestDeleteAll_Immediate verifies that DeleteAll performs a hard delete. AC-011
func TestDeleteAll_Immediate(t *testing.T) {
	t.Parallel()
	mgr, s := newTestExporter(t)

	insertEntries(t, s, "u1", 100)
	insertEntries(t, s, "u2", 5)

	require.NoError(t, mgr.DeleteAll(context.Background(), "u1"))

	n, err := s.countEntries(context.Background(), "u1")
	require.NoError(t, err)
	assert.Equal(t, 0, n, "hard delete must remove all u1 entries")

	// u2 must be unaffected.
	n2, err := s.countEntries(context.Background(), "u2")
	require.NoError(t, err)
	assert.Equal(t, 5, n2, "u2 entries must not be affected by u1 delete")
}

// TestDeleteByDateRange_PartialDelete verifies that only entries in the range are removed.
func TestDeleteByDateRange_PartialDelete(t *testing.T) {
	t.Parallel()
	mgr, s := newTestExporter(t)

	base := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	for i := range 5 {
		e := sampleEntry("u1", base.AddDate(0, 0, i))
		require.NoError(t, s.Insert(context.Background(), e))
	}

	from := base.AddDate(0, 0, 1)
	to := base.AddDate(0, 0, 3)
	require.NoError(t, mgr.DeleteByDateRange(context.Background(), "u1", from, to))

	remaining, err := s.ListByDateRange(context.Background(), "u1",
		base.AddDate(0, 0, -1), base.AddDate(0, 0, 5))
	require.NoError(t, err)
	assert.Len(t, remaining, 2, "days 0 and 4 must remain after deleting days 1-3")
}

// TestOptOut_PreservesData_WhenFlagFalse verifies that OptOut with deleteData=false
// does not remove any entries.
func TestOptOut_PreservesData_WhenFlagFalse(t *testing.T) {
	t.Parallel()
	mgr, s := newTestExporter(t)

	insertEntries(t, s, "u1", 10)
	require.NoError(t, mgr.OptOut(context.Background(), "u1", false))

	n, err := s.countEntries(context.Background(), "u1")
	require.NoError(t, err)
	assert.Equal(t, 10, n, "opt-out without delete must preserve all entries")
}

// TestOptOut_DeletesData_WhenFlagTrue verifies that OptOut with deleteData=true
// performs a hard delete.
func TestOptOut_DeletesData_WhenFlagTrue(t *testing.T) {
	t.Parallel()
	mgr, s := newTestExporter(t)

	insertEntries(t, s, "u1", 10)
	require.NoError(t, mgr.OptOut(context.Background(), "u1", true))

	n, err := s.countEntries(context.Background(), "u1")
	require.NoError(t, err)
	assert.Equal(t, 0, n, "opt-out with delete must remove all entries")
}

// TestExport_EmptyUserID_ErrInvalidUserID verifies that ExportAll rejects empty userID.
func TestExport_EmptyUserID_ErrInvalidUserID(t *testing.T) {
	t.Parallel()
	mgr, _ := newTestExporter(t)

	_, err := mgr.ExportAll(context.Background(), "")
	require.ErrorIs(t, err, ErrInvalidUserID)
}
