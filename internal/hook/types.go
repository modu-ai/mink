// Package hook implements the lifecycle hook event dispatcher and the
// useCanUseTool permission decision flow for AI.GOOSE.
// SPEC-GOOSE-HOOK-001
package hook

import (
	"context"
	"errors"
)

// HookEvent represents one of the 29 HookEvent constants (24 base + 5 ritual from SCHEDULER-001).
// AC-HK-001 / REQ-HK-001
type HookEvent string

// 29 HookEvent constants (24 base + 5 ritual from SCHEDULER-001).
// Base 24: derived from claude-primitives §5.1; Elicitation, ElicitationResult, InstructionsLoaded excluded (AC-HK-001).
// Ritual 5: added by SPEC-GOOSE-SCHEDULER-001 P1.
const (
	EvSetup              HookEvent = "Setup"
	EvSessionStart       HookEvent = "SessionStart"
	EvSubagentStart      HookEvent = "SubagentStart"
	EvUserPromptSubmit   HookEvent = "UserPromptSubmit"
	EvPreToolUse         HookEvent = "PreToolUse"
	EvPostToolUse        HookEvent = "PostToolUse"
	EvPostToolUseFailure HookEvent = "PostToolUseFailure"
	EvCwdChanged         HookEvent = "CwdChanged"
	EvFileChanged        HookEvent = "FileChanged"
	EvWorktreeCreate     HookEvent = "WorktreeCreate"
	EvWorktreeRemove     HookEvent = "WorktreeRemove"
	EvPermissionRequest  HookEvent = "PermissionRequest"
	EvPermissionDenied   HookEvent = "PermissionDenied"
	EvNotification       HookEvent = "Notification"
	EvPreCompact         HookEvent = "PreCompact"
	EvPostCompact        HookEvent = "PostCompact"
	EvStop               HookEvent = "Stop"
	EvStopFailure        HookEvent = "StopFailure"
	EvSubagentStop       HookEvent = "SubagentStop"
	EvTeammateIdle       HookEvent = "TeammateIdle"
	EvTaskCreated        HookEvent = "TaskCreated"
	EvTaskCompleted      HookEvent = "TaskCompleted"
	EvSessionEnd         HookEvent = "SessionEnd"
	EvConfigChange       HookEvent = "ConfigChange"
	// Ritual events added by SPEC-GOOSE-SCHEDULER-001 P1.
	EvMorningBriefingTime HookEvent = "MorningBriefingTime"
	EvPostBreakfastTime   HookEvent = "PostBreakfastTime"
	EvPostLunchTime       HookEvent = "PostLunchTime"
	EvPostDinnerTime      HookEvent = "PostDinnerTime"
	EvEveningCheckInTime  HookEvent = "EveningCheckInTime"
)

// HookEventNames returns the string set of all 29 HookEvent constants (24 base + 5 ritual from SCHEDULER-001).
// Validation helper for AC-HK-001.
func HookEventNames() []string {
	return []string{
		string(EvSetup),
		string(EvSessionStart),
		string(EvSubagentStart),
		string(EvUserPromptSubmit),
		string(EvPreToolUse),
		string(EvPostToolUse),
		string(EvPostToolUseFailure),
		string(EvCwdChanged),
		string(EvFileChanged),
		string(EvWorktreeCreate),
		string(EvWorktreeRemove),
		string(EvPermissionRequest),
		string(EvPermissionDenied),
		string(EvNotification),
		string(EvPreCompact),
		string(EvPostCompact),
		string(EvStop),
		string(EvStopFailure),
		string(EvSubagentStop),
		string(EvTeammateIdle),
		string(EvTaskCreated),
		string(EvTaskCompleted),
		string(EvSessionEnd),
		string(EvConfigChange),
		// Ritual events added by SPEC-GOOSE-SCHEDULER-001 P1.
		string(EvMorningBriefingTime),
		string(EvPostBreakfastTime),
		string(EvPostLunchTime),
		string(EvPostDinnerTime),
		string(EvEveningCheckInTime),
	}
}

// ToolInfo holds tool metadata used in tool-related events.
type ToolInfo struct {
	Name  string
	Input map[string]any
}

// HookError holds error information used in the PostToolUseFailure event.
type HookError struct {
	Message string
	Code    string
}

// HookInput is the common input structure shared by all events.
// REQ-HK-014: the dispatcher must pass an independent deep copy to each handler.
type HookInput struct {
	HookEvent    HookEvent
	ToolUseID    string    // tool-related events only
	Tool         *ToolInfo // PreToolUse/PostToolUse
	Input        map[string]any
	Output       any        // PostToolUse only
	Error        *HookError // PostToolUseFailure only
	ChangedPaths []string   // FileChanged only
	SessionID    string
	CustomData   map[string]any
}

