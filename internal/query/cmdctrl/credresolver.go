// Package cmdctrl provides credential pool resolution support for the
// LoopController implementation.
//
// SPEC: SPEC-GOOSE-CMDCTX-CREDPOOL-WIRE-001
package cmdctrl

import (
	"errors"
	"strings"

	"github.com/modu-ai/goose/internal/llm/credential"
)

// Sentinel errors
var (
	// ErrCredentialUnavailable is returned when a credential pool cannot be
	// obtained for the requested provider during RequestModelChange.
	// AC-CCWIRE-004: Sentinel error compatible with errors.Is.
	// AC-CCWIRE-008: Returned when provider has no pool.
	// AC-CCWIRE-010: Returned when pool has zero available credentials.
	ErrCredentialUnavailable = errors.New("credential unavailable for provider")
)

// CredentialPoolResolver is an abstraction for mapping provider identifiers
// to their associated credential pools.
//
// REQ-CCWIRE-001: Interface for pool resolution by provider string.
// AC-CCWIRE-001: Returns nil when provider has no pool (no error).
// AC-CCWIRE-002: Nil resolver means no credential validation (backward compatible).
//
// @MX:ANCHOR: [AUTO] Provider-to-pool mapping abstraction.
// @MX:REASON: Dependency injection point for credential pool wiring; fan_in >= 2 (LoopControllerImpl + tests).
// @MX:SPEC: SPEC-GOOSE-CMDCTX-CREDPOOL-WIRE-001 REQ-CCWIRE-001
type CredentialPoolResolver interface {
	// PoolFor returns the credential pool associated with the given provider
	// identifier. Returns nil if the provider has no configured pool.
	//
	// AC-CCWIRE-001: Nil return means provider has no pool.
	// AC-CCWIRE-008: Nil pool triggers ErrCredentialUnavailable.
	PoolFor(provider string) *credential.CredentialPool
}

// extractProvider extracts the provider identifier from a model ID.
//
// Model IDs are expected to be in the format "provider/model" (e.g., "openai/gpt-4").
// If no slash is present or the input is empty, returns an empty string.
//
// REQ-CCWIRE-005: Provider extraction via strings.SplitN.
// AC-CCWIRE-003: Returns first component before "/" slash.
//
// @MX:NOTE: [AUTO] Provider extraction from model ID - simple string split.
// @MX:SPEC: SPEC-GOOSE-CMDCTX-CREDPOOL-WIRE-001 REQ-CCWIRE-005
func extractProvider(modelID string) string {
	if modelID == "" {
		return ""
	}
	parts := strings.SplitN(modelID, "/", 2)
	if len(parts) < 2 {
		return ""
	}
	return parts[0]
}

// Option is a functional option pattern for configuring LoopControllerImpl.
//
// REQ-CCWIRE-002: Option pattern for backward-compatible constructor extension.
// AC-CCWIRE-002: Existing callers with 2 args continue to work.
//
// @MX:NOTE: [AUTO] Functional option for LoopController configuration.
// @MX:SPEC: SPEC-GOOSE-CMDCTX-CREDPOOL-WIRE-001 REQ-CCWIRE-002
type Option func(*LoopControllerImpl)

// WithCredentialPoolResolver sets the credential pool resolver for the controller.
//
// REQ-CCWIRE-002: Option for dependency injection.
// AC-CCWIRE-002: Nil resolver disables credential validation.
// AC-CCWIRE-005: Validation only occurs when resolver != nil.
//
// @MX:ANCHOR: [AUTO] Credential pool resolver injection.
// @MX:REASON: Public factory option; fan_in >= 3 (CLI wiring, DAEMON wiring, tests).
// @MX:SPEC: SPEC-GOOSE-CMDCTX-CREDPOOL-WIRE-001 REQ-CCWIRE-002
func WithCredentialPoolResolver(r CredentialPoolResolver) Option {
	return func(c *LoopControllerImpl) {
		c.credResolver = r
	}
}

// WithPreWarmRefresh enables or disables pre-warm refresh after model changes.
//
// REQ-CCWIRE-013: Async pre-warm after successful swap.
// REQ-CCWIRE-018: Optional feature controlled by boolean flag.
// AC-CCWIRE-021: Best-effort refresh; errors never propagated.
//
// @MX:NOTE: [AUTO] Pre-warm refresh configuration option.
// @MX:SPEC: SPEC-GOOSE-CMDCTX-CREDPOOL-WIRE-001 REQ-CCWIRE-013/018
func WithPreWarmRefresh(enabled bool) Option {
	return func(c *LoopControllerImpl) {
		c.preWarmRefresh = enabled
	}
}
