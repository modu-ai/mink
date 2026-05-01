// Package aliasconfig — stable ErrorCode identifiers and error categorization.
// SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001 REQ-AMEND-004, REQ-AMEND-011
package aliasconfig

import (
	"errors"

	"github.com/modu-ai/goose/internal/command"
)

// ErrorCategory enumerates the high-level classification of aliasconfig errors.
// Used by HOTRELOAD-001 §6.9 watcher log classification.
//
// SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001 REQ-AMEND-011
type ErrorCategory int

const (
	// CategoryUnknown is returned for errors that do not match any known sentinel.
	CategoryUnknown ErrorCategory = 0
	// CategoryLoad covers file-not-found and I/O errors.
	CategoryLoad ErrorCategory = 1
	// CategoryParse covers YAML malformed errors.
	CategoryParse ErrorCategory = 2
	// CategoryPermission covers OS permission denied errors.
	CategoryPermission ErrorCategory = 3
	// CategorySize covers file-exceeds-size-limit errors.
	CategorySize ErrorCategory = 4
	// CategoryFormat covers empty-entry and invalid-canonical errors.
	CategoryFormat ErrorCategory = 5
	// CategoryRegistry covers unknown-provider and unknown-model errors from strict validation.
	CategoryRegistry ErrorCategory = 6
)

// errCodeEntry maps a sentinel error to its stable code and category.
type errCodeEntry struct {
	sentinel error
	code     string
	category ErrorCategory
}

// sentinelTable is the closed enumeration of known sentinel → (code, category) mappings.
// Order matters: more specific entries are checked first.
var sentinelTable = []errCodeEntry{
	{ErrConfigNotFound, "ALIAS-001", CategoryLoad},
	{command.ErrEmptyAliasEntry, "ALIAS-010", CategoryFormat},
	{command.ErrInvalidCanonical, "ALIAS-011", CategoryFormat},
	{command.ErrAliasFileTooLarge, "ALIAS-020", CategorySize},
	{command.ErrMalformedAliasFile, "ALIAS-021", CategoryParse},
	{command.ErrUnknownProviderInAlias, "ALIAS-030", CategoryRegistry},
	{command.ErrUnknownModelInAlias, "ALIAS-031", CategoryRegistry},
}

// ErrorCodeOf returns the stable identifier (e.g., "ALIAS-001") for the
// wrapped sentinel in err. Returns empty string for nil or unrecognized errors.
//
// SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001 REQ-AMEND-004
func ErrorCodeOf(err error) string {
	if err == nil {
		return ""
	}
	for _, e := range sentinelTable {
		if errors.Is(err, e.sentinel) {
			return e.code
		}
	}
	return ""
}

// Categorize maps a wrapped sentinel chain to a high-level ErrorCategory.
// Returns CategoryUnknown for nil or non-sentinel errors.
//
// SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001 REQ-AMEND-011
func Categorize(err error) ErrorCategory {
	if err == nil {
		return CategoryUnknown
	}
	for _, e := range sentinelTable {
		if errors.Is(err, e.sentinel) {
			return e.category
		}
	}
	return CategoryUnknown
}
