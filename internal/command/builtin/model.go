// Package builtin provides the built-in slash commands for MINK.
// SPEC: SPEC-GOOSE-COMMAND-001
package builtin

import (
	"context"
	"fmt"

	"github.com/modu-ai/mink/internal/command"
)

// modelCommand implements /model <alias>.
type modelCommand struct{}

func (m *modelCommand) Name() string { return "model" }
func (m *modelCommand) Metadata() command.Metadata {
	return command.Metadata{
		Description:  "Switch the active model.",
		ArgumentHint: "<alias>",
		Source:       command.SourceBuiltin,
	}
}

// Execute resolves the alias and calls OnModelChange. REQ-CMD-008.
func (m *modelCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	if len(args.Positional) == 0 {
		return command.Result{
			Kind: command.ResultLocalReply,
			Text: "usage: /model <alias>",
		}, nil
	}
	alias := args.Positional[0]

	sctx := extractSctx(ctx)
	if sctx == nil {
		return command.Result{Kind: command.ResultLocalReply, Text: "context unavailable"}, nil
	}

	info, err := sctx.ResolveModelAlias(alias)
	if err != nil {
		return command.Result{
			Kind: command.ResultLocalReply,
			Text: fmt.Sprintf("unknown model: %s", alias),
		}, nil
	}

	if err := sctx.OnModelChange(*info); err != nil {
		return command.Result{}, fmt.Errorf("OnModelChange: %w", err)
	}

	return command.Result{
		Kind: command.ResultLocalReply,
		Text: fmt.Sprintf("model set to %s", info.ID),
	}, nil
}
