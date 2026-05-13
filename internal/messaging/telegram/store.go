package telegram

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "modernc.org/sqlite" // SQLite driver (CGo-free)

	"github.com/modu-ai/mink/internal/userpath"
)

// UserMapping records the association between a Telegram chat_id and a Mink
// user profile, plus the user's current allow/block status.
type UserMapping struct {
	// ChatID is the Telegram chat identifier (unique per user in private chats).
	ChatID int64
	// UserProfileID is the Mink-side identifier for this user.
	UserProfileID string
	// Allowed indicates whether the user may interact with the bot.
	Allowed bool
	// FirstSeenAt is the UTC timestamp of the user's first message.
	FirstSeenAt time.Time
	// LastSeenAt is the UTC timestamp of the user's most recent message.
	LastSeenAt time.Time
	// AutoAdmitted is true when the user was automatically granted access
	// by the auto_admit_first_user config flag.
	AutoAdmitted bool
}

// Store is the persistence interface for telegram channel state.
//
// @MX:ANCHOR: [AUTO] Store is the telegram channel persistence abstraction.
// @MX:REASON: SPEC-GOOSE-MSG-TELEGRAM-001; fan_in via BridgeQueryHandler, bootstrap, CLI commands, and tests (>= 4 callers).
type Store interface {
	// GetUserMapping returns the mapping for chatID and a boolean indicating
	// whether a row was found.
	GetUserMapping(ctx context.Context, chatID int64) (UserMapping, bool, error)
	// PutUserMapping inserts or replaces the mapping for m.ChatID.
	PutUserMapping(ctx context.Context, m UserMapping) error
	// ListAllowed returns all mappings where Allowed is true.
	ListAllowed(ctx context.Context) ([]UserMapping, error)
	// Approve sets Allowed=true for the given chatID.
	Approve(ctx context.Context, chatID int64) error
	// Revoke sets Allowed=false for the given chatID, preserving the row.
	Revoke(ctx context.Context, chatID int64) error
	// GetLastOffset returns the last acknowledged Telegram UpdateID + 1.
	// Returns 0 if no offset has been stored yet.
	GetLastOffset(ctx context.Context) (int64, error)
	// PutLastOffset atomically upserts the offset into the state table.
	PutLastOffset(ctx context.Context, offset int64) error
	// Close releases database resources.
	Close() error
}

// SqliteStore implements Store using a local SQLite database.
//
// @MX:WARN: [AUTO] SqliteStore opens a persistent sqlite file; path must be
// user-writable and is not validated at call site.
// @MX:REASON: SPEC-GOOSE-MSG-TELEGRAM-001 Option B decision; path comes from
// config/DefaultStorePath — caller is responsible for path safety.
type SqliteStore struct {
	db *sql.DB
}

// NewSqliteStore opens (or creates) a SQLite database at path and applies the
// schema. The returned store is ready to use.
func NewSqliteStore(path string) (*SqliteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("telegram store: open %s: %w", path, err)
	}

	// Enforce WAL mode for concurrent read safety.
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("telegram store: set WAL: %w", err)
	}

	s := &SqliteStore{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("telegram store: migrate: %w", err)
	}
	return s, nil
}

// migrate applies the initial schema if it does not already exist.
func (s *SqliteStore) migrate() error {
	const schema = `
CREATE TABLE IF NOT EXISTS users (
	chat_id        INTEGER PRIMARY KEY,
	user_profile_id TEXT NOT NULL,
	allowed        INTEGER NOT NULL,
	first_seen_at  INTEGER NOT NULL,
	last_seen_at   INTEGER NOT NULL,
	auto_admitted  INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS state (
	key   TEXT PRIMARY KEY,
	value INTEGER NOT NULL
);
`
	_, err := s.db.Exec(schema)
	return err
}

// GetUserMapping returns the UserMapping for chatID and a found flag.
func (s *SqliteStore) GetUserMapping(ctx context.Context, chatID int64) (UserMapping, bool, error) {
	const q = `SELECT chat_id, user_profile_id, allowed, first_seen_at, last_seen_at, auto_admitted
	           FROM users WHERE chat_id = ?`
	row := s.db.QueryRowContext(ctx, q, chatID)

	var m UserMapping
	var allowed, autoAdmitted int
	var firstSeen, lastSeen int64

	err := row.Scan(&m.ChatID, &m.UserProfileID, &allowed, &firstSeen, &lastSeen, &autoAdmitted)
	if err == sql.ErrNoRows {
		return UserMapping{}, false, nil
	}
	if err != nil {
		return UserMapping{}, false, fmt.Errorf("telegram store: get user mapping: %w", err)
	}

	m.Allowed = allowed != 0
	m.AutoAdmitted = autoAdmitted != 0
	m.FirstSeenAt = time.Unix(firstSeen, 0).UTC()
	m.LastSeenAt = time.Unix(lastSeen, 0).UTC()
	return m, true, nil
}

