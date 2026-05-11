package journal

import (
	"context"
	"encoding/json"
	"strings"
)

// llmSummaryRequest is the JSON payload sent to the LLM for weekly summary enhancement.
// Only aggregated statistics are included — no raw entry text. REQ-018, AC-020.
type llmSummaryRequest struct {
	AvgValence float64        `json:"avg_valence"`
	TopTags    []string       `json:"top_tags"`
	WordFreq   map[string]int `json:"word_freq"`
}

// llmSummaryResponse is the expected JSON structure returned by the LLM.
type llmSummaryResponse struct {
	OneLiner string `json:"one_liner"`
}

// systemPromptLLMSummary is the prompt for LLM-assisted weekly summary.
const systemPromptLLMSummary = "다음 주간 감정 통계를 바탕으로 한국어로 1문장 자연어 요약을 JSON {\"one_liner\":\"...\"}으로 반환하세요. 분석 결과 외에 어떤 조언이나 해석도 포함하지 마세요."

// LLMSummaryEnhancer generates a natural-language one-liner for a WeeklySummary.
// Only aggregated VAD + top tags + word frequency are sent to the LLM — no raw entry text.
// Clinical keyword rejection and parse-fail fallback are both silent.
//
// @MX:ANCHOR: [AUTO] LLM-assisted weekly summary enhancement
// @MX:REASON: Called by SummaryJob.RunWeekly, orchestrator, and tests — fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-JOURNAL-001 REQ-018, AC-020
type LLMSummaryEnhancer struct {
	client  LLMClient
	auditor *journalAuditWriter
}

// NewLLMSummaryEnhancer constructs an LLMSummaryEnhancer.
// If client is nil, EnhanceWeeklySummary is a no-op and returns the original summary.
func NewLLMSummaryEnhancer(client LLMClient, auditor *journalAuditWriter) *LLMSummaryEnhancer {
	return &LLMSummaryEnhancer{
		client:  client,
		auditor: auditor,
	}
}

// EnhanceWeeklySummary adds a natural-language one-liner to the given WeeklySummary.
// The input payload contains only aggregated statistics (avg_valence, top_tags, word_freq).
// Raw entry text is NEVER sent to the LLM.
//
// Fallback triggers:
//   - client nil → return original summary unchanged
//   - LLM error → return original summary unchanged
//   - JSON parse fail → return original summary unchanged
//   - Clinical keyword in response → return original summary unchanged
//
// On success, summary.OneLiner is populated and audit event is emitted.
func (e *LLMSummaryEnhancer) EnhanceWeeklySummary(ctx context.Context, summary *WeeklySummary) (*WeeklySummary, error) {
	if e.client == nil || summary == nil {
		return summary, nil
	}

	// Build the word frequency map from the word cloud tokens.
	// This is aggregated data only — no raw entry text.
	wordFreq := buildWordFreq(summary.WordCloud)

	req := llmSummaryRequest{
		AvgValence: summary.AvgValence,
		TopTags:    summary.TopTags,
		WordFreq:   wordFreq,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		// Serialisation failure — return original summary.
		return summary, nil
	}

	resp, err := e.client.Invoke(ctx, systemPromptLLMSummary, string(payload))
	if err != nil {
		return summary, nil
	}

	// Reject response if any clinical keyword is present.
	if containsClinicalKeyword(resp) {
		return summary, nil
	}

	// Parse the JSON response.
	var parsed llmSummaryResponse
	if jsonErr := json.Unmarshal([]byte(resp), &parsed); jsonErr != nil {
		return summary, nil
	}

	if strings.TrimSpace(parsed.OneLiner) == "" {
		return summary, nil
	}

	// Clone the summary to avoid mutating the original.
	enhanced := *summary
	enhanced.OneLiner = parsed.OneLiner

	e.auditor.emit("weekly_summary_llm_enhanced", map[string]string{
		"user_id_hash": hashUserID(summary.UserID),
	}, "ok")

	return &enhanced, nil
}

// buildWordFreq converts a word cloud slice into a frequency map.
// This is a best-effort reconstruction from the top-N token list.
// The frequency values are positional weights (rank-based).
func buildWordFreq(wordCloud []string) map[string]int {
	freq := make(map[string]int, len(wordCloud))
	for i, tok := range wordCloud {
		// Assign descending rank-based weight: first token gets highest weight.
		freq[tok] = len(wordCloud) - i
	}
	return freq
}
