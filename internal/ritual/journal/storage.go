package journal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite" // SQLite driver (MEMORY-001 reuse)
)

const (
	// sqliteDriver is the driver name registered by modernc.org/sqlite.
	sqliteDriver = "sqlite"

	// schemaSQL defines all DDL statements executed on first open.
	// All statements are idempotent (IF NOT EXISTS).
	schemaSQL = `
PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS journal_entries (
    id                  TEXT PRIMARY KEY,
    user_id             TEXT NOT NULL,
    date                TEXT NOT NULL,
    text                TEXT NOT NULL,
    emoji_mood          TEXT,
    vad_valence         REAL NOT NULL,
    vad_arousal         REAL NOT NULL,
    vad_dominance       REAL NOT NULL,
    emotion_tags        TEXT NOT NULL DEFAULT '[]',
    anniversary         TEXT,
    word_count          INTEGER NOT NULL,
    created_at          TEXT NOT NULL,
    allow_lora_training INTEGER NOT NULL DEFAULT 0,
    crisis_flag         INTEGER NOT NULL DEFAULT 0,
    attachment_paths    TEXT
);

CREATE INDEX IF NOT EXISTS idx_journal_user_date
    ON journal_entries(user_id, date);

CREATE INDEX IF NOT EXISTS idx_journal_user_created
    ON journal_entries(user_id, created_at DESC);

CREATE VIRTUAL TABLE IF NOT EXISTS journal_fts USING fts5(
    text,
    content='journal_entries',
    content_rowid='rowid',
    tokenize='unicode61 remove_diacritics 1'
);

CREATE TRIGGER IF NOT EXISTS journal_fts_ai
AFTER INSERT ON journal_entries BEGIN
    INSERT INTO journal_fts(rowid, text) VALUES (new.rowid, new.text);
END;

CREATE TRIGGER IF NOT EXISTS journal_fts_ad
AFTER DELETE ON journal_entries BEGIN
    DELETE FROM journal_fts WHERE rowid = old.rowid;
END;

CREATE TRIGGER IF NOT EXISTS journal_fts_au
AFTER UPDATE ON journal_entries BEGIN
    DELETE FROM journal_fts WHERE rowid = old.rowid;
    INSERT INTO journal_fts(rowid, text) VALUES (new.rowid, new.text);
END;
`
)

// Storage is the SQLite-backed journal store.
// It enforces file-permission invariants (0600 db file, 0700 directory).
//
// @MX:ANCHOR: [AUTO] Core persistence layer for all journal data
// @MX:REASON: Used by JournalWriter, ExportManager, and Orchestrator — fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-JOURNAL-001 REQ-002, AC-013
type Storage struct {
	db      *sql.DB
	dbPath  string
	dataDir string
}

// NewStorage opens (or creates) the journal SQLite database at dataDir/journal.db.
// File permissions are enforced: directory 0700, database file 0600.
// dataDir is the parent; NewStorage creates a "journal" subdirectory inside it
// when dataDir itself cannot be chmod'd (e.g. it is a system temp directory).
// In tests, pass t.TempDir() and NewStorage will create a secure subdirectory.
func NewStorage(dataDir string) (*Storage, error) {
	// Use a secure subdirectory so we always own the 0700 target.
	journalDir := filepath.Join(dataDir, "journal")
	if err := ensureDir(journalDir); err != nil {
		return nil, err
	}
	dataDir = journalDir

	dbPath := filepath.Join(dataDir, "journal.db")

	db, err := sql.Open(sqliteDriver, dbPath)
	if err != nil {
		return nil, fmt.Errorf("open journal db: %w", err)
	}

	db.SetMaxOpenConns(1) // SQLite WAL supports one writer.

	// Apply schema first — this forces the db file to be created by the driver.
	if _, err := db.Exec(schemaSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}

	// Now enforce 0600 on the db file (and WAL/SHM side files if present).
	for _, suf := range []string{"", "-wal", "-shm"} {
		p := dbPath + suf
		if _, err := os.Stat(p); err == nil {
			if err := os.Chmod(p, 0600); err != nil {
				_ = db.Close()
				return nil, fmt.Errorf("chmod %s: %w", p, err)
			}
		}
	}

	return &Storage{db: db, dbPath: dbPath, dataDir: dataDir}, nil
}

