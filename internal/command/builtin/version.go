// Package builtin provides the built-in slash commands for MINK.
// SPEC: SPEC-GOOSE-COMMAND-001
package builtin

import (
	"context"

	"github.com/modu-ai/mink/internal/command"
)

// Version is the current goose version string.
const Version = "0.1.0-dev"

// versionCommand implements /version.
type versionCommand struct{}

func (v *versionCommand) Name() string { return "version" }
func (v *versionCommand) Metadata() command.Metadata {
	return command.Metadata{
		Description: "Show the goose version.",
		Source:      command.SourceBuiltin,
	}
}

// Execute returns the version string as a LocalReply.
func (v *versionCommand) Execute(_ context.Context, _ command.Args) (command.Result, error) {
	return command.Result{
		Kind: command.ResultLocalReply,
		Text: "goose " + Version,
	}, nil
}
