package telegram_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/messaging/telegram"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// openTestStore creates a SqliteStore in a temporary directory.
func openTestStore(t *testing.T) telegram.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := telegram.NewSqliteStore(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// TestSqliteStore_RoundTrip_UserMapping verifies that a UserMapping written
// with PutUserMapping can be retrieved with GetUserMapping.
func TestSqliteStore_RoundTrip_UserMapping(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	now := time.Now().Truncate(time.Second)
	m := telegram.UserMapping{
		ChatID:        12345,
		UserProfileID: "tg-12345",
		Allowed:       true,
		FirstSeenAt:   now,
		LastSeenAt:    now,
		AutoAdmitted:  false,
	}

	err := s.PutUserMapping(ctx, m)
	require.NoError(t, err)

	got, found, err := s.GetUserMapping(ctx, 12345)
	require.NoError(t, err)
	require.True(t, found)

	assert.Equal(t, m.ChatID, got.ChatID)
	assert.Equal(t, m.UserProfileID, got.UserProfileID)
	assert.Equal(t, m.Allowed, got.Allowed)
	assert.Equal(t, m.AutoAdmitted, got.AutoAdmitted)
	assert.Equal(t, m.FirstSeenAt.Unix(), got.FirstSeenAt.Unix())
	assert.Equal(t, m.LastSeenAt.Unix(), got.LastSeenAt.Unix())
}

// TestSqliteStore_GetUserMapping_NotFound_ReturnsFalse verifies that querying
// a non-existent chat_id returns (zero, false, nil).
func TestSqliteStore_GetUserMapping_NotFound_ReturnsFalse(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	_, found, err := s.GetUserMapping(ctx, 99999)
	require.NoError(t, err)
	assert.False(t, found)
}

// TestSqliteStore_LastOffset_PersistsAcrossOpen verifies that closing and
// reopening the database preserves the stored offset.
func TestSqliteStore_LastOffset_PersistsAcrossOpen(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "persist.db")

	// Open, write offset, close.
	s1, err := telegram.NewSqliteStore(dbPath)
	require.NoError(t, err)
	require.NoError(t, s1.PutLastOffset(ctx, 42))
	require.NoError(t, s1.Close())

	// Reopen, verify offset survives.
	s2, err := telegram.NewSqliteStore(dbPath)
	require.NoError(t, err)
	defer s2.Close() //nolint:errcheck

	offset, err := s2.GetLastOffset(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(42), offset)
}

// TestSqliteStore_Approve_SetsAllowedTrue verifies that Approve transitions
// a user's Allowed field to true.
func TestSqliteStore_Approve_SetsAllowedTrue(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	now := time.Now().Truncate(time.Second)
	m := telegram.UserMapping{
		ChatID:        777,
		UserProfileID: "tg-777",
		Allowed:       false,
		FirstSeenAt:   now,
		LastSeenAt:    now,
	}
	require.NoError(t, s.PutUserMapping(ctx, m))
	require.NoError(t, s.Approve(ctx, 777))

	got, found, err := s.GetUserMapping(ctx, 777)
	require.NoError(t, err)
	require.True(t, found)
	assert.True(t, got.Allowed)
}

// TestSqliteStore_Revoke_SetsAllowedFalse_PreservesRow verifies that Revoke
// sets Allowed to false but does NOT delete the row (blacklist history is
// preserved per REQ-MTGM-N05 edge E2).
func TestSqliteStore_Revoke_SetsAllowedFalse_PreservesRow(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	now := time.Now().Truncate(time.Second)
	m := telegram.UserMapping{
		ChatID:        888,
		UserProfileID: "tg-888",
		Allowed:       true,
		FirstSeenAt:   now,
		LastSeenAt:    now,
	}
	require.NoError(t, s.PutUserMapping(ctx, m))
	require.NoError(t, s.Revoke(ctx, 888))

	// Row must still exist.
	got, found, err := s.GetUserMapping(ctx, 888)
	require.NoError(t, err)
	require.True(t, found, "row must be preserved after revoke")
	assert.False(t, got.Allowed, "Allowed must be false after revoke")
	assert.Equal(t, "tg-888", got.UserProfileID, "UserProfileID must be preserved")
}

// TestSqliteStore_ListAllowed_ExcludesRevoked verifies that ListAllowed only
// returns mappings where Allowed is true.
func TestSqliteStore_ListAllowed_ExcludesRevoked(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	now := time.Now().Truncate(time.Second)

	allowed := telegram.UserMapping{ChatID: 1, UserProfileID: "tg-1", Allowed: true, FirstSeenAt: now, LastSeenAt: now}
	blocked := telegram.UserMapping{ChatID: 2, UserProfileID: "tg-2", Allowed: false, FirstSeenAt: now, LastSeenAt: now}

	require.NoError(t, s.PutUserMapping(ctx, allowed))
	require.NoError(t, s.PutUserMapping(ctx, blocked))

	list, err := s.ListAllowed(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, int64(1), list[0].ChatID)
}

// TestSqliteStore_PutLastOffset_AtomicUpsert verifies that writing the offset
// multiple times results in the latest value being stored.
func TestSqliteStore_PutLastOffset_AtomicUpsert(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	require.NoError(t, s.PutLastOffset(ctx, 10))
	require.NoError(t, s.PutLastOffset(ctx, 20))
	require.NoError(t, s.PutLastOffset(ctx, 15))

	offset, err := s.GetLastOffset(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(15), offset)
}

// TestSqliteStore_GetLastOffset_DefaultsToZero verifies that an empty store
// returns offset 0.
func TestSqliteStore_GetLastOffset_DefaultsToZero(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	offset, err := s.GetLastOffset(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), offset)
}

// TestSqliteStore_PutUserMapping_UpdatesExisting verifies that calling
// PutUserMapping twice with the same ChatID updates the row rather than
// failing with a constraint error.
func TestSqliteStore_PutUserMapping_UpdatesExisting(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	now := time.Now().Truncate(time.Second)
	m := telegram.UserMapping{ChatID: 500, UserProfileID: "tg-500", Allowed: false, FirstSeenAt: now, LastSeenAt: now}
	require.NoError(t, s.PutUserMapping(ctx, m))

	m.Allowed = true
	m.UserProfileID = "tg-500-updated"
	require.NoError(t, s.PutUserMapping(ctx, m))

	got, found, err := s.GetUserMapping(ctx, 500)
	require.NoError(t, err)
	require.True(t, found)
	assert.True(t, got.Allowed)
	assert.Equal(t, "tg-500-updated", got.UserProfileID)
}
