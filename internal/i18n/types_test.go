package i18n

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubTranslator is a minimal test double that satisfies the Translator interface.
type stubTranslator struct {
	lang     string
	messages map[string]string
}

func (s *stubTranslator) Translate(key string, _ map[string]any) string {
	if msg, ok := s.messages[key]; ok {
		return msg
	}
	return key
}

func (s *stubTranslator) TranslatePlural(key string, count int, _ map[string]any) string {
	if msg, ok := s.messages[key]; ok {
		return msg
	}
	return key
}

func (s *stubTranslator) Lang() string { return s.lang }

// Verify stubTranslator implements Translator at compile time.
var _ Translator = (*stubTranslator)(nil)

func TestTranslator_InterfaceConformance(t *testing.T) {
	t.Parallel()
	stub := &stubTranslator{lang: "en", messages: map[string]string{"hello": "Hello"}}

	assert.Equal(t, "Hello", stub.Translate("hello", nil))
	assert.Equal(t, "missing", stub.Translate("missing", nil))
	assert.Equal(t, "en", stub.Lang())
}

func TestSentinelErrors_AreDistinct(t *testing.T) {
	t.Parallel()
	require.NotEqual(t, ErrUnknownLocale, ErrMissingTranslation)
	require.NotEqual(t, ErrUnknownLocale, ErrInvalidPluralRule)
	require.NotEqual(t, ErrMissingTranslation, ErrInvalidPluralRule)
}

func TestSentinelErrors_WrappedWithIs(t *testing.T) {
	t.Parallel()
	wrapped := errors.Join(ErrUnknownLocale, errors.New("detail"))
	assert.True(t, errors.Is(wrapped, ErrUnknownLocale))
}

func TestCatalog_Fields(t *testing.T) {
	t.Parallel()
	cat := Catalog{
		Lang:     "ko",
		Messages: map[string]string{"k": "v"},
	}
	assert.Equal(t, "ko", cat.Lang)
	assert.Equal(t, "v", cat.Messages["k"])
}
