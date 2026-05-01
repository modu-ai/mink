// Package aliasconfig — MergePolicy enum, ValidationPolicy, and ValidateAndPrune.
// SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001 REQ-AMEND-003, REQ-AMEND-020
package aliasconfig

import (
	"github.com/modu-ai/goose/internal/command"
	"github.com/modu-ai/goose/internal/llm/router"
)

// MergePolicy controls how user-file and project-file entries are combined
// in LoadDefault when both files are present.
//
// SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001 REQ-AMEND-020
type MergePolicy int

const (
	// MergePolicyProjectOverride parses both files; project entries override user entries on conflict.
	// This is the zero value and the default policy.
	MergePolicyProjectOverride MergePolicy = 0

	// MergePolicyUserOnly parses only the user file; the project file is ignored even when present.
	MergePolicyUserOnly MergePolicy = 1

	// MergePolicyProjectOnly parses only the project file (when present); the user file is ignored.
	MergePolicyProjectOnly MergePolicy = 2
)

// ValidationPolicy controls how ValidateAndPrune processes alias entries.
//
// SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001 REQ-AMEND-003
type ValidationPolicy struct {
	// Strict enables provider/model existence checks against the registry.
	// Requires a non-nil registry to have any effect.
	Strict bool

	// SkipOnError continues validating remaining entries after each error.
	// Always true in v0.1 — reserved for a future halt-on-first-error mode.
	SkipOnError bool

	// MaxErrors caps the returned error slice. 0 means unlimited.
	MaxErrors int

	// AggregateAsJoin returns a single errors.Join result when true.
	// Default (false) returns per-entry errors.
	AggregateAsJoin bool
}

// ValidateAndPrune validates each entry in m against the registry and returns
// a new map containing only the entries that passed validation, along with
// a slice of per-entry validation errors.
//
// The input map m is NOT mutated (REQ-AMEND-003).
// For the legacy variant that returns errors but does not prune, use Validate.
//
// SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001 REQ-AMEND-003
func ValidateAndPrune(
	m map[string]string,
	registry *router.ProviderRegistry,
	policy ValidationPolicy,
) (cleaned map[string]string, errs []error) {
	if m == nil {
		return map[string]string{}, nil
	}

	cleaned = make(map[string]string, len(m))
	for alias, canonical := range m {
		// Stop collecting errors when MaxErrors limit is reached.
		if policy.MaxErrors > 0 && len(errs) >= policy.MaxErrors {
			break
		}
		err := validateEntry(alias, canonical, registry, policy.Strict)
		if err != nil {
			errs = append(errs, err)
			// Do NOT add invalid entry to cleaned map.
			continue
		}
		cleaned[alias] = canonical
	}

	if policy.AggregateAsJoin && len(errs) > 0 {
		joined := errs[0]
		for i := 1; i < len(errs); i++ {
			joined = &joinError{joined, errs[i]}
		}
		return cleaned, []error{joined}
	}

	return cleaned, errs
}

// validateEntry validates a single alias → canonical mapping.
// Returns nil when the entry is valid.
func validateEntry(alias, canonical string, registry *router.ProviderRegistry, strict bool) error {
	if alias == "" || canonical == "" {
		return command.ErrEmptyAliasEntry
	}

	provider, model, ok := parseModelTarget(canonical)
	if !ok {
		return command.ErrInvalidCanonical
	}

	if strict && registry != nil {
		meta, exists := registry.Get(provider)
		if !exists {
			return command.ErrUnknownProviderInAlias
		}
		found := false
		for _, s := range meta.SuggestedModels {
			if s == model {
				found = true
				break
			}
		}
		if !found {
			return command.ErrUnknownModelInAlias
		}
	}

	return nil
}

// joinError is a minimal two-element error join used to avoid importing errors.Join
// (which requires Go 1.20+). This keeps the package compatible with Go 1.19+.
type joinError struct {
	lhs, rhs error
}

func (j *joinError) Error() string {
	if j.lhs == nil {
		return j.rhs.Error()
	}
	if j.rhs == nil {
		return j.lhs.Error()
	}
	return j.lhs.Error() + "; " + j.rhs.Error()
}

func (j *joinError) Unwrap() []error {
	return []error{j.lhs, j.rhs}
}
