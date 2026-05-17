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

func TestStubAdapter_AllMethodsReturnNotImplemented(t *testing.T) {
	a := NewStubAdapter()
	ctx := context.Background()

	if err := a.VerifySigningSecret(ctx, nil, ""); !errors.Is(err, ErrNotImplemented) {
		t.Errorf("VerifySigningSecret: got %v, want ErrNotImplemented", err)
	}
	if _, err := a.NormalizeEvent(ctx, nil); !errors.Is(err, ErrNotImplemented) {
		t.Errorf("NormalizeEvent: got %v, want ErrNotImplemented", err)
	}
	if err := a.HandleAppMention(ctx, CanonicalEvent{}); !errors.Is(err, ErrNotImplemented) {
		t.Errorf("HandleAppMention: got %v, want ErrNotImplemented", err)
	}
}

func TestCanonicalEvent_FieldsPreserved(t *testing.T) {
	evt := CanonicalEvent{
		UserID: "U123", ChannelID: "C456", TeamID: "T789",
		Text: "hello", Timestamp: "1234567890.000100", ThreadTS: "1234567890.000099",
	}
	if evt.UserID != "U123" || evt.ChannelID != "C456" {
		t.Errorf("field round-trip failed: %+v", evt)
	}
}

// Compile-time interface assertion.
var _ Adapter = (*stubAdapter)(nil)
