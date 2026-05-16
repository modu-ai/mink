package i18n

import "fmt"

// CLDRCategory represents a CLDR plural category name.
// go-i18n/v2 uses these names in YAML plural forms.
type CLDRCategory string

// CLDR plural categories as defined by Unicode Common Locale Data Repository.
// Each language uses a subset of these six categories.
// See: https://cldr.unicode.org/index/cldr-spec/plural-rules
const (
	// CLDRZero is used for languages that have a distinct zero form (e.g., Arabic).
	CLDRZero CLDRCategory = "zero"

	// CLDROne is used for singular forms. English uses this for count == 1.
	CLDROne CLDRCategory = "one"

	// CLDRTwo is used for languages with a distinct dual form (e.g., Arabic).
	CLDRTwo CLDRCategory = "two"

	// CLDRFew is used for small numbers (2-4 in Russian, 3-10 in Arabic, etc.).
	CLDRFew CLDRCategory = "few"

	// CLDRMany is used for larger quantities in Slavic and Arabic languages.
	CLDRMany CLDRCategory = "many"

	// CLDROther is the catch-all category used by all languages, including Korean
	// and Japanese where no distinctions by count are made.
	CLDROther CLDRCategory = "other"
)

// knownCategories is the complete set of valid CLDR plural category names.
var knownCategories = map[CLDRCategory]bool{
	CLDRZero:  true,
	CLDROne:   true,
	CLDRTwo:   true,
	CLDRFew:   true,
	CLDRMany:  true,
	CLDROther: true,
}

// ValidateCLDRCategory returns ErrInvalidPluralRule if cat is not a recognized
// CLDR plural category. Catalog authors can call this helper to validate form
// names before registering messages.
func ValidateCLDRCategory(cat CLDRCategory) error {
	if !knownCategories[cat] {
		return fmt.Errorf("%w: %q", ErrInvalidPluralRule, cat)
	}
	return nil
}

// languagePluralForms documents which CLDR categories each supported language
// uses, as a reference for catalog authors.
// This is informational; the actual resolution is performed by go-i18n/v2 using
// embedded CLDR data.
var languagePluralForms = map[string][]CLDRCategory{
	"en":    {CLDROne, CLDROther},
	"ko":    {CLDROther},
	"ja":    {CLDROther},
	"zh":    {CLDROther},
	"zh-CN": {CLDROther},
	"ru":    {CLDROne, CLDRFew, CLDRMany, CLDROther},
	"ar":    {CLDRZero, CLDROne, CLDRTwo, CLDRFew, CLDRMany, CLDROther},
	"pl":    {CLDROne, CLDRFew, CLDRMany, CLDROther},
	"fr":    {CLDROne, CLDRMany, CLDROther},
	"de":    {CLDROne, CLDROther},
	"es":    {CLDROne, CLDRMany, CLDROther},
	"pt-BR": {CLDROne, CLDRMany, CLDROther},
}

// PluralFormsForLang returns the CLDR plural categories used by the given BCP 47
// language tag. If the tag is not in the reference table, only CLDROther is returned
// as a safe default.
func PluralFormsForLang(lang string) []CLDRCategory {
	if forms, ok := languagePluralForms[lang]; ok {
		return forms
	}
	return []CLDRCategory{CLDROther}
}
