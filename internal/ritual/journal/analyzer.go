package journal

import (
	"context"
	"sort"
	"strings"
)

// EmotionAnalyzer analyses the text of a journal entry and returns a VAD triple
// plus a ranked list of emotion category labels.
//
// @MX:ANCHOR: [AUTO] Core emotion analysis interface for journal write pipeline
// @MX:REASON: Implemented by LocalDictAnalyzer, consumed by writer.Write and tests — fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-JOURNAL-001 REQ-006
type EmotionAnalyzer interface {
	// Analyze returns (vad, top3tags, err) for the given text and optional emoji mood.
	// An empty text returns the neutral fallback {0.5, 0.5, 0.5} with no tags.
	Analyze(ctx context.Context, text, emojiMood string) (*Vad, []string, error)
}

// LocalDictAnalyzer implements EmotionAnalyzer using the hardcoded Korean emotion dictionary.
// No external calls are made; this is the M1/M2 default analyzer.
// research.md §2
type LocalDictAnalyzer struct{}

// NewLocalDictAnalyzer returns a ready-to-use LocalDictAnalyzer.
func NewLocalDictAnalyzer() *LocalDictAnalyzer {
	return &LocalDictAnalyzer{}
}

// neutralVad is the fallback VAD triple when no keywords or emoji are matched.
var neutralVad = Vad{Valence: 0.5, Arousal: 0.5, Dominance: 0.5}

// emojiVadDelta maps individual emoji to a valence delta applied on top of keyword matching.
// Emoji with clearly negative sentiment contribute a negative delta.
var emojiVadDelta = map[string]float64{
	"😊": +0.1, "😄": +0.1, "🥰": +0.1, "😁": +0.1, "🎉": +0.05,
	"😢": -0.2, "😭": -0.25, "😔": -0.2, "😞": -0.15, "💔": -0.2,
	"😰": -0.15, "😟": -0.1, "😨": -0.15,
	"😠": -0.15, "😡": -0.2, "🤬": -0.25,
	"😴": -0.05, "🥱": -0.05, "😪": -0.05,
	"🤩": +0.15, "🥳": +0.15, "🎊": +0.1,
	"🙏": +0.1, "🥹": +0.1, "❤️": +0.1,
	"🥺": -0.15, "😶": -0.05,
	"😣": -0.1, "😑": -0.05, "🙄": -0.05,
	"💪": +0.1, "🏆": +0.15, "✨": +0.1,
}

// tagHit accumulates match information for a single emotion category.
type tagHit struct {
	tag   string
	count int
	vad   Vad
}

// Analyze implements EmotionAnalyzer using the local keyword dictionary.
//
// Algorithm (research.md §2):
//  1. Tokenise text by whitespace.
//  2. For each token, check against each emotion category's keyword list.
//  3. Apply negation window (5 tokens before keyword position).
//  4. Apply intensity modifier window (5 tokens before keyword position).
//  5. Aggregate hit counts per category.
//  6. Select top-3 categories by hit count.
//  7. Compute weighted-average VAD from the top categories.
//  8. Apply emoji bonus from emojiMood and inline emoji in text.
//  9. Clamp all VAD values to [0, 1].
//
// @MX:WARN: [AUTO] Cyclomatic complexity approaching threshold due to negation/intensity branches
// @MX:REASON: Algorithm is intentionally multi-step per research.md §2; refactor would break research alignment
func (a *LocalDictAnalyzer) Analyze(_ context.Context, text, emojiMood string) (*Vad, []string, error) {
	if strings.TrimSpace(text) == "" && emojiMood == "" {
		v := neutralVad
		return &v, nil, nil
	}

	// Tokenise by whitespace for keyword matching.
	tokens := strings.Fields(text)

	// hitMap accumulates per-category hit counts.
	hitMap := make(map[string]*tagHit, len(emotionDict))

	for i, token := range tokens {
		lToken := strings.ToLower(token)

		for category, entry := range emotionDict {
			for _, kw := range entry.Keywords {
				if !strings.Contains(lToken, strings.ToLower(kw)) {
					continue
				}
				// Keyword matched. Check negation window (5 tokens before).
				negated := isNegated(tokens, i)
				// Check intensity modifier window (5 tokens before).
				intense := isIntensified(tokens, i)

				h, ok := hitMap[category]
				if !ok {
					vad := entry.Vad
					h = &tagHit{tag: category, vad: vad}
					hitMap[category] = h
				}
				h.count++

				if negated {
					// Flip valence and reduce dominance to signal loss of control.
					h.vad.Valence = 1.0 - h.vad.Valence
					h.vad.Dominance = max(0, h.vad.Dominance-0.2)
				}
				if intense {
					// Amplify arousal (capped at 1.0).
					h.vad.Arousal = min(1.0, h.vad.Arousal*1.2)
				}
			}
		}
	}

	// Check inline emoji within text and the explicit emojiMood parameter.
	emojiValenceDelta := 0.0
	for emoji, delta := range emojiVadDelta {
		if strings.Contains(text, emoji) || strings.Contains(emojiMood, emoji) {
			emojiValenceDelta += delta
		}
	}

	if len(hitMap) == 0 {
		// No keyword match; return neutral + apply any emoji adjustment.
		v := neutralVad
		v.Valence = clamp01(v.Valence + emojiValenceDelta)
		return &v, nil, nil
	}

	// Sort categories by hit count descending, then alphabetically for stability.
	hits := make([]*tagHit, 0, len(hitMap))
	for _, h := range hitMap {
		hits = append(hits, h)
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].count != hits[j].count {
			return hits[i].count > hits[j].count
		}
		return hits[i].tag < hits[j].tag
	})

	// Top-3 categories.
	top := hits
	if len(top) > 3 {
		top = top[:3]
	}

	// Weighted-average VAD across top categories.
	totalCount := 0
	for _, h := range top {
		totalCount += h.count
	}
	var avgVad Vad
	for _, h := range top {
		w := float64(h.count) / float64(totalCount)
		avgVad.Valence += w * h.vad.Valence
		avgVad.Arousal += w * h.vad.Arousal
		avgVad.Dominance += w * h.vad.Dominance
	}

	// Apply emoji bonus on valence.
	avgVad.Valence = clamp01(avgVad.Valence + emojiValenceDelta)
	avgVad.Arousal = clamp01(avgVad.Arousal)
	avgVad.Dominance = clamp01(avgVad.Dominance)

	tags := make([]string, len(top))
	for i, h := range top {
		tags[i] = h.tag
	}

	return &avgVad, tags, nil
}

// isNegated reports whether any negation token appears within 5 tokens before or after position i.
// Korean negation can precede ("안 행복해") or follow ("행복하지 않아") the keyword.
func isNegated(tokens []string, i int) bool {
	start := max(0, i-5)
	end := min(len(tokens), i+6)
	window := append(tokens[start:i], tokens[i+1:end]...)
	for _, t := range window {
		for _, neg := range negationTokens {
			if strings.Contains(t, neg) {
				return true
			}
		}
	}
	return false
}

// isIntensified reports whether any intensity modifier appears within 5 tokens before position i.
func isIntensified(tokens []string, i int) bool {
	start := max(0, i-5)
	for _, t := range tokens[start:i] {
		for _, mod := range intensityTokens {
			if strings.Contains(t, mod) {
				return true
			}
		}
	}
	return false
}

// clamp01 restricts v to the closed interval [0, 1].
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
