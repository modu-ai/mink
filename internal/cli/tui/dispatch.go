// Package tui provides dispatcher integration for slash commands.
package tui

import (
	"context"
	"fmt"
)

// AppInterface defines the interface for App integration in TUI.
// @MX:NOTE This interface avoids import cycle between cli and tui packages.
type AppInterface interface {
	ProcessInput(ctx context.Context, input string) (*ProcessResult, error)
}

// ProcessResult represents the result of processing input through the dispatcher.
// @MX:ANCHOR This type defines the contract for dispatcher processing results.
type ProcessResult struct {
	Kind     ProcessedKind
	Messages []ProcessMessage
	Prompt   string
}

// ProcessMessage represents a message from the dispatcher.
// @MX:NOTE This is a simplified version of message.SDKMessage to avoid import cycle.
type ProcessMessage struct {
	Type    string
	Content string
}

// ProcessedKind represents the type of processing result.
// @MX:NOTE This matches command.ProcessedKind but avoids import cycle.
type ProcessedKind int

const (
	// ProcessLocal indicates the command was handled locally.
	ProcessLocal ProcessedKind = iota
	// ProcessProceed indicates the input should proceed to daemon.
	ProcessProceed
	// ProcessExit indicates the TUI should exit.
	ProcessExit
	// ProcessAbort indicates the input was rejected.
	ProcessAbort
)

// DispatchInput processes user input through the App dispatcher.
// @MX:ANCHOR This function bridges TUI input and App dispatcher.
// @MX:REASON: Called by TUI slash command handler - fan_in >= 2.
func DispatchInput(app AppInterface, ctx context.Context, input string) (*ProcessResult, error) {
	if app == nil {
		// No app available, return proceed to fall back to legacy handling
		return &ProcessResult{
			Kind:   ProcessProceed,
			Prompt: input,
		}, nil
	}

	return app.ProcessInput(ctx, input)
}

// FormatLocalResult formats a ProcessLocal result into a display string.
// @MX:NOTE This helper converts dispatcher messages to TUI display format.
func FormatLocalResult(result *ProcessResult) string {
	if result == nil || len(result.Messages) == 0 {
		return ""
	}

	var output string
	for _, msg := range result.Messages {
		if msg.Type == "message" {
			output += msg.Content + "\n"
		}
	}

	return output
}

// ShouldExit checks if the result indicates the TUI should exit.
// @MX:NOTE This helper checks for ProcessExit kind.
func ShouldExit(result *ProcessResult) bool {
	return result != nil && result.Kind == ProcessExit
}

// ShouldAbort checks if the result indicates the input was rejected.
// @MX:NOTE This helper checks for ProcessAbort kind.
func ShouldAbort(result *ProcessResult) bool {
	return result != nil && result.Kind == ProcessAbort
}

// GetPrompt returns the prompt to send to the daemon.
// @MX:NOTE This helper extracts the prompt from ProcessProceed result.
func GetPrompt(result *ProcessResult) string {
	if result == nil {
		return ""
	}
	return result.Prompt
}

// DispatchSlashCmd processes a slash command through the App dispatcher.
// @MX:ANCHOR This function is the main entry point for dispatcher-based slash command handling.
// @MX:REASON: Called by TUI model when slash command is detected - fan_in >= 2.
func DispatchSlashCmd(app AppInterface, ctx context.Context, cmd SlashCmd, m *Model) (string, *ProcessResult, error) {
	if app == nil {
		// No app available, fall back to legacy handling
		return "", nil, fmt.Errorf("no app available for dispatch")
	}

	// Reconstruct slash command string
	input := "/" + cmd.Name
	for _, arg := range cmd.Args {
		input += " " + arg
	}

	result, err := DispatchInput(app, ctx, input)
	if err != nil {
		return "", nil, fmt.Errorf("dispatch failed: %w", err)
	}

	// Format local response
	response := FormatLocalResult(result)

	return response, result, nil
}
