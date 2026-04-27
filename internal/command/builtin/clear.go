// Package builtin provides the built-in slash commands for AI.GOOSE.
// SPEC: SPEC-GOOSE-COMMAND-001
package builtin

import (
	"context"
	"fmt"

	"github.com/modu-ai/goose/internal/command"
)

// clearCommand implements /clear.
type clearCommand struct{}

func (c *clearCommand) Name() string { return "clear" }
func (c *clearCommand) Metadata() command.Metadata {
	return command.Metadata{
		Description: "Clear the current conversation history.",
		Source:      command.SourceBuiltin,
	}
}

// Execute invokes sctx.OnClear and returns a LocalReply. REQ-CMD-007.
func (c *clearCommand) Execute(ctx context.Context, _ command.Args) (command.Result, error) {
	sctx := extractSctx(ctx)
	if sctx != nil {
		if err := sctx.OnClear(); err != nil {
			return command.Result{}, fmt.Errorf("OnClear: %w", err)
		}
	}
	return command.Result{
		Kind: command.ResultLocalReply,
		Text: "conversation cleared",
	}, nil
}
