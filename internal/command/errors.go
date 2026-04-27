// Package command implements the slash command system for AI.GOOSE.
// SPEC: SPEC-GOOSE-COMMAND-001
package command

import "errors"

var (
	// ErrInvalidCommandName is returned when a command name contains characters
	// outside [a-z0-9_-] after lowercasing. REQ-CMD-003.
	ErrInvalidCommandName = errors.New("invalid command name")

	// ErrFrontmatterInvalid is returned when a custom command .md file has
	// missing required fields or unparseable YAML frontmatter.
	ErrFrontmatterInvalid = errors.New("invalid frontmatter")

	// ErrPromptTooLarge is returned when an expanded prompt exceeds
	// Config.MaxExpandedPromptBytes. REQ-CMD-014.
	ErrPromptTooLarge = errors.New("expanded prompt too large")

	// ErrUnknownCommand is returned when the registry cannot resolve a command name.
	ErrUnknownCommand = errors.New("unknown command")

	// ErrUnknownModel is returned by SlashCommandContext.ResolveModelAlias when the
	// alias does not map to any known model. REQ-CMD-008.
	ErrUnknownModel = errors.New("unknown model")

	// ErrPlanModeBlocked is returned when a mutating command is attempted while
	// plan mode is active. REQ-CMD-011.
	ErrPlanModeBlocked = errors.New("command disabled in plan mode")
)
