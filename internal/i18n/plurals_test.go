package i18n

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateCLDRCategory_Valid(t *testing.T) {
	t.Parallel()
	for _, cat := range []CLDRCategory{CLDRZero, CLDROne, CLDRTwo, CLDRFew, CLDRMany, CLDROther} {
		assert.NoError(t, ValidateCLDRCategory(cat), "expected valid: %s", cat)
	}
}

func TestValidateCLDRCategory_Invalid(t *testing.T) {
	t.Parallel()
	for _, cat := range []CLDRCategory{"six", "alot", "", "ONE"} {
		err := ValidateCLDRCategory(cat)
		require.Error(t, err, "expected invalid: %s", cat)
		assert.ErrorIs(t, err, ErrInvalidPluralRule)
	}
}

func TestPluralFormsForLang_English(t *testing.T) {
	t.Parallel()
	forms := PluralFormsForLang("en")
	assert.Contains(t, forms, CLDROne)
	assert.Contains(t, forms, CLDROther)
	assert.NotContains(t, forms, CLDRZero)
}

func TestPluralFormsForLang_Korean(t *testing.T) {
	t.Parallel()
	forms := PluralFormsForLang("ko")
	assert.Equal(t, []CLDRCategory{CLDROther}, forms)
}

func TestPluralFormsForLang_Arabic(t *testing.T) {
	t.Parallel()
	forms := PluralFormsForLang("ar")
	// Arabic has all 6 CLDR categories.
	assert.Contains(t, forms, CLDRZero)
	assert.Contains(t, forms, CLDROne)
	assert.Contains(t, forms, CLDRTwo)
	assert.Contains(t, forms, CLDRFew)
	assert.Contains(t, forms, CLDRMany)
	assert.Contains(t, forms, CLDROther)
}

func TestPluralFormsForLang_Unknown(t *testing.T) {
	t.Parallel()
	// Unknown languages fall back to "other" only.
	forms := PluralFormsForLang("xz-ZZ")
	assert.Equal(t, []CLDRCategory{CLDROther}, forms)
}

// TestPluralizer_Russian_FourForms verifies that the go-i18n/v2 engine correctly
// selects plural forms for Russian using a live Bundle.
// Russian uses: one (21→31 remainder 1), few (2-4, 22-24 ...), many (5-20, ...), other.
func TestPluralizer_Russian_FourForms(t *testing.T) {
	t.Parallel()

	ruYAML := `
messages:
  description: "message count"
  zero: "нет сообщений"
  one: "{{.Count}} сообщение"
  few: "{{.Count}} сообщения"
  many: "{{.Count}} сообщений"
  other: "{{.Count}} сообщений"
`
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ru.yaml"), []byte(ruYAML), 0o600))

	b := NewBundle("en")
	require.NoError(t, b.LoadDirectory(dir))
	tr := b.Translator("ru")

	// count=21 → one form (remainder 1 after 20).
	assert.Equal(t, "21 сообщение", tr.TranslatePlural("messages", 21, nil))
	// count=23 → few form (remainder 3).
	assert.Equal(t, "23 сообщения", tr.TranslatePlural("messages", 23, nil))
	// count=25 → many form (remainder 5).
	assert.Equal(t, "25 сообщений", tr.TranslatePlural("messages", 25, nil))
}

// TestPluralizer_English_OneOther verifies English one/other selection.
func TestPluralizer_English_OneOther(t *testing.T) {
	t.Parallel()
	b := newTestBundle(t)
	tr := b.Translator("en")

	assert.Equal(t, "1 step remaining", tr.TranslatePlural("install.step_count", 1, nil))
	assert.Equal(t, "5 steps remaining", tr.TranslatePlural("install.step_count", 5, nil))
	assert.Equal(t, "0 steps remaining", tr.TranslatePlural("install.step_count", 0, nil))
}
