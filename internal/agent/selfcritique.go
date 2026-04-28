// Package agent provides the outer orchestration layer for the Plan-Run-Reflect cycle.
// SPEC-GOOSE-SELF-CRITIQUE-001
package agent

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/modu-ai/goose/internal/message"
	"github.com/modu-ai/goose/internal/query"
	"github.com/modu-ai/goose/internal/query/loop"
	"go.uber.org/zap"
)

// SelfCritiqueConfig configures the self-critique reflect hook.
type SelfCritiqueConfig struct {
	// LLMCall is the LLM API function. Required.
	LLMCall query.LLMCallFunc
	// Logger receives structured output.
	Logger *zap.Logger
}

// NewSelfCritiqueHook creates a ReflectHook that performs 3-dimension self-critique.
// SPEC-GOOSE-SELF-CRITIQUE-001
//
// @MX:ANCHOR: [AUTO] Factory for LLM-based self-critique hooks
// @MX:REASON: SPEC-GOOSE-SELF-CRITIQUE-001 - 3-dimension critique (gap, inconsistency, unsupported)
func NewSelfCritiqueHook(cfg SelfCritiqueConfig) (ReflectHook, error) {
	if cfg.LLMCall == nil {
		return nil, fmt.Errorf("LLMCall is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = zap.NewNop()
	}

	hook := func(ctx context.Context, task Task, finalState loop.State) (*ReflectResult, error) {
		cfg.Logger.Info("self-critique started",
			zap.String("task_id", task.ID),
			zap.Int("messages_count", len(finalState.Messages)),
		)

		// Build critique prompt
		critiquePrompt := buildCritiquePrompt(task.Prompt, finalState.Messages)

		// Call LLM
		messages := []message.Message{
			{
				Role: "user",
				Content: []message.ContentBlock{
					{Type: "text", Text: critiquePrompt},
				},
			},
		}
		req := query.LLMCallReq{Messages: messages}
		ch, err := cfg.LLMCall(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("critique LLM call failed: %w", err)
		}

		// Collect response
		var output strings.Builder
		for evt := range ch {
			// Collect all text delta events
			if evt.Type == "content_block_delta" && evt.Delta != "" {
				output.WriteString(evt.Delta)
			}
		}

		rawOutput := output.String()
		cfg.Logger.Debug("self-critique LLM response",
			zap.String("task_id", task.ID),
			zap.String("response", rawOutput),
		)

		// Parse response
		result := parseCritiqueOutput(rawOutput)

		cfg.Logger.Info("self-critique completed",
			zap.String("task_id", task.ID),
			zap.Float64("score", result.Score),
		)

		return result, nil
	}

	return hook, nil
}

// buildCritiquePrompt constructs the self-critique prompt.
// SPEC-GOOSE-SELF-CRITIQUE-001 §3.1 Critique Prompt
func buildCritiquePrompt(taskPrompt string, messages []message.Message) string {
	var sb strings.Builder

	sb.WriteString("You are an AI output quality evaluator. Analyze the following task execution and provide a critique.\n\n")
	sb.WriteString("## Task\n")
	sb.WriteString(taskPrompt)
	sb.WriteString("\n\n## Execution History\n")

	// Include last N messages to avoid token overflow
	maxMessages := 20
	startIdx := max(len(messages)-maxMessages, 0)

	for i := startIdx; i < len(messages); i++ {
		msg := messages[i]
		fmt.Fprintf(&sb, "### %s\n", msg.Role)
		for _, block := range msg.Content {
			if block.Type == "text" {
				sb.WriteString(block.Text)
				sb.WriteString("\n")
			}
		}
	}

	sb.WriteString("\n## Critique Format\n")
	sb.WriteString("Provide your critique in the following exact format:\n\n")
	sb.WriteString("SCORE: <0.0 to 1.0>\n")
	sb.WriteString("GAP: <what's missing or incomplete>\n")
	sb.WriteString("INCONSISTENCY: <any contradictions or logic errors>\n")
	sb.WriteString("UNSUPPORTED: <claims without evidence or reasoning>\n")

	return sb.String()
}

// parseCritiqueOutput parses the LLM response to extract critique dimensions.
// SPEC-GOOSE-SELF-CRITIQUE-001 §3.2 Response Parsing
//
// @MX:NOTE: [AUTO] Regex-based parsing - structured format assumed
// @MX:REASON: SPEC-GOOSE-SELF-CRITIQUE-001 - LLM output follows strict SCORE/GAP/INCONSISTENCY/UNSUPPORTED format
func parseCritiqueOutput(output string) *ReflectResult {
	result := &ReflectResult{
		RawOutput: output,
	}

	// Extract SCORE
	scoreRegex := regexp.MustCompile(`SCORE:\s*([0-9.]+)`)
	if matches := scoreRegex.FindStringSubmatch(output); len(matches) > 1 {
		if score, err := strconv.ParseFloat(matches[1], 64); err == nil {
			result.Score = score
		}
	}
	// Default score if parsing fails
	if result.Score == 0 {
		result.Score = 0.5
	}

	// Extract GAP (multiline)
	gapRegex := regexp.MustCompile(`GAP:\s*([\s\S]*?)(?:\nINCONSISTENCY:|\nUNSUPPORTED:|$)`)
	if matches := gapRegex.FindStringSubmatch(output); len(matches) > 1 {
		result.Gap = strings.TrimSpace(matches[1])
	}

	// Extract INCONSISTENCY (multiline)
	incRegex := regexp.MustCompile(`INCONSISTENCY:\s*([\s\S]*?)(?:\nUNSUPPORTED:|$)`)
	if matches := incRegex.FindStringSubmatch(output); len(matches) > 1 {
		result.Inconsistency = strings.TrimSpace(matches[1])
	}

	// Extract UNSUPPORTED (multiline to end)
	unsupRegex := regexp.MustCompile(`UNSUPPORTED:\s*([\s\S]*?)$`)
	if matches := unsupRegex.FindStringSubmatch(output); len(matches) > 1 {
		result.Unsupported = strings.TrimSpace(matches[1])
	}

	return result
}
