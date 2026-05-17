// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package ollama

import (
	"context"
	"errors"
	"os"
	"syscall"
)

// ShouldFallbackToBM25 returns true when err represents a recoverable Ollama
// outage that should cause a silent fall-back to BM25 full-text search.
//
// The function returns false for nil and for any error that does not
// represent a known recoverable outage (e.g. programming errors, empty query,
// schema mismatches).
//
// True conditions:
//   - ErrOllamaUnreachable
//   - ErrOllamaTimeout
//   - ErrOllamaServer
//   - ErrCircuitOpen
//   - errors wrapping syscall.ECONNREFUSED
//   - errors wrapping os.ErrDeadlineExceeded
//   - errors wrapping context.DeadlineExceeded (5-s client timeout)
//
// SPEC: SPEC-MINK-MEMORY-QMD-001 T3.2
// REQ:  REQ-MEM-019
func ShouldFallbackToBM25(err error) bool {
	if err == nil {
		return false
	}

	// Check sentinel errors.
	if errors.Is(err, ErrOllamaUnreachable) {
		return true
	}
	if errors.Is(err, ErrOllamaTimeout) {
		return true
	}
	if errors.Is(err, ErrOllamaServer) {
		return true
	}
	if errors.Is(err, ErrCircuitOpen) {
		return true
	}

	// Check wrapped low-level errors.
	if errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	return false
}
