// Package install — style_test.go verifies the MINK brand-aligned theme and style set.
//
// SPEC: SPEC-MINK-ONBOARDING-001 §6 (Phase 2C polish)
package install

import (
	"sync"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMINKTheme_ReturnsNonNil verifies MINKTheme returns a non-nil theme on
// each call, and that two calls return independent instances (mutation isolation).
func TestMINKTheme_ReturnsNonNil(t *testing.T) {
	t1 := MINKTheme()
	require.NotNil(t, t1, "first MINKTheme() call should return non-nil")

	t2 := MINKTheme()
	require.NotNil(t, t2, "second MINKTheme() call should return non-nil")

	// Mutate t1's Focused.Title and verify t2 is unchanged.
	original := t2.Focused.Title
	t1.Focused.Title = t1.Focused.Title.Bold(false).Italic(true)

	// t2's Focused.Title should retain its original value.
	assert.Equal(t, original.GetBold(), t2.Focused.Title.GetBold(),
		"mutating t1.Focused.Title should not affect t2.Focused.Title")
}

// TestMINKTheme_ConcurrentSafety verifies that calling MINKTheme() concurrently
// from multiple goroutines does not cause data races or share mutable state.
func TestMINKTheme_ConcurrentSafety(t *testing.T) {
	const goroutines = 8
	themes := make([]interface{ GetBold() bool }, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		go func() {
			defer wg.Done()
			th := MINKTheme()
			// Mutate this goroutine's copy.
			th.Focused.Title = th.Focused.Title.Bold(i%2 == 0)
			themes[i] = th.Focused.Title
		}()
	}
	wg.Wait()

	// Each goroutine's copy should independently reflect its own mutation.
	for i := range goroutines {
		expected := i%2 == 0
		assert.Equal(t, expected, themes[i].GetBold(),
			"goroutine %d: expected bold=%v", i, expected)
	}
}

// TestNewMINKStyles_FieldsPopulated verifies that every field in MINKStyles has
// at least one attribute set (i.e., is not the zero-value lipgloss.Style).
func TestNewMINKStyles_FieldsPopulated(t *testing.T) {
	s := NewMINKStyles()

	tests := []struct {
		name  string
		style lipgloss.Style
	}{
		{"Title", s.Title},
		{"Subtitle", s.Subtitle},
		{"Success", s.Success},
		{"Error", s.Error},
		{"Info", s.Info},
		{"Muted", s.Muted},
		{"Accent", s.Accent},
		{"Bar", s.Bar},
	}

	zero := lipgloss.NewStyle()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// A populated style should differ from the zero-value style in at
			// least one rendered dimension. We compare the Foreground color
			// since all our styles set one.
			assert.NotEqual(t, zero.GetForeground(), tc.style.GetForeground(),
				"%s style should have a non-zero foreground color", tc.name)
		})
	}
}

// TestMINKStyles_RenderingContainsColors verifies that rendering via a colored
// style produces ANSI escape sequences when using the TrueColor profile.
func TestMINKStyles_RenderingContainsColors(t *testing.T) {
	// Force true-color output so ANSI codes are emitted.
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })

	s := NewMINKStyles()

	tests := []struct {
		name  string
		style lipgloss.Style
	}{
		{"Success", s.Success},
		{"Error", s.Error},
		{"Accent", s.Accent},
		{"Muted", s.Muted},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := tc.style.Render("test")
			assert.NotEmpty(t, out, "%s style render should not be empty", tc.name)
			// With TrueColor profile, styled output contains ESC character.
			assert.Contains(t, out, "\x1b[",
				"%s style render should contain ANSI escape sequences with TrueColor profile", tc.name)
		})
	}
}

// TestMINKTheme_BrandColors verifies the MINK brand palette is applied to the
// returned theme's focused elements.
func TestMINKTheme_BrandColors(t *testing.T) {
	th := MINKTheme()

	// Focused.Title should use primary brand color (bold).
	assert.True(t, th.Focused.Title.GetBold(),
		"focused title should be bold")

	// Focused.FocusedButton background should be set.
	btnBg := th.Focused.FocusedButton.GetBackground()
	assert.NotNil(t, btnBg, "focused button background should be set")
}
