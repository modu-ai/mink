package journal

import (
	"context"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestStorage creates a Storage in t.TempDir() for isolated testing.
func newTestStorage(t *testing.T) *Storage {
	t.Helper()
	dir := t.TempDir()
	s, err := NewStorage(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// sampleEntry creates a minimal valid StoredEntry for user u1.
func sampleEntry(userID string, date time.Time) *StoredEntry {
	return &StoredEntry{
		UserID:      userID,
		Date:        date,
		Text:        "오늘 하루 좋았어",
		EmotionTags: []string{"happy"},
		Vad:         Vad{Valence: 0.8, Arousal: 0.6, Dominance: 0.7},
		WordCount:   4,
		CreatedAt:   date,
	}
}

// TestStorage_InsertAndReadByID verifies that a written entry can be retrieved by ID.
func TestStorage_InsertAndReadByID(t *testing.T) {
	t.Parallel()
	s := newTestStorage(t)
	ctx := context.Background()

	today := time.Now().Truncate(24 * time.Hour)
	e := sampleEntry("u1", today)

	require.NoError(t, s.Insert(ctx, e))
	require.NotEmpty(t, e.ID, "Insert must assign an ID")

	got, err := s.GetByID(ctx, "u1", e.ID)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, e.ID, got.ID)
	assert.Equal(t, "u1", got.UserID)
	assert.Equal(t, "오늘 하루 좋았어", got.Text)
	assert.InDelta(t, 0.8, got.Vad.Valence, 0.001)
	assert.Equal(t, []string{"happy"}, got.EmotionTags)
}

// TestStorage_FilePermissions_0600_0700 verifies that the storage enforces
// the required file permissions. AC-013
func TestStorage_FilePermissions_0600_0700(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("Windows does not enforce Unix permission bits")
	}

	baseDir := t.TempDir()
	s, err := NewStorage(baseDir)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	// NewStorage creates baseDir/journal/ — verify that subdirectory.
	journalDir := s.DataDir()
	dirInfo, err := os.Stat(journalDir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0700), dirInfo.Mode().Perm(), "journal directory must be 0700")

	dbInfo, err := os.Stat(s.DBPath())
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), dbInfo.Mode().Perm(), "db file must be 0600")
}

// TestStorage_ListByDateRange verifies date-range filtering.
func TestStorage_ListByDateRange(t *testing.T) {
	t.Parallel()
	s := newTestStorage(t)
	ctx := context.Background()

	base := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	for i := range 5 {
		e := sampleEntry("u1", base.AddDate(0, 0, i))
		require.NoError(t, s.Insert(ctx, e))
	}

	from := base.AddDate(0, 0, 1)
	to := base.AddDate(0, 0, 3)
	entries, err := s.ListByDateRange(ctx, "u1", from, to)
	require.NoError(t, err)
	assert.Len(t, entries, 3, "should return exactly 3 entries for days 1-3")
}

// TestStorage_DeleteAll_HardDelete verifies that DeleteAll removes all rows. AC-011
func TestStorage_DeleteAll_HardDelete(t *testing.T) {
	t.Parallel()
	s := newTestStorage(t)
	ctx := context.Background()

	today := time.Now()
	for range 5 {
		require.NoError(t, s.Insert(ctx, sampleEntry("u1", today)))
	}
	// u2 entry must survive.
	require.NoError(t, s.Insert(ctx, sampleEntry("u2", today)))

	require.NoError(t, s.DeleteAll(ctx, "u1"))

	n, err := s.countEntries(ctx, "u1")
	require.NoError(t, err)
	assert.Equal(t, 0, n, "hard delete must remove all u1 entries")

	n2, err := s.countEntries(ctx, "u2")
	require.NoError(t, err)
	assert.Equal(t, 1, n2, "u2 entries must not be affected")
}

// TestStorage_DeleteByDateRange verifies partial deletion within a date range.
func TestStorage_DeleteByDateRange(t *testing.T) {
	t.Parallel()
	s := newTestStorage(t)
	ctx := context.Background()

	base := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	for i := range 5 {
		require.NoError(t, s.Insert(ctx, sampleEntry("u1", base.AddDate(0, 0, i))))
	}

	from := base.AddDate(0, 0, 1)
	to := base.AddDate(0, 0, 3)
	require.NoError(t, s.DeleteByDateRange(ctx, "u1", from, to))

	remaining, err := s.ListByDateRange(ctx, "u1", base, base.AddDate(0, 0, 4))
	require.NoError(t, err)
	assert.Len(t, remaining, 2, "only days 0 and 4 should remain")
}

// TestStorage_RetentionDays_NightlyCleanup verifies that ApplyRetention deletes old entries.
// AC-017
func TestStorage_RetentionDays_NightlyCleanup(t *testing.T) {
	t.Parallel()
	s := newTestStorage(t)
	ctx := context.Background()

	old := time.Now().AddDate(0, 0, -10)
	recent := time.Now().AddDate(0, 0, -1)

	require.NoError(t, s.Insert(ctx, sampleEntry("u1", old)))
	require.NoError(t, s.Insert(ctx, sampleEntry("u1", recent)))

	require.NoError(t, s.ApplyRetention(ctx, "u1", 7))

	n, err := s.countEntries(ctx, "u1")
	require.NoError(t, err)
	assert.Equal(t, 1, n, "only the recent entry should survive retention")
}

// TestStorage_UserScopedQuery verifies that queries never return other users' data.
func TestStorage_UserScopedQuery(t *testing.T) {
	t.Parallel()
	s := newTestStorage(t)
	ctx := context.Background()

	today := time.Now()
	require.NoError(t, s.Insert(ctx, sampleEntry("u1", today)))
	require.NoError(t, s.Insert(ctx, sampleEntry("u2", today)))
	require.NoError(t, s.Insert(ctx, sampleEntry("u2", today)))

	u1Entries, err := s.ListByDateRange(ctx, "u1", today.AddDate(0, 0, -1), today.AddDate(0, 0, 1))
	require.NoError(t, err)
	require.Len(t, u1Entries, 1)
	assert.Equal(t, "u1", u1Entries[0].UserID, "query must be user-scoped")
}
