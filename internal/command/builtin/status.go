// Package builtin provides the built-in slash commands for MINK.
// SPEC: SPEC-GOOSE-COMMAND-001
package builtin

import (
	"context"
	"fmt"

	"github.com/modu-ai/mink/internal/command"
)

// statusCommand implements /status.
type statusCommand struct{}

func (s *statusCommand) Name() string { return "status" }
func (s *statusCommand) Metadata() command.Metadata {
	return command.Metadata{
		Description: "Show the current session status.",
		Source:      command.SourceBuiltin,
	}
}

// Execute reads the session snapshot and returns a formatted LocalReply.
func (s *statusCommand) Execute(ctx context.Context, _ command.Args) (command.Result, error) {
	sctx := extractSctx(ctx)
	if sctx == nil {
		return command.Result{Kind: command.ResultLocalReply, Text: "status: no session context"}, nil
	}

	snap := sctx.SessionSnapshot()
	text := fmt.Sprintf("Session status:\n  turns: %d\n  model: %s\n  cwd:   %s\n",
		snap.TurnCount, snap.Model, snap.CWD)

	return command.Result{
		Kind: command.ResultLocalReply,
		Text: text,
	}, nil
}
