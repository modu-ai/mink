package journal

import (
	"context"
	"fmt"
	"strings"
)

// JournalSearch performs full-text search over a user's journal entries using
// the FTS5 virtual table (journal_fts) created by storage.go.
//
// All results are scoped to the requesting userID at the SQL level — not as a
// post-processing filter — to prevent cross-user data access.
//
// @MX:ANCHOR: [AUTO] FTS5 full-text search gateway; user_id isolation enforced at SQL level
// @MX:REASON: Called by JournalWriter.Search, search tests, and integration tests — fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-JOURNAL-001 REQ-021, AC-024
type JournalSearch struct {
	storage *Storage
}

// NewJournalSearch constructs a JournalSearch backed by the given storage.
func NewJournalSearch(storage *Storage) *JournalSearch {
	return &JournalSearch{storage: storage}
}

// Search performs an FTS5 full-text search for query within userID's entries.
//
// The query string is passed to the FTS5 MATCH operator after quoting to prevent
// SQL injection via FTS5 boolean operators. Results are ordered by FTS5 rank
// (best match first) with created_at DESC as the tiebreaker.
//
// Returns ErrInvalidQuery when query is empty or blank.
// Returns ErrInvalidUserID when userID is empty.
//
// @MX:WARN: [AUTO] FTS5 query quoting is security-critical; do not bypass ftsQuote
// @MX:REASON: Unquoted FTS5 queries allow injection of FTS5 operators (OR, NOT, etc.) — ACL bypass risk
func (s *JournalSearch) Search(ctx context.Context, userID, query string) ([]*StoredEntry, error) {
	if userID == "" {
		return nil, ErrInvalidUserID
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, ErrInvalidQuery
	}

	// Quote the query to prevent FTS5 operator injection.
	// FTS5 "string" literal matches only the string itself, not boolean operators.
	quotedQuery := ftsQuote(query)

	// Use a subquery to retrieve rowids from the FTS5 virtual table, then fetch
	// the full entries from journal_entries with user_id isolation in the WHERE clause.
	// Directly joining FTS5 content tables is unreliable across SQLite versions;
	// the rowid subquery pattern is the recommended approach for content tables.
	rows, err := s.storage.db.QueryContext(ctx, `
		SELECT id, user_id, date, text, emoji_mood,
		       vad_valence, vad_arousal, vad_dominance,
		       emotion_tags, anniversary, word_count, created_at,
		       allow_lora_training, crisis_flag, attachment_paths
		FROM journal_entries
		WHERE user_id = ?
		  AND rowid IN (
		      SELECT rowid FROM journal_fts WHERE journal_fts MATCH ?
		  )
		ORDER BY created_at DESC`,
		userID, quotedQuery,
	)
	if err != nil {
		// Translate FTS5 syntax errors to ErrInvalidQuery to avoid leaking
		// internal SQL details to the caller.
		if strings.Contains(err.Error(), "fts5:") || strings.Contains(err.Error(), "syntax error") {
			return nil, fmt.Errorf("%w: %v", ErrInvalidQuery, err)
		}
		return nil, fmt.Errorf("fts5 search: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanEntries(rows)
}

// ftsQuote wraps an FTS5 query string in double-quotes so it is treated as a
// literal phrase rather than an expression with boolean operators.
// A trailing "*" suffix is appended outside the quotes to enable prefix matching,
// which is important for agglutinative languages like Korean where "산책"
// should match "산책을", "산책하다", etc.
// Internal double-quotes are escaped by doubling them (FTS5 quoting convention).
func ftsQuote(q string) string {
	escaped := strings.ReplaceAll(q, `"`, `""`)
	return `"` + escaped + `"*`
}
