package builtin

import "errors"

// Sentinel errors for BuiltinProvider.
var (
	// ErrDBPathRequired is returned when NewBuiltin is called with an empty dbPath.
	ErrDBPathRequired = errors.New("builtin: database path is required")

	// ErrNotInitialized is returned when a method is called before Initialize.
	ErrNotInitialized = errors.New("builtin: provider not initialized")
)
