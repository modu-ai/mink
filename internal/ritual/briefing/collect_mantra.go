package briefing

import (
	"strings"
	"time"
)

// MantraCollector collects daily mantra information for the briefing.
type MantraCollector struct {
	cfg *Config
}

// NewMantraCollector creates a new MantraCollector.
func NewMantraCollector(cfg *Config) *MantraCollector {
	return &MantraCollector{cfg: cfg}
}

// clinicalVocabularyTerms lists Korean clinical psychology/psychiatry terms
// that should not appear in daily mantras (research.md §4.3 - harm prevention).
var clinicalVocabularyTerms = []string{
	"우울증", "depression", "우울",
	"불안장애", "anxiety",
	"조현병", "schizophrenia",
	"양극성", "bipolar",
	"강박", "obsessive",
	"공황", "panic",
	"자살", "suicide",
	"자해", "self-harm",
	"트라우마", "trauma",
	"PTSD",
	"성격장애", "personality",
}

// Collect fetches mantra data and returns a MantraModule.
// Status is "ok" if mantra available, "skipped" if clinical vocabulary detected.
func (c *MantraCollector) Collect(today time.Time) (*MantraModule, string) {
	module := &MantraModule{
		Text:   "",
		Source: "",
		Index:  0,
		Total:  0,
	}

	// Determine which mantra to use
	var mantraText string
	var mantraIndex int
	var mantraTotal int

	if c.cfg.Mantra != "" {
		// Single mantra takes priority
		mantraText = c.cfg.Mantra
		mantraIndex = 0
		mantraTotal = 1
	} else if len(c.cfg.Mantras) > 0 {
		// Rotate through mantras by ISO week number
		_, week := today.ISOWeek()
		mantraTotal = len(c.cfg.Mantras)
		mantraIndex = (week - 1) % mantraTotal
		mantraText = c.cfg.Mantras[mantraIndex]
	} else {
		// No mantra configured
		return module, "ok"
	}

	// Check for clinical vocabulary
	if c.containsClinicalVocabulary(mantraText) {
		return module, "skipped"
	}

	module.Text = mantraText
	module.Index = mantraIndex
	module.Total = mantraTotal

	return module, "ok"
}

// containsClinicalVocabulary checks if the text contains any clinical terms.
func (c *MantraCollector) containsClinicalVocabulary(text string) bool {
	lowerText := strings.ToLower(text)
	for _, term := range clinicalVocabularyTerms {
		if strings.Contains(lowerText, strings.ToLower(term)) {
			return true
		}
	}
	return false
}
