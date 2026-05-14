package briefing

import (
	"errors"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestAuditLogger_LogCollection_AllowsOnlySafeFields(t *testing.T) {
	observedZapCore, logs := observer.New(zapcore.InfoLevel)
	auditLogger := &AuditLogger{
		logger: zap.New(observedZapCore),
	}

	auditLogger.LogCollection("weather", "ok", 500*time.Millisecond, nil)

	if logs.Len() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logs.Len())
	}

	entry := logs.All()[0]
	checkField(t, entry.Context, "module", "weather")
	checkField(t, entry.Context, "status", "ok")
	checkField(t, entry.Context, "duration_ms", int64(500))

	for _, field := range entry.Context {
		key := field.Key
		if containsSensitiveKey(key) {
			t.Errorf("log should not contain sensitive field '%s'", key)
		}
	}
}

func TestAuditLogger_LogCollection_WithError(t *testing.T) {
	observedZapCore, logs := observer.New(zapcore.InfoLevel)
	auditLogger := &AuditLogger{
		logger: zap.New(observedZapCore),
	}

	testErr := errors.New("connection timeout after 30s")
	auditLogger.LogCollection("journal", "timeout", 30*time.Second, testErr)

	if logs.Len() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logs.Len())
	}

	entry := logs.All()[0]
	checkField(t, entry.Context, "error_type", "timeout")

	logMsg := entry.Message
	if strings.Contains(logMsg, testErr.Error()) {
		t.Errorf("log message should not contain full error details, got: %s", logMsg)
	}

	for _, field := range entry.Context {
		if field.Key == "error" {
			t.Errorf("should not log raw error field, got: %v", field)
		}
	}
}

func TestAuditLogger_LogOrchestration_WithoutContent(t *testing.T) {
	observedZapCore, logs := observer.New(zapcore.InfoLevel)
	auditLogger := &AuditLogger{
		logger: zap.New(observedZapCore),
	}

	payload := &BriefingPayload{
		Weather: WeatherModule{
			Current: &WeatherCurrent{
				Temp:      20.0,
				Condition: "Sunny",
			},
		},
		JournalRecall: RecallModule{
			Anniversaries: []*AnniversaryEntry{
				{
					YearsAgo: 1,
					Date:     "2025-05-14",
					Text:     "This is sensitive journal text that should not be logged",
				},
			},
		},
		DateCalendar: DateModule{
			Today:     "2026-05-14",
			DayOfWeek: "목요일",
		},
		Mantra: MantraModule{
			Text:   "Sensitive mantra text that must not appear in logs",
			Source: "Ancient Wisdom",
		},
		Status: map[string]string{
			"weather": "ok",
			"journal": "ok",
			"date":    "ok",
			"mantra":  "ok",
		},
		GeneratedAt: time.Now(),
	}

	auditLogger.LogOrchestration(payload, 2*time.Second)

	if logs.Len() != 5 {
		t.Fatalf("expected 5 log entries, got %d", logs.Len())
	}

	for _, entry := range logs.All() {
		var entryStr strings.Builder
		entryStr.WriteString(entry.Message)
		for _, field := range entry.Context {
			entryStr.WriteString(field.Key)
			entryStr.WriteString("=")
			entryStr.WriteString(field.String)
		}
		combined := entryStr.String()

		sensitiveStrings := []string{
			"This is sensitive journal text",
			"Sensitive mantra text",
		}

		for _, sensitive := range sensitiveStrings {
			if strings.Contains(combined, sensitive) {
				t.Errorf("log should not contain sensitive data: %s", sensitive)
			}
		}
	}
}

func TestRedactEntryText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"normal text", "This is my private journal entry", "[REDACTED_ENTRY_TEXT_32]"},
		{"korean text", "오늘 기분이 좋다", "[REDACTED_ENTRY_TEXT_9]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RedactEntryText(tt.input)
			if result != tt.expected {
				t.Errorf("RedactEntryText(%q) = %q, want %q", tt.input, result, tt.expected)
			}

			if tt.input != "" && strings.Contains(result, tt.input) {
				t.Errorf("redacted text should not contain original: %s", result)
			}
		})
	}
}

func TestRedactMantraText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"mantra text", "Every day is a new beginning", "[REDACTED_MANTRA_TEXT_28]"},
		{"korean mantra", "오늘도 좋은 하루", "[REDACTED_MANTRA_TEXT_9]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RedactMantraText(tt.input)
			if result != tt.expected {
				t.Errorf("RedactMantraText(%q) = %q, want %q", tt.input, result, tt.expected)
			}

			if tt.input != "" && strings.Contains(result, tt.input) {
				t.Errorf("redacted text should not contain original: %s", result)
			}
		})
	}
}

func TestRedactChatID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"numeric chat ID", "123456789", "[REDACTED_CHAT_ID]"},
		{"string chat ID", "telegram_chat_123", "[REDACTED_CHAT_ID]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RedactChatID(tt.input)
			if result != tt.expected {
				t.Errorf("RedactChatID(%q) = %q, want %q", tt.input, result, tt.expected)
			}

			if tt.input != "" && strings.Contains(result, tt.input) {
				t.Errorf("redacted ID should not contain original: %s", result)
			}
		})
	}
}

func TestRedactAPIKey(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"short key", "abc123", "[REDACTED_API_KEY]"},
		{"long key", "sk-1234567890abcdef1234567890abcdef", "sk-1...cdef"},
		{"exactly 8 chars", "12345678", "[REDACTED_API_KEY]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RedactAPIKey(tt.input)
			if result != tt.expected {
				t.Errorf("RedactAPIKey(%q) = %q, want %q", tt.input, result, tt.expected)
			}

			if tt.input != "" && tt.input != result && len(result) > 20 {
				if strings.Contains(result, tt.input[4:len(tt.input)-4]) {
					t.Errorf("redacted key should not expose middle portion: %s", result)
				}
			}
		})
	}
}

func TestErrorType(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{"nil error", nil, "none"},
		{"timeout error", errors.New("context deadline exceeded"), "timeout"},
		{"connection error", errors.New("connection refused"), "network_error"},
		{"not found error", errors.New("resource not found"), "not_found"},
		{"permission error", errors.New("permission denied"), "permission_denied"},
		{"validation error", errors.New("validation failed: invalid input"), "validation_error"},
		{"unknown error", errors.New("something unexpected happened"), "unknown_error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := errorType(tt.err)
			if result != tt.expected {
				t.Errorf("errorType(%v) = %q, want %q", tt.err, result, tt.expected)
			}

			if tt.err != nil && strings.Contains(result, tt.err.Error()) {
				t.Errorf("errorType should not return full error message, got: %s", result)
			}
		})
	}
}

func checkField(t *testing.T, context []zapcore.Field, key string, expected any) {
	t.Helper()

	for _, field := range context {
		if field.Key == key {
			switch v := expected.(type) {
			case string:
				if field.String == v {
					return
				}
				t.Errorf("field %s has value %q, want %q", key, field.String, v)
			case int64:
				if field.Integer == v {
					return
				}
				t.Errorf("field %s has value %d, want %d", key, field.Integer, v)
			default:
				t.Errorf("unsupported type for field %s", key)
			}
			return
		}
	}

	t.Errorf("field %s not found in context", key)
}

func containsSensitiveKey(key string) bool {
	sensitiveKeys := []string{
		"entry_text", "mantra_text", "chat_id", "api_key",
		"text", "message", "content", "body",
	}

	for _, sensitive := range sensitiveKeys {
		if strings.Contains(strings.ToLower(key), sensitive) {
			return true
		}
	}

	return false
}
