package memory

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

// providerNameRegex is the compiled regex for validating provider names.
// Pattern: ^[a-z][a-z0-9_-]{0,31}$
var providerNameRegex = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)

// TestProviderNameRegex_ValidInvalid tests the provider name validation regex.
func TestProviderNameRegex_ValidInvalid(t *testing.T) {
	validNames := []string{
		"a",
		"builtin",
		"my-plugin",
		"provider_2",
		"a-b-c",
		"x_y_z",
		"abc123",
		"a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6", // 31 chars after first
	}

	invalidNames := []string{
		"",
		"A",      // uppercase
		"1start", // starts with number
		"-start", // starts with hyphen
		"_start", // starts with underscore
		"has space",
		"has.dot",
		"has@symbol",
		"UPPERCASE",
		"aBc",                               // mixed case
		"a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q", // 32 chars after first (total 33)
	}

	for _, name := range validNames {
		t.Run("valid_"+name, func(t *testing.T) {
			assert.True(t, providerNameRegex.MatchString(name), "name should be valid: %s", name)
		})
	}

	for _, name := range invalidNames {
		t.Run("invalid_"+name, func(t *testing.T) {
			assert.False(t, providerNameRegex.MatchString(name), "name should be invalid: %s", name)
		})
	}
}
