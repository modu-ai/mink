// Package builtin provides the built-in slash commands for AI.GOOSE.
// SPEC: SPEC-GOOSE-COMMAND-001
package builtin

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/modu-ai/goose/internal/command"
)

// helpCommand implements /help [name].
type helpCommand struct {
	lister command.Lister
}

func newHelpCommand(lister command.Lister) *helpCommand {
	return &helpCommand{lister: lister}
}

func (h *helpCommand) Name() string { return "help" }
func (h *helpCommand) Metadata() command.Metadata {
	return command.Metadata{
		Description:  "List all commands or show details for a specific command.",
		ArgumentHint: "[command-name]",
		Source:       command.SourceBuiltin,
	}
}

// Execute renders the help text for all registered commands sorted alphabetically.
// REQ-CMD-019: argument-hint is included in the output.
func (h *helpCommand) Execute(_ context.Context, _ command.Args) (command.Result, error) {
	entries := h.lister.ListNamed()
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	var sb strings.Builder
	sb.WriteString("Available commands:\n\n")

	for _, e := range entries {
		line := fmt.Sprintf("  /%-12s %s", e.Name, e.Metadata.Description)
		if e.Metadata.ArgumentHint != "" {
			line += fmt.Sprintf(" (%s)", e.Metadata.ArgumentHint)
		}
		sb.WriteString(line)
		sb.WriteByte('\n')
	}

	return command.Result{
		Kind: command.ResultLocalReply,
		Text: sb.String(),
	}, nil
}
