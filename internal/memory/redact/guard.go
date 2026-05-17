// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package redact

// This file implements the sealed-content guard that enforces AC-MEM-028:
// "session transcript export bypassing the masking pipeline must be
// explicitly rejected with an error".
//
// The guard works by handing callers a RedactedContent value type that can
// only be sealed via the Apply constructor in this package.  Downstream
// Indexer implementations check Sealed() (or use Bytes()) and reject any
// value that was constructed externally, returning ErrRedactBypass.
//
// SPEC: SPEC-MINK-MEMORY-QMD-001 (M2, T2.4b)
// REQ:  REQ-MEM-028 (AC-MEM-028)

// RedactedContent wraps content that has gone through the Redact pipeline.
//
// The zero value is intentionally unsealed: any caller that constructs a
// RedactedContent literally (instead of via Apply) will get
// ErrRedactBypass from Bytes() and Sealed() == false.  This makes accidental
// bypass a type-system level mistake.
type RedactedContent struct {
	masked string
	sealed bool
}

// Apply wraps Redact and seals the result so downstream Indexers can
// verify the pipeline ran.  Returns the sealed content plus the structured
// hit record so callers may log redaction statistics.
//
// @MX:ANCHOR: [AUTO] Single legitimate constructor of sealed
// RedactedContent — fan_in >= 3 (M3 session export, M5 publish hooks,
// audit tests).
// @MX:REASON: Adding another sealing path would defeat the type-level
// guarantee that every sealed RedactedContent has passed through Redact.
// Callers needing the masked text without sealing should call Redact
// directly.
func Apply(content string) (RedactedContent, []Hit) {
	res := Redact(content)
	return RedactedContent{masked: res.Masked, sealed: true}, res.Hits
}

// Bytes returns the masked text for storage.  Calling Bytes on a
// zero-value RedactedContent returns ErrRedactBypass.
func (rc RedactedContent) Bytes() (string, error) {
	if !rc.sealed {
		return "", ErrRedactBypass
	}
	return rc.masked, nil
}

// Sealed reports whether the content has gone through Apply (true) or was
// constructed externally (false).
func (rc RedactedContent) Sealed() bool {
	return rc.sealed
}

// IngestRedacted is the canonical AC-MEM-028 reference implementation used
// by the unit tests and as a template for downstream Indexer
// implementations in M3 and M5.  It accepts a RedactedContent value,
// returns ErrRedactBypass on unsealed input, and otherwise hands the
// masked string to the supplied sink.
//
// Production Indexer implementations are expected to follow the same
// pattern: take RedactedContent, call Bytes(), surface the error.
func IngestRedacted(rc RedactedContent, sink func(masked string) error) error {
	masked, err := rc.Bytes()
	if err != nil {
		return err
	}
	if sink == nil {
		return nil
	}
	return sink(masked)
}
