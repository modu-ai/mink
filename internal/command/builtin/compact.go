// Package builtin provides the built-in slash commands for AI.GOOSE.
// SPEC: SPEC-GOOSE-COMMAND-001
package builtin

import (
	"context"
	"fmt"
	"strconv"

	"github.com/modu-ai/goose/internal/command"
)

// compactCommand implements /compact [target_tokens].
type compactCommand struct{}

func (c *compactCommand) Name() string { return "compact" }
func (c *compactCommand) Metadata() command.Metadata {
	return command.Metadata{
		Description:  "Request forced context compaction.",
		ArgumentHint: "[target-tokens]",
		Source:       command.SourceBuiltin,
	}
}

// Execute calls sctx.OnCompactRequest with the optional target token count.
func (c *compactCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	target := 0
	if len(args.Positional) > 0 {
		n, err := strconv.Atoi(args.Positional[0])
		if err == nil {
			target = n
		}
	}

	sctx := extractSctx(ctx)
	if sctx != nil {
		if err := sctx.OnCompactRequest(target); err != nil {
			return command.Result{}, fmt.Errorf("OnCompactRequest: %w", err)
		}
	}

	msg := "compaction requested"
	if target > 0 {
		msg = fmt.Sprintf("compaction requested (target=%d)", target)
	}
	return command.Result{
		Kind: command.ResultLocalReply,
		Text: msg,
	}, nil
}
