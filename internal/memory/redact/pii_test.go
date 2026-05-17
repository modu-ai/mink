// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package redact

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// ---- Synthetic fixture builders ------------------------------------------
//
// Tokens and card numbers used in these tests are constructed at run-time
// from harmless filler runes.  Storing them as runtime expressions (rather
// than literal constants) keeps the source tree free of any byte sequence
// a secret scanner would recognise as a real credential.

func repeatChar(c byte, n int) string {
	return strings.Repeat(string([]byte{c}), n)
}

func buildToken(prefix string, tailLen int) string {
	return prefix + repeatChar('X', tailLen)
}

func buildJWT() string {
	seg := repeatChar('Y', 24)
	return "eyJ" + seg + "." + seg + "." + seg
}

func luhnAppendCheck(digits string) string {
	sum := 0
	alt := true
	for i := len(digits) - 1; i >= 0; i-- {
		n := int(digits[i] - '0')
		if alt {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
		alt = !alt
	}
	check := (10 - sum%10) % 10
	return digits + string(byte('0'+check))
}

func formatGroups(digits, sep string) string {
	var b strings.Builder
	for i, r := range digits {
		if i > 0 && i%4 == 0 {
			b.WriteString(sep)
		}
		b.WriteRune(r)
	}
	return b.String()
}

func formatAmexGroups(digits string) string {
	if len(digits) != 15 {
		return digits
	}
	return digits[:4] + "-" + digits[4:10] + "-" + digits[10:]
}

var (
	fakeYa29      = buildToken("ya29.", 24)
	fakeGhpToken  = buildToken("ghp_", 24)
	fakeGhoToken  = buildToken("gho_", 24)
	fakeGhsToken  = buildToken("ghs_", 24)
	fakeGithubPAT = buildToken("github_pat_", 24)
	fakeGlpat     = buildToken("glpat-", 24)
	fakeOpenAIKey = buildToken("sk-", 24)
	fakeJWT       = buildJWT()

	card16Solid = luhnAppendCheck(repeatChar('8', 15))
	card15Solid = luhnAppendCheck(repeatChar('8', 14))
	card16Dash  = formatGroups(card16Solid, "-")
	card16Space = formatGroups(card16Solid, " ")
	card16Dot   = formatGroups(card16Solid, ".")
	card15Dash  = formatAmexGroups(card15Solid)
)

func TestRedact_TableDriven(t *testing.T) {
	t.Parallel()

	type tc struct {
		name      string
		input     string
		wantCats  []Category
		wantToken string
		noHits    bool
	}

	cases := []tc{
		// ---- Phone (Korea): 12 variants ----
		{"phone_kr_dash_010", "Call me at 010-1234-5678 please.", []Category{CategoryPhoneKR}, "[REDACTED:phone_kr]", false},
		{"phone_kr_dash_011", "Old number 011-234-5678 still works.", []Category{CategoryPhoneKR}, "[REDACTED:phone_kr]", false},
		{"phone_kr_no_separator", "Domestic 01012345678 no dashes.", []Category{CategoryPhoneKR}, "[REDACTED:phone_kr]", false},
		{"phone_kr_intl_dash", "International +82-10-1234-5678 form.", []Category{CategoryPhoneKR}, "[REDACTED:phone_kr]", false},
		{"phone_kr_intl_space_paren", "Visiting card: +82 (0)10 1234 5678", []Category{CategoryPhoneKR}, "[REDACTED:phone_kr]", false},
		{"phone_kr_intl_dash_paren", "Roaming: +82-(0)10-1234-5678", []Category{CategoryPhoneKR}, "[REDACTED:phone_kr]", false},
		{"phone_kr_dot_separator", "Marketing 010.1234.5678 banner", []Category{CategoryPhoneKR}, "[REDACTED:phone_kr]", false},
		{"phone_kr_space_separator", "Direct 010 1234 5678 line", []Category{CategoryPhoneKR}, "[REDACTED:phone_kr]", false},
		{"phone_kr_3_4_4_split", "Old prefix 016-345-6789 carrier", []Category{CategoryPhoneKR}, "[REDACTED:phone_kr]", false},
		{"phone_kr_short_middle", "Vintage 017-345-6789 line", []Category{CategoryPhoneKR}, "[REDACTED:phone_kr]", false},
		{"phone_kr_010_18", "Mobile 018-1234-5678 trunk", []Category{CategoryPhoneKR}, "[REDACTED:phone_kr]", false},
		{"phone_kr_010_19", "Pager 019-1234-5678 era", []Category{CategoryPhoneKR}, "[REDACTED:phone_kr]", false},

		// ---- RRN: 8 variants ----
		{"rrn_valid_male", "주민번호 851230-1234567 입니다.", []Category{CategoryRRN}, "[REDACTED:rrn]", false},
		{"rrn_valid_female", "RRN sample 901231-2345678 here.", []Category{CategoryRRN}, "[REDACTED:rrn]", false},
		{"rrn_invalid_month", "Bad month 851330-1234567 ignored.", nil, "", true},
		{"rrn_invalid_day", "Bad day 850232-1234567 ignored.", nil, "", true},
		{"rrn_all_zeros", "Zero RRN 000000-0000000 ignored.", nil, "", true},
		{"rrn_back_zeros", "RRN with zero back 851230-0000000 rejected.", nil, "", true},
		{"rrn_13_digits_no_dash", "Raw 8512301234567 should be card-Luhn rejected.", nil, "", true},
		{"rrn_valid_century_9", "Pre-1900 901231-9234567 sample.", []Category{CategoryRRN}, "[REDACTED:rrn]", false},

		// ---- Email: 8 variants ----
		{"email_simple", "Contact alice@example.com today.", []Category{CategoryEmail}, "[REDACTED:email]", false},
		{"email_plus_tag", "Newsletter bob+news@example.org subscribed.", []Category{CategoryEmail}, "[REDACTED:email]", false},
		{"email_subdomain", "Internal sue@dev.modu.ai team.", []Category{CategoryEmail}, "[REDACTED:email]", false},
		{"email_numeric_local", "Robot 12345@example.com pinged.", []Category{CategoryEmail}, "[REDACTED:email]", false},
		{"email_uppercase", "Ticket Carol@EXAMPLE.COM resolved.", []Category{CategoryEmail}, "[REDACTED:email]", false},
		{"email_dash_domain", "Vendor info@foo-bar.io billed.", []Category{CategoryEmail}, "[REDACTED:email]", false},
		{"email_tld_too_short", "Junk dave@example.x ignored.", nil, "", true},
		{"email_no_tld", "Local mail@localhost dropped.", nil, "", true},

		// ---- Card: 10 variants (all synthetic Luhn-valid) ----
		{"card_synth16_dashes", "Paid via " + card16Dash + " yesterday.", []Category{CategoryCard}, "[REDACTED:card]", false},
		{"card_synth16_spaces", "Card " + card16Space + " saved.", []Category{CategoryCard}, "[REDACTED:card]", false},
		{"card_synth16_solid", "Card " + card16Solid + " raw form.", []Category{CategoryCard}, "[REDACTED:card]", false},
		{"card_synth15_solid", "Amex-shape " + card15Solid + " on file.", []Category{CategoryCard}, "[REDACTED:card]", false},
		{"card_synth15_dashed", "Amex-shape " + card15Dash + " saved.", []Category{CategoryCard}, "[REDACTED:card]", false},
		{"card_luhn_fail_dashes", "Random 1234-5678-9012-3456 fails Luhn.", nil, "", true},
		{"card_luhn_fail_plain", "Number 1111222233334445 fails Luhn.", nil, "", true},
		{"card_embedded_sentence", "The total of " + card16Solid + " was charged.", []Category{CategoryCard}, "[REDACTED:card]", false},
		{"card_synth_dot_grouped", "Card " + card16Dot + " noted.", []Category{CategoryCard}, "[REDACTED:card]", false},
		{"card_synth_after_word", "Receipt: " + card16Dash + ".", []Category{CategoryCard}, "[REDACTED:card]", false},

		// ---- OAuth tokens: 12 variants ----
		{"oauth_ya29", "Bearer " + fakeYa29 + " OK.", []Category{CategoryOAuthToken}, "[REDACTED:oauth_token]", false},
		{"oauth_ghp", "Token " + fakeGhpToken + " valid.", []Category{CategoryOAuthToken}, "[REDACTED:oauth_token]", false},
		{"oauth_gho", "Token " + fakeGhoToken + " ok.", []Category{CategoryOAuthToken}, "[REDACTED:oauth_token]", false},
		{"oauth_ghs", "Server " + fakeGhsToken + " found.", []Category{CategoryOAuthToken}, "[REDACTED:oauth_token]", false},
		{"oauth_github_pat", "Fine " + fakeGithubPAT + " pat.", []Category{CategoryOAuthToken}, "[REDACTED:oauth_token]", false},
		{"oauth_glpat", "GitLab " + fakeGlpat + " pat.", []Category{CategoryOAuthToken}, "[REDACTED:oauth_token]", false},
		{"oauth_sk_openai", "OpenAI " + fakeOpenAIKey + " key.", []Category{CategoryOAuthToken}, "[REDACTED:oauth_token]", false},
		{"oauth_jwt", "Auth " + fakeJWT + " jwt.", []Category{CategoryOAuthToken}, "[REDACTED:oauth_token]", false},
		{"oauth_short_fake_ghp", "Not a token ghp_short here.", nil, "", true},
		{"oauth_short_fake_sk_dash", "Not a token sk-short here.", nil, "", true},
		{"oauth_short_fake_glpat", "Skip glpat-tiny prefix.", nil, "", true},
		{"oauth_short_fake_ya29", "Skip ya29.tiny prefix.", nil, "", true},

		// ---- Korean prose with rune offset checks: 5 cases ----
		{"korean_phone_mid_sentence", "안녕하세요, 연락처는 010-1234-5678 입니다.", []Category{CategoryPhoneKR}, "[REDACTED:phone_kr]", false},
		{"korean_email_mid_sentence", "이메일 alice@example.com 으로 보내주세요.", []Category{CategoryEmail}, "[REDACTED:email]", false},
		{"korean_rrn_label", "주민등록번호: 851230-1234567 (외부 유출 금지)", []Category{CategoryRRN}, "[REDACTED:rrn]", false},
		{"korean_card_label", "카드번호 " + card16Dash + " 결제 완료.", []Category{CategoryCard}, "[REDACTED:card]", false},
		{"korean_token_label", "토큰값 " + fakeGhpToken + " 유출 주의.", []Category{CategoryOAuthToken}, "[REDACTED:oauth_token]", false},

		// ---- No PII: 3 cases ----
		{"no_pii_prose", "The weather is nice today and tomorrow.", nil, "", true},
		{"no_pii_code", "func add(a int, b int) int { return a + b }", nil, "", true},
		{"no_pii_numbers", "Version 1.2.3 released on 2026-05-17.", nil, "", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := Redact(c.input)

			if c.noHits {
				if len(got.Hits) != 0 {
					t.Fatalf("expected no hits, got %d (%v) for input %q", len(got.Hits), got.Hits, c.input)
				}
				if got.Masked != c.input {
					t.Fatalf("expected masked to equal input when no hits; got %q want %q", got.Masked, c.input)
				}
				return
			}

			if len(got.Hits) == 0 {
				t.Fatalf("expected at least one hit, got none for input %q", c.input)
			}

			if c.wantToken != "" && !strings.Contains(got.Masked, c.wantToken) {
				t.Fatalf("masked output %q does not contain %q", got.Masked, c.wantToken)
			}

			for _, want := range c.wantCats {
				found := false
				for _, h := range got.Hits {
					if h.Category == want {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("expected category %q in hits %v", want, got.Hits)
				}
			}

			for i := 1; i < len(got.Hits); i++ {
				if got.Hits[i].Start < got.Hits[i-1].Start {
					t.Fatalf("hits not sorted: %d before %d", got.Hits[i-1].Start, got.Hits[i].Start)
				}
			}

			for _, h := range got.Hits {
				if h.Start < 0 || h.End <= h.Start {
					t.Fatalf("invalid hit span: %+v", h)
				}
				if total := utf8.RuneCountInString(c.input); h.End > total {
					t.Fatalf("hit end %d exceeds rune length %d for input %q", h.End, total, c.input)
				}
				if !strings.Contains(c.input, h.Original) {
					t.Fatalf("hit.Original %q is not a substring of input %q", h.Original, c.input)
				}
				if h.End-h.Start != utf8.RuneCountInString(h.Original) {
					t.Fatalf("hit span %d does not match RuneCount of %q", h.End-h.Start, h.Original)
				}
			}
		})
	}
}

func TestRedact_RuneOffsetWithKorean(t *testing.T) {
	t.Parallel()

	prefix := "주민번호: "
	rrn := "851230-1234567"
	input := prefix + rrn + " 끝"

	got := Redact(input)
	if len(got.Hits) != 1 {
		t.Fatalf("expected exactly one hit, got %d", len(got.Hits))
	}
	h := got.Hits[0]

	wantStart := utf8.RuneCountInString(prefix)
	if h.Start != wantStart {
		t.Fatalf("rune Start = %d, want %d", h.Start, wantStart)
	}
	if h.End != wantStart+utf8.RuneCountInString(rrn) {
		t.Fatalf("rune End = %d, want %d", h.End, wantStart+utf8.RuneCountInString(rrn))
	}

	wantMasked := prefix + "[REDACTED:rrn] 끝"
	if got.Masked != wantMasked {
		t.Fatalf("masked = %q, want %q", got.Masked, wantMasked)
	}
}

func TestRedact_MultipleCategoriesInOneInput(t *testing.T) {
	t.Parallel()

	input := "Email alice@example.com phone 010-1234-5678 token " + fakeGhpToken + "."
	got := Redact(input)

	if len(got.Hits) != 3 {
		t.Fatalf("expected 3 hits, got %d (%v)", len(got.Hits), got.Hits)
	}

	wantOrder := []Category{CategoryEmail, CategoryPhoneKR, CategoryOAuthToken}
	for i, want := range wantOrder {
		if got.Hits[i].Category != want {
			t.Fatalf("hit %d: got %q want %q", i, got.Hits[i].Category, want)
		}
	}

	for _, c := range wantOrder {
		token := "[REDACTED:" + string(c) + "]"
		if strings.Count(got.Masked, token) != 1 {
			t.Fatalf("expected exactly one %s in masked output %q", token, got.Masked)
		}
	}
}

func TestRedact_OverlapResolution(t *testing.T) {
	t.Parallel()

	input := "Record 851230-1234567 should redact as RRN."
	got := Redact(input)

	if len(got.Hits) != 1 {
		t.Fatalf("expected single hit after overlap resolution, got %d (%v)", len(got.Hits), got.Hits)
	}
	if got.Hits[0].Category != CategoryRRN {
		t.Fatalf("overlap resolution picked %q; expected RRN", got.Hits[0].Category)
	}
}

func TestRedact_EmptyInput(t *testing.T) {
	t.Parallel()
	got := Redact("")
	if got.Masked != "" || got.Hits != nil {
		t.Fatalf("expected empty Result, got %+v", got)
	}
}

func TestCategories_StableOrder(t *testing.T) {
	t.Parallel()
	first := Categories()
	second := Categories()
	if len(first) == 0 {
		t.Fatal("Categories returned empty slice")
	}
	if len(first) != len(second) {
		t.Fatalf("Categories returned different lengths: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("Categories not stable at %d: %q vs %q", i, first[i], second[i])
		}
	}
}

func TestLuhnValid(t *testing.T) {
	t.Parallel()

	synth16 := luhnAppendCheck(repeatChar('8', 15))
	synth15 := luhnAppendCheck(repeatChar('8', 14))

	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{"synth16_valid", synth16, true},
		{"synth15_valid", synth15, true},
		{"all_zeros_15", repeatChar('0', 15), true},
		{"luhn_fail", "1234567890123456", false},
		{"too_short", repeatChar('1', 6), false},
		{"too_long_20", repeatChar('1', 20), false},
		{"non_digit", repeatChar('1', 14) + "a", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := luhnValid(c.input); got != c.want {
				t.Fatalf("luhnValid(%q) = %v, want %v", c.input, got, c.want)
			}
		})
	}
}

