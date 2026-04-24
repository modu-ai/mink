# SPEC Review Report: SPEC-GOOSE-ONBOARDING-001
Iteration: 1/3
Verdict: **FAIL**
Overall Score: **0.48**

Reasoning context ignored per M1 Context Isolation. Audit based solely on
`spec.md` and `research.md` in `.moai/specs/SPEC-GOOSE-ONBOARDING-001/`.

---

## Must-Pass Results

- **[PASS] MP-1 REQ number consistency**
  - REQ-OB-001 through REQ-OB-019 are sequential, no gaps, no duplicates.
  - Verified end-to-end at spec.md:L198–L242.
  - Zero-padding consistent (3-digit).

- **[FAIL] MP-2 EARS format compliance**
  - EARS block (REQ-OB-001..019, spec.md:L198–L242) is well-formed across all five EARS patterns (Ubiquitous, Event-Driven, State-Driven, Unwanted, Optional) — this block would score 1.0 in isolation.
  - However, the ACCEPTANCE CRITERIA block (AC-OB-001..016, spec.md:L248–L326) is written entirely in Given/When/Then test-scenario form, NOT EARS. Example spec.md:L248–L251: "Given fresh install... When Desktop App 실행... Then 메인 UI 대신 온보딩 모달이 full-screen으로 표시". Per M3 rubric, GWT mislabeled/substituted for EARS in an "Acceptance Criteria" block anchors the dimension at 0.50 or below. MP-2 is strict: **every** acceptance criterion must match an EARS pattern — FAIL.

- **[FAIL] MP-3 YAML frontmatter validity**
  - spec.md:L1–L13 frontmatter:
    - `id: SPEC-GOOSE-ONBOARDING-001` ✓
    - `version: 0.1.0` ✓
    - `status: Planned` — **non-standard value** (allowed: draft/active/implemented/deprecated).
    - `created: 2026-04-22` — **field is `created`, required field is `created_at`**. MISSING.
    - `priority: P0` — **non-standard value** (allowed: critical/high/medium/low).
    - `labels:` — **MISSING** (required field).
  - Two required fields (`created_at`, `labels`) are absent, and two fields hold non-standard values. FAIL.

- **[N/A] MP-4 Section 22 language neutrality**
  - SPEC is scoped to a single-project Desktop App stack (Tauri v2 + React, Rust backend, Go internal). Multi-language tooling neutrality does not apply. Auto-passes per M5.

---

## Category Scores (0.0–1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.60 | ~0.50–0.75 | Most REQs unambiguous (spec.md:L198–L242). Major ambiguity: scope amendment header (spec.md:L17–L22) declares scope reduced to "CLI+Web UI install wizard" but entire body describes Desktop Tauri 8-step flow — reasonable engineers would diverge on what to build. REQ-OB-013 "soft notice" (spec.md:L226) is vague. |
| Completeness | 0.60 | ~0.50 band | All structural sections present (HISTORY L27, Overview L35, Background L63, Scope L109, REQ L194, AC L246, Technical L330, Dependencies L575, Risks L595, References L612, Exclusions L646). But frontmatter missing `created_at`+`labels`; amendment promises "기존 8-step 플로우는 재구성 필요" (spec.md:L21) yet no restructured REQ/AC reflects the new CLI+Web UI scope. |
| Testability | 0.55 | ~0.50 band | Many ACs are binary-testable (AC-OB-005 name empty, AC-OB-007 keychain, AC-OB-011 draft resume). But AC-OB-016 (spec.md:L326) says "총 소요 시간 ≤ 5분 (수동 측정 CI test)" — "수동 측정" (manual measurement) is not CI-automatable as stated. AC-OB-014 (spec.md:L313–L316) tests French locale but expected error string is in Korean (internal contradiction — see defect D6). REQ-OB-013 "soft notice" lacks exact copy text. |
| Traceability | 0.40 | ~0.25–0.50 band | 5/19 REQs lack any corresponding AC: REQ-OB-013 (ritual all-off notice), REQ-OB-014 (no external transmission), REQ-OB-015 (invalid API key rejection — AC-OB-007 covers valid path only), REQ-OB-016 (sensitive data prohibition), REQ-OB-019 (accessibility). Additionally, NO AC explicitly cites a REQ-XXX identifier — mapping is implicit via adjacency only. |

---

## Defects Found

