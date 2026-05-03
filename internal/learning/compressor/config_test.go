package compressor

import (
	"testing"
	"time"
)

// TestCompressionConfig_Defaults verifies AC-COMPRESSOR-001 — exact constant values.
func TestCompressionConfig_Defaults(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.TargetMaxTokens != 15_250 {
		t.Errorf("TargetMaxTokens: got %d, want 15250", cfg.TargetMaxTokens)
	}
	if cfg.SummaryTargetTokens != 750 {
		t.Errorf("SummaryTargetTokens: got %d, want 750", cfg.SummaryTargetTokens)
	}
	if cfg.TailProtectedTurns != 4 {
		t.Errorf("TailProtectedTurns: got %d, want 4", cfg.TailProtectedTurns)
	}
	if cfg.MaxConcurrentRequests != 5 {
		t.Errorf("MaxConcurrentRequests: got %d, want 5", cfg.MaxConcurrentRequests)
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries: got %d, want 3", cfg.MaxRetries)
	}
	if cfg.AdapterMaxRetries != 1 {
		t.Errorf("AdapterMaxRetries: got %d, want 1", cfg.AdapterMaxRetries)
	}
	if cfg.BaseDelay != 2*time.Second {
		t.Errorf("BaseDelay: got %v, want 2s", cfg.BaseDelay)
	}
	if cfg.PerTrajectoryTimeout != 300*time.Second {
		t.Errorf("PerTrajectoryTimeout: got %v, want 300s", cfg.PerTrajectoryTimeout)
	}
	if cfg.SummaryOvershootFactor != 2.0 {
		t.Errorf("SummaryOvershootFactor: got %v, want 2.0", cfg.SummaryOvershootFactor)
	}
}
