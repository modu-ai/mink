// Package goosev1 provides hand-rolled types for ResolvePermission RPC.
// NOTE: buf is not available in this environment, so the proto-generated
// types for ResolvePermission are written manually here.
// TODO(buf-codegen): regenerate when buf toolchain is available.
// SPEC-GOOSE-CLI-TUI-002 P3
package goosev1

// ResolvePermissionRequest records the user's permission decision for a tool.
type ResolvePermissionRequest struct {
	// ToolUseId correlates this decision with the originating tool_use event.
	ToolUseId string `json:"tool_use_id,omitempty"`
	// ToolName is the name of the tool (e.g., "Bash", "FileWrite").
	ToolName string `json:"tool_name,omitempty"`
	// Decision is the user's choice: "allow_once", "allow_always", "deny_once", "deny_always".
	Decision string `json:"decision,omitempty"`
}

// ResolvePermissionResponse is the daemon's acknowledgement.
type ResolvePermissionResponse struct {
	// Accepted indicates the daemon acknowledged the decision.
	Accepted bool `json:"accepted,omitempty"`
}
