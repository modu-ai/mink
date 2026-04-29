package audit

import (
	"testing"
	"time"
)

func TestComputeEventHash(t *testing.T) {
	event := AuditEvent{
		Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Type:      EventTypeGoosedStart,
		Severity:  SeverityInfo,
		Message:   "test message",
		Metadata:  map[string]string{"key": "value"},
	}
	hash1 := ComputeEventHash(event)
	hash2 := ComputeEventHash(event)
	if hash1 != hash2 {
		t.Error("ComputeEventHash is not deterministic")
	}
	if len(hash1) != 64 {
		t.Errorf("Expected 64-char hex hash, got %d chars", len(hash1))
	}
}

func TestComputeEventHashDifferentInputs(t *testing.T) {
	event1 := AuditEvent{
		Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Type:      EventTypeGoosedStart,
		Severity:  SeverityInfo,
		Message:   "message 1",
	}
	event2 := AuditEvent{
		Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Type:      EventTypeGoosedStart,
		Severity:  SeverityInfo,
		Message:   "message 2",
	}
	if ComputeEventHash(event1) == ComputeEventHash(event2) {
		t.Error("Different events produced same hash")
	}
}

func TestVerifyChainIntact(t *testing.T) {
	events := make([]AuditEvent, 3)
	events[0] = AuditEvent{Message: "first"}
	events[0].PrevHash = ""

	events[1] = AuditEvent{Message: "second"}
	events[1].PrevHash = ComputeEventHash(events[0])

	events[2] = AuditEvent{Message: "third"}
	events[2].PrevHash = ComputeEventHash(events[1])

	idx, err := VerifyChain(events)
	if err != nil {
		t.Fatalf("VerifyChain error: %v", err)
	}
	if idx != -1 {
		t.Errorf("Expected intact chain (-1), got break at index %d", idx)
	}
}

func TestVerifyChainBroken(t *testing.T) {
	events := make([]AuditEvent, 3)
	events[0] = AuditEvent{Message: "first"}
	events[0].PrevHash = ""

	events[1] = AuditEvent{Message: "second"}
	events[1].PrevHash = ComputeEventHash(events[0])

	events[2] = AuditEvent{Message: "tampered"}
	events[2].PrevHash = "invalid_hash"

	idx, err := VerifyChain(events)
	if err != nil {
		t.Fatalf("VerifyChain error: %v", err)
	}
	if idx != 2 {
		t.Errorf("Expected break at index 2, got %d", idx)
	}
}

func TestVerifyChainEmptyPrevHash(t *testing.T) {
	events := []AuditEvent{
		{Message: "anchor1", PrevHash: ""},
		{Message: "anchor2", PrevHash: ""},
	}
	idx, err := VerifyChain(events)
	if err != nil {
		t.Fatalf("VerifyChain error: %v", err)
	}
	if idx != -1 {
		t.Errorf("Expected intact chain for anchors (-1), got %d", idx)
	}
}

func TestVerifyChainEmpty(t *testing.T) {
	idx, err := VerifyChain([]AuditEvent{})
	if err != nil {
		t.Fatalf("VerifyChain error: %v", err)
	}
	if idx != -1 {
		t.Errorf("Expected -1 for empty chain, got %d", idx)
	}
}

func TestMarshalUnmarshalWithHash(t *testing.T) {
	event := AuditEvent{
		Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Type:      EventTypeGoosedStart,
		Severity:  SeverityInfo,
		Message:   "test",
		PrevHash:  "abc123",
	}
	data, err := event.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON error: %v", err)
	}

	var decoded AuditEvent
	if err := decoded.UnmarshalJSON(data); err != nil {
		t.Fatalf("UnmarshalJSON error: %v", err)
	}
	if decoded.PrevHash != "abc123" {
		t.Errorf("PrevHash mismatch: got %q, want %q", decoded.PrevHash, "abc123")
	}
}