// PermissionDecision is the permission decision returned by a hook.
type PermissionDecision struct {
	Approve bool
	Reason  string
	Details map[string]any
}

// HookJSONOutput is the stdout JSON output structure for shell command hooks.
// REQ-HK-006: parse JSON on exit code 0; treat exit code 2 as a blocking signal.
type HookJSONOutput struct {
	Continue             *bool               `json:"continue,omitempty"`
	SuppressOutput       bool                `json:"suppressOutput,omitempty"`
	Async                bool                `json:"async,omitempty"`
	AsyncTimeout         int                 `json:"asyncTimeout,omitempty"`
	PermissionDecision   *PermissionDecision `json:"permissionDecision,omitempty"`
	InitialUserMessage   string              `json:"initialUserMessage,omitempty"`
	WatchPaths           []string            `json:"watchPaths,omitempty"`
	AdditionalContext    string              `json:"additionalContext,omitempty"`
	UpdatedMCPToolOutput any                 `json:"updatedMCPToolOutput,omitempty"`
}

// HookHandler is the interface for a single hook handler.
// REQ-HK-002: handlers are called in registration order (FIFO).
type HookHandler interface {
	// Handle processes the hook event.
	Handle(ctx context.Context, input HookInput) (HookJSONOutput, error)
	// Matches reports whether this handler matches the given input.
	// REQ-HK-020: matcher system (glob or "regex:" prefix)
	Matches(input HookInput) bool
}

// HookBinding is a registration entry inside HookRegistry.
type HookBinding struct {
	Event   HookEvent
	Matcher string // glob or "regex:" prefix
	Handler HookHandler
	Source  string // "inline" | "plugin" | "builtin"
}

// PermissionBehavior is the outcome of a useCanUseTool decision.
// REQ-HK-009
type PermissionBehavior int

const (
	// PermAllow grants immediate tool execution.
	PermAllow PermissionBehavior = iota
	// PermDeny refuses tool execution.
	PermDeny
	// PermAsk waits for an external decision.
	PermAsk
)

// String returns the string representation of PermissionBehavior.
func (b PermissionBehavior) String() string {
	switch b {
	case PermAllow:
		return "allow"
	case PermDeny:
		return "deny"
	case PermAsk:
		return "ask"
	default:
		return "unknown"
	}
}

// DecisionReason carries the reason for a permission decision.
type DecisionReason struct {
	Type    string // "yolo_auto" | "handler" | "interactive" | "coordinator" | "swarm" | "no_interactive_fallback"
	Reason  string
	Details map[string]any
}

// PermissionResult is the return type of useCanUseTool.
type PermissionResult struct {
	Behavior       PermissionBehavior
	DecisionReason *DecisionReason
}

// Role is the set of permCtx.Role values.
// D12 resolution: role enum used by SUBAGENT-001 and CLI-001.
type Role string

const (
	// RoleCoordinator is the SUBAGENT-001 coordinator mode.
	RoleCoordinator Role = "coordinator"
	// RoleSwarmWorker is the SUBAGENT-001 swarm worker mode.
	RoleSwarmWorker Role = "swarm_worker"
	// RoleInteractive is an interactive user terminal session.
	RoleInteractive Role = "interactive"
	// RoleNonTTY is an automated / CI / pipeline environment.
	RoleNonTTY Role = "non_tty"
)

// PermissionContext is the context passed to useCanUseTool.
type PermissionContext struct {
	Role    Role
	Details map[string]any
}

// PermissionQueueOps is the interface for the YOLO classifier and auto-mode recording.
// REQ-HK-012: SetYoloClassifierApproval, REQ-HK-009: RecordAutoModeDenial
type PermissionQueueOps interface {
	SetYoloClassifierApproval(toolPattern string)
	RecordAutoModeDenial(toolName string, reason string)
	LogPermissionDecision(result PermissionResult, toolName string)
}

// InteractiveHandler is the permission-handling interface for interactive user terminal sessions.
// Implemented by CLI-001.
type InteractiveHandler interface {
	PromptUser(ctx context.Context, toolName string, input map[string]any) (PermissionResult, error)
}

// CoordinatorHandler is the permission-handling interface for the SUBAGENT-001 coordinator mode.
type CoordinatorHandler interface {
	RequestPermission(ctx context.Context, toolName string, input map[string]any) (PermissionResult, error)
}

// SwarmWorkerHandler is the permission-handling interface for the SUBAGENT-001 swarm worker mode.
type SwarmWorkerHandler interface {
	BubbleUpPermission(ctx context.Context, toolName string, input map[string]any) (PermissionResult, error)
}

