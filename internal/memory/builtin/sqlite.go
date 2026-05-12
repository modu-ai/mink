package builtin

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/modu-ai/mink/internal/memory"
)

// createSchema creates the facts table and FTS5 virtual table.
func (b *BuiltinProvider) createSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS facts (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id  TEXT NOT NULL,
		key         TEXT NOT NULL,
		content     TEXT NOT NULL,
		source      TEXT NOT NULL,
		confidence  REAL NOT NULL DEFAULT 1.0,
		created_at  INTEGER NOT NULL,
		updated_at  INTEGER NOT NULL,
		UNIQUE(session_id, key)
	);

	CREATE VIRTUAL TABLE IF NOT EXISTS facts_fts USING fts5(
		content,
		content=facts,
		content_rowid=id,
		tokenize='porter unicode61'
	);

	CREATE TRIGGER IF NOT EXISTS facts_ai AFTER INSERT ON facts
	BEGIN
		INSERT INTO facts_fts(rowid, content) VALUES (new.id, new.content);
	END;

	CREATE TRIGGER IF NOT EXISTS facts_ad AFTER DELETE ON facts
	BEGIN
		INSERT INTO facts_fts(facts_fts, rowid, content) VALUES ('delete', old.id, old.content);
	END;

	CREATE TRIGGER IF NOT EXISTS facts_au AFTER UPDATE OF content ON facts
	BEGIN
		INSERT INTO facts_fts(facts_fts, rowid, content) VALUES ('delete', old.id, old.content);
		INSERT INTO facts_fts(rowid, content) VALUES (new.id, new.content);
	END;

	CREATE INDEX IF NOT EXISTS idx_facts_session ON facts(session_id);
	CREATE INDEX IF NOT EXISTS idx_facts_updated ON facts(updated_at DESC);
	`

	_, err := b.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("create schema: %w", err)
	}

	// Verify FTS5 is enabled
	var fts5Enabled bool
	err = b.db.QueryRow(`
		SELECT 1 FROM pragma_compile_options
		WHERE compile_options = 'ENABLE_FTS5'
		LIMIT 1
	`).Scan(&fts5Enabled)

	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("check FTS5 support: %w", err)
	}

	if !fts5Enabled {
		b.logger.Warn("FTS5 is not enabled in this SQLite build, full-text search will not work")
	}

	return nil
}

// sanitizeFTSQuery escapes special characters in FTS5 queries.
// FTS5 treats *, ", AND, OR, NOT as operators.
// For now, we use simple AND query between words to enable fuzzy matching.
//
// @MX:NOTE: Empty input returns the literal "" so the FTS5 MATCH clause does
// not error on bind. Non-empty input is passed through unchanged to preserve
// porter stemming. Hardening against malicious operator injection (Risk R1 in
// tasks.md) is deferred — current callers feed only internal recall queries.
// @MX:SPEC: SPEC-GOOSE-MEMORY-001
func sanitizeFTSQuery(query string) string {
	if query == "" {
		return "\"\""
	}

	// Don't wrap in quotes for now - let FTS5 do natural language search
	// This enables porter stemming and fuzzy matching
	// In production, you might want more sophisticated escaping
	return query
}

// insertFact inserts or updates a fact in the database.
func (b *BuiltinProvider) insertFact(sessionID, key, content, source string, createdAt, updatedAt int64) error {
	_, err := b.db.Exec(`
		INSERT INTO facts (session_id, key, content, source, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(session_id, key) DO UPDATE SET
			content = excluded.content,
			updated_at = excluded.updated_at
	`, sessionID, key, content, source, createdAt, updatedAt)

	return err
}

// countFactsBySession returns the number of facts for a given session.
func (b *BuiltinProvider) countFactsBySession(sessionID string) (int, error) {
	var count int
	err := b.db.QueryRow(`
		SELECT COUNT(*) FROM facts WHERE session_id = ?
	`, sessionID).Scan(&count)

	return count, err
}

// deleteOldestFact deletes the oldest fact for a given session (FIFO eviction).
func (b *BuiltinProvider) deleteOldestFact(sessionID string) (int64, error) {
	result, err := b.db.Exec(`
		DELETE FROM facts WHERE id = (
			SELECT id FROM facts WHERE session_id = ? ORDER BY created_at ASC, id ASC LIMIT 1
		)
	`, sessionID)

	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

// searchFacts performs FTS5 full-text search for the given query within a session.
func (b *BuiltinProvider) searchFacts(query, sessionID string, limit int) ([]memory.RecallItem, error) {
	sanitizedQuery := sanitizeFTSQuery(query)

	rows, err := b.db.Query(`
		SELECT f.id, f.content, f.source, f.created_at
		FROM facts f
		JOIN facts_fts fts ON f.id = fts.rowid
		WHERE f.session_id = ? AND facts_fts MATCH ?
		ORDER BY rank
		LIMIT ?
	`, sessionID, sanitizedQuery, limit)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []memory.RecallItem
	for rows.Next() {
		var id int
		var content, source string
		var createdAt int64

		if err := rows.Scan(&id, &content, &source, &createdAt); err != nil {
			return nil, err
		}

		items = append(items, memory.RecallItem{
			Content:   content,
			Source:    source,
			Score:     1.0, // FTS5 rank is not normalized to [0,1], use default score
			SessionID: sessionID,
			Timestamp: time.Unix(createdAt, 0),
		})
	}

	return items, nil
}
