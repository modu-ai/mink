package journal

import "strings"

// promptVault holds all evening-prompt templates categorised by tone.
// Every template must:
//   - End with a question mark (open question principle).
//   - Not contain any of the forbidden phrases defined in AC-019.
//   - Not pressure the user or request sensitive personal information.
//
// research.md §7.3
var promptVault = map[string][]string{
	"neutral": {
		"오늘 하루 어떠셨어요?",
		"잠들기 전, 생각나는 순간이 있어요?",
		"기분 한 줄로 표현한다면?",
		"오늘 하루를 색깔로 표현하면 어떤 색이에요?",
	},
	"low_mood_sequence": {
		"요즘 많이 힘드시죠. 오늘은 어땠어요?",
		"힘든 날이 계속되고 있는 것 같아요. 오늘은 어떠셨어요?",
		"언제든 이야기해주세요. 오늘 하루는 어떠셨나요?",
	},
	"anniversary_happy": {
		"오늘은 [date_name]이네요. 어떻게 보내셨어요?",
		"특별한 날인 오늘, 어떤 하루였어요?",
		"오늘은 [date_name]! 오늘 하루 기억에 남는 게 있어요?",
	},
	"anniversary_sensitive": {
		"오늘은 특별한 날이죠. 어떻게 지내셨어요?",
		"오늘 하루 어떠셨어요?",
		"오늘 같은 날은 쉬어가도 좋아요. 어떻게 지내셨어요?",
	},
}

// PickNeutral returns a neutral prompt template deterministically selected by seed.
// Seed is typically the day-of-year so that the prompt rotates daily.
//
// @MX:ANCHOR: [AUTO] Primary prompt selector used by the orchestrator
// @MX:REASON: Called by orchestrator.Prompt, orchestrator tests, and integration test — fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-JOURNAL-001 REQ-013
func PickNeutral(seed int) string {
	templates := promptVault["neutral"]
	return templates[seed%len(templates)]
}

// PickLowMood returns a soft-tone prompt for users who have shown low valence recently.
// REQ-009, AC-015
func PickLowMood() string {
	templates := promptVault["low_mood_sequence"]
	// Always return the first soft-tone prompt for consistency.
	return templates[0]
}

// PickAnniversary returns an anniversary-aware prompt.
// dateName is the human-readable event label, e.g. "결혼기념일".
// If the anniversary has a negative emotional history the caller should
// use the "anniversary_sensitive" variant (M2 orchestrator decision).
func PickAnniversary(dateName string) string {
	templates := promptVault["anniversary_happy"]
	tpl := templates[0]
	return strings.ReplaceAll(tpl, "[date_name]", dateName)
}

// All returns every prompt template across all categories.
// Used by tests to verify invariants over the entire vault.
func All() []string {
	var out []string
	for _, templates := range promptVault {
		out = append(out, templates...)
	}
	return out
}
