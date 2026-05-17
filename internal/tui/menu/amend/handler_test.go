// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package amend

import (
	"context"
	"errors"
	"testing"
)

func TestStubHandler_ExecuteReturnsNotImplemented(t *testing.T) {
	h := NewStubHandler()
	ctx := context.Background()

	if _, err := h.Execute(ctx, CmdMemory, nil); !errors.Is(err, ErrNotImplemented) {
		t.Errorf("Execute: got %v, want ErrNotImplemented", err)
	}
}

func TestSlashCommands_Defined(t *testing.T) {
	cmds := []SlashCommand{CmdMemory, CmdBriefing, CmdJournal, CmdWeather, CmdRitual, CmdConfig}
	if len(cmds) != 6 {
		t.Errorf("expected 6 amended slash commands, got %d", len(cmds))
	}
}

var _ Handler = (*stubHandler)(nil)
