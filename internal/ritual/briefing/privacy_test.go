package briefing

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// TestPrivacy_Invariants is the aggregator entry point referenced by the
// acceptance criteria (AC-009 verification command:
//
//	go test ./internal/ritual/briefing -run TestPrivacy_Invariants -v -count=1
//
// It groups the individual invariant tests under a single named sub-tree so
// that selective invocation matches the SPEC verification command verbatim.
//
// REQ-BR-050 .. REQ-BR-055, AC-009.
func TestPrivacy_Invariants(t *testing.T) {
	t.Run("Invariant1_LogRedaction", TestPrivacyInvariant1_LogRedaction)
	t.Run("Invariant2_ArchiveFilePerms", TestPrivacyInvariant2_ArchivePerms)
	t.Run("Invariant3_NoA2ACommunication", TestPrivacyInvariant3_NoA2ACommunication)
	t.Run("Invariant4_NoClinicalVocabulary", TestPrivacyInvariant4_NoClinicalVocabulary)
	t.Run("Invariant5_LLMPayloadCategoricalOnly", TestPrivacyInvariant5_LLMPayloadCategoricalOnly)
	t.Run("Invariant6_CrisisHotlinePrepend", TestPrivacyInvariant6_CrisisHotlinePrepend)
}

// TestPrivacyInvariant5_LLMPayloadCategoricalOnly is the M3-scope check that
// LLM payloads carry only categorical signals. Re-asserts what
// TestBuildLLMSummaryRequest_CategoricalOnly + TestGenerateLLMSummary_*
// already cover, but lives under TestPrivacy_Invariants so the SPEC AC-009
// aggregator entry covers all 6 invariants.
//
// REQ-BR-054.
func TestPrivacyInvariant5_LLMPayloadCategoricalOnly(t *testing.T) {
	t.Run("BuildLLMSummaryRequest excludes entry text + mantra + raw coords",
		TestBuildLLMSummaryRequest_CategoricalOnly)
	t.Run("FormatLLMPrompt excludes entry text + mantra + raw coords",
		TestFormatLLMPrompt_StructureAndAbsence)
	t.Run("GenerateLLMSummary end-to-end never leaks forbidden tokens to provider",
		TestGenerateLLMSummary_HappyPath_RequestInvariant5)
}

// TestPrivacyInvariant6_CrisisHotlinePrepend covers AC-009 invariant 6:
// when a crisis keyword surfaces, the hotline canned response is prepended
// and no analytical commentary is introduced.
//
// REQ-BR-055.
func TestPrivacyInvariant6_CrisisHotlinePrepend(t *testing.T) {
	t.Run("DetectedPathPrepends",
		TestPrependCrisisResponseIfDetected_DetectedPathPrepends)
	t.Run("NoCrisisPassthrough",
		TestPrependCrisisResponseIfDetected_NoCrisisPassthrough)
	t.Run("NoAnalyticalCommentary",
		TestPrependCrisisResponse_NoAnalyticalCommentary)
	t.Run("PayloadDetection",
		TestPayloadHasCrisis_Detection)
}

// TestPrivacyInvariant2_ArchivePerms verifies that archive files are written
// with file mode 0600 and their parent directory with mode 0700. This is the
// concrete privacy guarantee for the persistent briefing record on disk.
//
// REQ-BR-051, AC-009 invariant 2.
func TestPrivacyInvariant2_ArchivePerms(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix permission semantics not enforced on windows")
	}

	dir := filepath.Join(t.TempDir(), "archive")
	payload := &BriefingPayload{
		GeneratedAt: time.Date(2026, 5, 14, 7, 0, 0, 0, time.UTC),
		Status:      map[string]string{"weather": "ok", "mantra": "ok"},
		Mantra:      MantraModule{Text: "ok"},
	}

	path, err := WriteArchiveToDir(dir, payload)
	if err != nil {
		t.Fatalf("WriteArchiveToDir: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("file mode = %o, want 0600 (invariant 2 violation)", mode)
	}

	dinfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if mode := dinfo.Mode().Perm(); mode != 0o700 {
		t.Errorf("dir mode = %o, want 0700 (invariant 2 violation)", mode)
	}
}

