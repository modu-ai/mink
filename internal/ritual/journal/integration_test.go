//go:build integration

package journal

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestEveningHookDispatch_TriggersOrchestratorPrompt verifies the full integration
// path from the HOOK-001 callback through the orchestrator to the writer and storage.
//
// This test uses a real SQLite storage (via newTestStorage) and a synthetic prompt
// function that returns a fixed response without any I/O. It exercises the end-to-end
// flow of OnEveningCheckIn → Prompt → Write → Storage.Insert.
func TestEveningHookDispatch_TriggersOrchestratorPrompt(t *testing.T) {
	// Not t.Parallel() — integration tests may share filesystem resources.

	ctx := context.Background()
	s := newTestStorage(t)

	cfg := Config{
		Enabled:          true,
		RetentionDays:    -1,
		PromptTimeoutMin: 5,
	}

	// A synthetic prompt function simulating a user responding to the evening check-in.
	responseText := "오늘은 친구들과 즐거운 시간을 보냈어요."
	promptFn := func(_ context.Context, _ string) (string, error) {
		return responseText, nil
	}

	callbackCount := 0
	onEntry := func(_ context.Context, _ *StoredEntry) { callbackCount++ }

	w := NewJournalWriter(cfg, s, nil, nil, newJournalAuditWriter(nil), zap.NewNop(), onEntry)
	o := NewJournalOrchestrator(cfg, w, s, zap.NewNop(), newJournalAuditWriter(nil), promptFn)

	// Simulate HOOK-001 dispatch.
	o.OnEveningCheckIn(ctx, "u1")

	// Verify the entry was persisted.
	n, err := s.countEntries(ctx, "u1")
	require.NoError(t, err)
	assert.Equal(t, 1, n, "HOOK-001 dispatch must persist exactly one entry")

	// Verify the INSIGHTS callback was invoked.
	assert.Equal(t, 1, callbackCount, "INSIGHTS callback must be invoked exactly once")

	// Verify idempotency: a second dispatch on the same day must be a no-op.
	o.OnEveningCheckIn(ctx, "u1")

	n2, err := s.countEntries(ctx, "u1")
	require.NoError(t, err)
	assert.Equal(t, 1, n2, "second HOOK-001 dispatch on same day must be skipped (AC-014)")
	assert.Equal(t, 1, callbackCount, "INSIGHTS callback must not be invoked on skip")
}
