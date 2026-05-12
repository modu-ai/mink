package insights

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/modu-ai/mink/internal/learning/trajectory"
)

// evidenceSnippetCap is the maximum rune length of an Evidence snippet (PII protection).
const evidenceSnippetCap = 50

// heuristicRule defines a single pattern detection rule for the analyzer.
type heuristicRule struct {
	category    InsightCategory
	title       string
	description string
	match       func(t *trajectory.Trajectory) bool
	narrative   func(n int) string
}

// shortResponseKeywords are terms that indicate a user preference for brevity.
var shortResponseKeywords = []string{
	"short", "brief", "concise", "더 짧게", "짧게", "간단히",
}

// Analyzer classifies trajectories into 4 qualitative categories.
// It uses deterministic heuristics; LLM calls are optional.
type Analyzer struct {
	rules []heuristicRule
}

// NewAnalyzer constructs an Analyzer with the default heuristic rule set.
func NewAnalyzer() *Analyzer {
	a := &Analyzer{}
	a.rules = defaultRules()
	return a
}

// defaultRules returns the standard heuristic rule set (REFACTOR target: data-driven table).
func defaultRules() []heuristicRule {
	return []heuristicRule{
		{
			category:    CategoryPreference,
			title:       "prefers_shorter_responses",
			description: "User frequently requests shorter or more concise responses",
			match: func(t *trajectory.Trajectory) bool {
				return containsKeywordInHuman(t, shortResponseKeywords)
			},
			narrative: func(n int) string {
				return fmt.Sprintf("User requested shorter responses %d time(s). Consider defaulting to concise output.", n)
			},
		},
	}
}

// AnalyzePatterns detects repeated behavioral patterns.
// Returns insights where the same prompt prefix (first 50 chars) occurs >= 3 times.
func (a *Analyzer) AnalyzePatterns(trajectories []*trajectory.Trajectory) []Insight {
	prefixCounts := make(map[string][]EvidenceRef)

	for _, t := range trajectories {
		for _, entry := range t.Conversations {
			if entry.From != trajectory.RoleHuman {
				continue
			}
			prefix := snippetOf(entry.Value)
			if prefix == "" {
				continue
			}
			prefixCounts[prefix] = append(prefixCounts[prefix], EvidenceRef{
				SessionID: t.SessionID,
				Timestamp: t.Timestamp,
				Snippet:   prefix,
			})
		}
	}

	var insights []Insight
	for prefix, refs := range prefixCounts {
		if len(refs) < 3 {
			continue
		}
		// Deduplicate evidence by session (keep first occurrence per session).
		dedupedRefs := deduplicateBySession(refs)
		if len(dedupedRefs) == 0 {
			continue
		}
		n := len(refs)
		conf := DefaultConfidence(n, 0.5)
		insights = append(insights, Insight{
			Category:     CategoryPattern,
			Title:        "repeated_prompt_pattern",
			Description:  fmt.Sprintf("Recurring prompt prefix: %q", prefix),
			Observations: n,
			Confidence:   conf,
			Evidence:     dedupedRefs,
			Narrative:    fmt.Sprintf("Prompt starting with %q was sent %d times. This may be a recurring workflow.", prefix, n),
			CreatedAt:    time.Now().UTC(),
		})
	}

	// Sort by confidence descending for determinism.
	sort.Slice(insights, func(i, j int) bool {
		return insights[i].Confidence > insights[j].Confidence
	})
	return insights
}

// AnalyzePreferences detects user preference signals.
func (a *Analyzer) AnalyzePreferences(trajectories []*trajectory.Trajectory) []Insight {
	// Group trajectories by heuristic rule match.
	ruleHits := make(map[string][]EvidenceRef)
	ruleMeta := make(map[string]heuristicRule)

	for _, rule := range a.rules {
		if rule.category != CategoryPreference {
			continue
		}
		for _, t := range trajectories {
			if !rule.match(t) {
				continue
			}
			snippet := firstHumanSnippet(t, shortResponseKeywords)
			ruleHits[rule.title] = append(ruleHits[rule.title], EvidenceRef{
				SessionID: t.SessionID,
				Timestamp: t.Timestamp,
				Snippet:   snippet,
			})
			ruleMeta[rule.title] = rule
		}
	}

	var insights []Insight
	for title, refs := range ruleHits {
		if len(refs) < 3 {
			continue
		}
		n := len(refs)
		rule := ruleMeta[title]
		conf := DefaultConfidence(n, 0.5)
		insights = append(insights, Insight{
			Category:     CategoryPreference,
			Title:        title,
			Description:  rule.description,
			Observations: n,
			Confidence:   conf,
			Evidence:     refs,
			Narrative:    rule.narrative(n),
			CreatedAt:    time.Now().UTC(),
		})
	}

	sort.Slice(insights, func(i, j int) bool {
		return insights[i].Confidence > insights[j].Confidence
	})
	return insights
}

