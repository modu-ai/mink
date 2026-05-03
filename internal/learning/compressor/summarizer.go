package compressor

import (
	"bytes"
	"context"
	"errors"
	"math/rand/v2"
	"text/template"
	"time"

	"github.com/modu-ai/goose/internal/learning/trajectory"
)

// Summarizer abstracts the LLM call used to summarize middle trajectory sections.
// REQ-COMPRESSOR-OUT: actual HTTP calls are delegated to ADAPTER-001 / ROUTER-001.
type Summarizer interface {
	// Summarize returns a summary of turns in at most maxTokens tokens.
	// The returned string is the summary body only (no wrapper).
	Summarize(ctx context.Context, turns []trajectory.TrajectoryEntry, maxTokens int) (string, error)
}

// Sentinel errors returned by the compressor.
var (
	// ErrTransient indicates a retriable error (HTTP 429/503, network timeout).
	ErrTransient = errors.New("summarizer: transient error")
	// ErrPermanent indicates a non-retriable error (HTTP 4xx except 429, auth failure).
	ErrPermanent = errors.New("summarizer: permanent error")
	// ErrSummarizerOvershot indicates the summary exceeded 2x SummaryTargetTokens.
	ErrSummarizerOvershot = errors.New("summarizer: response exceeded 2x max tokens")
	// ErrCompressionFailed indicates all retries were exhausted.
	ErrCompressionFailed = errors.New("compression failed after retries")
)

// defaultPromptTemplate is the built-in summarization prompt.
// Supports variables: .Turns ([]trajectory.TrajectoryEntry), .ModelName (string), .TargetTokens (int).
const defaultPromptTemplate = `You are summarizing a middle section of an AI agent's tool-augmented conversation.
The summary will replace the middle turns in a trajectory. The head and tail are preserved.

CONSTRAINTS:
- Maximum {{.TargetTokens}} tokens.
- Preserve: tool names invoked, error outcomes, key decisions, file paths mentioned.
- Drop: verbose tool output bodies, boilerplate assistant explanations.
- Format: 3-7 bullet points, each starting with a verb.

TURNS TO SUMMARIZE:
{{range .Turns}}
[{{.From}}] {{.Value}}
{{end}}

SUMMARY:`

// promptData is the template data struct.
type promptData struct {
	Turns        []trajectory.TrajectoryEntry
	ModelName    string
	TargetTokens int
}

// buildPrompt renders the summarization prompt from the given template string (or
// the built-in default) and returns the rendered string.
func buildPrompt(tmplStr string, turns []trajectory.TrajectoryEntry, modelName string, targetTokens int) (string, error) {
	if tmplStr == "" {
		tmplStr = defaultPromptTemplate
	}
	tmpl, err := template.New("summary").Parse(tmplStr)
	if err != nil {
		return "", err
	}
	data := promptData{
		Turns:        turns,
		ModelName:    modelName,
		TargetTokens: targetTokens,
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// isTransient reports whether an error should trigger a retry.
func isTransient(err error) bool {
	return errors.Is(err, ErrTransient)
}

// summarizeWithRetry calls summarizer.Summarize with jittered exponential backoff.
// Returns (summary, apiCalls, errCount, finalErr).
// REQ-COMPRESSOR-007/008
func summarizeWithRetry(
	ctx context.Context,
	summarizer Summarizer,
	turns []trajectory.TrajectoryEntry,
	maxTokens int,
	maxRetries int,
	baseDelay time.Duration,
) (summary string, apiCalls int, errCount int, finalErr error) {
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Jittered exponential backoff: delay = baseDelay * 2^(attempt-1) * rand[0.5, 1.5)
			// attempt 1 (first retry): baseDelay * 1 * jitter → [0.5*base, 1.5*base]
			// attempt 2             : baseDelay * 2 * jitter → [1.0*base, 3.0*base]
			// attempt 3             : baseDelay * 4 * jitter → [2.0*base, 6.0*base]
			exp := int64(1) << (attempt - 1) // 2^(attempt-1)
			jitter := 0.5 + rand.Float64()   // [0.5, 1.5)
			delay := time.Duration(float64(baseDelay) * float64(exp) * jitter)
			select {
			case <-ctx.Done():
				finalErr = ctx.Err()
				return
			case <-time.After(delay):
			}
		}

		var s string
		var callErr error
		s, callErr = summarizer.Summarize(ctx, turns, maxTokens)
		apiCalls++

		if callErr == nil {
			summary = s
			finalErr = nil // clear any previous transient error
			return
		}
		errCount++
		if !isTransient(callErr) {
			// Non-retriable error: stop immediately.
			finalErr = callErr
			return
		}
		finalErr = callErr
	}
	// All attempts exhausted.
	finalErr = errors.Join(ErrCompressionFailed, finalErr)
	return
}
