// Package fsaccess provides filesystem access control with policy-based security.
// SPEC-GOOSE-FS-ACCESS-001
package fsaccess

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// SecurityPolicy defines the filesystem access control policy.
// It contains three lists of glob patterns for different access levels.
//
// REQ-FSACCESS-001: 3-stage decision flow (blocked > write > read)
// REQ-FSACCESS-002: Glob pattern matching precision
//
// @MX:ANCHOR: [AUTO] Core security policy structure
// @MX:REASON: Central access control configuration, fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-FS-ACCESS-001 REQ-FSACCESS-001, REQ-FSACCESS-002
type SecurityPolicy struct {
	// WritePaths contains glob patterns for paths that can be written
	WritePaths []string `yaml:"write_paths"`

	// ReadPaths contains glob patterns for paths that can be read
	ReadPaths []string `yaml:"read_paths"`

	// BlockedAlways contains glob patterns for paths that are never allowed
	BlockedAlways []string `yaml:"blocked_always"`
}

// LoadSecurityPolicy loads a security policy from a YAML file.
// The file must exist and contain valid YAML with the three required keys:
// write_paths, read_paths, and blocked_always.
//
// REQ-FSACCESS-004: Hot reload capability (this function can be called on file change)
//
// @MX:ANCHOR: [AUTO] Policy loader function
// @MX:REASON: Used by DecisionEngine and PolicyReloader, fan_in >= 3
func LoadSecurityPolicy(configPath string) (*SecurityPolicy, error) {
	// Read the YAML file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read security config: %w", err)
	}

	// Parse YAML
	var policy SecurityPolicy
	if err := yaml.Unmarshal(data, &policy); err != nil {
		return nil, fmt.Errorf("failed to parse security config: %w", err)
	}

	// Ensure lists are non-nil (even if empty in YAML)
	if policy.WritePaths == nil {
		policy.WritePaths = []string{}
	}
	if policy.ReadPaths == nil {
		policy.ReadPaths = []string{}
	}
	if policy.BlockedAlways == nil {
		policy.BlockedAlways = []string{}
	}

	return &policy, nil
}