// SkillsFileChangedConsumer is the external consumer type called after a FileChanged event.
// Registered by SKILLS-001; this SPEC only invokes it.
// D11 resolution
type SkillsFileChangedConsumer func(ctx context.Context, changed []string) []string

// WorkspaceRootResolver is the interface that returns the workspace root for a given sessionID.
// Implemented by SPEC-GOOSE-CORE-001; this SPEC is a consumer.
// D15 resolution / REQ-HK-021 b clause
type WorkspaceRootResolver interface {
	WorkspaceRoot(sessionID string) (string, error)
}

// PluginHookLoader is the interface that loads hooks from a plugin manifest.
// Implemented by PLUGIN-001.
type PluginHookLoader interface {
	// IsLoading reports whether the plugin loader is currently loading.
	// REQ-HK-013: Register calls are rejected while loading is in progress.
	IsLoading() bool
	// Load loads hooks from the manifest and registers them in the registry.
	Load(manifest any, registry *HookRegistry) error
}

// Error sentinels
var (
	// ErrRegistryLocked is returned when Register is called while PluginHookLoader.IsLoading is true.
	// REQ-HK-013 / AC-HK-014
	ErrRegistryLocked = errors.New("hook: registry is locked while plugin loader is loading")

	// ErrInvalidHookInput is returned when HookInput fails schema validation.
	// REQ-HK-015 / AC-HK-016
	ErrInvalidHookInput = errors.New("hook: HookInput failed schema validation")

	// ErrHookPayloadTooLarge is returned when the HookInput JSON exceeds 4 MiB.
	// REQ-HK-022 / AC-HK-024
	ErrHookPayloadTooLarge = errors.New("hook: HookInput JSON exceeds 4 MiB limit")

	// ErrHookSessionUnresolved is returned when the WorkspaceRoot resolver returns an empty path or an error.
	// REQ-HK-021 b clause / AC-HK-022
	ErrHookSessionUnresolved = errors.New("hook: WorkspaceRoot resolver returned empty path or failed")

	// ErrInvalidConsumer is returned when a nil consumer or resolver is registered.
	// SPEC-GOOSE-DAEMON-WIRE-001 REQ-WIRE-008
	ErrInvalidConsumer = errors.New("hook: cannot register nil consumer or resolver")
)

// InteractiveOptsInternal is the receiver type for InteractiveOpt functions.
// Exported so that wire-up code can read the opts state.
// SPEC-GOOSE-DAEMON-WIRE-001 REQ-WIRE-009
type InteractiveOptsInternal struct {
	ExplicitNoOp bool
}

// InteractiveOpt is the option type for wireInteractiveHandler.
// SPEC-GOOSE-DAEMON-WIRE-001 REQ-WIRE-009
type InteractiveOpt func(*InteractiveOptsInternal)

// WithExplicitNoOp marks a nil handler registration as an intentional placeholder,
// distinguishing it from an accidental nil covered by REQ-WIRE-008.
// SPEC-GOOSE-DAEMON-WIRE-001 REQ-WIRE-009
func WithExplicitNoOp() InteractiveOpt {
	return func(o *InteractiveOptsInternal) { o.ExplicitNoOp = true }
}

// maxPayloadBytes is the maximum size of a HookInput JSON payload (4 MiB).
// REQ-HK-022 / D16: exactly 4 MiB is allowed; exceeding it returns an error.
const maxPayloadBytes = 4 * 1024 * 1024

// defaultShellTimeout is the default timeout for shell command hooks in seconds.
// REQ-HK-006 c, D17: used when cfg.Timeout <= 0.
const defaultShellTimeout = 30 // seconds

// defaultAsyncTimeout is the default timeout for async hooks in seconds.
// REQ-HK-010
const defaultAsyncTimeout = 60 // seconds

// PreToolUseResult is the return type of DispatchPreToolUse.
type PreToolUseResult struct {
	Blocked            bool
	PermissionDecision *PermissionDecision
}

// PostToolUseResult is the return type of DispatchPostToolUse.
type PostToolUseResult struct {
	SuppressOutput       bool
	AdditionalContext    string
	UpdatedMCPToolOutput any
	Outputs              []HookJSONOutput
}

// SessionStartResult is the return type of DispatchSessionStart.
type SessionStartResult struct {
	InitialUserMessage string
	WatchPaths         []string
	AdditionalContext  string
}

// DispatchResult is the return type of the generic Dispatch function.
type DispatchResult struct {
	HandlerCount int
	Outcome      string // "ok" | "blocked" | "handler_error" | "timeout"
	Error        error
}
