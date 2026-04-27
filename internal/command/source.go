// Package command implements the slash command system for AI.GOOSE.
// SPEC: SPEC-GOOSE-COMMAND-001
package command

// Source identifies the origin of a registered command.
// Lower numeric value means higher resolution priority.
type Source int

const (
	// SourceBuiltin is the highest priority: built-in commands compiled into the binary.
	SourceBuiltin Source = iota
	// SourceCustomProject is project-scoped custom commands from .goose/commands/.
	SourceCustomProject
	// SourceCustomUser is user-scoped custom commands from ~/.goose/commands/.
	SourceCustomUser
	// SourceSkill is the lowest priority: skill-backed commands via RegisterProvider.
	SourceSkill
)
