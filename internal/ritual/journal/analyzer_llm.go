package journal

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

// systemPromptLLMAnalyzer is the exact system prompt required by REQ-017.
// Changing this literal requires a SPEC amendment.
const systemPromptLLMAnalyzer = "다음 일기의 감정을 VAD 모델로 분석하고 Top-3 감정 태그를 JSON으로 반환하세요. 분석 결과 외에 어떤 조언이나 해석도 포함하지 마세요."

// clinicalRejectKeywords are Korean clinical/advisory terms that must never appear in
// LLM responses. Responses containing any of these trigger a silent reject + local fallback.
// REQ-020, AC-023.
var clinicalRejectKeywords = []string{
	"진단", "우울증", "우울", "장애", "처방", "상담받으세요", "치료", "증상", "평가",
}

// ErrLLMParseFail is returned when the LLM response cannot be parsed as valid JSON.
var ErrLLMParseFail = errors.New("llm response JSON parse failed")

// ErrLLMResponseInvalid is returned when the LLM response contains rejected content.
var ErrLLMResponseInvalid = errors.New("llm response contains invalid content")

// LLMClient is the minimal interface for invoking an LLM with a system prompt and user text.
// Only the text field is sent as userText — no user_id, date, attachment, or personal metadata.
//
// @MX:ANCHOR: [AUTO] LLM client interface for journal emotion analysis
// @MX:REASON: Implemented by mock in tests; consumed by LLMEmotionAnalyzer — fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-JOURNAL-001 REQ-017, AC-020
type LLMClient interface {
	// Invoke sends systemPrompt and userText to the LLM and returns the raw response.
	// Only entry.Text is passed as userText — never user_id, date, or attachments.
	Invoke(ctx context.Context, systemPrompt, userText string) (string, error)
}

// llmVadResponse is the expected JSON structure returned by the LLM.
type llmVadResponse struct {
	Valence   float64  `json:"valence"`
	Arousal   float64  `json:"arousal"`
	Dominance float64  `json:"dominance"`
	Tags      []string `json:"tags"`
}

// LLMEmotionAnalyzer implements EmotionAnalyzer by invoking an LLM for deeper analysis.
// When the LLM is unavailable, parse fails, or clinical keywords are detected in the response,
// it silently falls back to the LocalDictAnalyzer.
//
// @MX:ANCHOR: [AUTO] LLM-assisted emotion analyzer with privacy invariants
// @MX:REASON: Called by writer.Write when EmotionLLMAssisted=true; fan_in >= 3 (writer + summary + tests)
// @MX:SPEC: SPEC-GOOSE-JOURNAL-001 REQ-017, AC-020, AC-023
type LLMEmotionAnalyzer struct {
	client        LLMClient
	localFallback *LocalDictAnalyzer
	auditor       *journalAuditWriter
}

// NewLLMEmotionAnalyzer constructs an LLMEmotionAnalyzer.
// If client is nil, Analyze always falls back to localFallback.
// If auditor is nil, audit events are discarded.
func NewLLMEmotionAnalyzer(client LLMClient, localFallback *LocalDictAnalyzer, auditor *journalAuditWriter) *LLMEmotionAnalyzer {
	if localFallback == nil {
		localFallback = NewLocalDictAnalyzer()
	}
	return &LLMEmotionAnalyzer{
		client:        client,
		localFallback: localFallback,
		auditor:       auditor,
	}
}

// Analyze implements EmotionAnalyzer.
//
// Privacy invariants (AC-020):
//   - Only entry text is sent to the LLM via userText parameter.
//   - user_id, date, attachment_paths, emoji_mood, private_mode, allow_lora_training
//     are NEVER included in the LLM payload.
//
// Fallback triggers:
//   - LLM client nil or Invoke error → local fallback (silent)
//   - Invalid JSON in response → local fallback + audit "llm_parse_fail"
//   - Clinical keyword in response → local fallback + audit "llm_clinical_reject"
//
// @MX:WARN: [AUTO] Multiple fallback branches — each must preserve the silent-fail invariant
// @MX:REASON: User-visible errors from LLM failures break the privacy-first UX contract (REQ-017)
func (a *LLMEmotionAnalyzer) Analyze(ctx context.Context, text, emojiMood string) (*Vad, []string, error) {
	// Nil client → unconditional local fallback (no error logged to user).
	if a.client == nil {
		return a.localFallback.Analyze(ctx, text, emojiMood)
	}

	// PRIVACY CRITICAL: only text is sent. No user_id, date, attachment, emoji, or private_mode.
	resp, err := a.client.Invoke(ctx, systemPromptLLMAnalyzer, text)
	if err != nil {
		// LLM call failed — silent fallback, no user-visible error.
		return a.localFallback.Analyze(ctx, text, emojiMood)
	}

	// Reject response if any clinical keyword is present.
	if containsClinicalKeyword(resp) {
		a.auditor.emitOperation("llm_clinical_reject", "", "ok")
		return a.localFallback.Analyze(ctx, text, emojiMood)
	}

	// Parse JSON response.
	var parsed llmVadResponse
	if jsonErr := json.Unmarshal([]byte(resp), &parsed); jsonErr != nil {
		a.auditor.emitOperation("llm_parse_fail", "", "ok")
		// Silent fallback — do not surface parse error to caller.
		return a.localFallback.Analyze(ctx, text, emojiMood)
	}

	vad := &Vad{
		Valence:   clamp01(parsed.Valence),
		Arousal:   clamp01(parsed.Arousal),
		Dominance: clamp01(parsed.Dominance),
	}

	// Limit tags to top 3.
	tags := parsed.Tags
	if len(tags) > 3 {
		tags = tags[:3]
	}

	return vad, tags, nil
}

// containsClinicalKeyword reports whether s contains any of the clinical reject keywords.
func containsClinicalKeyword(s string) bool {
	for _, kw := range clinicalRejectKeywords {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}
