package telegram

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/modu-ai/mink/internal/audit"
	"go.uber.org/zap"
)

// AuditWrapper records inbound and outbound messaging events to an audit.Writer.
// Raw message bodies are never stored — only a SHA-256 content hash is included.
// REQ-MTGM-U01: all inbound/outbound events are logged.
// REQ-MTGM-N06: raw message body must not appear in audit records.
//
// @MX:ANCHOR: [AUTO] AuditWrapper is the single audit gateway for the telegram messaging channel.
// @MX:REASON: SPEC-GOOSE-MSG-TELEGRAM-001; fan_in via BridgeQueryHandler, bootstrap, and tests (>= 3 callers).
type AuditWrapper struct {
	writer audit.Writer
	logger *zap.Logger
}

// NewAuditWrapper creates an AuditWrapper backed by the given audit.Writer and logger.
func NewAuditWrapper(w audit.Writer, logger *zap.Logger) *AuditWrapper {
	return &AuditWrapper{writer: w, logger: logger}
}

// RecordInbound logs an inbound message event.
// Only a SHA-256 hex digest of body is recorded; the raw body is discarded.
// metadata may carry auxiliary flags (e.g. length_exceeded, dropped_blocked).
// If the underlying writer returns an error, it is logged as a warning and nil is returned.
func (a *AuditWrapper) RecordInbound(ctx context.Context, chatID int64, msgID int64, body string, metadata map[string]any) error {
	return a.record(ctx, audit.EventTypeMessagingInbound, "inbound", chatID, msgID, body, metadata)
}

// RecordOutbound logs an outbound message event.
// Only a SHA-256 hex digest of body is recorded; the raw body is discarded.
// If the underlying writer returns an error, it is logged as a warning and nil is returned.
func (a *AuditWrapper) RecordOutbound(ctx context.Context, chatID int64, msgID int64, body string, metadata map[string]any) error {
	return a.record(ctx, audit.EventTypeMessagingOutbound, "outbound", chatID, msgID, body, metadata)
}

// record is the shared implementation for both directions.
func (a *AuditWrapper) record(
	_ context.Context,
	evType audit.EventType,
	direction string,
	chatID int64,
	msgID int64,
	body string,
	meta map[string]any,
) error {
	contentHash := sha256Hex(body)

	// Build string metadata for the audit event.
	evMeta := map[string]string{
		"direction":    direction,
		"chat_id":      fmt.Sprintf("%d", chatID),
		"message_id":   fmt.Sprintf("%d", msgID),
		"content_hash": contentHash,
	}

	// Propagate caller-supplied flags.
	for k, v := range meta {
		evMeta[k] = fmt.Sprintf("%v", v)
	}

	ev := audit.NewAuditEvent(
		time.Now(),
		evType,
		audit.SeverityInfo,
		fmt.Sprintf("messaging %s chat_id=%d msg_id=%d", direction, chatID, msgID),
		evMeta,
	)

	if err := a.writer.Write(ev); err != nil {
		a.logger.Warn("audit write failed; message processing continues",
			zap.Error(err),
			zap.String("direction", direction),
			zap.Int64("chat_id", chatID),
		)
		// Audit failures must NOT block the messaging channel.
		return nil
	}
	return nil
}

// sha256Hex returns the lowercase hex-encoded SHA-256 digest of s.
func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", sum)
}
