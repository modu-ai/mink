package journal

import "strings"

// crisisKeywords lists the direct Korean expressions associated with suicidal ideation.
// Only exact substring matches are used in v0.1 (indirect expressions deferred to v0.2+).
// research.md §4.1
var crisisKeywords = []string{
	"죽고 싶",
	"자살",
	"사라지고 싶",
	"살기 싫",
	"끝내고 싶",
	"없어지고 싶",
	"목숨을 끊",
	"스스로 목숨",
}

// CrisisResponse is the literal canned reply emitted when a crisis keyword is detected.
// It must include all three hotline numbers: 1577-0199, 1393, 1388.
// research.md §4.3 — No clinical language, no diagnosis, no advice.
const CrisisResponse = `지금 많이 힘드시죠. 말씀해주셔서 고마워요.

혼자 감당하기 어려운 순간에는 전문가의 도움이 큰 힘이 되어요:
- 생명의전화: 1577-0199 (24시간, 무료)
- 자살예방상담전화: 1393 (24시간)
- 청소년전화: 1388

당신의 이야기를 진심으로 들어줄 사람들이 있어요.`

// CrisisDetector checks text for crisis keywords using case-insensitive substring matching.
type CrisisDetector struct{}

// NewCrisisDetector returns a CrisisDetector ready for use.
func NewCrisisDetector() *CrisisDetector {
	return &CrisisDetector{}
}

// Check reports whether text contains any crisis keyword.
// Matching is case-insensitive but requires exact substring (no stemming in v0.1).
//
// @MX:ANCHOR: [AUTO] Crisis detection gateway for all journal write operations
// @MX:REASON: Called by JournalWriter.Write, orchestrator, and crisis tests — fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-JOURNAL-001 REQ-015, AC-005
func (c *CrisisDetector) Check(text string) bool {
	lower := strings.ToLower(text)
	for _, kw := range crisisKeywords {
		if strings.Contains(lower, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}
