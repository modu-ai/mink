// Package trajectory implements Layer 1 of the Goose self-evolution pipeline.
// It collects conversation turns from QueryEngine, redacts PII, and persists
// trajectories as ShareGPT-compatible JSON-L files for downstream consumers
// (COMPRESSOR-001, INSIGHTS-001).
package trajectory

import "time"

// Role represents the ShareGPT-compatible speaker role enum.
// Only these four values are valid per REQ-TRAJECTORY-002.
type Role string

const (
	// RoleSystem represents the system prompt role.
	// Redact rules are skipped for system entries by default (REQ-TRAJECTORY-016).
	RoleSystem Role = "system"

	// RoleHuman maps from QueryEngine "user" role (ShareGPT convention).
	RoleHuman Role = "human"

	// RoleGPT maps from QueryEngine "assistant" role (ShareGPT historical naming).
	RoleGPT Role = "gpt"

	// RoleTool maps from tool_use / tool_result content blocks.
	RoleTool Role = "tool"
)

// TrajectoryEntry is one element in the ShareGPT conversations array.
// Both fields are required; Value must be non-empty (REQ-TRAJECTORY-002).
type TrajectoryEntry struct {
	From  Role   `json:"from"`
	Value string `json:"value"`
}

// Trajectory represents a complete session conversation plus metadata.
// It is the unit of serialization written to disk as a single JSON-L line.
type Trajectory struct {
	Conversations []TrajectoryEntry  `json:"conversations"`
	Timestamp     time.Time          `json:"timestamp"`
	Model         string             `json:"model"`
	Completed     bool               `json:"completed"`
	SessionID     string             `json:"session_id"`
	Metadata      TrajectoryMetadata `json:"metadata,omitzero"`
}

// TrajectoryMetadata carries per-trajectory auxiliary fields.
type TrajectoryMetadata struct {
	// Tags are downstream filtering labels (e.g. ["skill:code-review", "model:anthropic"]).
	Tags []string `json:"tags,omitempty"`

	// FailureReason is populated from Terminal.error when completed==false.
	FailureReason string `json:"failure_reason,omitempty"`

	// Partial marks buffer-spill fragments written before session end.
	Partial bool `json:"partial,omitempty"`

	// TurnCount is the number of conversation entries in this record.
	TurnCount int `json:"turn_count"`

	// DurationMs is wall-clock duration from session start to flush.
	DurationMs int64 `json:"duration_ms"`

	// TokensInput and TokensOutput are optional token counts from the LLM.
	TokensInput  int `json:"tokens_input,omitempty"`
	TokensOutput int `json:"tokens_output,omitempty"`
}

// TelemetryConfig maps config.yaml telemetry.trajectory.* fields.
type TelemetryConfig struct {
	// Enabled gates the entire collector; false means no-op (REQ-TRAJECTORY-011).
	Enabled bool `yaml:"enabled" json:"enabled"`

	// RetentionDays is the number of UTC calendar days to keep trajectory files.
	// Default 90 per REQ-TRAJECTORY-009.
	RetentionDays int `yaml:"retention_days" json:"retention_days"`

	// MaxFileBytes triggers size-based rotation (default 10485760 = 10MB).
	// REQ-TRAJECTORY-007.
	MaxFileBytes int64 `yaml:"max_file_bytes" json:"max_file_bytes"`

	// InMemoryTurnCap is the maximum turns buffered per session before spill.
	// Default 1000 per REQ-TRAJECTORY-012.
	InMemoryTurnCap int `yaml:"in_memory_turn_cap" json:"in_memory_turn_cap"`

	// GooseHome overrides the base directory (default: $HOME/.goose).
	GooseHome string `yaml:"goose_home" json:"goose_home"`
}

// DefaultTelemetryConfig returns production defaults.
func DefaultTelemetryConfig() TelemetryConfig {
	return TelemetryConfig{
		Enabled:         true,
		RetentionDays:   90,
		MaxFileBytes:    10_485_760, // 10 MB
		InMemoryTurnCap: 1000,
	}
}
