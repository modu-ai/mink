// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package phase5

import (
	"context"
	"errors"
	"testing"
)

func TestStubHandler_AllMethodsReturnNotImplemented(t *testing.T) {
	h := NewStubHandler()
	ctx := context.Background()

	if _, err := h.SubmitStep(ctx, StepLLMProviders, "sid1", nil); !errors.Is(err, ErrNotImplemented) {
		t.Errorf("SubmitStep: got %v, want ErrNotImplemented", err)
	}
	if _, err := h.SkipStep(ctx, StepChannels, "sid1"); !errors.Is(err, ErrNotImplemented) {
		t.Errorf("SkipStep: got %v, want ErrNotImplemented", err)
	}
}

func TestSteps_Defined(t *testing.T) {
	steps := []Step{StepLLMProviders, StepChannels, StepBrandTheme, StepReview}
	if len(steps) != 4 {
		t.Errorf("expected 4 Phase 5 steps, got %d", len(steps))
	}
}

var _ Handler = (*stubHandler)(nil)
