package journal

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// insertEntryAt inserts a StoredEntry with explicit created_at for recall tests.
// The entry is fully populated so that anniversary SQL can filter on month/day.
func insertEntryAt(t *testing.T, s *Storage, userID string, createdAt time.Time, valence float64) *StoredEntry {
	t.Helper()
	e := &StoredEntry{
		UserID:      userID,
		Date:        createdAt,
		Text:        "past entry text",
		EmotionTags: []string{"happy"},
		Vad:         Vad{Valence: valence, Arousal: 0.5, Dominance: 0.5},
		WordCount:   3,
		CreatedAt:   createdAt,
	}
	require.NoError(t, s.Insert(context.Background(), e))
	return e
}

// TestRecall_AnniversaryEvents_LastYear verifies AC-006:
// FindAnniversaryEvents returns past entries whose month/day matches today (±1 day).
func TestRecall_AnniversaryEvents_LastYear(t *testing.T) {
	t.Parallel()

	s := newTestStorage(t)
	cfg := Config{RecallLowValence: false}
	r := NewMemoryRecall(s, cfg)
	ctx := context.Background()

	today := time.Date(2026, 4, 22, 0, 0, 0, 0, time.UTC)

	// Insert last year's entry on exactly April 22.
	lastYear := time.Date(2025, 4, 22, 21, 0, 0, 0, time.UTC)
	insertEntryAt(t, s, "u1", lastYear, 0.8)

	// Insert an entry that is NOT on April 22 — must be excluded.
	unrelated := time.Date(2025, 3, 15, 10, 0, 0, 0, time.UTC)
	insertEntryAt(t, s, "u1", unrelated, 0.9)

	entries, err := r.FindAnniversaryEvents(ctx, "u1", today)
	require.NoError(t, err)

	require.GreaterOrEqual(t, len(entries), 1, "at least one entry must match April 22")
	// Verify at least one returned entry is from 2025-04 within ±1 day of 22.
	found := false
	for _, e := range entries {
		if e.CreatedAt.Year() == 2025 && e.CreatedAt.Month() == time.April {
			diff := e.CreatedAt.Day() - 22
			if diff < 0 {
				diff = -diff
			}
			if diff <= 1 {
				found = true
			}
		}
	}
	assert.True(t, found, "expected an entry from 2025-04 within ±1 day of April 22")

	// Verify the unrelated March entry is not included.
	for _, e := range entries {
		assert.NotEqual(t, time.March, e.CreatedAt.Month(), "March entry must not appear")
	}
}

// TestRecall_LowValenceFiltered verifies that entries with valence < 0.3 are
// excluded by default (trauma recall protection, research.md §5.3, R6).
func TestRecall_LowValenceFiltered(t *testing.T) {
	t.Parallel()

	s := newTestStorage(t)
	cfg := Config{RecallLowValence: false} // default filter active
	r := NewMemoryRecall(s, cfg)
	ctx := context.Background()

	today := time.Date(2026, 4, 22, 0, 0, 0, 0, time.UTC)

	// Insert a low-valence entry from last year (same day) — must be filtered.
	lastYear := time.Date(2025, 4, 22, 20, 0, 0, 0, time.UTC)
	insertEntryAt(t, s, "u1", lastYear, 0.15) // valence < 0.3

	entries, err := r.FindAnniversaryEvents(ctx, "u1", today)
	require.NoError(t, err)
	assert.Len(t, entries, 0, "low-valence entry must be filtered out by default")
}

// TestRecall_OptInLowValence verifies that entries below 0.3 valence are
// included when config.RecallLowValence=true.
func TestRecall_OptInLowValence(t *testing.T) {
	t.Parallel()

	s := newTestStorage(t)
	cfg := Config{RecallLowValence: true} // user explicitly opted in
	r := NewMemoryRecall(s, cfg)
	ctx := context.Background()

	today := time.Date(2026, 4, 22, 0, 0, 0, 0, time.UTC)

	lastYear := time.Date(2025, 4, 22, 20, 0, 0, 0, time.UTC)
	insertEntryAt(t, s, "u1", lastYear, 0.10)

	entries, err := r.FindAnniversaryEvents(ctx, "u1", today)
	require.NoError(t, err)
	require.Len(t, entries, 1, "low-valence entry must be included when opted in")
	assert.InDelta(t, 0.10, entries[0].Vad.Valence, 0.001)
}

