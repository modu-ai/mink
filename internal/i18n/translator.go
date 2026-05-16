package i18n

import (
	"maps"

	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

// localizer is the concrete implementation of Translator backed by go-i18n's *Localizer.
// It is unexported; callers obtain values via Bundle.Translator or Default().
type localizer struct {
	loc  *goi18n.Localizer
	lang string
}

// Translate returns the localized string for key.
// Fallback chain: active lang → default lang (usually "en") → key string itself.
// Template parameters in params are applied via Go's text/template engine.
// Translate never returns an empty string.
func (l *localizer) Translate(key string, params map[string]any) string {
	cfg := &goi18n.LocalizeConfig{
		MessageID:    key,
		TemplateData: params,
	}
	result, err := l.loc.Localize(cfg)
	if err != nil || result == "" {
		// Fallback to the key string; never return empty.
		return key
	}
	return result
}

// TranslatePlural returns a pluralized string for key using count and CLDR plural
// rules for the active language. The Count field is automatically added to params
// if not already present so that {{.Count}} placeholders in the message template
// are resolved without requiring the caller to pass it explicitly.
func (l *localizer) TranslatePlural(key string, count int, params map[string]any) string {
	merged := make(map[string]any, len(params)+1)
	maps.Copy(merged, params)
	merged["Count"] = count

	cfg := &goi18n.LocalizeConfig{
		MessageID:    key,
		TemplateData: merged,
		PluralCount:  count,
	}
	result, err := l.loc.Localize(cfg)
	if err != nil || result == "" {
		return key
	}
	return result
}

// Lang returns the BCP 47 language tag this Translator was constructed for.
func (l *localizer) Lang() string {
	return l.lang
}
