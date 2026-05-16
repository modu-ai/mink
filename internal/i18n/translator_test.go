package i18n

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestBundle creates a Bundle loaded from the embedded catalog (en + ko).
func newTestBundle(t *testing.T) *Bundle {
	t.Helper()
	b := NewBundle("en")
	require.NoError(t, b.LoadFS(catalogFS, "catalog"))
	return b
}

func TestTranslator_SimpleKey_English(t *testing.T) {
	t.Parallel()
	b := newTestBundle(t)
	tr := b.Translator("en")
	assert.Equal(t, "Welcome to MINK", tr.Translate("install.welcome", nil))
}

func TestTranslator_SimpleKey_Korean(t *testing.T) {
	t.Parallel()
	b := newTestBundle(t)
	tr := b.Translator("ko")
	assert.Equal(t, "MINK에 오신 것을 환영합니다", tr.Translate("install.welcome", nil))
}

func TestTranslator_ParamSubstitution(t *testing.T) {
	t.Parallel()
	b := newTestBundle(t)
	tr := b.Translator("en")
	result := tr.Translate("install.greeting", map[string]any{"Name": "Alice"})
	assert.Equal(t, "Hello, Alice!", result)
}

func TestTranslator_ParamSubstitution_Korean(t *testing.T) {
	t.Parallel()
	b := newTestBundle(t)
	tr := b.Translator("ko")
	result := tr.Translate("install.greeting", map[string]any{"Name": "민수"})
	assert.Equal(t, "안녕하세요, 민수님!", result)
}

func TestTranslator_MissingKey_FallsBackToKey(t *testing.T) {
	t.Parallel()
	b := newTestBundle(t)
	tr := b.Translator("en")
	assert.Equal(t, "no.such.key", tr.Translate("no.such.key", nil))
}

func TestTranslator_MissingKey_NeverReturnsEmpty(t *testing.T) {
	t.Parallel()
	b := newTestBundle(t)
	tr := b.Translator("en")
	result := tr.Translate("definitely.missing", nil)
	assert.NotEmpty(t, result)
}

func TestTranslator_ZeroParams(t *testing.T) {
	t.Parallel()
	b := newTestBundle(t)
	tr := b.Translator("en")
	// Passing nil params must not panic.
	assert.NotPanics(t, func() {
		_ = tr.Translate("install.welcome", nil)
	})
}

func TestTranslator_EnglishFallback_WhenLangMissing(t *testing.T) {
	t.Parallel()
	b := newTestBundle(t)
	// "fr" has no catalog — must fall back to "en".
	tr := b.Translator("fr")
	assert.Equal(t, "Welcome to MINK", tr.Translate("install.welcome", nil))
}

func TestTranslator_Lang(t *testing.T) {
	t.Parallel()
	b := newTestBundle(t)
	tr := b.Translator("ko")
	assert.Equal(t, "ko", tr.Lang())
}

func TestTranslator_Plural_English_One(t *testing.T) {
	t.Parallel()
	b := newTestBundle(t)
	tr := b.Translator("en")
	result := tr.TranslatePlural("install.step_count", 1, nil)
	assert.Equal(t, "1 step remaining", result)
}

func TestTranslator_Plural_English_Other(t *testing.T) {
	t.Parallel()
	b := newTestBundle(t)
	tr := b.Translator("en")
	result := tr.TranslatePlural("install.step_count", 5, nil)
	assert.Equal(t, "5 steps remaining", result)
}

func TestTranslator_Plural_Korean_OtherOnly(t *testing.T) {
	t.Parallel()
	b := newTestBundle(t)
	tr := b.Translator("ko")
	// Korean has no singular/plural distinction — "other" applies to all counts.
	result1 := tr.TranslatePlural("install.step_count", 1, nil)
	result5 := tr.TranslatePlural("install.step_count", 5, nil)
	assert.Contains(t, result1, "1")
	assert.Contains(t, result5, "5")
}

func TestTranslator_Plural_CountInParams(t *testing.T) {
	t.Parallel()
	b := newTestBundle(t)
	tr := b.Translator("en")
	// Count should be injected automatically; caller does not need to pass it.
	result := tr.TranslatePlural("install.downloading", 3, nil)
	assert.Contains(t, result, "3")
}

func TestTranslator_Plural_MissingKey_FallsBackToKey(t *testing.T) {
	t.Parallel()
	b := newTestBundle(t)
	tr := b.Translator("en")
	result := tr.TranslatePlural("definitely.missing.plural", 5, nil)
	assert.Equal(t, "definitely.missing.plural", result)
}

func TestTranslator_Plural_NeverReturnsEmpty(t *testing.T) {
	t.Parallel()
	b := newTestBundle(t)
	tr := b.Translator("en")
	result := tr.TranslatePlural("no.such.key", 0, nil)
	assert.NotEmpty(t, result)
}

func TestTranslator_CancelledMessage_ExactMatch(t *testing.T) {
	t.Parallel()
	b := newTestBundle(t)
	// Teatest in Phase 2A asserts "Cancelled." exactly. Verify the en catalog matches.
	tr := b.Translator("en")
	assert.Equal(t, "Cancelled.", tr.Translate("install.cancelled", nil))
}

func TestTranslator_CompletedMessage_ExactMatch(t *testing.T) {
	t.Parallel()
	b := newTestBundle(t)
	tr := b.Translator("en")
	assert.Equal(t, "Onboarding complete. Run mink to start.", tr.Translate("install.completed", nil))
}
