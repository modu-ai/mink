// Package builtin provides the built-in slash commands for MINK.
// SPEC: SPEC-GOOSE-COMMAND-001
package builtin

import (
	"context"

	"github.com/modu-ai/mink/internal/command"
)

// exitCommand implements /exit.
type exitCommand struct{}

func (e *exitCommand) Name() string { return "exit" }
func (e *exitCommand) Metadata() command.Metadata {
	return command.Metadata{
		Description: "Exit the goose session.",
		Source:      command.SourceBuiltin,
	}
}

// Execute returns ResultExit with code 0. REQ-CMD-007 (AC-CMD-007).
func (e *exitCommand) Execute(_ context.Context, _ command.Args) (command.Result, error) {
	return command.Result{Kind: command.ResultExit, Exit: 0}, nil
}