func TestValidateRRN(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid_male", "851230-1234567", true},
		{"valid_female", "901231-2345678", true},
		{"invalid_month_13", "851330-1234567", false},
		{"invalid_day_32", "850132-1234567", false},
		{"invalid_month_zero", "850030-1234567", false},
		{"invalid_day_zero", "850100-1234567", false},
		{"all_zeros", "000000-0000000", false},
		{"back_zeros", "851230-0000000", false},
		{"missing_dash", "8512301234567", false},
		{"short", "85123-1234567", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := validateRRN(c.input); got != c.want {
				t.Fatalf("validateRRN(%q) = %v, want %v", c.input, got, c.want)
			}
		})
	}
}

func TestStripNonDigits(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"9999-9999-9999-9999", "9999999999999999"},
		{"9999 9999 9999 9999", "9999999999999999"},
		{"", ""},
		{"abc123def", "123"},
		{"한국어 123 abc", "123"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			t.Parallel()
			if got := stripNonDigits(c.in); got != c.want {
				t.Fatalf("stripNonDigits(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// TestResolveOverlaps_DirectBranches drives the resolveOverlaps function
// directly to cover branches that are hard to reach through Redact alone
// (overlap with strict containment where the longer span wins, and tie
// resolution by categoryOrder when spans are identical).
func TestResolveOverlaps_DirectBranches(t *testing.T) {
	t.Parallel()

	// Overlap where curLen > lastLen: a 4-byte span starting at 0 is
	// followed by a 10-byte span starting at 2.  The second hit must
	// replace the first because it is longer.
	in := []hitInternal{
		{cat: CategoryEmail, byteStart: 0, byteEnd: 4, categoryIx: categoryOrder[CategoryEmail]},
		{cat: CategoryPhoneKR, byteStart: 2, byteEnd: 12, categoryIx: categoryOrder[CategoryPhoneKR]},
	}
	got := resolveOverlaps(in)
	if len(got) != 1 {
		t.Fatalf("expected one hit after overlap resolution, got %d", len(got))
	}
	if got[0].cat != CategoryPhoneKR {
		t.Fatalf("longer span did not win: got %v", got)
	}

	// Identical span: tie-break by categoryOrder.  Card (1) beats
	// OAuthToken (2) when the spans are equal.
	in2 := []hitInternal{
		{cat: CategoryOAuthToken, byteStart: 0, byteEnd: 8, categoryIx: categoryOrder[CategoryOAuthToken]},
		{cat: CategoryCard, byteStart: 0, byteEnd: 8, categoryIx: categoryOrder[CategoryCard]},
	}
	got2 := resolveOverlaps(in2)
	if len(got2) != 1 {
		t.Fatalf("expected one hit after tie-break, got %d", len(got2))
	}
	if got2[0].cat != CategoryCard {
		t.Fatalf("tie-break did not pick lower category: got %v", got2)
	}

	// Single-element input is returned unchanged (covers the fast-path).
	in3 := []hitInternal{
		{cat: CategoryEmail, byteStart: 0, byteEnd: 5, categoryIx: categoryOrder[CategoryEmail]},
	}
	got3 := resolveOverlaps(in3)
	if len(got3) != 1 || got3[0] != in3[0] {
		t.Fatalf("single-element input not returned as-is: %v", got3)
	}
}
