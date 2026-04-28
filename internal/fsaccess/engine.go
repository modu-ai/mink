// Package fsaccess provides filesystem access control with policy-based security.
// SPEC-GOOSE-FS-ACCESS-001
package fsaccess

import "sync"

// Operation represents the type of filesystem operation being requested.
// REQ-FSACCESS-001: Check blocked_always first, then write_paths/read_paths, then AskUserQuestion
type Operation int

const (
	// OperationRead represents a read operation (reading file contents)
	OperationRead Operation = iota
	// OperationWrite represents a write operation (modifying file contents)
	OperationWrite
	// OperationCreate represents a create operation (creating new files)
	OperationCreate
	// OperationDelete represents a delete operation (removing files)
	OperationDelete
)

// String returns the string representation of the operation.
func (o Operation) String() string {
	switch o {
	case OperationRead:
		return "read"
	case OperationWrite:
		return "write"
	case OperationCreate:
		return "create"
	case OperationDelete:
		return "delete"
	default:
		return "unknown"
	}
}

// Decision represents the access control decision.
// REQ-FSACCESS-001: 3-stage decision flow (blocked > write > read > ask)
type Decision int

const (
	// DecisionAllow means the operation is permitted
	DecisionAllow Decision = iota
	// DecisionDeny means the operation is forbidden
	DecisionDeny
	// DecisionAsk means user confirmation is required
	DecisionAsk
)

// String returns the string representation of the decision.
func (d Decision) String() string {
	switch d {
	case DecisionAllow:
		return "allow"
	case DecisionDeny:
		return "deny"
	case DecisionAsk:
		return "ask"
	default:
		return "unknown"
	}
}

// AccessResult represents the result of an access control check.
// AC-01: 3-stage decision flow with reason and policy reference
//
// @MX:ANCHOR: [AUTO] Core access control result structure
// @MX:REASON: Returned by all CheckAccess calls, used by audit logging, fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-FS-ACCESS-001 REQ-FSACCESS-001, AC-01
type AccessResult struct {
	// Decision is the final access control decision
	Decision Decision
	// Reason explains why this decision was made
	Reason string
	// Policy indicates which policy rule matched (e.g., "write_paths", "blocked_always")
	Policy string
}

// DecisionEngine performs filesystem access control checks based on security policy.
// It implements the 3-stage decision flow:
// 1. Check blocked_always (unconditional deny)
// 2. Check write_paths or read_paths (depending on operation)
// 3. Return Ask if no match (requires user confirmation)
//
// REQ-FSACCESS-001: 3-stage decision flow
// AC-01: Complete decision logic with reason and policy tracking
//
// @MX:ANCHOR: [AUTO] Core decision engine
// @MX:REASON: Central access control logic, fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-FS-ACCESS-001 REQ-FSACCESS-001, AC-01
type DecisionEngine struct {
	policy *SecurityPolicy
	mu     sync.RWMutex
}

// NewDecisionEngine creates a new DecisionEngine with the given security policy.
// The policy must not be nil.
//
// @MX:ANCHOR: [AUTO] DecisionEngine constructor
// @MX:REASON: Used by all fsaccess consumers, fan_in >= 3
func NewDecisionEngine(policy *SecurityPolicy) *DecisionEngine {
	return &DecisionEngine{
		policy: policy,
	}
}

// CheckAccess determines whether a filesystem operation should be allowed.
// It implements the 3-stage decision flow:
//
// Stage 1: Check if path matches blocked_always (unconditional deny)
// Stage 2: Check if path matches write_paths or read_paths (depending on operation)
// Stage 3: Return Ask if no match found (requires user confirmation)
//
// REQ-FSACCESS-001: Check blocked_always first, then write_paths/read_paths, then AskUserQuestion
// REQ-FSACCESS-003: blocked_always override impossible (always deny, no user override)
// AC-01: Complete 3-stage decision flow
//
// @MX:ANCHOR: [AUTO] Core access control check function
// @MX:REASON: Called for every FS operation, fan_in >= 5
// @MX:SPEC: SPEC-GOOSE-FS-ACCESS-001 REQ-FSACCESS-001, REQ-FSACCESS-003, AC-01
func (e *DecisionEngine) CheckAccess(path string, operation Operation) AccessResult {
	e.mu.RLock()
	policy := e.policy
	e.mu.RUnlock()

	// Stage 1: Check blocked_always (highest priority, unconditional deny)
	for _, pattern := range policy.BlockedAlways {
		if GlobMatch(pattern, path) {
			return AccessResult{
				Decision: DecisionDeny,
				Reason:   "Path matches blocked_always policy (cannot be overridden)",
				Policy:   "blocked_always",
			}
		}
	}

	// Determine which paths to check based on operation
	// Write/Create/Delete operations use write_paths
	// Read operations use read_paths
	var pathsToCheck []string
	var policyName string

	if operation == OperationWrite || operation == OperationCreate || operation == OperationDelete {
		pathsToCheck = policy.WritePaths
		policyName = "write_paths"
	} else {
		// OperationRead
		pathsToCheck = policy.ReadPaths
		policyName = "read_paths"
	}

	// Stage 2: Check if path matches allowed paths
	for _, pattern := range pathsToCheck {
		if GlobMatch(pattern, path) {
			return AccessResult{
				Decision: DecisionAllow,
				Reason:   "Path matches allowed " + policyName + " policy",
				Policy:   policyName,
			}
		}
	}

	// Stage 3: No match found - require user confirmation
	return AccessResult{
		Decision: DecisionAsk,
		Reason:   "Path does not match any policy rule (user confirmation required)",
		Policy:   "no matching policy",
	}
}

// UpdatePolicy atomically replaces the engine's security policy.
// Safe to call concurrently with CheckAccess.
//
// @MX:ANCHOR: [AUTO] Atomic policy swap for hot-reload
// @MX:REASON: Called by PolicyReloader during file change detection, fan_in >= 2
// @MX:SPEC: SPEC-GOOSE-FS-ACCESS-001 REQ-FSACCESS-004
func (e *DecisionEngine) UpdatePolicy(policy *SecurityPolicy) {
	e.mu.Lock()
	e.policy = policy
	e.mu.Unlock()
}