**D1. spec.md:L17–L22, L25, L15, L42 — CRITICAL — Scope/title internal contradiction**
The v0.2 Amendment block (L17–L22) declares the SPEC is now scoped to "`goose init` CLI 마법사 + Web UI 설치·설정 마법사 (비개발자 대응)" and explicitly removes "Apple Native 초기 설정" and mobile device pairing. The title (L15) is "CLI + Web UI Install Wizard". But L25 keeps the original title "First-Install 8-Step Onboarding Flow ★ 스코프 축소됨" and the ENTIRE body (§1 Overview L35, §2 Background L63, §3 Scope L109, §4 EARS L194, §5 AC L246, §6 Technical L330) describes a Desktop Tauri 8-step onboarding with mobile pairing (Step 7, Step 8, AC-OB-013 Mobile pairing, REQ-OB-018 mobile QR). The amendment explicitly says "기존 8-step 플로우는 재구성 필요" but the restructuring is absent. Engineers reading this SPEC cannot determine authoritative scope — CLI+Web UI wizard or Desktop 8-step. Severity: critical — ambiguous deliverable.

**D2. spec.md:L1–L13 — MAJOR — YAML frontmatter missing required fields**
Required `created_at` and `labels` fields are absent. `created` (L5) is a non-standard alias for `created_at`. Severity: major — MP-3 FAIL.

**D3. spec.md:L4, L8 — MAJOR — YAML frontmatter non-standard enumeration values**
`status: Planned` (L4) is not in the allowed set {draft, active, implemented, deprecated}. `priority: P0` (L8) is not in the allowed set {critical, high, medium, low}. Severity: major — MP-3 FAIL.

**D4. spec.md:L246–L326 — MAJOR — ACs written as Given/When/Then, not EARS**
All 16 ACs (AC-OB-001..016) use Given/When/Then test-scenario form. Per MP-2, every acceptance criterion must match one of five EARS patterns. GWT scenarios are valuable for testing but do not satisfy MP-2. Either convert to EARS (e.g., "When a fresh install is launched, the app shall render the Step 1 Welcome modal as a full-screen overlay") or restructure the document so the EARS block IS the acceptance criteria and GWT is labeled "Test Scenarios". Severity: major — MP-2 FAIL.

**D5. spec.md:L226 (REQ-OB-013), L242 (REQ-OB-019), L230 (REQ-OB-014), L234 (REQ-OB-016), L232 (REQ-OB-015) — MAJOR — 5 REQs lack AC coverage**
- REQ-OB-013 (State-Driven, "soft notice" when all rituals unchecked): no AC exercises the notice display.
- REQ-OB-014 (Unwanted, no external transmission except Step 7/8 opt-in): no AC verifies the non-transmission invariant.
- REQ-OB-015 (Unwanted, invalid API key rejection): AC-OB-007 tests the valid-key path only; no AC covers malformed/wrong-prefix rejection.
- REQ-OB-016 (Unwanted, no sensitive data collection): no AC verifies fields list.
- REQ-OB-019 (Optional, accessibility): no AC verifies `prefers-reduced-motion` / WCAG AA contrast behavior.
Traceability break: 5/19 = 26% of REQs are uncovered. Severity: major.

**D6. spec.md:L313–L316 (AC-OB-014) — MAJOR — Language/locale contradiction**
AC-OB-014 specifies `country="FR"` and expects the error message "명시적 동의는 스킵할 수 없습니다" (Korean), annotated "(프랑스어)". For a French user, UI text should be French per REQ-OB-003 (spec.md:L202). Either the expected string or the annotation is wrong. Severity: major — test cannot pass as written and directly contradicts REQ-OB-003.

**D7. spec.md:L248–L326 — MAJOR — No AC explicitly references a REQ-XXX identifier**
AC-OB-001..016 contain zero explicit REQ-OB-XXX citations. Mapping relies on implicit adjacency. Traceability cannot be automated; a REQ deletion or renumber would silently orphan ACs. Severity: major.

**D8. spec.md:L323–L326 (AC-OB-016) — MINOR — "수동 측정 CI test" is self-contradictory**
An AC that says "manual measurement" cannot also be a "CI test". If automated, specify the Playwright script (research.md §2.3 provides it — reference it). Severity: minor.

**D9. spec.md:L226 (REQ-OB-013) — MINOR — "soft notice" is non-measurable copy**
"soft notice: 'You can always enable rituals later from Preferences'" gives the copy but "soft notice" is UX-vague (banner? toast? inline text?). Specify the surface. Severity: minor.

