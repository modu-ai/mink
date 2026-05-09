//go:build nokeyring

package telegram

import "errors"

// OSKeyring is a stub Keyring used when the nokeyring build tag is set.
// This is intended for CI environments where the OS secret service (dbus) is
// unavailable, and for test builds that inject MemoryKeyring instead.
//
// Calling any method on this stub returns an error instructing the caller to
// use MemoryKeyring or provide the token via an environment variable.
type OSKeyring struct{}

// NewOSKeyring constructs the stub OSKeyring.
func NewOSKeyring() *OSKeyring { return &OSKeyring{} }

// Store always returns an error in nokeyring builds.
func (OSKeyring) Store(_, _ string, _ []byte) error {
	return errors.New("OSKeyring: disabled at build time (-tags=nokeyring); use MemoryKeyring or GOOSE_TELEGRAM_BOT_TOKEN env var")
}

// Retrieve always returns an error in nokeyring builds.
func (OSKeyring) Retrieve(_, _ string) ([]byte, error) {
	return nil, errors.New("OSKeyring: disabled at build time (-tags=nokeyring); use MemoryKeyring or GOOSE_TELEGRAM_BOT_TOKEN env var")
}
