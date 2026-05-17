// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package ollama

import (
	"context"
	"errors"
	"fmt"
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldFallbackToBM25_nil(t *testing.T) {
	assert.False(t, ShouldFallbackToBM25(nil))
}

func TestShouldFallbackToBM25_ErrOllamaUnreachable(t *testing.T) {
	assert.True(t, ShouldFallbackToBM25(ErrOllamaUnreachable))
}

func TestShouldFallbackToBM25_wrapped_ErrOllamaUnreachable(t *testing.T) {
	wrapped := fmt.Errorf("ollama.Client.Embed: %w: dial refused", ErrOllamaUnreachable)
	assert.True(t, ShouldFallbackToBM25(wrapped))
}

func TestShouldFallbackToBM25_ErrOllamaTimeout(t *testing.T) {
	assert.True(t, ShouldFallbackToBM25(ErrOllamaTimeout))
}

func TestShouldFallbackToBM25_ErrOllamaServer(t *testing.T) {
	assert.True(t, ShouldFallbackToBM25(ErrOllamaServer))
}

func TestShouldFallbackToBM25_ErrCircuitOpen(t *testing.T) {
	assert.True(t, ShouldFallbackToBM25(ErrCircuitOpen))
}

func TestShouldFallbackToBM25_ECONNREFUSED(t *testing.T) {
	assert.True(t, ShouldFallbackToBM25(syscall.ECONNREFUSED))
}

func TestShouldFallbackToBM25_wrapped_ECONNREFUSED(t *testing.T) {
	wrapped := fmt.Errorf("connect: %w", syscall.ECONNREFUSED)
	assert.True(t, ShouldFallbackToBM25(wrapped))
}

func TestShouldFallbackToBM25_os_ErrDeadlineExceeded(t *testing.T) {
	assert.True(t, ShouldFallbackToBM25(os.ErrDeadlineExceeded))
}

func TestShouldFallbackToBM25_context_DeadlineExceeded(t *testing.T) {
	assert.True(t, ShouldFallbackToBM25(context.DeadlineExceeded))
}

func TestShouldFallbackToBM25_wraps_context_DeadlineExceeded(t *testing.T) {
	wrapped := fmt.Errorf("request timed out: %w", context.DeadlineExceeded)
	assert.True(t, ShouldFallbackToBM25(wrapped))
}

func TestShouldFallbackToBM25_randomError_false(t *testing.T) {
	err := errors.New("some unexpected programming error")
	assert.False(t, ShouldFallbackToBM25(err))
}

func TestShouldFallbackToBM25_ErrEndpointDenied_false(t *testing.T) {
	// ErrEndpointDenied is a configuration error, not a recoverable outage.
	assert.False(t, ShouldFallbackToBM25(ErrEndpointDenied))
}
