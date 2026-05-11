package journal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"
)

// MemoryRecall retrieves past journal entries relevant to the current moment.
// It implements anniversary-based recall (same month/day in previous years) and
// mood-similarity recall (cosine distance over the Vad triple).
//
// @MX:ANCHOR: [AUTO] Long-term memory recall gateway for journal entries
// @MX:REASON: Called by orchestrator, summary job, and tests — fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-JOURNAL-001 REQ-007, AC-006
type MemoryRecall struct {
	storage *Storage
	cfg     Config
}

// NewMemoryRecall constructs a MemoryRecall backed by the given storage.
func NewMemoryRecall(storage *Storage, cfg Config) *MemoryRecall {
	return &MemoryRecall{storage: storage, cfg: cfg}
}

// maxRecallYears is the oldest past entries FindAnniversaryEvents will consider.
const maxRecallYears = 10

// recallLowValenceThreshold is the default minimum valence for returned entries.
// Entries below this threshold are suppressed unless config.RecallLowValence is set.
// research.md §5.3 — trauma recall protection.
const recallLowValenceThreshold = 0.3

// FindAnniversaryEvents returns past journal entries whose date matches today's
// month and day (±1 day window) and that were created in a previous year.
//
// Entries with valence < recallLowValenceThreshold are filtered by default.
// When config.RecallLowValence is true the filter is skipped.
// Entries older than maxRecallYears are excluded regardless.
//
// @MX:WARN: [AUTO] Trauma recall protection: low-valence filter must not be removed
// @MX:REASON: research.md §5.3 — returning traumatic memories without consent is harmful (R6)
func (r *MemoryRecall) FindAnniversaryEvents(ctx context.Context, userID string, today time.Time) ([]*StoredEntry, error) {
	if userID == "" {
		return nil, ErrInvalidUserID
	}

	// Build the three candidate dates: today-1, today, today+1 (month/day only).
	// We search across all previous years, up to maxRecallYears back.
	cutoffYear := today.Year() - maxRecallYears

	rows, err := r.storage.db.QueryContext(ctx, `
		SELECT id, user_id, date, text, emoji_mood,
		       vad_valence, vad_arousal, vad_dominance,
		       emotion_tags, anniversary, word_count, created_at,
		       allow_lora_training, crisis_flag, attachment_paths
		FROM journal_entries
		WHERE user_id = ?
		  AND CAST(strftime('%Y', created_at) AS INTEGER) < ?
		  AND CAST(strftime('%Y', created_at) AS INTEGER) >= ?
		  AND (
		      (strftime('%m', created_at) = strftime('%m', ?) AND strftime('%d', created_at) = strftime('%d', ?))
		   OR (strftime('%m', created_at) = strftime('%m', ?) AND strftime('%d', created_at) = strftime('%d', ?))
		   OR (strftime('%m', created_at) = strftime('%m', ?) AND strftime('%d', created_at) = strftime('%d', ?))
		  )
		ORDER BY created_at DESC`,
		userID,
		today.Year(),
		cutoffYear,
		// today
		today.UTC().Format(time.RFC3339),
		today.UTC().Format(time.RFC3339),
		// today - 1
		today.AddDate(0, 0, -1).UTC().Format(time.RFC3339),
		today.AddDate(0, 0, -1).UTC().Format(time.RFC3339),
		// today + 1
		today.AddDate(0, 0, 1).UTC().Format(time.RFC3339),
		today.AddDate(0, 0, 1).UTC().Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("anniversary query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	entries, err := scanEntries(rows)
	if err != nil {
		return nil, err
	}

	// Apply trauma recall protection (R6).
	if r.cfg.RecallLowValence {
		return entries, nil
	}
	return filterByValence(entries, recallLowValenceThreshold), nil
}

// filterByValence removes entries with Vad.Valence strictly below threshold.
func filterByValence(entries []*StoredEntry, threshold float64) []*StoredEntry {
	out := entries[:0] // reuse backing array
	for _, e := range entries {
		if e.Vad.Valence >= threshold {
			out = append(out, e)
		}
	}
	return out
}

// FindSimilarMood returns the top-k past entries whose Vad triple is closest
// to targetVad by cosine similarity.
//
// All entries for userID are considered (no year restriction).
// Entries with identical content to targetVad return similarity = 1.0.
func (r *MemoryRecall) FindSimilarMood(ctx context.Context, userID string, targetVad Vad, k int) ([]*StoredEntry, error) {
	if userID == "" {
		return nil, ErrInvalidUserID
	}
	if k <= 0 {
		return nil, nil
	}

	rows, err := r.storage.db.QueryContext(ctx, `
		SELECT id, user_id, date, text, emoji_mood,
		       vad_valence, vad_arousal, vad_dominance,
		       emotion_tags, anniversary, word_count, created_at,
		       allow_lora_training, crisis_flag, attachment_paths
		FROM journal_entries
		WHERE user_id = ?
		ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("similar mood query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	all, err := scanEntries(rows)
	if err != nil {
		return nil, err
	}

	// Sort by descending cosine similarity.
	type scored struct {
		entry *StoredEntry
		sim   float64
	}
	candidates := make([]scored, 0, len(all))
	for _, e := range all {
		sim := vadCosine(targetVad, e.Vad)
		candidates = append(candidates, scored{e, sim})
	}
	// Simple selection sort for top-k (k is typically small, ≤ 10).
	result := make([]*StoredEntry, 0, min(k, len(candidates)))
	for range min(k, len(candidates)) {
		best := -1
		for i, c := range candidates {
			if best == -1 || c.sim > candidates[best].sim {
				best = i
			}
		}
		result = append(result, candidates[best].entry)
		candidates = append(candidates[:best], candidates[best+1:]...)
	}
	return result, nil
}

// vadCosine computes the cosine similarity between two Vad triples.
// Returns 0 when either vector has zero magnitude.
func vadCosine(a, b Vad) float64 {
	dot := a.Valence*b.Valence + a.Arousal*b.Arousal + a.Dominance*b.Dominance
	magA := math.Sqrt(a.Valence*a.Valence + a.Arousal*a.Arousal + a.Dominance*a.Dominance)
	magB := math.Sqrt(b.Valence*b.Valence + b.Arousal*b.Arousal + b.Dominance*b.Dominance)
	if magA == 0 || magB == 0 {
		return 0
	}
	return dot / (magA * magB)
}

// recallListByCreatedRange is a raw query helper that selects entries by the
// created_at timestamp range (RFC3339 string comparison — SQLite uses lexicographic
// ordering on ISO timestamps, which is correct for UTC strings).
func recallListByCreatedRange(ctx context.Context, db *sql.DB, userID string, from, to time.Time) ([]*StoredEntry, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, user_id, date, text, emoji_mood,
		       vad_valence, vad_arousal, vad_dominance,
		       emotion_tags, anniversary, word_count, created_at,
		       allow_lora_training, crisis_flag, attachment_paths
		FROM journal_entries
		WHERE user_id = ?
		  AND created_at >= ?
		  AND created_at <= ?
		ORDER BY created_at DESC`,
		userID,
		from.UTC().Format(time.RFC3339),
		to.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanEntries(rows)
}

// recallEntryFromRow scans one row into a StoredEntry using the inline JSON
// unmarshalling needed for anniversary and tag fields.
func recallEntryFromRow(
	id, userID, dateStr, text string,
	emojiMood sql.NullString,
	valence, arousal, dominance float64,
	tagsJSON string,
	annJSON sql.NullString,
	wordCount int,
	createdAtStr string,
	loraInt, crisisInt int,
	attachJSON string,
) (*StoredEntry, error) {
	e := &StoredEntry{
		ID:                id,
		UserID:            userID,
		Text:              text,
		EmojiMood:         emojiMood.String,
		Vad:               Vad{Valence: valence, Arousal: arousal, Dominance: dominance},
		WordCount:         wordCount,
		AllowLoRATraining: loraInt != 0,
		CrisisFlag:        crisisInt != 0,
	}
	if d, err := time.Parse("2006-01-02", dateStr); err == nil {
		e.Date = d
	}
	if t, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
		e.CreatedAt = t
	}
	if err := json.Unmarshal([]byte(tagsJSON), &e.EmotionTags); err != nil {
		e.EmotionTags = nil
	}
	if annJSON.Valid && annJSON.String != "" {
		var ann Anniversary
		if err := json.Unmarshal([]byte(annJSON.String), &ann); err == nil {
			e.Anniversary = &ann
		}
	}
	if err := json.Unmarshal([]byte(attachJSON), &e.AttachmentPaths); err != nil {
		e.AttachmentPaths = nil
	}
	return e, nil
}

// ErrInvalidQuery is returned when a search or recall query is empty or invalid.
const ErrInvalidQuery = journalError("invalid query: must be non-empty")

// ensure recallListByCreatedRange and recallEntryFromRow are used (compile guard).
var _ = recallListByCreatedRange
var _ = recallEntryFromRow
var _ = errors.New // import guard
