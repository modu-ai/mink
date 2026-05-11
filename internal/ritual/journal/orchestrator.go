package journal

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// PromptFunc is called by the orchestrator to emit a prompt string to the user.
// The returned string is the user's response (empty string on timeout or skip).
type PromptFunc func(ctx context.Context, prompt string) (response string, err error)

// JournalOrchestrator drives the evening ritual: emits a nightly prompt,
// waits for user input, and delegates to JournalWriter.
//
// @MX:ANCHOR: [AUTO] Evening ritual coordinator; fan_in >= 3
// @MX:REASON: Called by HOOK-001 dispatcher, integration tests, and orchestrator tests — fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-JOURNAL-001 REQ-005, REQ-008, REQ-009, REQ-013
type JournalOrchestrator struct {
	cfg     Config
	writer  JournalWriter
	storage *Storage
	logger  *zap.Logger
	auditor *journalAuditWriter
	// prompt is the user-facing prompt emission function.
	// Tests can substitute this to avoid blocking on real I/O.
	prompt PromptFunc
}

// NewJournalOrchestrator creates an orchestrator for the evening journal ritual.
func NewJournalOrchestrator(
	cfg Config,
	writer JournalWriter,
	storage *Storage,
	logger *zap.Logger,
	auditor *journalAuditWriter,
	prompt PromptFunc,
) *JournalOrchestrator {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &JournalOrchestrator{
		cfg:     cfg,
		writer:  writer,
		storage: storage,
		logger:  logger,
		auditor: auditor,
		prompt:  prompt,
	}
}

// Prompt executes the full evening-prompt flow for userID.
// It is the HOOK-001 callback for the EveningCheckInTime event.
//
// Flow:
//  1. Check config.enabled — no-op if disabled.
//  2. Check if today's entry already exists — skip if so (AC-014).
//  3. Determine low-mood vs neutral prompt based on recent valence history.
//  4. Anniversary check — always returns nil in M1 (M2 activates this branch).
//  5. Emit prompt and wait up to prompt_timeout_min for a response.
//  6. On response, call JournalWriter.Write.
//
// @MX:WARN: [AUTO] Blocking wait on user response; use context with deadline
// @MX:REASON: prompt waits for real user I/O; callers must set context deadline via Config.PromptTimeoutMin
func (o *JournalOrchestrator) Prompt(ctx context.Context, userID string) error {
	// Step 1: Config gate.
	if !o.cfg.Enabled {
		return nil
	}

	// Step 2: Skip if the user already journalled today.
	exists, err := o.storage.existsTodayEntry(ctx, userID)
	if err != nil {
		o.logger.Warn("failed to check today's entry; proceeding with prompt",
			zap.String("user_id_hash", hashUserID(userID)),
			zap.Error(err),
		)
	}
	if exists {
		o.logger.Info("evening journal prompt skipped: entry already exists today",
			zap.String("user_id_hash", hashUserID(userID)),
			zap.String("operation", "evening_prompt_skip"),
		)
		o.auditor.emitOperation("evening_prompt_skip", userID, "ok")
		return nil
	}

	// Step 3: Determine prompt tone from recent valence history.
	var selectedPrompt string
	valences, err := o.storage.recentValences(ctx, userID, 3)
	if err == nil && isLowMood(valences) {
		selectedPrompt = PickLowMood()
	} else {
		selectedPrompt = PickNeutral(dayOfYear())
	}

	// Step 4: Anniversary check — M1 always nil; M2 will call AnniversaryDetector.

	// Step 5: Emit prompt and wait for user response.
	o.auditor.emitOperation("evening_prompt_emit", userID, "ok")

	timeoutCtx, cancel := context.WithTimeout(ctx,
		time.Duration(o.cfg.PromptTimeoutMin)*time.Minute)
	defer cancel()

	response, err := o.prompt(timeoutCtx, selectedPrompt)
	if err != nil || response == "" {
		o.logger.Info("evening journal prompt timed out or no response",
			zap.String("user_id_hash", hashUserID(userID)),
			zap.String("operation", "evening_prompt_timeout"),
		)
		o.auditor.emitOperation("evening_prompt_timeout", userID, "ok")
		return nil
	}

	// Step 6: Write the journal entry.
	_, err = o.writer.Write(ctx, JournalEntry{
		UserID: userID,
		Date:   time.Now(),
		Text:   response,
	})
	return err
}

// OnEveningCheckIn is the HOOK-001 callback signature for EveningCheckInTime.
func (o *JournalOrchestrator) OnEveningCheckIn(ctx context.Context, userID string) {
	if err := o.Prompt(ctx, userID); err != nil {
		o.logger.Error("evening journal prompt failed",
			zap.String("user_id_hash", hashUserID(userID)),
			zap.Error(err),
		)
	}
}

// isLowMood reports whether the user's recent valence history indicates low mood.
// Returns true only if there are at least 3 recent entries, all with valence < 0.3.
// REQ-009, AC-015
func isLowMood(valences []float64) bool {
	if len(valences) < 3 {
		return false
	}
	for _, v := range valences {
		if v >= 0.3 {
			return false
		}
	}
	return true
}

// dayOfYear returns the current day-of-year used to rotate neutral prompts.
func dayOfYear() int {
	return time.Now().YearDay()
}
