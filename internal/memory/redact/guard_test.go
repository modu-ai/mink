// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package redact

import (
	"errors"
	"strings"
	"testing"
)

// TestApply_SealsResult verifies that Apply returns a RedactedContent
// whose Sealed() reports true.
func TestApply_SealsResult(t *testing.T) {
	t.Parallel()

	rc, _ := Apply("alice@example.com sent the message")
	if !rc.Sealed() {
		t.Fatal("expected Sealed() to be true after Apply")
	}
}

// TestApply_BytesRoundTrip verifies a sealed RedactedContent yields its
// masked payload via Bytes() with no error.
func TestApply_BytesRoundTrip(t *testing.T) {
	t.Parallel()

	rc, hits := Apply("contact alice@example.com today")
	got, err := rc.Bytes()
	if err != nil {
		t.Fatalf("Bytes() returned error: %v", err)
	}
	if !strings.Contains(got, "[REDACTED:email]") {
		t.Fatalf("masked bytes %q missing email token", got)
	}
	if len(hits) != 1 || hits[0].Category != CategoryEmail {
		t.Fatalf("expected single email hit, got %v", hits)
	}
}

// TestZeroValue_BytesReturnsErrRedactBypass enforces the AC-MEM-028
// contract — any externally-constructed RedactedContent yields
// ErrRedactBypass when its Bytes() is invoked.
func TestZeroValue_BytesReturnsErrRedactBypass(t *testing.T) {
	t.Parallel()

	var rc RedactedContent
	_, err := rc.Bytes()
	if !errors.Is(err, ErrRedactBypass) {
		t.Fatalf("expected ErrRedactBypass, got %v", err)
	}
	if rc.Sealed() {
		t.Fatal("zero value must report Sealed() == false")
	}
}

// TestApply_NoPIIPassthrough verifies that content without any PII is
// still sealed and round-trips unchanged.
func TestApply_NoPIIPassthrough(t *testing.T) {
	t.Parallel()

	content := "The weather report has no PII."
	rc, hits := Apply(content)
	if !rc.Sealed() {
		t.Fatal("expected Sealed() to be true")
	}
	got, err := rc.Bytes()
	if err != nil {
		t.Fatalf("Bytes() returned error: %v", err)
	}
	if got != content {
		t.Fatalf("expected %q, got %q", content, got)
	}
	if len(hits) != 0 {
		t.Fatalf("expected zero hits, got %v", hits)
	}
}

// TestIngestRedacted_HappyPath uses the AC-MEM-028 reference Indexer to
// confirm that sealed content reaches the sink.
func TestIngestRedacted_HappyPath(t *testing.T) {
	t.Parallel()

	rc, _ := Apply("call me at 010-1234-5678 ok")

	var seen string
	err := IngestRedacted(rc, func(masked string) error {
		seen = masked
		return nil
	})
	if err != nil {
		t.Fatalf("IngestRedacted returned %v", err)
	}
	if !strings.Contains(seen, "[REDACTED:phone_kr]") {
		t.Fatalf("sink received unmasked content: %q", seen)
	}
}

// TestIngestRedacted_BypassRejected is the canonical AC-MEM-028 fixture:
// passing an unsealed RedactedContent to the Indexer surface must return
// ErrRedactBypass and never invoke the sink.
func TestIngestRedacted_BypassRejected(t *testing.T) {
	t.Parallel()

	var rc RedactedContent // unsealed
	called := false
	err := IngestRedacted(rc, func(masked string) error {
		called = true
		return nil
	})
	if !errors.Is(err, ErrRedactBypass) {
		t.Fatalf("expected ErrRedactBypass, got %v", err)
	}
	if called {
		t.Fatal("sink must not be invoked when content is unsealed")
	}
}

// TestIngestRedacted_NilSink verifies a sealed content with no sink
// returns nil (no error) — useful when callers want bypass detection
// without actually consuming the payload.
func TestIngestRedacted_NilSink(t *testing.T) {
	t.Parallel()

	rc, _ := Apply("no pii here")
	if err := IngestRedacted(rc, nil); err != nil {
		t.Fatalf("expected nil error with nil sink, got %v", err)
	}
}

// TestErrRedactBypass_Identity ensures the sentinel value is comparable
// via errors.Is (i.e., callers can assert against it directly).
func TestErrRedactBypass_Identity(t *testing.T) {
	t.Parallel()

	if !errors.Is(ErrRedactBypass, ErrRedactBypass) {
		t.Fatal("ErrRedactBypass is not identity-comparable")
	}
	if errors.Is(ErrRedactBypass, errors.New("other")) {
		t.Fatal("ErrRedactBypass falsely matches a different error")
	}
}