// PutUserMapping inserts or replaces the mapping for m.ChatID.
func (s *SqliteStore) PutUserMapping(ctx context.Context, m UserMapping) error {
	const q = `INSERT INTO users (chat_id, user_profile_id, allowed, first_seen_at, last_seen_at, auto_admitted)
	           VALUES (?, ?, ?, ?, ?, ?)
	           ON CONFLICT(chat_id) DO UPDATE SET
	               user_profile_id = excluded.user_profile_id,
	               allowed         = excluded.allowed,
	               last_seen_at    = excluded.last_seen_at,
	               auto_admitted   = excluded.auto_admitted`
	allowed := 0
	if m.Allowed {
		allowed = 1
	}
	autoAdmitted := 0
	if m.AutoAdmitted {
		autoAdmitted = 1
	}
	_, err := s.db.ExecContext(ctx, q,
		m.ChatID,
		m.UserProfileID,
		allowed,
		m.FirstSeenAt.Unix(),
		m.LastSeenAt.Unix(),
		autoAdmitted,
	)
	if err != nil {
		return fmt.Errorf("telegram store: put user mapping: %w", err)
	}
	return nil
}

// ListAllowed returns all mappings where Allowed is true.
func (s *SqliteStore) ListAllowed(ctx context.Context) ([]UserMapping, error) {
	const q = `SELECT chat_id, user_profile_id, allowed, first_seen_at, last_seen_at, auto_admitted
	           FROM users WHERE allowed = 1`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("telegram store: list allowed: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var out []UserMapping
	for rows.Next() {
		var m UserMapping
		var allowed, autoAdmitted int
		var firstSeen, lastSeen int64
		if err := rows.Scan(&m.ChatID, &m.UserProfileID, &allowed, &firstSeen, &lastSeen, &autoAdmitted); err != nil {
			return nil, fmt.Errorf("telegram store: scan user mapping: %w", err)
		}
		m.Allowed = allowed != 0
		m.AutoAdmitted = autoAdmitted != 0
		m.FirstSeenAt = time.Unix(firstSeen, 0).UTC()
		m.LastSeenAt = time.Unix(lastSeen, 0).UTC()
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("telegram store: iterate allowed: %w", err)
	}
	return out, nil
}

// Approve sets Allowed=true for the given chatID.
func (s *SqliteStore) Approve(ctx context.Context, chatID int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET allowed = 1 WHERE chat_id = ?`, chatID)
	if err != nil {
		return fmt.Errorf("telegram store: approve chat_id=%d: %w", chatID, err)
	}
	return nil
}

// Revoke sets Allowed=false for the given chatID, preserving the row for
// blacklist history (REQ-MTGM-N05 edge E2).
func (s *SqliteStore) Revoke(ctx context.Context, chatID int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET allowed = 0 WHERE chat_id = ?`, chatID)
	if err != nil {
		return fmt.Errorf("telegram store: revoke chat_id=%d: %w", chatID, err)
	}
	return nil
}

// GetLastOffset returns the last stored Telegram update offset.
// Returns 0 if no offset has been stored.
func (s *SqliteStore) GetLastOffset(ctx context.Context) (int64, error) {
	const q = `SELECT value FROM state WHERE key = 'last_offset'`
	var offset int64
	err := s.db.QueryRowContext(ctx, q).Scan(&offset)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("telegram store: get last offset: %w", err)
	}
	return offset, nil
}

// PutLastOffset atomically upserts the offset (REQ-MTGM-U05 atomicity).
func (s *SqliteStore) PutLastOffset(ctx context.Context, offset int64) error {
	const q = `INSERT INTO state (key, value) VALUES ('last_offset', ?)
	           ON CONFLICT(key) DO UPDATE SET value = excluded.value`
	if _, err := s.db.ExecContext(ctx, q, offset); err != nil {
		return fmt.Errorf("telegram store: put last offset: %w", err)
	}
	return nil
}

// Close closes the underlying database connection.
func (s *SqliteStore) Close() error {
	return s.db.Close()
}

// DefaultStorePath returns the default path for the telegram SQLite store.
// REQ-MINK-UDM-002: userpath.UserHomeE() 경유 → ~/.mink/messaging/telegram.db.
func DefaultStorePath() (string, error) {
	home, err := userpath.UserHomeE()
	if err != nil {
		// fallback: $HOME/.mink/messaging/telegram.db
		home = os.Getenv("HOME") + "/.mink"
	}
	return home + "/messaging/telegram.db", nil
}
