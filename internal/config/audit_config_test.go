package config

import (
	"testing"
)

// TestDefaultConfig_HasAuditDefaults tests that default config includes audit settings
func TestDefaultConfig_HasAuditDefaults(t *testing.T) {
	// Arrange & Act: Get default config
	cfg := defaultConfig()

	// Assert: Audit config should have default values
	if !cfg.Audit.Enabled {
		t.Error("Expected audit.enabled to be true by default")
	}
	if cfg.Audit.MaxSizeMB != 100 {
		t.Errorf("Expected audit.max_size_mb to be 100, got %d", cfg.Audit.MaxSizeMB)
	}
	// REQ-MINK-UDM-002: defaults が .mink に移行済み
	if cfg.Audit.GlobalDir != "~/.mink/logs" {
		t.Errorf("Expected audit.global_dir to be '~/.mink/logs', got %s", cfg.Audit.GlobalDir)
	}
	if cfg.Audit.LocalDir != "./.mink/logs" {
		t.Errorf("Expected audit.local_dir to be './.mink/logs', got %s", cfg.Audit.LocalDir)
	}
}

// TestLoadFromMap_WithAuditConfig tests loading audit config from map
func TestLoadFromMap_WithAuditConfig(t *testing.T) {
	// Arrange: Create config map with audit settings
	configMap := map[string]any{
		"audit": map[string]any{
			"enabled":     false,
			"max_size_mb": 200,
			"global_dir":  "/var/log/goose",
			"local_dir":   "./project/logs",
		},
	}

	// Act: Load config from map
	cfg, err := LoadFromMap(configMap)
	if err != nil {
		t.Fatalf("LoadFromMap failed: %v", err)
	}

	// Assert: Audit config should match input
	if cfg.Audit.Enabled != false {
		t.Errorf("Expected audit.enabled to be false, got %v", cfg.Audit.Enabled)
	}
	if cfg.Audit.MaxSizeMB != 200 {
		t.Errorf("Expected audit.max_size_mb to be 200, got %d", cfg.Audit.MaxSizeMB)
	}
	if cfg.Audit.GlobalDir != "/var/log/goose" {
		t.Errorf("Expected audit.global_dir to be '/var/log/goose', got %s", cfg.Audit.GlobalDir)
	}
	if cfg.Audit.LocalDir != "./project/logs" {
		t.Errorf("Expected audit.local_dir to be './project/logs', got %s", cfg.Audit.LocalDir)
	}
}