**D10. spec.md:L20–L21 vs L44–L58 — MINOR — Amendment delta not propagated**
Amendment states CLI wizard and Web UI wizard replace the Desktop 8-step. Steps 1–8 (L44–L58) still list egg hatching animation, Mobile QR pairing (Step 7/8 at L55–L58 and REQ-OB-018 at L240), and Desktop-only Tauri flow. If CLI+Web UI is now authoritative, this content must be rewritten, not left as "재구성 필요" (restructuring needed). Severity: minor (dependent on D1 resolution).

---

## Chain-of-Verification Pass

Second-look findings:

- Re-read each of REQ-OB-001..019 end-to-end: confirmed sequential, all EARS patterns applied correctly. No duplicates.
- Re-verified AC→REQ coverage by tabulating each REQ against each AC: confirmed D5 finding (5 uncovered REQs). Spot-check became complete enumeration.
- Re-read Exclusions (spec.md:L646–L658): specific and concrete (11 numbered exclusions) — this is strong. No defect here.
- Additional contradiction check: spec.md:L42 "Desktop App(DESKTOP-001)에 호스트되며" directly contradicts the amendment's CLI+Web UI scope (L19). This reinforces D1 — the contradiction is pervasive, not isolated.
- Research.md cross-check: research.md (all sections) is consistent with the OLD 8-step Desktop model — it references Tauri, framer-motion, keychain, mobile pairing, and 8 steps throughout. Research does NOT reflect the v0.2 amendment either. The amendment is orphaned in the header without propagation to research.
- Verified Exclusions (L657 "CLI 환경에서 온보딩을 강제하지 않는다. CLI-001은 환경변수 + config.yaml 직접 편집 경로 유지") **directly contradicts** the amendment claim that a `goose init` CLI wizard is now IN SCOPE. This is an additional contradiction, added below as D11.

**D11. spec.md:L657 vs L19 — CRITICAL — Exclusion contradicts amended IN SCOPE**
Exclusion explicitly says "본 SPEC은 CLI 환경에서 온보딩을 강제하지 않는다. CLI-001은 환경변수 + config.yaml 직접 편집 경로 유지." This directly contradicts the v0.2 amendment (L19) which establishes "`goose init` CLI 마법사" as a core deliverable. Either the amendment or the exclusion must be corrected — the SPEC cannot both mandate and exclude the CLI wizard. Severity: critical.

No other defects surfaced in the second pass.

---

## Regression Check
N/A — iteration 1.

---

## Recommendation

**Verdict: FAIL** — 2 critical defects (D1, D11), 6 major defects (D2–D7), 3 minor defects (D8–D10). Two must-pass criteria fail (MP-2, MP-3). manager-spec must revise before this SPEC is eligible for the Run phase.

Required fixes (ordered by priority):

1. **Resolve scope contradiction (D1, D11)** — Orchestrator must decide: is the deliverable (a) the original Desktop Tauri 8-step onboarding, or (b) the amended `goose init` CLI wizard + Web UI installer? Whichever path is chosen, ALL of the following must reflect it consistently: title (L15), L25 secondary title, §1 Overview (L35–L61), §2 Background (L63–L107), §3 Scope (L109–L191), EARS REQs (L194–L242), ACs (L246–L326), Technical Approach (L330–L572), Exclusions (L646–L658), and `research.md` in its entirety. Do not merge both.

2. **Fix YAML frontmatter (D2, D3)** — Rename `created` → `created_at`; add `labels:` (array); normalize `status: Planned` → `status: draft` (or `active` if implementation is approved); normalize `priority: P0` → `priority: critical`.

3. **Convert ACs to EARS or split the document structure (D4)** — Either (a) rewrite AC-OB-001..016 as EARS statements, or (b) rename the "수용 기준 (Acceptance Criteria)" section to "Test Scenarios" and let the EARS REQ block serve as the acceptance criteria — and add explicit REQ→Test cross-references.

4. **Close AC coverage gaps (D5)** — Add at least one AC for each of REQ-OB-013, REQ-OB-014, REQ-OB-015 (invalid-key path), REQ-OB-016, REQ-OB-019.

5. **Add explicit REQ-XXX references to every AC (D7)** — Each AC should start with or include "(verifies REQ-OB-NNN)".

6. **Fix AC-OB-014 locale inconsistency (D6)** — If country=FR, either use French error text or change the country in the example to one whose UI language matches the quoted string.

7. **Tighten vague language (D8, D9)** — Replace "수동 측정 CI test" with the concrete Playwright test reference; specify the surface/component for the "soft notice" in REQ-OB-013.

8. **Propagate or revert the amendment (D10)** — Either fully restructure the body to match the amendment or explicitly revert the amendment and remove L17–L22.

After revision, re-submit for iteration 2 audit.
