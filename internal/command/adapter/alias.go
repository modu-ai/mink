package adapter

import (
	"slices"
	"strings"

	"github.com/modu-ai/goose/internal/command"
	"github.com/modu-ai/goose/internal/llm/router"
)

// resolveAlias implements the alias resolution algorithm described in SPEC §6.4.
//
// Algorithm:
//  1. If registry is nil, return ErrUnknownModel.
//  2. If alias is in aliasMap, use canonical = aliasMap[alias]; otherwise canonical = alias.
//  3. Split canonical on first "/" into provider and model.
//  4. If len(parts) != 2, return ErrUnknownModel.
//  5. Look up provider in registry; if not found, return ErrUnknownModel.
//  6. Verify model is in meta.SuggestedModels (strict mode); if not, return ErrUnknownModel.
//  7. Return ModelInfo with ID = "provider/model", DisplayName = meta.DisplayName + " " + model.
func resolveAlias(
	registry *router.ProviderRegistry,
	aliasMap map[string]string,
	alias string,
) (*command.ModelInfo, error) {
	if registry == nil {
		return nil, command.ErrUnknownModel
	}

	// Step 2: alias map lookup first.
	canonical := alias
	if aliasMap != nil {
		if mapped, ok := aliasMap[alias]; ok {
			canonical = mapped
		}
	}

	// Step 3: split on first "/".
	parts := strings.SplitN(canonical, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, command.ErrUnknownModel
	}
	providerName := parts[0]
	modelName := parts[1]

	// Step 5: provider lookup.
	meta, ok := registry.Get(providerName)
	if !ok {
		return nil, command.ErrUnknownModel
	}

	// Step 6: strict model verification.
	if !slices.Contains(meta.SuggestedModels, modelName) {
		return nil, command.ErrUnknownModel
	}

	// Step 7: construct result.
	return &command.ModelInfo{
		ID:          canonical,
		DisplayName: meta.DisplayName + " " + modelName,
	}, nil
}
