-- MINK QMD Memory Index Schema
-- SPEC: SPEC-MINK-MEMORY-QMD-001 §7.3
-- License: AGPL-3.0-only

-- Regular tables
CREATE TABLE IF NOT EXISTS files (
    file_id        INTEGER PRIMARY KEY AUTOINCREMENT,
    collection     TEXT NOT NULL,
    source_path    TEXT NOT NULL UNIQUE,
    content_hash   TEXT NOT NULL,
    created_at     INTEGER NOT NULL,
    updated_at     INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS chunks (
    chunk_id       TEXT PRIMARY KEY,
    file_id        INTEGER NOT NULL REFERENCES files(file_id) ON DELETE CASCADE,
    start_line     INTEGER NOT NULL,
    end_line       INTEGER NOT NULL,
    content        TEXT NOT NULL,
    prev_chunk_id  TEXT,
    next_chunk_id  TEXT,
    embedding_pending INTEGER NOT NULL DEFAULT 1,
    model_version  TEXT NOT NULL,
    created_at     INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS metadata (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- Virtual tables (sqlite-vec + FTS5)
-- Note: The vec0 virtual table requires the sqlite-vec extension to be loaded.
-- If the extension is unavailable, this statement will be skipped gracefully
-- (M1 does not query vec0; vector search is wired in M3).
CREATE VIRTUAL TABLE IF NOT EXISTS embeddings USING vec0(
    chunk_id TEXT PRIMARY KEY,
    embedding FLOAT[768]
);

CREATE VIRTUAL TABLE IF NOT EXISTS chunks_fts USING fts5(
    chunk_id UNINDEXED,
    content,
    tokenize = 'unicode61'
);

-- Indexes on regular tables
CREATE INDEX IF NOT EXISTS idx_chunks_file_id   ON chunks(file_id);
CREATE INDEX IF NOT EXISTS idx_chunks_model_ver ON chunks(model_version);
CREATE INDEX IF NOT EXISTS idx_files_collection ON files(collection);
