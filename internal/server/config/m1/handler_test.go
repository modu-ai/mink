// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package m1

import (
	"context"
	"errors"
	"testing"
)

func TestStubHandler_AllMethodsReturnNotImplemented(t *testing.T) {
	h := NewStubHandler()
	ctx := context.Background()

	if _, err := h.Snapshot(ctx); !errors.Is(err, ErrNotImplemented) {
		t.Errorf("Snapshot: got %v, want ErrNotImplemented", err)
	}
	if err := h.UpdateSection(ctx, SectionAuthStore, nil); !errors.Is(err, ErrNotImplemented) {
		t.Errorf("UpdateSection: got %v, want ErrNotImplemented", err)
	}
}

func TestConfigSections_Defined(t *testing.T) {
	sections := []ConfigSection{
		SectionAuthStore, SectionLLMProviders, SectionChannels,
		SectionBrandTheme, SectionLocale,
	}
	if len(sections) != 5 {
		t.Errorf("expected 5 sections, got %d", len(sections))
	}
}

var _ Handler = (*stubHandler)(nil)
