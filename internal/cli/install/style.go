// Package install — style.go defines the MINK brand-aligned lipgloss styles
// and huh theme used by the onboarding wizard.
//
// Brand palette (provisional, mirrors .moai/project/brand/visual-identity.md
// when present; otherwise this file is the source of truth):
//
//	primary   = #6B5BFF  (purple — MINK signature)
//	accent    = #FFB347  (orange — call-to-action / progress)
//	muted     = #6E6A86  (slate — secondary text)
//	surface   = #1E1B2E  (near-black — backgrounds)
//	error     = #FF5C7C  (rose — destructive)
//	success   = #4ADE80  (mint — confirmations)
//
// SPEC: SPEC-MINK-ONBOARDING-001 §6 (Phase 2C polish)
package install

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// Brand color constants — single source of truth for the MINK palette.
const (
	colorPrimary = "#6B5BFF" // purple — MINK signature
	colorAccent  = "#FFB347" // orange — call-to-action / progress
	colorMuted   = "#6E6A86" // slate — secondary text
	colorSurface = "#1E1B2E" // near-black — backgrounds
	colorError   = "#FF5C7C" // rose — destructive
	colorSuccess = "#4ADE80" // mint — confirmations
	colorWhite   = "#F5F5F5" // near-white — base text
)

// MINKStyles holds the package-level style set used by progress.go and
// ad-hoc status messages.
// Exported only for tests; application code uses NewMINKStyles.
type MINKStyles struct {
	// Title is used for step headings — bold + primary color + top padding.
	Title lipgloss.Style
	// Subtitle is used for secondary headings.
	Subtitle lipgloss.Style
	// Success is used for success indicators (checkmarks, completion messages).
	Success lipgloss.Style
	// Error is used for error messages.
	Error lipgloss.Style
	// Info is used for informational messages.
	Info lipgloss.Style
	// Muted is used for secondary/hint text.
	Muted lipgloss.Style
	// Accent is used for call-to-action items and download progress.
	Accent lipgloss.Style
	// Bar is used for the progress bar wrapper line.
	Bar lipgloss.Style
}

// NewMINKStyles returns a fresh MINKStyles instance.
// Each call is independent so concurrent callers don't share mutable state.
func NewMINKStyles() MINKStyles {
	return MINKStyles{
		Title: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorPrimary)).
			Bold(true).
			PaddingTop(1),

		Subtitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorPrimary)).
			Bold(false),

		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorSuccess)).
			Bold(true),

		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorError)).
			Bold(true),

		Info: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorWhite)),

		Muted: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorMuted)),

		Accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorAccent)).
			Bold(true),

		Bar: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorAccent)),
	}
}

// MINKTheme returns a fresh huh.Theme tuned to the MINK brand palette.
// Each call returns an independent copy so concurrent forms don't share mutable state.
//
// The theme starts from huh.ThemeCharm() as a softer baseline, then overrides
// specific colours with the MINK brand palette.
//
// @MX:ANCHOR: [AUTO] Single theming entry point used by all 17 huh.NewForm() call sites.
// @MX:REASON: Centralising huh theme construction means a single change affects every
// wizard step; callers MUST NOT construct huh.Theme directly.
func MINKTheme() *huh.Theme {
	t := huh.ThemeCharm()

	// Primary color overrides for focused fields.
	t.Focused.Base = t.Focused.Base.
		BorderForeground(lipgloss.Color(colorPrimary))
	t.Focused.Title = t.Focused.Title.
		Foreground(lipgloss.Color(colorPrimary)).
		Bold(true)
	t.Focused.Description = t.Focused.Description.
		Foreground(lipgloss.Color(colorMuted))
	t.Focused.ErrorIndicator = t.Focused.ErrorIndicator.
		Foreground(lipgloss.Color(colorError))
	t.Focused.ErrorMessage = t.Focused.ErrorMessage.
		Foreground(lipgloss.Color(colorError))
	t.Focused.SelectSelector = t.Focused.SelectSelector.
		Foreground(lipgloss.Color(colorAccent)).
		Bold(true)
	t.Focused.Option = t.Focused.Option.
		Foreground(lipgloss.Color(colorWhite))
	t.Focused.SelectedOption = t.Focused.SelectedOption.
		Foreground(lipgloss.Color(colorSuccess)).
		Bold(true)
	t.Focused.SelectedPrefix = t.Focused.SelectedPrefix.
		Foreground(lipgloss.Color(colorSuccess))
	t.Focused.UnselectedOption = t.Focused.UnselectedOption.
		Foreground(lipgloss.Color(colorMuted))
	t.Focused.FocusedButton = t.Focused.FocusedButton.
		Background(lipgloss.Color(colorPrimary)).
		Foreground(lipgloss.Color(colorWhite)).
		Bold(true)
	t.Focused.BlurredButton = t.Focused.BlurredButton.
		Background(lipgloss.Color(colorSurface)).
		Foreground(lipgloss.Color(colorMuted))

	// Blurred field overrides.
	t.Blurred.Title = t.Blurred.Title.
		Foreground(lipgloss.Color(colorMuted))
	t.Blurred.SelectSelector = t.Blurred.SelectSelector.
		Foreground(lipgloss.Color(colorMuted))

	// Field separator.
	t.FieldSeparator = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorSurface))

	return t
}
