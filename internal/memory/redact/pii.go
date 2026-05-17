// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

// Package redact applies PII masking to machine-generated markdown content
// before it reaches the SQLite index or any logging path.
//
// The package is consumed by:
//
//   - M3 session auto-export (LLM-ROUTING-V2 transcripts)
//   - M5 publish hooks (JOURNAL / BRIEFING / WEATHER / RITUAL ingestion)
//   - The Apply guard which seals masked content so the Indexer boundary can
//     reject any bypass (see guard.go, AC-MEM-028).
//
// Detector implementations are deterministic and never log the original
// substring.  All offsets surfaced to callers are rune offsets (not byte
// offsets) so Korean prose intermixed with Latin tokens is reported
// correctly.
//
// SPEC: SPEC-MINK-MEMORY-QMD-001 (M2, T2.4)
// REQ:  REQ-MEM-027, REQ-MEM-028
package redact

import (
	"errors"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// Category enumerates PII categories the redactor recognises.
type Category string

// Recognised categories.  The order here also defines the tie-break order
// used when two detectors match the exact same span (earlier wins).
const (
	CategoryRRN        Category = "rrn"
	CategoryCard       Category = "card"
	CategoryOAuthToken Category = "oauth_token"
	CategoryPhoneKR    Category = "phone_kr"
	CategoryEmail      Category = "email"
)

// categoryOrder gives every category a deterministic rank used as the
// tie-breaker in resolveOverlaps when two hits have identical spans.
var categoryOrder = map[Category]int{
	CategoryRRN:        0,
	CategoryCard:       1,
	CategoryOAuthToken: 2,
	CategoryPhoneKR:    3,
	CategoryEmail:      4,
}

// allCategories returns the categories in declaration order so the public
// Categories() helper has a stable result.
func allCategories() []Category {
	return []Category{
		CategoryRRN,
		CategoryCard,
		CategoryOAuthToken,
		CategoryPhoneKR,
		CategoryEmail,
	}
}

// Hit describes a single masked occurrence.
//
// Start and End are rune offsets into the original (unmasked) content.
// Original is retained for audit / unit-test purposes only; callers MUST
// NOT log it.
type Hit struct {
	Category Category
	Start    int
	End      int
	Original string
}

// Result is returned by Redact.
type Result struct {
	// Masked is the content with all matches replaced by the canonical
	// token "[REDACTED:<category>]".
	Masked string
	// Hits are the masked occurrences, ordered by Start ascending.  Two
	// hits never overlap (overlap resolution is part of Redact).
	Hits []Hit
}

// ErrRedactBypass signals that content was passed to an ingestion path
// without going through the Redact pipeline.  Callers that bypass the
// masking pipeline (e.g. session export, publish hooks) MUST be rejected
// at the Indexer boundary.
//
// @MX:ANCHOR: [AUTO] Sentinel contract enforcing AC-MEM-028.
// @MX:REASON: Removing this error would let unsanitised transcripts reach
// the SQLite index — the bypass detection at the Indexer relies on this
// exact sentinel for equality comparison.
var ErrRedactBypass = errors.New("redact: ingestion bypassed PII redaction pipeline")

// ----- Detector regular expressions ------------------------------------------------
//
// Each detector regex is compiled once at package init via regexp.MustCompile
// and then shared.  Detectors are intentionally a touch conservative — the
// callers (M3 session export, M5 publish hooks) prefer false negatives over
// false positives, because every false positive masks legitimate content.

var (
	// Korean phone numbers.
	//
	// Carrier prefixes: 010 / 011 / 016 / 017 / 018 / 019.
	// Domestic form requires the leading "0".  International form
	// "+82-10-..." or "+82 (0)10 ..." drops or parenthesises the zero, so
	// we accept either alternation explicitly.
	// Separators between number groups may be "-", ".", " " or absent.
	rePhoneKR = regexp.MustCompile(
		`(?:` +
			// International with "(0)" preserved.
			`\+?82[-. ]?\(0\)[-. ]?1[016789]` +
			`|` +
			// International, leading zero dropped.
			`\+?82[-. ]?1[016789]` +
			`|` +
			// Domestic.
			`01[016789]` +
			`)` +
			`(?:[-. ]\d{3,4}[-. ]\d{4}|\d{3,4}[-. ]\d{4}|[-. ]\d{4}\d{4}|\d{7,8})`,
	)

	// Resident Registration Number (Korean): YYMMDD-Xxxxxxx (exactly 13
	// digits with a dash in the middle).  Date validation happens in code
	// (validateRRN) — the regex stays simple so it does not become
	// unreadable.
	reRRN = regexp.MustCompile(`\b(\d{6})-(\d{7})\b`)

	// Email — simplified RFC 5322.  Local part allows the common
	// punctuation set; domain requires at least one dot and a 2+ char TLD
	// composed of letters.
	reEmail = regexp.MustCompile(
		`(?i)[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}`,
	)

	// Credit card candidates: 14-19 digits, optionally grouped by 4 with
	// single-character separators (space, dash, dot).  Luhn validation runs
	// after the regex (see luhnValid).
	//
	// The lower bound is 14 (not the theoretical 13) so 13-digit Korean
	// resident numbers, even when the dash-stripped form happens to pass
	// Luhn, are not falsely identified as cards.  Diners Club 13-digit
	// numbers are rare enough that the trade-off favours fewer false
	// positives on PII-adjacent shapes.
	reCard = regexp.MustCompile(
		`\b(?:\d[ \-.]?){13,18}\d\b`,
	)

	// OAuth / API tokens.
	//
	// The OR-alternation order is significant: longer specific prefixes
	// first so the regex engine does not commit to a shorter match.
	reOAuthToken = regexp.MustCompile(
		`(?:` +
			`ya29\.[A-Za-z0-9_\-]{20,}` + // Google OAuth access tokens
			`|github_pat_[A-Za-z0-9_]{20,}` + // GitHub fine-grained PAT
			`|gh[pousr]_[A-Za-z0-9]{20,}` + // GitHub classic tokens
			`|glpat-[A-Za-z0-9_\-]{20,}` + // GitLab PAT
			`|sk-[A-Za-z0-9_\-]{20,}` + // OpenAI secret keys
			`|eyJ[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}` + // JWT (header starts with eyJ)
			`)`,
	)
)

// detector ties a category to its compiled regex.
type detector struct {
	cat Category
	re  *regexp.Regexp
}

// detectors holds the package-level detector table.  Tests rely on
// allCategories() — keep it in sync with this slice.
var detectors = []detector{
	{CategoryRRN, reRRN},
	{CategoryCard, reCard},
	{CategoryOAuthToken, reOAuthToken},
	{CategoryPhoneKR, rePhoneKR},
	{CategoryEmail, reEmail},
}

// Categories returns a stable slice of all recognised categories — useful
// for callers that want to filter or list.
func Categories() []Category {
	return allCategories()
}

// Redact applies all category detectors to content and returns the masked
// text plus a structured record of hits.  It is deterministic and does NOT
// log the original substring.
//
// @MX:ANCHOR: [AUTO] Public masking entry point — fan_in >= 3 once M3 and
// M5 wiring lands (session export, publish hooks, audit pipeline).
// @MX:REASON: Behaviour of this function is the privacy invariant for the
// memory subsystem.  Any change must preserve "no PII characters reach the
// SQLite index or the log".
func Redact(content string) Result {
	if content == "" {
		return Result{Masked: "", Hits: nil}
	}

	rawHits := collectHits(content)
	hits := resolveOverlaps(rawHits)

	// Convert byte spans to rune spans and build the masked string in one
	// pass through the content.
	return rewrite(content, hits)
}

// hitInternal is the byte-offset form used internally before we surface
// rune offsets to callers.
type hitInternal struct {
	cat        Category
	byteStart  int
	byteEnd    int
	categoryIx int
}

// collectHits runs every detector and returns the union of matches.  Card
// and RRN matches are post-filtered (Luhn check, date validity).
func collectHits(content string) []hitInternal {
	var out []hitInternal

	for _, d := range detectors {
		matches := d.re.FindAllStringIndex(content, -1)
		for _, m := range matches {
			span := content[m[0]:m[1]]

			switch d.cat {
			case CategoryCard:
				if !luhnValid(stripNonDigits(span)) {
					continue
				}
			case CategoryRRN:
				if !validateRRN(span) {
					continue
				}
			}

			out = append(out, hitInternal{
				cat:        d.cat,
				byteStart:  m[0],
				byteEnd:    m[1],
				categoryIx: categoryOrder[d.cat],
			})
		}
	}
	return out
}

// resolveOverlaps keeps only one hit per overlapping group.
//
// Rule: longest span wins.  Ties are broken by categoryOrder (lower wins),
// then by earlier start.  The output is sorted by start ascending and
// contains no overlaps.
func resolveOverlaps(in []hitInternal) []hitInternal {
	if len(in) <= 1 {
		return in
	}

	// Sort by (start asc, length desc, categoryOrder asc) so the first hit
	// in each overlap group is already the winning one.
	sort.SliceStable(in, func(i, j int) bool {
		if in[i].byteStart != in[j].byteStart {
			return in[i].byteStart < in[j].byteStart
		}
		li := in[i].byteEnd - in[i].byteStart
		lj := in[j].byteEnd - in[j].byteStart
		if li != lj {
			return li > lj
		}
		return in[i].categoryIx < in[j].categoryIx
	})

	out := make([]hitInternal, 0, len(in))
	for _, h := range in {
		if len(out) == 0 {
			out = append(out, h)
			continue
		}
		last := out[len(out)-1]

		if h.byteStart >= last.byteEnd {
			// No overlap.
			out = append(out, h)
			continue
		}

		// Overlap: pick the longer span; ties broken by category order.
		curLen := h.byteEnd - h.byteStart
		lastLen := last.byteEnd - last.byteStart
		switch {
		case curLen > lastLen:
			out[len(out)-1] = h
		case curLen == lastLen && h.categoryIx < last.categoryIx:
			out[len(out)-1] = h
		default:
			// Keep last.
		}
	}
	return out
}

// rewrite builds the masked string and surfaces Hit entries with rune
// offsets.  We scan the original content once, counting runes, and emit
// the replacement token whenever we enter a hit span.
func rewrite(content string, hits []hitInternal) Result {
	if len(hits) == 0 {
		return Result{Masked: content, Hits: nil}
	}

	var (
		b           strings.Builder
		runeOffset  int
		byteCursor  int
		surfaceHits = make([]Hit, 0, len(hits))
	)
	b.Grow(len(content))

	for _, h := range hits {
		// Emit bytes up to the hit's byte start, counting runes as we go.
		// We do not update byteCursor here because it is unconditionally
		// reset to h.byteEnd just below.
		if h.byteStart > byteCursor {
			segment := content[byteCursor:h.byteStart]
			b.WriteString(segment)
			runeOffset += utf8.RuneCountInString(segment)
		}

		// Capture rune-offset start, compute rune-offset end by counting
		// runes in the matched span.
		original := content[h.byteStart:h.byteEnd]
		runeStart := runeOffset
		runeLen := utf8.RuneCountInString(original)
		runeOffset += runeLen
		byteCursor = h.byteEnd

		// Emit the replacement token.
		b.WriteString("[REDACTED:")
		b.WriteString(string(h.cat))
		b.WriteString("]")

		surfaceHits = append(surfaceHits, Hit{
			Category: h.cat,
			Start:    runeStart,
			End:      runeStart + runeLen,
			Original: original,
		})
	}

	// Trailing content after the last hit.
	if byteCursor < len(content) {
		b.WriteString(content[byteCursor:])
	}

	return Result{Masked: b.String(), Hits: surfaceHits}
}

// ----- Validators -----------------------------------------------------------

// stripNonDigits returns the input with every non-digit rune removed.
// Used by the card-number Luhn check.
func stripNonDigits(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// luhnValid runs the Luhn (mod-10) checksum on a digits-only string.
//
// @MX:WARN: [AUTO] The card detector regex is intentionally broad — any
// digit run with the right shape will match.  Without this Luhn check,
// dates, order numbers and version strings get falsely masked as cards.
// @MX:REASON: Luhn validation is the only line of defence between a noisy
// regex and the user's actual content; do not bypass it.
func luhnValid(digits string) bool {
	if len(digits) < 13 || len(digits) > 19 {
		return false
	}

	sum := 0
	alt := false
	for i := len(digits) - 1; i >= 0; i-- {
		c := digits[i]
		if c < '0' || c > '9' {
			return false
		}
		n := int(c - '0')
		if alt {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
		alt = !alt
	}
	return sum%10 == 0
}

// validateRRN performs a lightweight sanity check on a 6+7 digit Korean
// RRN candidate of the form "YYMMDD-Sxxxxxx".  It rejects obviously bogus
// values (all zeros, impossible month, impossible day, future-only century
// marker outside 0-9).
//
// Full RRN checksum is intentionally NOT validated here — the regex hit
// plus the date sanity check is enough to mask the field, and validating
// the checksum risks leaking which formats are valid.
func validateRRN(s string) bool {
	if len(s) != 14 || s[6] != '-' {
		return false
	}
	front, back := s[:6], s[7:]

	// All zeros are clearly bogus.
	if front == "000000" && back == "0000000" {
		return false
	}

	month := (int(front[2]-'0') * 10) + int(front[3]-'0')
	day := (int(front[4]-'0') * 10) + int(front[5]-'0')

	if month < 1 || month > 12 {
		return false
	}
	if day < 1 || day > 31 {
		return false
	}

	// The first digit of the back half encodes century + gender; valid
	// values are 1-8 (and historically 9-0 for pre-1900 records).  We
	// accept the full 0-9 range and only reject if the entire back half is
	// zeros.
	if back == "0000000" {
		return false
	}
	return true
}
