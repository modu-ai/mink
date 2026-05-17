// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

// Package amend contains the M1 milestone scaffolding for
// SPEC-MINK-CLI-TUI-003-AMEND-001 — TUI/CLI parity amendment.
//
// M1 establishes the amendment handler interface and stub implementation.
// The amendment introduces missing /memory, /briefing, /journal, /weather,
// /ritual, /config TUI slash commands at parity with their CLI counterparts.
// Real wiring lands in M2~M3 via expert-refactoring.
//
// SPEC: SPEC-MINK-CLI-TUI-003-AMEND-001 (REQ-CTA-001~006, AC-CTA-001~005)
package amend

import (
	"context"
	"errors"
)

// ErrNotImplemented marks a method whose real implementation lands in M2~M3.
var ErrNotImplemented = errors.New("cli-tui amend m1: not implemented (real impl in M2~M3)")

// SlashCommand identifies a TUI slash command being added at parity.
type SlashCommand string

const (
	CmdMemory   SlashCommand = "/memory"
	CmdBriefing SlashCommand = "/briefing"
	CmdJournal  SlashCommand = "/journal"
	CmdWeather  SlashCommand = "/weather"
	CmdRitual   SlashCommand = "/ritual"
	CmdConfig   SlashCommand = "/config"
)

// CommandResult is the result returned to the TUI renderer.
type CommandResult struct {
	Command  SlashCommand
	Output   string
	IsError  bool
	ErrorMsg string
}

// Handler is the M1 TUI slash-command handler interface.
type Handler interface {
	// Execute invokes the named slash command with the given arguments.
	Execute(ctx context.Context, cmd SlashCommand, args []string) (CommandResult, error)
}

type stubHandler struct{}

// NewStubHandler returns the M1 stub handler.
func NewStubHandler() Handler { return &stubHandler{} }

func (*stubHandler) Execute(_ context.Context, _ SlashCommand, _ []string) (CommandResult, error) {
	return CommandResult{}, ErrNotImplemented
}
