package journal

import (
	"crypto/sha256"
	"fmt"
	"maps"
	"strconv"
	"time"

	"github.com/modu-ai/mink/internal/audit"
)

// journalAuditWriter wraps the shared audit.FileWriter and emits journal-specific events.
// PRIVACY: entry text is NEVER included in metadata. user_id is hashed. AC-008
type journalAuditWriter struct {
	w *audit.FileWriter
}

// newJournalAuditWriter creates a journalAuditWriter backed by an audit.FileWriter.
// Pass nil to disable auditing (useful in unit tests that only verify log redaction).
func newJournalAuditWriter(w *audit.FileWriter) *journalAuditWriter {
	return &journalAuditWriter{w: w}
}

// emit writes a single journal audit event.
// meta must never contain entry text or raw user_id — callers enforce this invariant.
//
// @MX:ANCHOR: [AUTO] Audit emission point for all journal operations
// @MX:REASON: Called by writer.Write, export, orchestrator, and storage operations — fan_in >= 4
// @MX:SPEC: SPEC-GOOSE-JOURNAL-001 REQ-004, AC-008
func (a *journalAuditWriter) emit(operation string, extra map[string]string, outcome string) {
	if a == nil || a.w == nil {
		return
	}

	meta := map[string]string{
		"operation": operation,
		"outcome":   outcome,
	}
	maps.Copy(meta, extra)

	event := audit.NewAuditEvent(
		time.Now(),
		audit.EventTypeRitualJournalInvoke,
		audit.SeverityInfo,
		"journal operation: "+operation,
		meta,
	)
	_ = a.w.Write(event)
}

// emitWrite emits an audit event for a journal write operation.
// The entry text is not passed; only derived metadata is included.
func (a *journalAuditWriter) emitWrite(userID string, entry *StoredEntry, outcome string) {
	a.emit("write", map[string]string{
		"user_id_hash":        hashUserID(userID),
		"entry_length_bucket": lengthBucket(len(entry.Text)),
		"emotion_tags_count":  strconv.Itoa(len(entry.EmotionTags)),
		"has_attachment":      strconv.FormatBool(len(entry.AttachmentPaths) > 0),
		"crisis_flag":         strconv.FormatBool(entry.CrisisFlag),
	}, outcome)
}

// emitOperation emits a generic journal operation audit event.
func (a *journalAuditWriter) emitOperation(operation, userID, outcome string) {
	a.emit(operation, map[string]string{
		"user_id_hash": hashUserID(userID),
	}, outcome)
}

// hashUserID returns the first 8 hex chars of SHA-256(userID).
// This is a one-way hash — the original user_id cannot be reconstructed.
func hashUserID(userID string) string {
	sum := sha256.Sum256([]byte(userID))
	return fmt.Sprintf("%x", sum[:4]) // 4 bytes = 8 hex chars
}

// lengthBucket classifies entry text length into a coarse bucket.
func lengthBucket(byteLen int) string {
	switch {
	case byteLen < 100:
		return "<100"
	case byteLen < 500:
		return "100-500"
	default:
		return "500+"
	}
}
