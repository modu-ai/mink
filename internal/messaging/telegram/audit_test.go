package telegram_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/audit"
	"github.com/modu-ai/mink/internal/messaging/telegram"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// errMockWriter is a mock audit.Writer that always returns an error on Write.
type errMockWriter struct {
	audit.MockWriter
	writeErr error
}

func (e *errMockWriter) Write(event audit.AuditEvent) error {
	_ = e.MockWriter.Write(event) // still record for inspection
	return e.writeErr
}

// TestRecordInbound_HashesContent verifies that the raw body is not stored in
// the audit event — only a SHA-256 hex digest is recorded.
func TestRecordInbound_HashesContent(t *testing.T) {
	mw := audit.NewMockWriter()
	aw := telegram.NewAuditWrapper(mw, zap.NewNop())

	err := aw.RecordInbound(context.Background(), 111, 1, "secret body text", nil)
	require.NoError(t, err)

	require.Len(t, mw.Events, 1)
	ev := mw.Events[0]

	// Raw body must be absent.
	assert.NotContains(t, ev.Message, "secret body text")

	// content_hash must be present and 64 hex chars (SHA-256).
	hash, ok := ev.Metadata["content_hash"]
	require.True(t, ok, "content_hash must be in metadata")
	assert.Len(t, hash, 64, "SHA-256 hex string must be 64 chars")

	// direction must be "inbound".
	assert.Equal(t, "inbound", ev.Metadata["direction"])
}

// TestRecordOutbound_HashesContent verifies the same hash-only guarantee for
// outbound events.
func TestRecordOutbound_HashesContent(t *testing.T) {
	mw := audit.NewMockWriter()
	aw := telegram.NewAuditWrapper(mw, zap.NewNop())

	err := aw.RecordOutbound(context.Background(), 222, 2, "response text", nil)
	require.NoError(t, err)

	require.Len(t, mw.Events, 1)
	ev := mw.Events[0]

	assert.NotContains(t, ev.Message, "response text")

	hash, ok := ev.Metadata["content_hash"]
	require.True(t, ok)
	assert.Len(t, hash, 64)

	assert.Equal(t, "outbound", ev.Metadata["direction"])
}

// TestRecordInbound_WriterFailDoesNotPropagate verifies that an audit.Writer
// error is logged but NOT returned to the caller (REQ-MTGM-N06 non-blocking
// audit requirement).
func TestRecordInbound_WriterFailDoesNotPropagate(t *testing.T) {
	writeErr := errors.New("disk full")
	mw := &errMockWriter{
		MockWriter: *audit.NewMockWriter(),
		writeErr:   writeErr,
	}
	aw := telegram.NewAuditWrapper(mw, zap.NewNop())

	err := aw.RecordInbound(context.Background(), 333, 3, "body", nil)
	// Must return nil even when the writer fails.
	assert.NoError(t, err)
}

// TestRecordOutbound_WriterFailDoesNotPropagate mirrors the inbound test for
// outbound paths.
func TestRecordOutbound_WriterFailDoesNotPropagate(t *testing.T) {
	writeErr := errors.New("remote unreachable")
	mw := &errMockWriter{
		MockWriter: *audit.NewMockWriter(),
		writeErr:   writeErr,
	}
	aw := telegram.NewAuditWrapper(mw, zap.NewNop())

	err := aw.RecordOutbound(context.Background(), 444, 4, "response", nil)
	assert.NoError(t, err)
}

// TestRecordInbound_LengthExceededFlag verifies that when metadata contains
// "length_exceeded": "true", the "length" field is also present in the event
// metadata.
func TestRecordInbound_LengthExceededFlag(t *testing.T) {
	mw := audit.NewMockWriter()
	aw := telegram.NewAuditWrapper(mw, zap.NewNop())

	meta := map[string]any{
		"length_exceeded": true,
		"length":          5000,
	}
	err := aw.RecordInbound(context.Background(), 555, 5, "very long text", meta)
	require.NoError(t, err)

	require.Len(t, mw.Events, 1)
	ev := mw.Events[0]

	assert.Equal(t, "true", ev.Metadata["length_exceeded"])
	_, hasLength := ev.Metadata["length"]
	assert.True(t, hasLength, "length field must be present when length_exceeded is true")
}

// TestSHA256HashFormat checks that the content hash is a valid 64-character
// lowercase hex string for a known input.
func TestSHA256HashFormat(t *testing.T) {
	mw := audit.NewMockWriter()
	aw := telegram.NewAuditWrapper(mw, zap.NewNop())

	_ = aw.RecordInbound(context.Background(), 1, 1, "hello", nil)
	require.Len(t, mw.Events, 1)

	hash := mw.Events[0].Metadata["content_hash"]
	// SHA-256 of "hello" (lowercase hex)
	assert.Equal(t, "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824", hash)
}

// TestRecordInbound_EventTypeIsMessagingInbound checks the EventType value.
func TestRecordInbound_EventTypeIsMessagingInbound(t *testing.T) {
	mw := audit.NewMockWriter()
	aw := telegram.NewAuditWrapper(mw, zap.NewNop())

	_ = aw.RecordInbound(context.Background(), 1, 1, "msg", nil)
	require.Len(t, mw.Events, 1)
	assert.Equal(t, audit.EventTypeMessagingInbound, mw.Events[0].Type)
}

// TestRecordOutbound_EventTypeIsMessagingOutbound checks the outbound EventType.
func TestRecordOutbound_EventTypeIsMessagingOutbound(t *testing.T) {
	mw := audit.NewMockWriter()
	aw := telegram.NewAuditWrapper(mw, zap.NewNop())

	_ = aw.RecordOutbound(context.Background(), 1, 1, "resp", nil)
	require.Len(t, mw.Events, 1)
	assert.Equal(t, audit.EventTypeMessagingOutbound, mw.Events[0].Type)
}

// TestRecordInbound_MetadataChatAndMessageID verifies that chat_id and
// message_id are persisted as metadata strings.
func TestRecordInbound_MetadataChatAndMessageID(t *testing.T) {
	mw := audit.NewMockWriter()
	aw := telegram.NewAuditWrapper(mw, zap.NewNop())

	_ = aw.RecordInbound(context.Background(), 777, 42, "text", nil)
	require.Len(t, mw.Events, 1)
	ev := mw.Events[0]

	assert.Equal(t, "777", ev.Metadata["chat_id"])
	assert.Equal(t, "42", ev.Metadata["message_id"])
}

// TestRecordInbound_TimestampIsRecent verifies that the event timestamp is set
// and is within the last minute.
func TestRecordInbound_TimestampIsRecent(t *testing.T) {
	mw := audit.NewMockWriter()
	aw := telegram.NewAuditWrapper(mw, zap.NewNop())

	before := time.Now()
	_ = aw.RecordInbound(context.Background(), 1, 1, "msg", nil)
	after := time.Now()

	require.Len(t, mw.Events, 1)
	ts := mw.Events[0].Timestamp
	assert.True(t, !ts.Before(before) || ts.After(before.Add(-time.Second)),
		"timestamp should be close to now")
	assert.False(t, ts.After(after.Add(time.Second)), "timestamp should not be in the far future")
}
