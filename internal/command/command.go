// Package command implements the slash command system for AI.GOOSE.
// SPEC: SPEC-GOOSE-COMMAND-001
package command

import "context"

// Command is the abstract unit of a slash command.
// Built-in commands implement this directly; custom commands wrap a Markdown
// template body; skill-backed commands are wrapped by SKILLS-001.
type Command interface {
	// Name returns the display name of the command (may differ from canonical form).
	Name() string
	// Metadata returns descriptive and routing metadata.
	Metadata() Metadata
	// Execute runs the command with the provided arguments.
	Execute(ctx context.Context, args Args) (Result, error)
}

// Metadata describes a command for help display and routing decisions.
type Metadata struct {
	// Description is a one-line human-readable summary.
	Description string
	// ArgumentHint is shown in /help output (e.g. "plan|run|sync SPEC-ID").
	ArgumentHint string
	// AllowedTools is reserved for SKILLS-001 tool permission enforcement.
	AllowedTools []string
	// Mutates, when true, causes the dispatcher to block the command in plan mode.
	// REQ-CMD-011.
	Mutates bool
	// Source records the origin for precedence logging and shadowing warnings.
	Source Source
	// FilePath is the filesystem path for custom and skill-backed commands.
	FilePath string
}

// Args holds parsed arguments passed to Command.Execute.
type Args struct {
	// RawArgs is the trimmed argument string following the command name.
	RawArgs string
	// Positional contains whitespace-separated tokens (respecting quotes).
	Positional []string
	// Flags holds key-value pairs from --key=value or --key value tokens.
	Flags map[string]string
	// OriginalLine is the full original input line for logging or re-transmission.
	OriginalLine string
}

// ResultKind discriminates the action the dispatcher should take.
type ResultKind int

const (
	// ResultLocalReply delivers text directly to the user without an LLM call.
	ResultLocalReply ResultKind = iota
	// ResultPromptExpansion replaces the user input with an expanded prompt sent to QUERY-001.
	ResultPromptExpansion
	// ResultExit signals the CLI to terminate.
	ResultExit
	// ResultAbort signals that the command was cancelled.
	ResultAbort
)

// Result is a discriminated union returned by Command.Execute.
type Result struct {
	// Kind selects the dispatcher branch.
	Kind ResultKind
	// Text is populated for ResultLocalReply.
	Text string
	// Prompt is populated for ResultPromptExpansion.
	Prompt string
	// Exit is the process exit code for ResultExit.
	Exit int
	// Meta carries auxiliary event signals (e.g. "cleared": true).
	Meta map[string]any
}

// Lister is a read-only view of registered command metadata.
// It is implemented by *Registry and used by the /help builtin command.
//
// RegisterAlias is included here so that builtin.Register can accept a single
// registrar argument that satisfies both listing and alias registration without
// an additional interface type. Callers that only need to list commands may
// ignore RegisterAlias.
type Lister interface {
	List() []Metadata
	ListNamed() []NamedMetadata
	// RegisterAlias stores an alias mapping for use by builtin.Register.
	RegisterAlias(alias, canonical string)
}

// NamedMetadata pairs a canonical command name with its Metadata.
type NamedMetadata struct {
	// Name is the lowercased canonical command name.
	Name string
	// Metadata contains descriptive and routing information.
	Metadata Metadata
}