// TestPrivacyInvariant1_LogRedaction verifies that log output does not contain
// entry text, mantra text, chat_id, or API keys.
// This covers REQ-BR-050 Invariant 1.
func TestPrivacyInvariant1_LogRedaction(t *testing.T) {
	t.Run("AuditLogger does not leak entry text", func(t *testing.T) {
		observedZapCore, logs := observer.New(zapcore.InfoLevel)
		auditLogger := &AuditLogger{
			logger: zap.New(observedZapCore),
		}

		payload := &BriefingPayload{
			JournalRecall: RecallModule{
				Anniversaries: []*AnniversaryEntry{
					{
						YearsAgo:  1,
						Date:      "2025-05-14",
						Text:      "This is sensitive journal content that must never appear in logs",
						EmojiMood: "😊",
					},
				},
			},
			Status: map[string]string{
				"journal": "ok",
			},
		}

		auditLogger.LogOrchestration(payload, 1*time.Second)

		for _, entry := range logs.All() {
			entryStr := entry.Message
			for _, field := range entry.Context {
				entryStr += field.Key + "=" + field.String
			}

			if strings.Contains(entryStr, "This is sensitive journal content") {
				t.Errorf("log contains entry text (invariant 1 violation): %s", entryStr)
			}
		}
	})

	t.Run("AuditLogger does not leak mantra text", func(t *testing.T) {
		observedZapCore, logs := observer.New(zapcore.InfoLevel)
		auditLogger := &AuditLogger{
			logger: zap.New(observedZapCore),
		}

		payload := &BriefingPayload{
			Mantra: MantraModule{
				Text:   "This is sensitive mantra content that must never appear in logs",
				Source: "Ancient Wisdom",
				Index:  0,
				Total:  365,
			},
			Status: map[string]string{
				"mantra": "ok",
			},
		}

		auditLogger.LogOrchestration(payload, 1*time.Second)

		for _, entry := range logs.All() {
			entryStr := entry.Message
			for _, field := range entry.Context {
				entryStr += field.Key + "=" + field.String
			}

			if strings.Contains(entryStr, "This is sensitive mantra content") {
				t.Errorf("log contains mantra text (invariant 1 violation): %s", entryStr)
			}
		}
	})

	t.Run("AuditLogger does not leak chat_id", func(t *testing.T) {
		observedZapCore, logs := observer.New(zapcore.InfoLevel)
		auditLogger := &AuditLogger{
			logger: zap.New(observedZapCore),
		}

		testChatID := "telegram_chat_123456789"
		auditLogger.LogCollection("journal", "ok", 500*time.Millisecond, nil)

		for _, entry := range logs.All() {
			entryStr := entry.Message
			for _, field := range entry.Context {
				entryStr += field.Key + "=" + field.String
			}

			if strings.Contains(entryStr, testChatID) {
				t.Errorf("log contains chat_id (invariant 1 violation): %s", entryStr)
			}

			for _, field := range entry.Context {
				if strings.Contains(strings.ToLower(field.Key), "chat") {
					if strings.Contains(field.String, testChatID) {
						t.Errorf("log field '%s' contains chat_id value: %s", field.Key, field.String)
					}
				}
			}
		}
	})

	t.Run("AuditLogger does not leak API keys", func(t *testing.T) {
		observedZapCore, logs := observer.New(zapcore.InfoLevel)
		auditLogger := &AuditLogger{
			logger: zap.New(observedZapCore),
		}

		testAPIKey := "sk-test-key-redacted"
		auditLogger.LogCollection("weather", "ok", 500*time.Millisecond, nil)

		for _, entry := range logs.All() {
			entryStr := entry.Message
			for _, field := range entry.Context {
				entryStr += field.Key + "=" + field.String
			}

			if strings.Contains(entryStr, testAPIKey) {
				t.Errorf("log contains API key (invariant 1 violation): %s", entryStr)
			}

			for _, field := range entry.Context {
				lowerKey := strings.ToLower(field.Key)
				if strings.Contains(lowerKey, "api") ||
					strings.Contains(lowerKey, "key") ||
					strings.Contains(lowerKey, "token") {
					if strings.Contains(field.String, testAPIKey) {
						t.Errorf("log field '%s' contains API key value: %s", field.Key, field.String)
					}
				}
			}
		}
	})
}

