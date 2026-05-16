package i18n

import "errors"

// Translator resolves localized strings for a specific language at runtime.
// All methods are safe for concurrent use; implementations must not mutate state.
//
// @MX:ANCHOR: [AUTO] Core interface — 3+ callers: Default(), translator.go impl, test doubles.
// @MX:REASON: Interface additions break all implementations; removals break all callers.
type Translator interface {
	// Translate returns the localized string for key, substituting any template
	// parameters from params. If the key is missing for this language, the
	// implementation falls back to the default language, then to the key itself.
	// Translate never returns an empty string.
	Translate(key string, params map[string]any) string

	// TranslatePlural returns a pluralized localized string for key using count
	// and CLDR plural rules for the active language. params may include additional
	// template variables beyond Count.
	TranslatePlural(key string, count int, params map[string]any) string

	// Lang returns the BCP 47 language tag this Translator was built for
	// (e.g., "en", "ko", "ja"). For multi-region tags like "fr-CA" the Bundle
	// first tries the full tag, then truncates to "fr" per REQ-I18N-019.
	Lang() string
}

// Catalog holds a loaded, validated translation map for a single language.
// Callers obtain Catalog values via Bundle.LoadMessageFile or Bundle.LoadDirectory.
type Catalog struct {
	// Lang is the BCP 47 primary language tag (e.g., "en", "ko").
	Lang string

	// Messages is the raw key→value map after YAML unmarshalling.
	// Values may still contain go-template placeholders ({{.Name}}).
	Messages map[string]string
}

// Sentinel errors returned by Bundle operations.
var (
	// ErrUnknownLocale is returned when a Translator is requested for a language
	// that has no bundle and no default-language fallback can be loaded.
	ErrUnknownLocale = errors.New("i18n: unknown locale")

	// ErrMissingTranslation signals that a key was not found in any loaded bundle.
	// Translate and TranslatePlural never return this error; they fall back to the
	// key string. The error is available for callers that need strict key validation.
	ErrMissingTranslation = errors.New("i18n: missing translation key")

	// ErrInvalidPluralRule is returned when a catalog YAML value declares a plural
	// form that is not recognized by CLDR (e.g., "six" or an empty form name).
	ErrInvalidPluralRule = errors.New("i18n: invalid plural rule")
)
