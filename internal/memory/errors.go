package memory

import "errors"

// Sentinel errors for memory provider system.
// Use errors.Is() to check for these errors.

var (
	// ErrBuiltinRequired indicates that RegisterBuiltin was not called before other operations.
	ErrBuiltinRequired = errors.New("memory: BuiltinProvider must be registered first")

	// ErrOnlyOnePluginAllowed indicates that more than one plugin was attempted to be registered.
	ErrOnlyOnePluginAllowed = errors.New("memory: at most one external plugin allowed")

	// ErrNameCollision indicates that a provider with the same name (case-insensitive) is already registered.
	ErrNameCollision = errors.New("memory: provider name collides (case-insensitive)")

	// ErrToolNameCollision indicates that a tool name is already registered by another provider.
	ErrToolNameCollision = errors.New("memory: tool name collides between providers")

	// ErrInvalidProviderName indicates that the provider name does not match the required pattern.
	ErrInvalidProviderName = errors.New("memory: provider name must match ^[a-z][a-z0-9_-]{0,31}$")

	// ErrUserMdReadOnly indicates that a write operation was attempted on USER.md (read-only).
	ErrUserMdReadOnly = errors.New("memory: USER.md is read-only")

	// ErrProviderNotInit indicates that the provider was not initialized for the session.
	ErrProviderNotInit = errors.New("memory: provider not initialized for this session")

	// ErrUnknownPlugin indicates that the plugin name is not registered in the factory registry.
	ErrUnknownPlugin = errors.New("memory: plugin name not registered in factory registry")
)

// IsErrUnknownPlugin reports whether err is ErrUnknownPlugin.
func IsErrUnknownPlugin(err error) bool {
	return errors.Is(err, ErrUnknownPlugin)
}
