package journal

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds all runtime settings for the journal feature.
// All fields default to the most privacy-preserving value (opt-in semantics).
// REQ-001, REQ-003, REQ-010, REQ-011, REQ-018
type Config struct {
	// Enabled gates the entire feature. Default: false (privacy opt-in).
	Enabled bool `yaml:"enabled"`
	// DataDir is the directory for the SQLite journal database.
	// Default: ~/.goose/journal/
	DataDir string `yaml:"data_dir"`
	// EmotionLLMAssisted enables LLM-based emotion analysis (M3). Default: false.
	EmotionLLMAssisted bool `yaml:"emotion_llm_assisted"`
	// AllowLoRATraining controls whether entries may be used for LoRA fine-tuning. Default: false.
	AllowLoRATraining bool `yaml:"allow_lora_training"`
	// CloudBackup enables E2E-encrypted cloud backup (v0.2+). Default: false.
	CloudBackup bool `yaml:"cloud_backup"`
	// RetentionDays is the number of days to keep entries before nightly cleanup.
	// -1 means keep forever (default).
	RetentionDays int `yaml:"retention_days"`
	// PromptTimeoutMin is how long the orchestrator waits for a user response before timing out.
	// Default: 60 minutes.
	PromptTimeoutMin int `yaml:"prompt_timeout_min"`
	// WeeklySummary enables the weekly digest job (M2 cadence, M3 LLM). Default: false.
	WeeklySummary bool `yaml:"weekly_summary"`
}

// defaultConfig returns the privacy-preserving defaults.
func defaultConfig() Config {
	return Config{
		Enabled:            false,
		EmotionLLMAssisted: false,
		AllowLoRATraining:  false,
		CloudBackup:        false,
		RetentionDays:      -1,
		PromptTimeoutMin:   60,
		WeeklySummary:      false,
	}
}

// LoadJournalConfig reads a YAML config file and merges it with the defaults.
// If path is empty or the file does not exist, the privacy-safe defaults are returned.
// Fields absent from the YAML file retain their default values.
func LoadJournalConfig(path string) (Config, error) {
	cfg := defaultConfig()
	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}

	// Empty file → defaults
	if len(data) == 0 {
		return cfg, nil
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}

	// Re-apply fields that must never be zero-defaulted by yaml.Unmarshal.
	// yaml.Unmarshal leaves missing keys at Go zero-value, which for int is 0.
	// RetentionDays = 0 would mean "delete everything tonight", so we keep -1
	// unless the YAML explicitly sets a non-zero positive value.
	// We detect this by checking whether the user wrote retention_days in YAML.
	var raw map[string]any
	if parseErr := yaml.Unmarshal(data, &raw); parseErr == nil {
		if _, exists := raw["retention_days"]; !exists {
			cfg.RetentionDays = -1
		}
		if _, exists := raw["prompt_timeout_min"]; !exists {
			cfg.PromptTimeoutMin = 60
		}
	}

	return cfg, nil
}