// ensureDir creates dataDir with 0700 permissions.
// If the directory already exists it is chmod'd to 0700 unconditionally.
// The journal subdirectory is always created and owned by this process,
// so forcing 0700 is safe and required by AC-013.
func ensureDir(dataDir string) error {
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return err
	}
	// Explicitly chmod to override umask.
	return os.Chmod(dataDir, 0700)
}

// Insert persists a new StoredEntry. The entry is assigned a new UUID if ID is empty.
func (s *Storage) Insert(ctx context.Context, entry *StoredEntry) error {
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}

	tagsJSON, err := json.Marshal(entry.EmotionTags)
	if err != nil {
		return fmt.Errorf("marshal emotion_tags: %w", err)
	}
	attachJSON, err := json.Marshal(entry.AttachmentPaths)
	if err != nil {
		return fmt.Errorf("marshal attachment_paths: %w", err)
	}

	var annJSON []byte
	if entry.Anniversary != nil {
		annJSON, err = json.Marshal(entry.Anniversary)
		if err != nil {
			return fmt.Errorf("marshal anniversary: %w", err)
		}
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO journal_entries (
			id, user_id, date, text, emoji_mood,
			vad_valence, vad_arousal, vad_dominance,
			emotion_tags, anniversary, word_count, created_at,
			allow_lora_training, crisis_flag, attachment_paths
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.ID,
		entry.UserID,
		entry.Date.Format("2006-01-02"),
		entry.Text,
		entry.EmojiMood,
		entry.Vad.Valence,
		entry.Vad.Arousal,
		entry.Vad.Dominance,
		string(tagsJSON),
		nullableJSON(annJSON),
		entry.WordCount,
		entry.CreatedAt.UTC().Format(time.RFC3339),
		boolToInt(entry.AllowLoRATraining),
		boolToInt(entry.CrisisFlag),
		string(attachJSON),
	)
	return err
}

// GetByID retrieves a single entry by primary key, scoped to userID.
func (s *Storage) GetByID(ctx context.Context, userID, id string) (*StoredEntry, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, date, text, emoji_mood,
		       vad_valence, vad_arousal, vad_dominance,
		       emotion_tags, anniversary, word_count, created_at,
		       allow_lora_training, crisis_flag, attachment_paths
		FROM journal_entries
		WHERE id = ? AND user_id = ?`, id, userID)
	return scanEntry(row)
}

// ListByDateRange returns all entries for userID with date between from and to inclusive.
// Both bounds are in YYYY-MM-DD format.
func (s *Storage) ListByDateRange(ctx context.Context, userID string, from, to time.Time) ([]*StoredEntry, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, date, text, emoji_mood,
		       vad_valence, vad_arousal, vad_dominance,
		       emotion_tags, anniversary, word_count, created_at,
		       allow_lora_training, crisis_flag, attachment_paths
		FROM journal_entries
		WHERE user_id = ?
		  AND date >= ?
		  AND date <= ?
		ORDER BY date ASC`,
		userID,
		from.Format("2006-01-02"),
		to.Format("2006-01-02"),
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanEntries(rows)
}

// DeleteAll permanently removes every entry belonging to userID.
// This is a hard delete (no soft delete). AC-011
func (s *Storage) DeleteAll(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM journal_entries WHERE user_id = ?`, userID)
	return err
}

// DeleteByDateRange permanently removes entries for userID within [from, to].
func (s *Storage) DeleteByDateRange(ctx context.Context, userID string, from, to time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM journal_entries
		WHERE user_id = ?
		  AND date >= ?
		  AND date <= ?`,
		userID,
		from.Format("2006-01-02"),
		to.Format("2006-01-02"),
	)
	return err
}

