package briefing

import (
	"strings"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/ritual/journal"
)

// TestCheckCrisis_PositiveAndNegative validates the underlying detector
// integration.
func TestCheckCrisis_PositiveAndNegative(t *testing.T) {
	positive := []string{
		"오늘 너무 힘들어서 죽고 싶다",
		"자살하고 싶은 생각이 든다",
		"사라지고 싶어",
		"살기 싫어요",
	}
	for _, p := range positive {
		if !CheckCrisis(p) {
			t.Errorf("CheckCrisis(%q) = false, want true", p)
		}
	}
	negative := []string{
		"오늘도 한 걸음",
		"Every day is a new beginning",
		"맛있는 저녁을 먹었다",
		"",
	}
	for _, n := range negative {
		if CheckCrisis(n) {
			t.Errorf("CheckCrisis(%q) = true, want false", n)
		}
	}
}

// TestPrependCrisisResponseIfDetected_DetectedPathPrepends covers
// AC-009 invariant 6 happy path: the hotline canned response is the very
// first line of the output when a crisis keyword is detected.
func TestPrependCrisisResponseIfDetected_DetectedPathPrepends(t *testing.T) {
	rendered := "MORNING BRIEFING\n\nMantra: 죽고 싶다\n"
	out := PrependCrisisResponseIfDetected(rendered)

	if !strings.HasPrefix(out, journal.CrisisResponse) {
		t.Errorf("output does not start with CrisisResponse.\nout:\n%s", out)
	}
	for _, hotline := range []string{"1577-0199", "1393", "1388"} {
		if !strings.Contains(out, hotline) {
			t.Errorf("output missing hotline %q\nout:\n%s", hotline, out)
		}
	}
	if !strings.Contains(out, rendered) {
		t.Errorf("original rendered briefing missing from output:\n%s", out)
	}
}

// TestPrependCrisisResponseIfDetected_NoCrisisPassthrough verifies the
// unchanged-pass when no keyword is present.
func TestPrependCrisisResponseIfDetected_NoCrisisPassthrough(t *testing.T) {
	rendered := "MORNING BRIEFING\n\nMantra: 오늘도 한 걸음\n"
	out := PrependCrisisResponseIfDetected(rendered)
	if out != rendered {
		t.Errorf("expected unchanged passthrough, got:\n%s", out)
	}
}

// TestPrependCrisisResponse_NoAnalyticalCommentary asserts AC-009 invariant 6
// "shall not include analytical commentary": the prepend MUST NOT include
// clinical / diagnostic / interpretive vocabulary.
func TestPrependCrisisResponse_NoAnalyticalCommentary(t *testing.T) {
	rendered := "Mantra: 자살이라는 단어가 들어간 텍스트"
	out := PrependCrisisResponseIfDetected(rendered)

	// JOURNAL CrisisResponse has been audited (journal/crisis_test.go
	// TestCrisis_NoClinicalLanguage). Re-verify here to defend against
	// future drift in the source canned text.
	clinical := []string{
		"depression", "Depression", "depressive",
		"diagnosis", "diagnose",
		"우울증", "정신질환", "공황", "PTSD",
	}
	for _, c := range clinical {
		if strings.Contains(out, c) {
			t.Errorf("crisis prepend leaked clinical token %q\nout:\n%s", c, out)
		}
	}

	// Spot-check the canned text contains only the supportive lines.
	if !strings.Contains(out, "혼자 감당하기 어려운") {
		t.Errorf("expected supportive line, got:\n%s", out)
	}
}

// TestPayloadHasCrisis_Detection covers the early-detection helper across
// the visible payload fields.
func TestPayloadHasCrisis_Detection(t *testing.T) {
	// Mantra-triggered
	p1 := &BriefingPayload{
		Mantra: MantraModule{Text: "죽고 싶다"},
	}
	if !PayloadHasCrisis(p1) {
		t.Error("mantra crisis not detected")
	}

	// LLMSummary-triggered (M3)
	p2 := &BriefingPayload{
		Mantra:     MantraModule{Text: "ok"},
		LLMSummary: "자살이라는 단어가 들어간 요약",
	}
	if !PayloadHasCrisis(p2) {
		t.Error("LLMSummary crisis not detected")
	}

	// AnniversaryEntry-triggered (defensive scan)
	p3 := &BriefingPayload{
		Mantra: MantraModule{Text: "ok"},
		JournalRecall: RecallModule{
			Anniversaries: []*AnniversaryEntry{
				{YearsAgo: 1, Date: "2025-05-14", Text: "살기 싫다고 생각했다"},
			},
		},
	}
	if !PayloadHasCrisis(p3) {
		t.Error("anniversary crisis not detected")
	}

	// Negative case
	p4 := &BriefingPayload{
		Mantra: MantraModule{Text: "오늘도 한 걸음"},
		JournalRecall: RecallModule{
			Anniversaries: []*AnniversaryEntry{
				{YearsAgo: 1, Date: "2025-05-14", Text: "맛있는 저녁을 먹었다"},
			},
		},
		LLMSummary:  "오늘은 맑은 하루",
		GeneratedAt: time.Now(),
	}
	if PayloadHasCrisis(p4) {
		t.Error("false positive on benign payload")
	}

	if PayloadHasCrisis(nil) {
		t.Error("nil payload should be false")
	}
}
