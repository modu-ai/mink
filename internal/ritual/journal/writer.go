package journal

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// OnJournalEntryFunc is the callback signature for INSIGHTS-001 consumers.
// The callback is invoked synchronously after a successful Write.
type OnJournalEntryFunc func(ctx context.Context, entry *StoredEntry)

// JournalWriter is the primary entry point for persisting diary entries.
//
// @MX:ANCHOR: [AUTO] Primary journal persistence interface
// @MX:REASON: Implemented by sqliteJournalWriter; consumed by orchestrator, export tests, writer tests — fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-JOURNAL-001 REQ-001, REQ-002, REQ-005, REQ-010, REQ-012, REQ-015
type JournalWriter interface {
	// Write persists a journal entry and returns the stored record.
	Write(ctx context.Context, entry JournalEntry) (*StoredEntry, error)
	// Read retrieves a stored entry by ID, scoped to userID.
	Read(ctx context.Context, userID, entryID string) (*StoredEntry, error)
	// ListByDate returns entries for userID between from and to inclusive.
	ListByDate(ctx context.Context, userID string, from, to time.Time) ([]*StoredEntry, error)
	// Search is a stub in M1; it returns an empty slice. Full FTS5 search is implemented in M2.
	Search(ctx context.Context, userID, query string) ([]*StoredEntry, error)
}

// sqliteJournalWriter implements JournalWriter backed by a SQLite Storage.
// It enforces all privacy invariants from SPEC-GOOSE-JOURNAL-001.
type sqliteJournalWriter struct {
	cfg      Config
	storage  *Storage
	analyzer EmotionAnalyzer
	// llmAnalyzer is the optional LLM-assisted emotion analyzer (M3).
	// When non-nil and config.EmotionLLMAssisted is true, it overrides the local
	// analyzer result unless the entry is PrivateMode or a crisis entry.
	llmAnalyzer *LLMEmotionAnalyzer
	crisis      *CrisisDetector
	auditor     *journalAuditWriter
	logger      *zap.Logger
	// onEntry is called after a successful write (INSIGHTS-001 consumer). May be nil.
	onEntry OnJournalEntryFunc
	// searcher implements FTS5 search (M2). Initialised lazily on first Search call.
	searcher *JournalSearch
}

// maxRetries is the number of storage insert attempts before giving up.
const maxRetries = 3