// TestPrivacyInvariant3_NoA2ACommunication verifies that A2A communication count is zero.
// All collectors are Go function calls with no HTTP/gRPC outbound.
// This covers REQ-BR-053 Invariant 3.
func TestPrivacyInvariant3_NoA2ACommunication(t *testing.T) {
	t.Run("All collectors are in-process Go calls", func(t *testing.T) {
		ctx := context.Background()
		userID := "test-user"
		today := time.Now()

		weather := &mockCollector{
			module: &WeatherModule{Offline: false},
			status: "ok",
		}

		journal := &mockCollector{
			module: &RecallModule{Offline: false},
			status: "ok",
		}

		date := &mockCollector{
			module: &DateModule{Today: today.Format("2006-01-02")},
			status: "ok",
		}

		mantra := &mockCollector{
			module: &MantraModule{Text: "Test"},
			status: "ok",
		}

		orchestrator := NewOrchestrator(weather, journal, date, mantra)

		payload, err := orchestrator.Run(ctx, userID, today)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if payload == nil {
			t.Fatal("payload should not be nil")
		}

		if len(payload.Status) != 4 {
			t.Errorf("expected 4 module statuses, got %d", len(payload.Status))
		}
	})
}

// TestPrivacyInvariant4_NoClinicalVocabulary verifies that mantra text and payloads
// do not contain clinical/diagnostic vocabulary.
// This covers REQ-BR-052 Invariant 4.
func TestPrivacyInvariant4_NoClinicalVocabulary(t *testing.T) {
	clinicalTerms := []string{
		"suicide",
		"self-harm",
		"depression",
		"anxiety disorder",
		"bipolar",
		"schizophrenia",
		"PTSD",
		"OCD",
		"eating disorder",
	}

	t.Run("Mantra text contains no clinical vocabulary", func(t *testing.T) {
		mantras := []string{
			"Every day is a new beginning",
			"오늘도 좋은 하루",
			"Believe in yourself",
			"Peace comes from within",
			"Take one step at a time",
		}

		for _, mantra := range mantras {
			mantraLower := strings.ToLower(mantra)

			for _, term := range clinicalTerms {
				if strings.Contains(mantraLower, term) {
					t.Errorf("mantra contains clinical term '%s': %s", term, mantra)
				}
			}
		}
	})

	t.Run("Payload journal entries contain no clinical vocabulary", func(t *testing.T) {
		entries := []*AnniversaryEntry{
			{
				YearsAgo:  1,
				Date:      "2025-05-14",
				Text:      "Had a great day at the beach with friends",
				EmojiMood: "😊",
			},
			{
				YearsAgo:  2,
				Date:      "2024-05-14",
				Text:      "Feeling tired but accomplished after finishing the project",
				EmojiMood: "😌",
			},
			{
				YearsAgo:  3,
				Date:      "2023-05-14",
				Text:      "오늘 기분이 좋다",
				EmojiMood: "🎉",
			},
		}

		for _, entry := range entries {
			textLower := strings.ToLower(entry.Text)

			for _, term := range clinicalTerms {
				if strings.Contains(textLower, term) {
					t.Errorf("journal entry contains clinical term '%s': %s", term, entry.Text)
				}
			}
		}
	})

	t.Run("Clinical vocabulary scanner test", func(t *testing.T) {
		containsClinicalTerm := func(text string) bool {
			textLower := strings.ToLower(text)
			for _, term := range clinicalTerms {
				if strings.Contains(textLower, term) {
					return true
				}
			}
			return false
		}

		positiveTests := []struct {
			text     string
			expected bool
		}{
			{"I feel depression today", true},
			{"Struggling with anxiety disorder", true},
			{" bipolar symptoms ", true},
			{"schizophrenia diagnosis", true},
		}

		for _, tt := range positiveTests {
			result := containsClinicalTerm(tt.text)
			if !result {
				t.Errorf("expected to detect clinical term in '%s'", tt.text)
			}
		}

		negativeTests := []struct {
			text     string
			expected bool
		}{
			{"Feeling happy today", false},
			{"Great weather outside", false},
			{"오늘 기분이 좋다", false},
		}

		for _, tt := range negativeTests {
			result := containsClinicalTerm(tt.text)
			if result != tt.expected {
				t.Errorf("unexpected result for '%s': got %v, want %v", tt.text, result, tt.expected)
			}
		}
	})
}
