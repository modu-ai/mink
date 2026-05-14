package briefing

import (
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// AuditLogger is a zap logger wrapper that automatically redacts sensitive information.
// It enforces REQ-BR-050 invariant: log output must not contain entry text, mantra text,
// chat_id, or API keys.
type AuditLogger struct {
	logger *zap.Logger
}

// NewAuditLogger creates a new audit logger with redaction enabled.
func NewAuditLogger() *AuditLogger {
	// Create a development config with console encoder
	config := zap.NewDevelopmentConfig()
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

	logger, _ := config.Build()
	return &AuditLogger{
		logger: logger,
	}
}

// LogCollection logs a module collection result with redaction.
// Only allowed fields are logged: module, status, duration_ms, error_type.
// Sensitive fields (entry text, mantra text, chat_id, API keys) are excluded.
func (a *AuditLogger) LogCollection(module string, status string, duration time.Duration, err error) {
	// Build fields with only allowed information
	fields := []zap.Field{
		zap.String("module", module),
		zap.String("status", status),
		zap.Int64("duration_ms", duration.Milliseconds()),
	}

	// Add error type without sensitive details
	if err != nil {
		fields = append(fields, zap.String("error_type", errorType(err)))
	}

	a.logger.Info("collection completed", fields...)
}

// LogOrchestration logs the orchestration result with redaction.
// Only logs module statuses, not the actual payload content.
func (a *AuditLogger) LogOrchestration(payload *BriefingPayload, totalDuration time.Duration) {
	a.logger.Info("orchestration completed",
		zap.Int64("total_duration_ms", totalDuration.Milliseconds()),
		zap.Int("modules_collected", len(payload.Status)),
	)

	// Log each module status without content
	for module, status := range payload.Status {
		a.logger.Info("module_status",
			zap.String("module", module),
			zap.String("status", status),
		)
	}
}

// errorType extracts the error type without exposing sensitive details.
func errorType(err error) string {
	if err == nil {
		return "none"
	}

	errMsg := err.Error()

	// Check for common error patterns and return generic types
	switch {
	case strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "deadline"):
		return "timeout"
	case strings.Contains(errMsg, "connection") || strings.Contains(errMsg, "network"):
		return "network_error"
	case strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "404"):
		return "not_found"
	case strings.Contains(errMsg, "permission") || strings.Contains(errMsg, "unauthorized"):
		return "permission_denied"
	case strings.Contains(errMsg, "validation"):
		return "validation_error"
	default:
		// Return generic error type, not the message
		return "unknown_error"
	}
}

// RedactEntryText removes sensitive text from journal entries.
// This is a helper for ensuring no entry text leaks into logs.
func RedactEntryText(text string) string {
	if text == "" {
		return ""
	}
	// Return redaction placeholder with rune count (character count)
	runeCount := len([]rune(text))
	return fmt.Sprintf("[REDACTED_ENTRY_TEXT_%d]", runeCount)
}

// RedactMantraText removes sensitive text from mantras.
// This is a helper for ensuring no mantra text leaks into logs.
func RedactMantraText(text string) string {
	if text == "" {
		return ""
	}
	// Return redaction placeholder with rune count (character count)
	runeCount := len([]rune(text))
	return fmt.Sprintf("[REDACTED_MANTRA_TEXT_%d]", runeCount)
}

// RedactChatID removes chat IDs from logs.
func RedactChatID(chatID string) string {
	if chatID == "" {
		return ""
	}
	return "[REDACTED_CHAT_ID]"
}

// RedactAPIKey removes API keys from logs.
func RedactAPIKey(key string) string {
	if key == "" {
		return ""
	}
	// Show only first 4 and last 4 characters
	if len(key) <= 8 {
		return "[REDACTED_API_KEY]"
	}
	return fmt.Sprintf("%s...%s", key[:4], key[len(key)-4:])
}

// Sync flushes the logger buffers.
func (a *AuditLogger) Sync() error {
	return a.logger.Sync()
}