// NewJournalWriter constructs a sqliteJournalWriter with all required dependencies.
//
// If analyzer is a *LLMEmotionAnalyzer, it is stored in the llmAnalyzer field and the
// step-4 local analyzer is set to the LLMEmotionAnalyzer's embedded localFallback.
// This ensures step 4 always runs LocalDictAnalyzer, and step 5 conditionally runs LLM.
func NewJournalWriter(
	cfg Config,
	storage *Storage,
	analyzer EmotionAnalyzer,
	crisis *CrisisDetector,
	auditor *journalAuditWriter,
	logger *zap.Logger,
	onEntry OnJournalEntryFunc,
) JournalWriter {
	var llmAnalyzer *LLMEmotionAnalyzer
	localAnalyzer := EmotionAnalyzer(NewLocalDictAnalyzer())

	switch a := analyzer.(type) {
	case *LLMEmotionAnalyzer:
		// Separate the LLM analyzer from the local fallback.
		// Step 4 uses local; step 5 uses LLM (guarded by privacy invariants).
		llmAnalyzer = a
		localAnalyzer = a.localFallback
	case nil:
		// Use defaults already set above.
	default:
		localAnalyzer = a
	}

	if crisis == nil {
		crisis = NewCrisisDetector()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &sqliteJournalWriter{
		cfg:         cfg,
		storage:     storage,
		analyzer:    localAnalyzer,
		llmAnalyzer: llmAnalyzer,
		crisis:      crisis,
		auditor:     auditor,
		logger:      logger,
		onEntry:     onEntry,
		searcher:    NewJournalSearch(storage),
	}
}

// Write persists a journal entry following the 11-step sequence in tasks.md T-013.
//
// @MX:WARN: [AUTO] Write encapsulates 11 ordered privacy-critical steps; do not reorder
// @MX:REASON: Each step enforces a distinct invariant (opt-in, user scope, crisis, LLM gate, retries, audit)
// @MX:SPEC: SPEC-GOOSE-JOURNAL-001 REQ-001 through REQ-019
func (w *sqliteJournalWriter) Write(ctx context.Context, entry JournalEntry) (*StoredEntry, error) {
	// Step 1: Config gate — feature must be explicitly enabled.
	if !w.cfg.Enabled {
		return nil, ErrJournalDisabled
	}

	// Step 2: User isolation validation.
	if entry.UserID == "" {
		return nil, ErrInvalidUserID
	}

	// Step 3: Crisis detection — set flag and prepend canned response.
	isCrisis := w.crisis.Check(entry.Text)

	// Step 4: Local emotion analysis (no LLM in M1).
	vad, tags, err := w.analyzer.Analyze(ctx, entry.Text, entry.EmojiMood)
	if err != nil {
		// Fallback to neutral; do not fail the write.
		v := neutralVad
		vad = &v
		tags = nil
		w.logger.Warn("emotion analysis failed; using neutral fallback",
			// NOTE: no entry text logged here (AC-008)
			zap.String("user_id_hash", hashUserID(entry.UserID)),
			zap.Error(err),
		)
	}

	// Step 5: LLM branch — active in M3 when EmotionLLMAssisted=true.
	// Privacy invariants (AC-020, AC-023, REQ-003):
	//   - PrivateMode=true → LLM access permanently forbidden.
	//   - CrisisFlag=true → LLM must not be called (crisis entries go local-only).
	// When all guards pass, LLMEmotionAnalyzer overrides the local step-4 result.
	// LLMEmotionAnalyzer handles parse-fail and clinical-reject silently.
	if w.cfg.EmotionLLMAssisted && !entry.PrivateMode && !isCrisis && w.llmAnalyzer != nil {
		if llmVad, llmTags, llmErr := w.llmAnalyzer.Analyze(ctx, entry.Text, entry.EmojiMood); llmErr == nil {
			vad = llmVad
			tags = llmTags
		}
		// On LLM error, keep the local analysis result from step 4 (already set).
	}

	// Step 6: Anniversary detection — always nil in M1.
	var anniversary *Anniversary

	// Step 7: Word count.
	wc := wordCount(entry.Text)

	// Step 8: LoRA training flag from config.
	allowLoRA := w.cfg.AllowLoRATraining

	// Build the stored entry.
	if entry.Date.IsZero() {
		entry.Date = time.Now()
	}

	stored := &StoredEntry{
		UserID:            entry.UserID,
		Date:              entry.Date,
		Text:              entry.Text,
		EmojiMood:         entry.EmojiMood,
		Vad:               *vad,
		EmotionTags:       tags,
		Anniversary:       anniversary,
		WordCount:         wc,
		CreatedAt:         time.Now(),
		AllowLoRATraining: allowLoRA,
		CrisisFlag:        isCrisis,
		AttachmentPaths:   entry.AttachmentPaths,
	}

	// Step 9: Storage insert with retry (max 3 attempts).
	var insertErr error
	for attempt := range maxRetries {
		insertErr = w.storage.Insert(ctx, stored)
		if insertErr == nil {
			break
		}
		w.logger.Warn("journal insert retry",
			zap.Int("attempt", attempt+1),
			zap.String("user_id_hash", hashUserID(entry.UserID)),
			zap.Error(insertErr),
		)
	}

	// Step 10: Persist failure handling.
	if insertErr != nil {
		w.logger.Error("journal persist failed after retries",
			zap.String("user_id_hash", hashUserID(entry.UserID)),
			zap.Error(insertErr),
		)
		w.auditor.emitWrite(entry.UserID, stored, "err")
		return nil, fmt.Errorf("%w: %v", ErrPersistFailed, insertErr)
	}

	// Step 11: Success — emit audit event and INSIGHTS callback.
	w.auditor.emitWrite(entry.UserID, stored, "ok")
	w.logger.Info("journal entry written",
		// NOTE: entry text, raw user_id, and attachments are never logged (AC-008)
		zap.String("user_id_hash", hashUserID(entry.UserID)),
		zap.String("entry_id", stored.ID),
		zap.String("entry_length_bucket", lengthBucket(len(entry.Text))),
		zap.Bool("crisis_flag", stored.CrisisFlag),
	)

	if w.onEntry != nil {
		w.onEntry(ctx, stored)
	}

	// Prepend crisis response to the returned user-facing message if detected.
	// The StoredEntry is not modified — CrisisResponse is surfaced by the orchestrator.
	if isCrisis {
		// Return a sentinel entry so callers can detect crisis and emit the canned response.
		stored.CrisisFlag = true
	}

	return stored, nil
}

// Read retrieves a stored entry by ID, scoped to the given userID.
func (w *sqliteJournalWriter) Read(ctx context.Context, userID, entryID string) (*StoredEntry, error) {
	if userID == "" {
		return nil, ErrInvalidUserID
	}
	entry, err := w.storage.GetByID(ctx, userID, entryID)
	if err != nil {
		return nil, err
	}
	return entry, nil
}

// ListByDate returns all entries for userID with date between from and to.
func (w *sqliteJournalWriter) ListByDate(ctx context.Context, userID string, from, to time.Time) ([]*StoredEntry, error) {
	if userID == "" {
		return nil, ErrInvalidUserID
	}
	return w.storage.ListByDateRange(ctx, userID, from, to)
}

// Search performs an FTS5 full-text search for query within userID's journal entries.
// Delegates to JournalSearch which enforces user-scoped isolation at the SQL level.
// Returns ErrInvalidQuery when query is empty, ErrInvalidUserID when userID is empty.
func (w *sqliteJournalWriter) Search(ctx context.Context, userID, query string) ([]*StoredEntry, error) {
	return w.searcher.Search(ctx, userID, query)
}
