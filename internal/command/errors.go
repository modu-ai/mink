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

	// Sentinel errors for alias configuration (SPEC-GOOSE-ALIAS-CONFIG-001).

	// ErrMalformedAliasFile is returned when YAML parsing of the alias file fails.
	// Replaces generic ErrInvalidFormat for alias-specific errors.
	ErrMalformedAliasFile = errors.New("aliasconfig: malformed alias file")

	// ErrEmptyAliasEntry is returned when an alias key or canonical value is empty.
	// Empty key: ("": "anthropic/claude-opus-4-7")
	// Empty value: ("opus": "")
	ErrEmptyAliasEntry = errors.New("aliasconfig: empty alias key or value")

	// ErrInvalidCanonical is returned when the canonical value is not in "provider/model" format.
	// Cases: "claudeopus47", "/claude", "anthropic/"
	ErrInvalidCanonical = errors.New("aliasconfig: canonical must be provider/model")

	// ErrUnknownProviderInAlias is returned in strict mode when the provider in the
	// canonical value is not registered in the ProviderRegistry.
	ErrUnknownProviderInAlias = errors.New("aliasconfig: unknown provider in alias canonical")

	// ErrUnknownModelInAlias is returned in strict mode when the model in the
	// canonical value is not in the provider's SuggestedModels list.
	ErrUnknownModelInAlias = errors.New("aliasconfig: unknown model in alias canonical")

	// ErrAliasFileTooLarge is returned when the alias file exceeds 1 MiB (1,048,576 bytes).
	// The file is rejected without parsing.
	ErrAliasFileTooLarge = errors.New("aliasconfig: alias file exceeds 1 MiB cap")
)
