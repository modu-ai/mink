package i18n

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefault_ReturnsTranslator(t *testing.T) {
	// Not parallel: Default() touches the package-level sync.Once.
	tr := Default()
	require.NotNil(t, tr)
	assert.NotEmpty(t, tr.Lang())
}

func TestDefault_TranslatesWelcome(t *testing.T) {
	tr := Default()
	// Default() falls back to "en" in test environments.
	msg := tr.Translate("install.welcome", nil)
	assert.NotEmpty(t, msg)
	// The key itself is the worst-case fallback; actual catalog returns a real string.
	assert.NotEqual(t, "install.welcome", msg,
		"expected a real translation, got the key string fallback")
}

func TestDefaultFor_WithBackground(t *testing.T) {
	tr := DefaultFor(context.Background())
	require.NotNil(t, tr)
	// Background context has no locale signal, so detection falls back to "en".
	assert.NotEmpty(t, tr.Translate("install.cancelled", nil))
}

// TestEnglishCatalog_CancelledExactMatch validates the en catalog value that
// teatest Phase 2A assertions depend on. We use the embedded bundle directly
// rather than Default() so that the test is locale-independent.
func TestEnglishCatalog_CancelledExactMatch(t *testing.T) {
	t.Parallel()
	b := NewBundle("en")
	require.NoError(t, b.LoadFS(catalogFS, "catalog"))
	tr := b.Translator("en")
	// Must match exactly so teatest Phase 2A assertions pass unmodified.
	assert.Equal(t, "Cancelled.", tr.Translate("install.cancelled", nil))
}

// TestEnglishCatalog_CompletedExactMatch validates the en catalog value used in init.go.
func TestEnglishCatalog_CompletedExactMatch(t *testing.T) {
	t.Parallel()
	b := NewBundle("en")
	require.NoError(t, b.LoadFS(catalogFS, "catalog"))
	tr := b.Translator("en")
	assert.Equal(t, "Onboarding complete. Run mink to start.",
		tr.Translate("install.completed", nil))
}
