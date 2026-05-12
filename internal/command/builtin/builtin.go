// Package builtin provides the built-in slash commands for AI.GOOSE.
// SPEC: SPEC-GOOSE-COMMAND-001
package builtin

import (
	"context"

	"github.com/modu-ai/mink/internal/command"
)

// registrar is the minimal interface required by Register.
// It is satisfied by *command.Registry.
type registrar interface {
	Register(c command.Command, src command.Source) error
	command.Lister
}

// extractSctx retrieves the SlashCommandContext injected by the Dispatcher.
func extractSctx(ctx context.Context) command.SlashCommandContext {
	val := ctx.Value(command.SctxContextKey())
	if val == nil {
		return nil
	}
	sctx, _ := val.(command.SlashCommandContext)
	return sctx
}

// Register adds all builtin commands and their aliases to the provided registry.
//
// Commands registered (8):
//   - /help   (alias: /?)
//   - /clear
//   - /exit   (alias: /quit)
//   - /model
//   - /compact
//   - /status
//   - /version
//   - /plan   (alias: /planmode)
//
// @MX:ANCHOR: [AUTO] Single registration point for all builtin commands.
// @MX:REASON: Fan-in >= 3: application startup, integration tests, builtin tests.
// @MX:SPEC: SPEC-GOOSE-COMMAND-001 §6.5
func Register(reg registrar) {
	mustRegister := func(c command.Command) {
		if err := reg.Register(c, command.SourceBuiltin); err != nil {
			panic("builtin registration failed: " + err.Error())
		}
	}

	mustRegister(newHelpCommand(reg))
	mustRegister(&clearCommand{})
	mustRegister(&exitCommand{})
	mustRegister(&modelCommand{})
	mustRegister(&compactCommand{})
	mustRegister(&statusCommand{})
	mustRegister(&versionCommand{})
	mustRegister(&planCommand{}) // REQ-PMC-006

	// Register aliases for the builtin command set.
	// These bypass strict name validation since "?" is a special-case allowed alias.
	reg.RegisterAlias("quit", "exit")
	reg.RegisterAlias("?", "help")
	reg.RegisterAlias("planmode", "plan") // REQ-PMC-006
}
