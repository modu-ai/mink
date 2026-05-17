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

	if err := a.VerifyEd25519(ctx, "", "", "", nil); !errors.Is(err, ErrNotImplemented) {
		t.Errorf("VerifyEd25519: got %v, want ErrNotImplemented", err)
	}
	if _, err := a.NormalizeInteraction(ctx, nil); !errors.Is(err, ErrNotImplemented) {
		t.Errorf("NormalizeInteraction: got %v, want ErrNotImplemented", err)
	}
	if err := a.HandleSlashCommand(ctx, CanonicalInteraction{}); !errors.Is(err, ErrNotImplemented) {
		t.Errorf("HandleSlashCommand: got %v, want ErrNotImplemented", err)
	}
}

func TestCanonicalInteraction_FieldsPreserved(t *testing.T) {
	intr := CanonicalInteraction{
		UserID: "u1", ChannelID: "c1", GuildID: "g1",
		Type: 2, Content: "/mink prompt", Token: "tk1", AppID: "app1",
	}
	if intr.Type != 2 || intr.Content != "/mink prompt" {
		t.Errorf("field round-trip failed: %+v", intr)
	}
}

var _ Adapter = (*stubAdapter)(nil)
