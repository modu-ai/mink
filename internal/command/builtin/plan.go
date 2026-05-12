// Package builtin provides the built-in slash commands for AI.GOOSE.
// SPEC: SPEC-GOOSE-COMMAND-001
package builtin

import (
	"context"
	"fmt"
	"strings"

	"github.com/modu-ai/mink/internal/command"
)

// Compile-time assertion: planCommand implements command.Command.
// REQ-PMC-003, AC-PMC-003.
var _ command.Command = (*planCommand)(nil)

// planCommand implements /plan [on|off|toggle|status].
//
// @MX:NOTE: [AUTO] The /plan builtin toggles ContextAdapter.SetPlanMode via
// type assertion on command.PlanModeSetter. Mutates=false: the /plan command
// itself must remain reachable while plan mode is active to allow the user
// to disable plan mode.
// @MX:SPEC: SPEC-GOOSE-PLANMODE-CMD-001 REQ-PMC-003, REQ-PMC-004, REQ-PMC-012
type planCommand struct{}

func (p *planCommand) Name() string {
	return "plan"
}

func (p *planCommand) Metadata() command.Metadata {
	return command.Metadata{
		Description:  "Toggle plan mode (read-only mode for inspection).",
		ArgumentHint: "[on|off|toggle|status]",
		Mutates:      false, // REQ-PMC-004: must NOT be Mutates=true (deadlock risk).
		Source:       command.SourceBuiltin,
	}
}

// Execute parses the first positional argument and toggles plan mode via
// the PlanModeSetter type assertion on the injected SlashCommandContext.
// REQ-PMC-007 ~ REQ-PMC-011, REQ-PMC-015 ~ REQ-PMC-018.
func (p *planCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	sctx := extractSctx(ctx)
	if sctx == nil {
		// REQ-PMC-017: nil sctx graceful.
		return command.Result{
			Kind: command.ResultLocalReply,
			Text: "plan: context unavailable",
		}, nil
	}

	sub := ""
	if len(args.Positional) > 0 {
		sub = strings.ToLower(args.Positional[0])
	}

	// REQ-PMC-011: more than one positional argument is invalid.
	if len(args.Positional) > 1 {
		return usageReply(), nil
	}

	switch sub {
	case "", "status":
		// REQ-PMC-010: read-only status query.
		return statusReply(sctx.PlanModeActive()), nil

	case "on":
		return p.applySet(sctx, true)

	case "off":
		return p.applySet(sctx, false)

	case "toggle":
		return p.applySet(sctx, !sctx.PlanModeActive())

	default:
		// REQ-PMC-011: unknown sub-command.
		return usageReply(), nil
	}
}

// applySet performs the type-asserted SetPlanMode call.
// Returns a graceful LocalReply when the sctx does not implement PlanModeSetter.
// REQ-PMC-018.
func (p *planCommand) applySet(sctx command.SlashCommandContext, target bool) (command.Result, error) {
	setter, ok := sctx.(command.PlanModeSetter)
	if !ok {
		return command.Result{
			Kind: command.ResultLocalReply,
			Text: "plan: this session does not support plan mode toggling",
		}, nil
	}

	current := sctx.PlanModeActive()
	setter.SetPlanMode(target)

	var text string
	switch {
	case target && current:
		text = "plan mode: on (already active)"
	case target && !current:
		text = "plan mode: on"
	case !target && !current:
		text = "plan mode: off (already inactive)"
	case !target && current:
		text = "plan mode: off"
	}

	return command.Result{
		Kind: command.ResultLocalReply,
		Text: text,
	}, nil
}

func statusReply(active bool) command.Result {
	state := "off"
	if active {
		state = "on"
	}
	return command.Result{
		Kind: command.ResultLocalReply,
		Text: fmt.Sprintf("plan mode: %s", state),
	}
}

func usageReply() command.Result {
	return command.Result{
		Kind: command.ResultLocalReply,
		Text: "usage: /plan [on|off|toggle|status]",
	}
}
