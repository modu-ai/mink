// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package v2_test

import (
	"errors"
	"testing"

	v2 "github.com/modu-ai/mink/internal/llm/provider/v2"
)

func TestSentinelErrors_Identity(t *testing.T) {
	t.Parallel()

	sentinels := []struct {
		name string
		err  error
	}{
		{"ErrNotImplemented", v2.ErrNotImplemented},
		{"ErrInvalidRequest", v2.ErrInvalidRequest},
		{"ErrAPIKey", v2.ErrAPIKey},
		{"ErrRateLimited", v2.ErrRateLimited},
		{"ErrModelNotFound", v2.ErrModelNotFound},
		{"ErrStreamClosed", v2.ErrStreamClosed},
	}

	for _, s := range sentinels {
		t.Run(s.name, func(t *testing.T) {
			t.Parallel()
			// Wrapping must preserve identity.
			wrapped := errors.Join(s.err, errors.New("context"))
			if !errors.Is(wrapped, s.err) {
				t.Errorf("errors.Is(%v wrapped, %v) = false, want true", s.name, s.name)
			}
		})
	}
}

func TestSentinelErrors_Distinct(t *testing.T) {
	t.Parallel()

	all := []error{
		v2.ErrNotImplemented,
		v2.ErrInvalidRequest,
		v2.ErrAPIKey,
		v2.ErrRateLimited,
		v2.ErrModelNotFound,
		v2.ErrStreamClosed,
	}

	for i, a := range all {
		for j, b := range all {
			if i == j {
				continue
			}
			if errors.Is(a, b) {
				t.Errorf("sentinel %d and sentinel %d should be distinct but errors.Is returns true", i, j)
			}
		}
	}
}