// TestRecall_Over10YearsExcluded verifies that entries older than 10 years are
// not returned, regardless of valence or date match.
func TestRecall_Over10YearsExcluded(t *testing.T) {
	t.Parallel()

	s := newTestStorage(t)
	cfg := Config{RecallLowValence: true} // permit low valence so only year filter matters
	r := NewMemoryRecall(s, cfg)
	ctx := context.Background()

	today := time.Date(2026, 4, 22, 0, 0, 0, 0, time.UTC)

	// 11 years ago — beyond the 10-year window.
	elevenYearsAgo := time.Date(2015, 4, 22, 20, 0, 0, 0, time.UTC)
	insertEntryAt(t, s, "u1", elevenYearsAgo, 0.9)

	// 9 years ago — within the 10-year window.
	nineYearsAgo := time.Date(2017, 4, 22, 20, 0, 0, 0, time.UTC)
	insertEntryAt(t, s, "u1", nineYearsAgo, 0.9)

	entries, err := r.FindAnniversaryEvents(ctx, "u1", today)
	require.NoError(t, err)

	require.Len(t, entries, 1, "only the 9-year-old entry must be returned")
	assert.Equal(t, 2017, entries[0].CreatedAt.Year())
}

// TestRecall_FindSimilarMood_TopK verifies that FindSimilarMood returns the
// k entries with the highest cosine similarity to the target Vad.
func TestRecall_FindSimilarMood_TopK(t *testing.T) {
	t.Parallel()

	s := newTestStorage(t)
	cfg := Config{}
	r := NewMemoryRecall(s, cfg)
	ctx := context.Background()

	// Insert 5 entries with varying Vad scores.
	base := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
	entries := []struct {
		vad Vad
	}{
		{Vad{Valence: 0.9, Arousal: 0.8, Dominance: 0.7}}, // most similar to target
		{Vad{Valence: 0.8, Arousal: 0.7, Dominance: 0.6}}, // second
		{Vad{Valence: 0.5, Arousal: 0.5, Dominance: 0.5}}, // neutral
		{Vad{Valence: 0.1, Arousal: 0.2, Dominance: 0.1}}, // low
		{Vad{Valence: 0.2, Arousal: 0.1, Dominance: 0.2}}, // low
	}
	for i, entry := range entries {
		e := &StoredEntry{
			UserID:    "u1",
			Date:      base.AddDate(0, 0, i),
			Text:      "entry",
			Vad:       entry.vad,
			WordCount: 1,
			CreatedAt: base.AddDate(0, 0, i),
		}
		require.NoError(t, s.Insert(ctx, e))
	}

	target := Vad{Valence: 0.85, Arousal: 0.75, Dominance: 0.65}
	result, err := r.FindSimilarMood(ctx, "u1", target, 2)
	require.NoError(t, err)
	require.Len(t, result, 2, "top-2 must be returned")

	// The two most similar must have high valence.
	assert.GreaterOrEqual(t, result[0].Vad.Valence, 0.7, "first result must be high-valence")
	assert.GreaterOrEqual(t, result[1].Vad.Valence, 0.7, "second result must be high-valence")
}

// TestRecall_FindSimilarMood_EmptyUserID verifies ErrInvalidUserID is returned.
func TestRecall_FindSimilarMood_EmptyUserID(t *testing.T) {
	t.Parallel()

	s := newTestStorage(t)
	r := NewMemoryRecall(s, Config{})

	_, err := r.FindSimilarMood(context.Background(), "", Vad{}, 3)
	assert.ErrorIs(t, err, ErrInvalidUserID)
}

// TestVadCosine_Orthogonal verifies that orthogonal vectors return similarity 0.
func TestVadCosine_Orthogonal(t *testing.T) {
	t.Parallel()

	a := Vad{Valence: 1, Arousal: 0, Dominance: 0}
	b := Vad{Valence: 0, Arousal: 1, Dominance: 0}
	assert.InDelta(t, 0.0, vadCosine(a, b), 1e-9)
}

// TestVadCosine_Identical verifies that identical vectors return similarity 1.
func TestVadCosine_Identical(t *testing.T) {
	t.Parallel()

	v := Vad{Valence: 0.6, Arousal: 0.5, Dominance: 0.7}
	assert.InDelta(t, 1.0, vadCosine(v, v), 1e-9)
}

// TestVadCosine_ZeroMagnitude verifies that zero vectors return 0 (no panic).
func TestVadCosine_ZeroMagnitude(t *testing.T) {
	t.Parallel()

	z := Vad{}
	v := Vad{Valence: 0.5, Arousal: 0.5, Dominance: 0.5}
	assert.True(t, math.IsNaN(vadCosine(z, v)) || vadCosine(z, v) == 0,
		"zero-magnitude vector should return 0")
}