// ExportAll returns all entries for userID as a JSON-encoded byte slice.
// Only rows matching userID are returned (strict WHERE clause, not post-processing).
// AC-010
func (s *Storage) ExportAll(ctx context.Context, userID string) ([]*StoredEntry, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, date, text, emoji_mood,
		       vad_valence, vad_arousal, vad_dominance,
		       emotion_tags, anniversary, word_count, created_at,
		       allow_lora_training, crisis_flag, attachment_paths
		FROM journal_entries
		WHERE user_id = ?
		ORDER BY created_at ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanEntries(rows)
}

// ApplyRetention deletes entries older than retentionDays for userID.
// A retentionDays value of -1 means no deletion.
func (s *Storage) ApplyRetention(ctx context.Context, userID string, retentionDays int) error {
	if retentionDays < 0 {
		return nil
	}
	cutoff := time.Now().AddDate(0, 0, -retentionDays).Format("2006-01-02")
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM journal_entries
		WHERE user_id = ? AND date < ?`, userID, cutoff)
	return err
}

// Close closes the underlying database connection.
func (s *Storage) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

// DBPath returns the path to the SQLite database file.
func (s *Storage) DBPath() string { return s.dbPath }

// DataDir returns the data directory used for this storage.
func (s *Storage) DataDir() string { return s.dataDir }

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanEntry(row rowScanner) (*StoredEntry, error) {
	var e StoredEntry
	var dateStr, createdAtStr string
	var tagsJSON, attachJSON string
	var annJSON sql.NullString
	var loraInt, crisisInt int

	err := row.Scan(
		&e.ID, &e.UserID, &dateStr, &e.Text, &e.EmojiMood,
		&e.Vad.Valence, &e.Vad.Arousal, &e.Vad.Dominance,
		&tagsJSON, &annJSON, &e.WordCount, &createdAtStr,
		&loraInt, &crisisInt, &attachJSON,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	e.AllowLoRATraining = loraInt != 0
	e.CrisisFlag = crisisInt != 0

	if d, err := time.Parse("2006-01-02", dateStr); err == nil {
		e.Date = d
	}
	if t, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
		e.CreatedAt = t
	}
	if err := json.Unmarshal([]byte(tagsJSON), &e.EmotionTags); err != nil {
		e.EmotionTags = nil
	}
	if err := json.Unmarshal([]byte(attachJSON), &e.AttachmentPaths); err != nil {
		e.AttachmentPaths = nil
	}
	if annJSON.Valid && annJSON.String != "" {
		var ann Anniversary
		if err := json.Unmarshal([]byte(annJSON.String), &ann); err == nil {
			e.Anniversary = &ann
		}
	}
	return &e, nil
}

func scanEntries(rows *sql.Rows) ([]*StoredEntry, error) {
	var result []*StoredEntry
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		if e != nil {
			result = append(result, e)
		}
	}
	return result, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullableJSON(b []byte) any {
	if b == nil {
		return nil
	}
	return string(b)
}

// countEntries is a helper used in tests to verify hard deletes via raw SQL.
func (s *Storage) countEntries(ctx context.Context, userID string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM journal_entries WHERE user_id = ?`, userID).Scan(&n)
	return n, err
}

// recentValences returns the valence values of the most recent n entries for userID,
// ordered newest first. Used by the orchestrator to detect low-mood sequences.
func (s *Storage) recentValences(ctx context.Context, userID string, n int) ([]float64, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT vad_valence
		FROM journal_entries
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT ?`, userID, n)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var vals []float64
	for rows.Next() {
		var v float64
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		vals = append(vals, v)
	}
	return vals, rows.Err()
}

// existsTodayEntry reports whether userID already has an entry for today.
func (s *Storage) existsTodayEntry(ctx context.Context, userID string) (bool, error) {
	today := time.Now().Format("2006-01-02")
	var n int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM journal_entries
		WHERE user_id = ? AND date = ?`, userID, today).Scan(&n)
	return n > 0, err
}

// wordCount returns the number of whitespace-separated tokens in s.
func wordCount(s string) int {
	return len(strings.Fields(s))
}
