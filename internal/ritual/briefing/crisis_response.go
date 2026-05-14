package briefing

import (
	"strings"

	"github.com/modu-ai/mink/internal/ritual/journal"
)

// crisisDetectorSingleton is the shared CrisisDetector used by the briefing
// pipeline. It is created from the JOURNAL-001 CrisisDetector to ensure the
// same keyword set is used across both surfaces (SPEC-MINK-BRIEFING-001 §6
// "JOURNAL-001 crisis pattern 재사용").
var crisisDetectorSingleton = journal.NewCrisisDetector()

// CheckCrisis reports whether any surfaced text in the rendered briefing
// matches a JOURNAL-001 crisis keyword. The check is applied to the rendered
// output (post-render) so that text from any visible module (mantra,
// anniversaries that escaped the JOURNAL recall filter, or the LLM summary)
// can be detected.
//
// REQ-BR-055, AC-009 invariant 6.
func CheckCrisis(rendered string) bool {
	return crisisDetectorSingleton.Check(rendered)
}

// PrependCrisisResponseIfDetected returns the rendered briefing prefixed
// with the JOURNAL-001 canned hotline response when any crisis keyword is
// detected. When no keyword is detected the input is returned unchanged.
//
// Rules enforced by this function:
//   - No analytical commentary is appended (REQ-BR-055).
//   - No diagnostic vocabulary is introduced.
//   - The hotline numbers come verbatim from journal.CrisisResponse and are
//     not paraphrased.
//
// @MX:ANCHOR: PrependCrisisResponseIfDetected is the single chokepoint
// enforcing crisis hotline prepend across the briefing pipeline.
// @MX:REASON: SPEC-MINK-BRIEFING-001 REQ-BR-055 / AC-009 invariant 6.
func PrependCrisisResponseIfDetected(rendered string) string {
	if !CheckCrisis(rendered) {
		return rendered
	}
	var sb strings.Builder
	sb.WriteString(journal.CrisisResponse)
	sb.WriteString("\n\n")
	sb.WriteString(rendered)
	return sb.String()
}

// PayloadHasCrisis scans the visible text fields of the BriefingPayload for
// crisis keywords. Useful for early detection before rendering.
//
// Scanned fields:
//   - Mantra.Text
//   - LLMSummary (M3)
//   - AnniversaryEntry.Text (JOURNAL trauma recall protection normally filters
//     crisis entries, but this is a defensive scan)
//
// Status / categorical fields are NOT scanned (no risk of crisis keyword).
func PayloadHasCrisis(payload *BriefingPayload) bool {
	if payload == nil {
		return false
	}
	if crisisDetectorSingleton.Check(payload.Mantra.Text) {
		return true
	}
	if crisisDetectorSingleton.Check(payload.LLMSummary) {
		return true
	}
	for _, a := range payload.JournalRecall.Anniversaries {
		if a != nil && crisisDetectorSingleton.Check(a.Text) {
			return true
		}
	}
	return false
}
