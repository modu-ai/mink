package aliasconfig

import (
	"fmt"
	"slices"
	"strings"

	"github.com/modu-ai/goose/internal/llm/router"
)

// Validate checks alias entries against a provider registry.
//
// In strict mode (strict=true), it returns errors for unknown providers or models.
// REQ-ALIAS-023, REQ-ALIAS-034, REQ-ALIAS-035.
//
// In lenient mode (strict=false), it removes invalid entries from the map in-place
// and returns warning errors for each removed entry. REQ-ALIAS-024.
//
// Empty maps always return an empty error slice. REQ-ALIAS-004.
func Validate(aliases map[string]string, registry *router.ProviderRegistry, strict bool) []error {
	var errs []error

	// Empty map returns empty error slice
	if len(aliases) == 0 {
		return errs
	}

	// Collect invalid keys for removal in lenient mode
	var invalidKeys []string

	for alias, canonical := range aliases {
		// Split canonical into provider and model
		parts := strings.SplitN(canonical, "/", 2)
		if len(parts) != 2 {
			// This should have been caught by validateAliases during Load,
			// but we check again for safety.
			if strict {
				errs = append(errs, fmt.Errorf("%w: alias %q has invalid canonical %q", ErrInvalidCanonical, alias, canonical))
			} else {
				invalidKeys = append(invalidKeys, alias)
				errs = append(errs, fmt.Errorf("warning: removing alias %q with invalid canonical %q", alias, canonical))
			}
			continue
		}

		providerName := parts[0]
		modelName := parts[1]

		// Check if provider exists
		meta, ok := registry.Get(providerName)
		if !ok {
			if strict {
				errs = append(errs, fmt.Errorf("%w: alias %q references unknown provider %q", ErrUnknownProviderInAlias, alias, providerName))
			} else {
				invalidKeys = append(invalidKeys, alias)
				errs = append(errs, fmt.Errorf("warning: removing alias %q with unknown provider %q", alias, providerName))
			}
			continue
		}

		// Check if model is in SuggestedModels
		if !slices.Contains(meta.SuggestedModels, modelName) {
			if strict {
				errs = append(errs, fmt.Errorf("%w: alias %q references unknown model %q for provider %q", ErrUnknownModelInAlias, alias, modelName, providerName))
			} else {
				invalidKeys = append(invalidKeys, alias)
				errs = append(errs, fmt.Errorf("warning: removing alias %q with unknown model %q for provider %q", alias, modelName, providerName))
			}
			continue
		}
	}

	// Remove invalid entries in lenient mode
	if !strict && len(invalidKeys) > 0 {
		for _, key := range invalidKeys {
			delete(aliases, key)
		}
	}

	return errs
}
