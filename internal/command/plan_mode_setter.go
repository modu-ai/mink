// Package command implements the slash command system for MINK.
// SPEC: SPEC-GOOSE-COMMAND-001
package command

// PlanModeSetter is an OPTIONAL capability that a SlashCommandContext
// implementation MAY provide. The /plan builtin uses a type assertion to
// detect support and toggles plan mode when supported.
//
// This interface is intentionally separate from SlashCommandContext to
// avoid forcing every implementation (mocks, fakes, alternative adapters)
// to implement SetPlanMode. *adapter.ContextAdapter satisfies this
// interface implicitly via its existing SetPlanMode(bool) method.
//
// @MX:NOTE: [AUTO] Narrow interface for the /plan builtin's type assertion.
// @MX:SPEC: SPEC-GOOSE-PLANMODE-CMD-001 REQ-PMC-001
type PlanModeSetter interface {
	SetPlanMode(active bool)
}
