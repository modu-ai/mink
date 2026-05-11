package journal

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCrisis_DirectKeyword_Match verifies that all eight direct crisis expressions
// are detected as crisis signals. AC-005
func TestCrisis_DirectKeyword_Match(t *testing.T) {
	t.Parallel()
	detector := NewCrisisDetector()

	cases := []struct {
		text string
	}{
		{"오늘 정말 죽고 싶다"},
		{"자살하고 싶어"},
		{"사라지고 싶어요"},
		{"살기 싫어"},
		{"모든 걸 끝내고 싶다"},
		{"없어지고 싶어"},
		{"목숨을 끊고 싶어"},
		{"스스로 목숨을 끊을 생각이 들어"},
	}

	for _, tc := range cases {
		t.Run(tc.text, func(t *testing.T) {
			t.Parallel()
			assert.True(t, detector.Check(tc.text), "should detect crisis in: %q", tc.text)
		})
	}
}

// TestCrisis_NoFalsePositive_HappyText verifies that positive text is not flagged.
func TestCrisis_NoFalsePositive_HappyText(t *testing.T) {
	t.Parallel()
	detector := NewCrisisDetector()

	happyTexts := []string{
		"오늘 정말 행복했어, 오랜만에 웃었어",
		"친구들이랑 맛있는 거 먹었다",
		"열심히 운동했더니 기분이 좋아",
		"오늘 발표 잘 끝났다!",
		"고양이가 너무 귀여워서 힐링됐어",
	}

	for _, text := range happyTexts {
		t.Run(text, func(t *testing.T) {
			t.Parallel()
			assert.False(t, detector.Check(text), "should NOT detect crisis in: %q", text)
		})
	}
}

// TestCrisis_CaseInsensitive verifies detection is case-insensitive.
func TestCrisis_CaseInsensitive(t *testing.T) {
	t.Parallel()
	detector := NewCrisisDetector()

	// Korean does not have case, but ASCII parts of keywords should not affect matching.
	assert.True(t, detector.Check("자살 생각이 들어"), "lowercase keyword must match")
	assert.True(t, detector.Check("SELF-HARM ASIDE: 죽고 싶다"), "mixed ASCII must still match Korean keyword")
}

// TestCrisis_CannedResponseHasHotline verifies that CrisisResponse contains all
// three required hotline numbers. AC-005, research.md §4.3
func TestCrisis_CannedResponseHasHotline(t *testing.T) {
	t.Parallel()

	assert.Contains(t, CrisisResponse, "1577-0199", "must include 생명의전화")
	assert.Contains(t, CrisisResponse, "1393", "must include 자살예방상담전화")
	assert.Contains(t, CrisisResponse, "1388", "must include 청소년전화")
}

// TestCrisis_NoClinicalLanguage verifies that CrisisResponse avoids clinical vocabulary.
// AC-023 — no "진단", "우울", "치료", "처방", "PHQ" in the canned response.
func TestCrisis_NoClinicalLanguage(t *testing.T) {
	t.Parallel()

	forbiddenTerms := []string{"진단", "우울", "치료", "처방", "PHQ"}
	for _, term := range forbiddenTerms {
		assert.False(t, strings.Contains(CrisisResponse, term),
			"CrisisResponse must not contain clinical term: %q", term)
	}
}