// AnalyzeErrors aggregates failure reasons from failed trajectories.
func (a *Analyzer) AnalyzeErrors(trajectories []*trajectory.Trajectory) []Insight {
	reasonRefs := make(map[string][]EvidenceRef)

	for _, t := range trajectories {
		reason := t.Metadata.FailureReason
		if reason == "" {
			continue
		}
		reasonRefs[reason] = append(reasonRefs[reason], EvidenceRef{
			SessionID: t.SessionID,
			Timestamp: t.Timestamp,
			Snippet:   snippetOf(reason),
		})
	}

	var insights []Insight
	for reason, refs := range reasonRefs {
		n := len(refs)
		if n == 0 {
			continue
		}
		conf := DefaultConfidence(n, 0.5)
		insights = append(insights, Insight{
			Category:     CategoryError,
			Title:        reason,
			Description:  fmt.Sprintf("Failure reason %q occurred %d time(s)", reason, n),
			Observations: n,
			Confidence:   conf,
			Evidence:     refs,
			Narrative:    fmt.Sprintf("Failure %q happened %d time(s). Investigate root cause.", reason, n),
			CreatedAt:    time.Now().UTC(),
		})
	}

	// Sort by observation count descending (primary), confidence descending (secondary).
	sort.Slice(insights, func(i, j int) bool {
		if insights[i].Observations != insights[j].Observations {
			return insights[i].Observations > insights[j].Observations
		}
		return insights[i].Confidence > insights[j].Confidence
	})
	return insights
}

// AnalyzeOpportunities detects unused tools or improvement potential.
// availableTools is the list of tool names exposed by providers (e.g. MemoryManager).
// usedTools is the set of tools actually called in the trajectory set.
func (a *Analyzer) AnalyzeOpportunities(trajectories []*trajectory.Trajectory, availableTools []string) []Insight {
	// Build set of tools actually used.
	usedTools := make(map[string]struct{})
	var allEvidence []EvidenceRef
	for _, t := range trajectories {
		for _, entry := range t.Conversations {
			if entry.From != trajectory.RoleTool {
				continue
			}
			toolName := extractToolName(entry.Value)
			if toolName != "" {
				usedTools[toolName] = struct{}{}
			}
		}
		allEvidence = append(allEvidence, EvidenceRef{
			SessionID: t.SessionID,
			Timestamp: t.Timestamp,
		})
	}

	var insights []Insight
	for _, tool := range availableTools {
		if _, used := usedTools[tool]; used {
			continue
		}
		// Tool is available but never called.
		// Confidence is 1.0 since absence of usage is definitive.
		ev := []EvidenceRef{}
		if len(allEvidence) > 0 {
			ev = []EvidenceRef{allEvidence[0]}
		}
		insights = append(insights, Insight{
			Category:     CategoryOpportunity,
			Title:        fmt.Sprintf("%s_never_used", tool),
			Description:  fmt.Sprintf("Tool %q is available but was never called (0 invocations)", tool),
			Observations: len(trajectories),
			Confidence:   1.0,
			Evidence:     ev,
			Narrative:    fmt.Sprintf("%q tool never used. Consider exploring its capabilities.", tool),
			CreatedAt:    time.Now().UTC(),
		})
	}

	sort.Slice(insights, func(i, j int) bool {
		return insights[i].Title < insights[j].Title
	})
	return insights
}

// Analyze runs all four category analyzers and returns a consolidated map.
func (a *Analyzer) Analyze(trajectories []*trajectory.Trajectory, availableTools []string) map[InsightCategory][]Insight {
	result := map[InsightCategory][]Insight{
		CategoryPattern:     a.AnalyzePatterns(trajectories),
		CategoryPreference:  a.AnalyzePreferences(trajectories),
		CategoryError:       a.AnalyzeErrors(trajectories),
		CategoryOpportunity: a.AnalyzeOpportunities(trajectories, availableTools),
	}
	return result
}

// snippetOf returns up to evidenceSnippetCap runes from s (PII protection).
func snippetOf(s string) string {
	s = strings.TrimSpace(s)
	count := 0
	for i := range s {
		if count >= evidenceSnippetCap {
			return s[:i]
		}
		count++
	}
	if utf8.RuneCountInString(s) <= evidenceSnippetCap {
		return s
	}
	return s
}

// containsKeywordInHuman reports whether any human turn in t contains any of the keywords.
func containsKeywordInHuman(t *trajectory.Trajectory, keywords []string) bool {
	for _, entry := range t.Conversations {
		if entry.From != trajectory.RoleHuman {
			continue
		}
		lower := strings.ToLower(entry.Value)
		for _, kw := range keywords {
			if strings.Contains(lower, strings.ToLower(kw)) {
				return true
			}
		}
	}
	return false
}

// firstHumanSnippet returns a snippet from the first human turn containing any keyword.
func firstHumanSnippet(t *trajectory.Trajectory, keywords []string) string {
	for _, entry := range t.Conversations {
		if entry.From != trajectory.RoleHuman {
			continue
		}
		lower := strings.ToLower(entry.Value)
		for _, kw := range keywords {
			if strings.Contains(lower, strings.ToLower(kw)) {
				return snippetOf(entry.Value)
			}
		}
	}
	return ""
}

// deduplicateBySession keeps only the first evidence ref per session ID.
func deduplicateBySession(refs []EvidenceRef) []EvidenceRef {
	seen := make(map[string]struct{})
	var out []EvidenceRef
	for _, r := range refs {
		if _, ok := seen[r.SessionID]; ok {
			continue
		}
		seen[r.SessionID] = struct{}{}
		out = append(out, r)
	}
	return out
}
