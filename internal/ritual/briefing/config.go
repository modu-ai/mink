package briefing

import (
	"errors"
	"slices"
)

var (
	// ErrNoMantra is returned when no mantra is configured.
	ErrNoMantra = errors.New("briefing: no mantra configured")

	// ErrBothMantraTypes is returned when both Mantra and Mantras are set.
	ErrBothMantraTypes = errors.New("briefing: cannot specify both mantra and mantras")

	// ErrEmptyMantra is returned when mantra string is empty.
	ErrEmptyMantra = errors.New("briefing: mantra cannot be empty")

	// ErrEmptyMantras is returned when mantras array is empty.
	ErrEmptyMantras = errors.New("briefing: mantras array cannot be empty")

	// ErrEmptyMantraInArray is returned when mantras array contains empty string.
	ErrEmptyMantraInArray = errors.New("briefing: mantras array cannot contain empty strings")
)

// Config holds configuration for the briefing system.
type Config struct {
	// Mantra is a single daily mantra (mutually exclusive with Mantras).
	Mantra string `json:"mantra,omitempty" yaml:"mantra,omitempty"`

	// Mantras is an array of mantras for rotation (mutually exclusive with Mantra).
	Mantras []string `json:"mantras,omitempty" yaml:"mantras,omitempty"`

	// LLMSummary enables M3 optional LLM-generated 2-3 line briefing summary.
	// Default false (deterministic-only briefing). When true, an LLMProvider
	// must be passed to the orchestrator and the LLM payload carries only
	// categorical signals (REQ-BR-054, AC-009 invariant 5).
	//
	// SPEC-MINK-BRIEFING-001 REQ-BR-032.
	LLMSummary bool `json:"llm_summary,omitempty" yaml:"llm_summary,omitempty"`
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	// Check if both are set
	if c.Mantra != "" && len(c.Mantras) > 0 {
		return ErrBothMantraTypes
	}

	// Check if neither is set
	if c.Mantra == "" && len(c.Mantras) == 0 {
		return ErrNoMantra
	}

	// Validate single mantra
	if c.Mantra != "" {
		if c.Mantra == "" {
			return ErrEmptyMantra
		}
		return nil
	}

	// Validate mantras array
	if len(c.Mantras) == 0 {
		return ErrEmptyMantras
	}

	if slices.Contains(c.Mantras, "") {
		return ErrEmptyMantraInArray
	}

	return nil
}

// DefaultConfig returns a valid default configuration.
func DefaultConfig() *Config {
	return &Config{
		Mantra: "Every day is a new beginning. Take a deep breath and start.",
	}
}
