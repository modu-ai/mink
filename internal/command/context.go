// Package command implements the slash command system for AI.GOOSE.
// SPEC: SPEC-GOOSE-COMMAND-001
package command

// SlashCommandContext is the runtime hook injected by QUERY-001 into the Dispatcher.
// Commands communicate side effects (clear, model change, compaction) through this
// interface without directly coupling to the engine's internal state.
//
// @MX:ANCHOR: [AUTO] Primary integration boundary between Dispatcher and QUERY-001 engine.
// @MX:REASON: Called by every builtin command; fan_in >= 7.
// @MX:SPEC: SPEC-GOOSE-COMMAND-001 REQ-CMD-007, REQ-CMD-008, REQ-CMD-011
type SlashCommandContext interface {
	// OnClear is called when /clear requests a conversation reset.
	OnClear() error

	// OnModelChange is called when /model <alias> resolves successfully.
	OnModelChange(info ModelInfo) error

	// OnCompactRequest is called when /compact requests forced compaction.
	// target is the desired token count after compaction (0 = use default).
	OnCompactRequest(target int) error

	// ResolveModelAlias translates a user-supplied alias to a concrete model descriptor.
	// Returns ErrUnknownModel when the alias is not recognised.
	ResolveModelAlias(alias string) (*ModelInfo, error)

	// SessionSnapshot returns a read-only snapshot of the current session state
	// for use by /status.
	SessionSnapshot() SessionSnapshot

	// PlanModeActive reports whether plan mode is currently engaged.
	// REQ-CMD-011.
	PlanModeActive() bool
}

// ModelInfo describes a resolved model identity.
type ModelInfo struct {
	// ID is the canonical model identifier (e.g. "gpt-4o-2024-08-06").
	ID string
	// DisplayName is the human-readable name.
	DisplayName string
}

// SessionSnapshot is a point-in-time view of session state returned by
// SlashCommandContext.SessionSnapshot for the /status command.
type SessionSnapshot struct {
	// TurnCount is the number of completed turns in the current session.
	TurnCount int
	// Model is the active model identifier.
	Model string
	// CWD is the current working directory of the process.
	CWD string
}
