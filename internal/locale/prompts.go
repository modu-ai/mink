package locale

import (
	"fmt"
	"strings"
	"time"
)

// BuildSystemPromptAddendum constructs a deterministic UTF-8 string that encodes
// the user's locale and cultural context for injection into LLM system prompts.
//
// The output is ≤ 400 cl100k_base tokens (REQ-LC-007, AC-LC-011).
// When LocaleContext.SecondaryLanguage is set, both languages are included and
// the code-switching directive is appended (REQ-LC-009, AC-LC-005).
//
// @MX:ANCHOR: [AUTO] LLM system prompt injection entry point consumed by QueryEngine.
// @MX:REASON: Token count constraint (≤ 400) is a hard limit; changes to the template
// must be re-measured with tiktoken before merging.
func BuildSystemPromptAddendum(loc LocaleContext, cul CulturalContext) string {
	var sb strings.Builder

	// --- Locale Context block ---
	sb.WriteString("# Locale Context\n")
	fmt.Fprintf(&sb, "- Country: %s\n", loc.Country)

	// Language(s): primary always present, secondary appended with code-switch note.
	if loc.SecondaryLanguage != "" {
		fmt.Fprintf(&sb,
			"- Languages: primary=%s, secondary=%s — code-switching is natural\n",
			loc.PrimaryLanguage, loc.SecondaryLanguage,
		)
	} else {
		fmt.Fprintf(&sb, "- Language: %s\n", loc.PrimaryLanguage)
	}

	// Timezone with current local time hint (best-effort; skipped if TZ is empty).
	tzLine := loc.Timezone
	if tzLine != "" {
		if localTime := currentLocalTime(loc.Timezone); localTime != "" {
			tzLine = fmt.Sprintf("%s (currently %s)", loc.Timezone, localTime)
		}
	}
	fmt.Fprintf(&sb, "- Timezone: %s\n", tzLine)
	fmt.Fprintf(&sb, "- Currency: %s\n", loc.Currency)
	fmt.Fprintf(&sb, "- Measurement: %s\n", loc.MeasurementSystem)
	fmt.Fprintf(&sb, "- Calendar: %s\n", loc.CalendarSystem)

	// --- Cultural Context block ---
	sb.WriteString("\n# Cultural Context\n")
	fmt.Fprintf(&sb, "- Formality: %s by default\n", cul.FormalityDefault)
	fmt.Fprintf(&sb, "- Honorific system: %s\n", cul.HonorificSystem)
	fmt.Fprintf(&sb, "- Name order: %s\n", cul.NameOrder)

	if len(cul.WeekendDays) > 0 {
		fmt.Fprintf(&sb, "- Weekend: %s\n", strings.Join(cul.WeekendDays, ", "))
	}

	if len(cul.LegalFlags) > 0 {
		fmt.Fprintf(&sb, "- Legal framework: %s\n", strings.Join(cul.LegalFlags, ", "))
	}

	// Append Detection accuracy line when present (amendment-v0.2 REQ-LC-044).
	// Omitted for legacy data (empty Accuracy) to preserve backward compatibility.
	if loc.Accuracy != "" {
		fmt.Fprintf(&sb, "\nDetection: %s", string(loc.Accuracy))
	}

	sb.WriteString("\nApply these conventions unless the user's conversational style overrides them.")

	return sb.String()
}

// currentLocalTime returns the current time in the given IANA timezone as a
// short human-readable string (e.g., "2006-01-02 15:04"). Returns "" when the
// timezone cannot be loaded (non-fatal; omitted from the prompt).
func currentLocalTime(iana string) string {
	if iana == "" {
		return ""
	}
	loc, err := time.LoadLocation(iana)
	if err != nil {
		return ""
	}
	return time.Now().In(loc).Format("2006-01-02 15:04")
}
