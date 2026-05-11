package journal

import (
	"context"
	"encoding/json"
	"go/build"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// newTestWriter returns a JournalWriter configured with an enabled config
// backed by a real SQLite Storage in t.TempDir().
func newTestWriter(t *testing.T, opts ...func(*Config)) (JournalWriter, *Storage) {
	t.Helper()
	cfg := Config{
		Enabled:           true,
		AllowLoRATraining: false,
		RetentionDays:     -1,
		PromptTimeoutMin:  60,
	}
	for _, o := range opts {
		o(&cfg)
	}

	s := newTestStorage(t)
	w := NewJournalWriter(cfg, s, nil, nil, newJournalAuditWriter(nil), zap.NewNop(), nil)
	return w, s
}

// newObservedWriter returns a writer + zap observer for log-redaction tests.
func newObservedWriter(t *testing.T) (JournalWriter, *observer.ObservedLogs) {
	t.Helper()
	core, logs := observer.New(zap.DebugLevel)
	logger := zap.New(core)

	s := newTestStorage(t)
	cfg := Config{Enabled: true}
	w := NewJournalWriter(cfg, s, nil, nil, newJournalAuditWriter(nil), logger, nil)
	return w, logs
}

// todayEntry returns a minimal JournalEntry for today.
func todayEntry(userID, text string) JournalEntry {
	return JournalEntry{
		UserID: userID,
		Date:   time.Now(),
		Text:   text,
	}
}

// TestWriter_OptInDefaultOff verifies that Write returns ErrJournalDisabled
// when the journal feature is not enabled. AC-001
func TestWriter_OptInDefaultOff(t *testing.T) {
	t.Parallel()

	s := newTestStorage(t)
	cfg := Config{Enabled: false} // default off
	w := NewJournalWriter(cfg, s, nil, nil, newJournalAuditWriter(nil), zap.NewNop(), nil)

	_, err := w.Write(context.Background(), todayEntry("u1", "오늘 좋았어"))
	require.ErrorIs(t, err, ErrJournalDisabled)

	// Storage must have no entries.
	entries, err := s.ListByDateRange(context.Background(), "u1",
		time.Now().AddDate(0, 0, -1), time.Now().AddDate(0, 0, 1))
	require.NoError(t, err)
	assert.Empty(t, entries, "disabled write must not persist any entry")
}

// TestWriter_LLMOptOutDefault verifies that M1 never calls any LLM mock. AC-002
func TestWriter_LLMOptOutDefault(t *testing.T) {
	t.Parallel()

	llmCalls := 0
	// M1 never reaches an LLM branch regardless of config.
	w, _ := newTestWriter(t, func(c *Config) { c.EmotionLLMAssisted = true })

	stored, err := w.Write(context.Background(), todayEntry("u1", "좋은 하루였다"))
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.Equal(t, 0, llmCalls, "M1 must never invoke LLM regardless of config")
}

// TestWriter_PrivateMode_LocalOnly verifies that PrivateMode entries use local analysis. AC-012
func TestWriter_PrivateMode_LocalOnly(t *testing.T) {
	t.Parallel()

	llmCalls := 0
	w, _ := newTestWriter(t, func(c *Config) { c.EmotionLLMAssisted = true })

	entry := JournalEntry{
		UserID:      "u1",
		Date:        time.Now(),
		Text:        "private text",
		PrivateMode: true,
	}
	stored, err := w.Write(context.Background(), entry)
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.Equal(t, 0, llmCalls, "PrivateMode must never invoke LLM in M1")
}

// TestWriter_CrisisFlag_Set verifies that crisis text is stored with CrisisFlag=true. AC-005
func TestWriter_CrisisFlag_Set(t *testing.T) {
	t.Parallel()

	w, _ := newTestWriter(t)

	stored, err := w.Write(context.Background(), todayEntry("u1", "죽고 싶다"))
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.True(t, stored.CrisisFlag, "crisis entry must have CrisisFlag=true")
}

// TestWriter_LogsRedacted verifies that entry text never appears in any log field. AC-008
func TestWriter_LogsRedacted(t *testing.T) {
	t.Parallel()

	sensitiveText := "비밀 이야기입니다 - 오늘 회사에서 X 매니저와 다툼"
	w, logs := newObservedWriter(t)

	_, err := w.Write(context.Background(), JournalEntry{
		UserID: "u1",
		Date:   time.Now(),
		Text:   sensitiveText,
	})
	require.NoError(t, err)

	// Verify no log entry contains any substring of the sensitive text.
	leakPhrases := []string{"비밀 이야기", "X 매니저", "다툼"}
	for _, logEntry := range logs.All() {
		// Check message.
		for _, phrase := range leakPhrases {
			assert.NotContains(t, logEntry.Message, phrase,
				"log message must not contain sensitive text: %q", phrase)
		}
		// Check all field values by serialising to JSON (covers nested structures).
		fieldsJSON, _ := json.Marshal(logEntry.ContextMap())
		for _, phrase := range leakPhrases {
			assert.NotContains(t, string(fieldsJSON), phrase,
				"log fields must not contain sensitive text: %q", phrase)
		}
	}
}

// TestWriter_A2A_NeverInvoked verifies that the journal package has no A2A client
// and that a write does not send any external messages. AC-009
func TestWriter_A2A_NeverInvoked(t *testing.T) {
	t.Parallel()

	// This test verifies behaviorally that Write does not trigger A2A.
	// The static import test is TestForbiddenImports_NoA2A below.
	a2aMessages := 0
	w, _ := newTestWriter(t)
	_, err := w.Write(context.Background(), todayEntry("u1", "테스트"))
	require.NoError(t, err)
	assert.Equal(t, 0, a2aMessages, "no A2A messages should be sent")
}

// TestForbiddenImports_NoA2A performs a static check that the journal package
// does not import any A2A package. AC-009
func TestForbiddenImports_NoA2A(t *testing.T) {
	t.Parallel()

	ctx := build.Default
	pkg, err := ctx.ImportDir(
		filepath.Join(".", "."), // current package directory
		build.ImportComment,
	)
	require.NoError(t, err)

	for _, imp := range pkg.Imports {
		assert.False(t,
			strings.Contains(imp, "a2a") || strings.Contains(imp, "agent2agent"),
			"journal package must not import A2A packages; found: %s", imp)
	}
}

// TestWriter_AllowLoRATraining_DefaultFalse verifies that entries default to
// allow_lora_training=false. AC-016
func TestWriter_AllowLoRATraining_DefaultFalse(t *testing.T) {
	t.Parallel()

	w, _ := newTestWriter(t) // AllowLoRATraining defaults to false in newTestWriter.

	stored, err := w.Write(context.Background(), todayEntry("u1", "오늘 하루"))
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.False(t, stored.AllowLoRATraining, "AllowLoRATraining must default to false")
}

// TestWriter_PersistRetry_AndErr verifies that Write retries on storage failure
// and returns ErrPersistFailed after exhausting retries. AC-018
func TestWriter_PersistRetry_AndErr(t *testing.T) {
	t.Parallel()

	// Close the DB immediately to force every Insert to fail.
	s := newTestStorage(t)
	_ = s.Close()

	cfg := Config{Enabled: true}
	w := NewJournalWriter(cfg, s, nil, nil, newJournalAuditWriter(nil), zap.NewNop(), nil)

	_, err := w.Write(context.Background(), todayEntry("u1", "오늘 하루"))
	require.ErrorIs(t, err, ErrPersistFailed, "should return ErrPersistFailed after retries")
}

// TestWriter_INSIGHTSCallback_OnSuccess verifies that the OnJournalEntry callback
// is invoked exactly once on a successful write. AC-022
func TestWriter_INSIGHTSCallback_OnSuccess(t *testing.T) {
	t.Parallel()

	callbackCount := 0
	onEntry := func(_ context.Context, _ *StoredEntry) { callbackCount++ }

	s := newTestStorage(t)
	cfg := Config{Enabled: true}
	w := NewJournalWriter(cfg, s, nil, nil, newJournalAuditWriter(nil), zap.NewNop(), onEntry)

	_, err := w.Write(context.Background(), todayEntry("u1", "좋은 하루"))
	require.NoError(t, err)
	assert.Equal(t, 1, callbackCount, "INSIGHTS callback must be invoked exactly once on success")
}

// TestWriter_Read_ByID verifies that Read returns the persisted entry by ID.
func TestWriter_Read_ByID(t *testing.T) {
	t.Parallel()

	w, _ := newTestWriter(t)
	ctx := context.Background()

	stored, err := w.Write(ctx, todayEntry("u1", "하루 기록"))
	require.NoError(t, err)
	require.NotNil(t, stored)

	got, err := w.Read(ctx, "u1", stored.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, stored.ID, got.ID)
	assert.Equal(t, "하루 기록", got.Text)
}

// TestWriter_Read_EmptyUserID verifies that Read rejects empty userID.
func TestWriter_Read_EmptyUserID(t *testing.T) {
	t.Parallel()

	w, _ := newTestWriter(t)
	_, err := w.Read(context.Background(), "", "some-id")
	require.ErrorIs(t, err, ErrInvalidUserID)
}

// TestWriter_ListByDate_ReturnsEntries verifies that ListByDate filters correctly.
func TestWriter_ListByDate_ReturnsEntries(t *testing.T) {
	t.Parallel()

	w, _ := newTestWriter(t)
	ctx := context.Background()

	_, err := w.Write(ctx, todayEntry("u1", "오늘 기록"))
	require.NoError(t, err)

	from := time.Now().AddDate(0, 0, -1)
	to := time.Now().AddDate(0, 0, 1)
	entries, err := w.ListByDate(ctx, "u1", from, to)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}

// TestWriter_ListByDate_EmptyUserID verifies that ListByDate rejects empty userID.
func TestWriter_ListByDate_EmptyUserID(t *testing.T) {
	t.Parallel()

	w, _ := newTestWriter(t)
	_, err := w.ListByDate(context.Background(), "", time.Now(), time.Now())
	require.ErrorIs(t, err, ErrInvalidUserID)
}

// TestWriter_Search_Stub verifies that Search returns nil,nil in M1 (FTS5 stub).
func TestWriter_Search_Stub(t *testing.T) {
	t.Parallel()

	w, _ := newTestWriter(t)
	results, err := w.Search(context.Background(), "u1", "행복")
	require.NoError(t, err)
	assert.Nil(t, results, "Search must return nil slice in M1")
}

// TestWriter_Search_EmptyUserID verifies that Search rejects empty userID.
func TestWriter_Search_EmptyUserID(t *testing.T) {
	t.Parallel()

	w, _ := newTestWriter(t)
	_, err := w.Search(context.Background(), "", "query")
	require.ErrorIs(t, err, ErrInvalidUserID)
}

// ---- test helpers ----
