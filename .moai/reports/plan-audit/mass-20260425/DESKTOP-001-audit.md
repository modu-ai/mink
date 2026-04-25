# SPEC Review Report: SPEC-GOOSE-DESKTOP-001
Iteration: 1/3
Verdict: FAIL
Overall Score: 0.52

Reasoning context ignored per M1 Context Isolation. This audit was conducted by reading only spec.md (primary) and research.md (cross-reference). No author reasoning, prior drafts, or external context was consulted.

---

## Plausible Failure Modes Checked (M2 adversarial stance)

Before reading the SPEC, I enumerated these failure modes to check:
1. REQ numbering gaps / duplicates
2. EARS non-compliance in acceptance criteria (common: BDD Given/When/Then disguised as EARS)
3. YAML frontmatter missing required fields or wrong types
4. HOW leak into REQs (library names, API schemas, versions)
5. Broken traceability (orphan REQs, orphan ACs)
6. Hardcoded tool/library names in multi-language content
7. Weak Exclusions section
8. Internal contradictions
9. Weasel words in ACs
10. Desktop app specifics: tech-stack commitment without backup, OS scope incompleteness, daemon communication contract gaps, auto-update security contract, packaging matrix completeness

Findings per mode: #1 PASS (gap-free 001-015). #2 MAJOR FAIL (all 12 ACs are Given/When/Then BDD, none in EARS form). #3 FAIL (`created` not `created_at`, `labels` missing). #4 PASS (REQs largely WHAT/WHY with minor HOW leak). #5 FAIL (3 orphan REQs + 1 orphan AC). #6 MAJOR FAIL (hard-commits to Tauri v2 + specific plugin versions + Rust crates in REQ context). #7 PASS (§9 Exclusions specific). #8 MINOR (OUT OF SCOPE §3.2 partially overlaps §9 Exclusions). #9 MINOR (AC-DK-007 has a numeric bound, others rely on phrases like "즉시" / "1초 이내").

---

## Must-Pass Results

### [FAIL] MP-1 REQ number consistency
REQ-DK-001 through REQ-DK-015 are sequential, no duplicates, consistent zero-padding (spec.md:L113-L142). REQ-DK sequencing itself is clean. However, AC sequencing (AC-DK-001 through AC-DK-012) is also gap-free (spec.md:L220-L266).
Evidence (REQs enumerated): L113, L114, L115, L116, L120, L121, L122, L123, L127, L128, L132, L133, L137, L138, L142.
→ **PASS for REQ consistency** (reclassified). MP-1 is **PASS**.

Correction: MP-1 passes.

### [FAIL] MP-2 EARS format compliance
**ALL 12 acceptance criteria use BDD Given/When/Then format, not EARS patterns.** EARS requires:
- Ubiquitous: "The [system] shall [response]"
- Event-driven: "When [trigger], the [system] shall [response]"
- State-driven: "While [condition], the [system] shall [response]"
- Optional: "Where [feature exists], the [system] shall [response]"
- Unwanted: "If [undesired], then the [system] shall [response]"

ACs as written (spec.md:L220-L266) are test scenarios in BDD form, not EARS requirements:
- AC-DK-001 (L222): "**Given** 시스템에 `goosed`가 실행 중이지 않고 **When** 사용자가 Desktop App을 실행 **Then** 5초 이내에..."
- AC-DK-002 (L226): "**Given** Desktop이 실행 중 **When** 세션 상태가 learning → active로 변경 **Then**..."
- AC-DK-003 (L230), AC-DK-004 (L234), AC-DK-005 (L238), AC-DK-006 (L242), AC-DK-007 (L246), AC-DK-008 (L250), AC-DK-009 (L254), AC-DK-010 (L258), AC-DK-011 (L262), AC-DK-012 (L266): all use identical Given/When/Then scaffolding.

Per M3 rubric: "**Score 0.25** — Fewer than a quarter of ACs use EARS patterns; most are ... Given/When/Then test scenarios mislabeled as EARS." That describes this SPEC exactly — 0/12 ACs in EARS form = **0.0** rubric band.

Note: The label "수락 기준 (Acceptance Criteria)" (§7) implies these are executable test scenarios. If the author's intent was that §5 EARS Requirements ARE the acceptance contract and §7 is the test plan, the labeling is misleading and violates the standard MoAI SPEC contract where §7 ACs must be EARS-conformant.

→ **FAIL**. This is the critical defect of the SPEC.

### [FAIL] MP-3 YAML frontmatter validity
Required fields check (spec.md:L1-L13):
- `id: SPEC-GOOSE-DESKTOP-001` (string) — **PASS** (L2)
- `version: 0.1.0` (string) — **PASS** (L3)
- `status: Planned` — **PASS** (string present, L4). Note: "Planned" is not in the canonical enum {draft, active, implemented, deprecated}. This may be acceptable if the project extends the enum, but it is non-standard.
- `created_at` — **MISSING**. L5 has `created: 2026-04-21` which uses a different key. Per MP-3 requirement: "Required fields are: ... created_at (ISO date string)." → **FAIL**.
- `priority: P0` (string) — present (L8) but uses `P0` instead of the standard enum {critical, high, medium, low}. This is a type-value mismatch against the documented enum.
- `labels` — **MISSING** from frontmatter entirely. The required field is not declared. → **FAIL**.

Two required-field failures (`created_at` key, `labels` field) → MP-3 **FAIL**.

### [FAIL] MP-4 Section 22 language neutrality
This SPEC covers Desktop App UI, which is explicitly single-stack per §2.2 "왜 Tauri v2인가" (L46). However, the scoping question for MP-4 is whether the content is template-bound or universal.

Analysis:
- The SPEC hard-commits to Tauri v2, Rust, React 19, TypeScript, Vitest, Playwright, zustand, tailwindcss, shadcn/ui, framer-motion, i18next, react-i18next (L101-L106).
- It does NOT claim to be language-neutral or multi-language. It is explicitly a single-stack Desktop SPEC.
- Therefore, MP-4 is **N/A** per the rule: "If the SPEC is clearly scoped to a single-language project, this criterion is N/A and auto-passes."

However, a secondary concern: the OS matrix (§3.1 item 9, L84, and AC-DK-012 L265) claims "macOS(x64/arm64), Linux(x64/arm64), Windows(x64)" — 5 platforms. macOS x64 is listed but **Windows ARM64 is excluded without rationale**. If the Desktop promises cross-platform parity with a Rust/Tauri toolchain that natively supports Windows-on-ARM, omitting it is an OS-scope completeness gap. This is not a MP-4 failure (it is OS platforms, not programming languages), but flagged as defect D11.

→ **N/A: single-language SPEC.**

---

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.70 | 0.75 band (minor ambiguity) | REQ-DK-002 (L114) "reflects GOOSE's current mood (calm, active, learning, alert)" clear; REQ-DK-009 (L127) exponential backoff concrete (1s/2s/4s/30s); but REQ-DK-006 (L121) "notification event (morning briefing, ritual reminder, proactive suggestion)" depends on unspecified `goosed` event schema; REQ-DK-011 (L132) "biometrics" provider API unspecified. |
| Completeness | 0.60 | between 0.50-0.75 band | All major sections present (HISTORY L17, Background L37, Scope L67, Dependencies L97, Requirements L109, Types L146, Acceptance L218, TDD L270, Exclusions L279). Frontmatter missing `labels` and uses wrong key `created` instead of `created_at`. No explicit §10 Security or Threat Model for what is clearly a security-sensitive SPEC (daemon signature verification, auto-update signing, biometric gating). |
| Testability | 0.55 | 0.50 band (several weasel words / judgment-required) | AC-DK-002 (L226) "1초 이내" testable; AC-DK-007 (L246) "청크 도착 간격 ≤50ms" precise; AC-DK-001 (L222) "5초 이내" precise. BUT: AC-DK-004 (L234) "모든 UI 문자열이 ko로 갱신" — "모든" requires enumeration; AC-DK-005 (L238) "즉시 다크 테마 적용" — "즉시" is a weasel word without a numeric bound; AC-DK-009 (L254) "서명 실패 메시지" — no spec for message content/user channel; AC-DK-011 (L262) "확인 다이얼로그" — no spec on copy, button labels, default focus. |
| Traceability | 0.45 | between 0.25-0.50 band | Bidirectional mapping broken. REQ-DK-006 (notifications, L121), REQ-DK-011 (biometrics, L132), REQ-DK-012 (native menu bar, L133) have NO corresponding AC. AC-DK-012 (cross-platform build, L266) does not reference any REQ. No REQ mandates "CI produces 5-platform artifacts" — AC-DK-012 is orphaned. Additionally, no AC explicitly covers REQ-DK-007 (auto-updater release-notes prompt) — AC-DK-009 covers signature failure but not the happy-path prompt-before-download. |

---

## Defects Found

**D1 (critical). spec.md:L220-L266 — All 12 acceptance criteria use BDD Given/When/Then form instead of EARS patterns (MP-2 violation).** Every AC opens with "**Given** ... **When** ... **Then** ...". None match the five EARS patterns. Severity: **critical**. Fix: either (a) rewrite each AC in EARS form (e.g., AC-DK-008 rewritten: "When the user clicks the window close button while the main window is visible, the Desktop App shall hide the window and keep the tray icon active without terminating the process"), or (b) explicitly declare §5 EARS Requirements as the acceptance contract and rename §7 to "Test Scenarios" so readers do not mistake BDD scenarios for EARS ACs. Option (a) is preferred.

**D2 (critical). spec.md:L5 — Frontmatter uses `created:` instead of the required `created_at:` key (MP-3 violation).** The schema requires `created_at`. A different key name means downstream tooling that parses the schema will report the field as missing. Severity: **critical**. Fix: rename `created:` to `created_at:` on L5.

**D3 (critical). spec.md:L1-L13 — Frontmatter omits the required `labels` field entirely (MP-3 violation).** The schema requires `labels` (array or string). No such field exists anywhere in the frontmatter block. Severity: **critical**. Fix: add `labels:` with at least one domain tag (e.g., `[desktop, tauri, ui, phase-6]`).

**D4 (major). spec.md:L121 — REQ-DK-006 (notification dispatch) has no corresponding acceptance criterion.** The REQ mandates OS-native notification dispatch on `goosed` events, but §7 contains no AC verifying this behavior. Severity: **major**. Fix: add AC-DK-013 covering notification event → OS notification flow (e.g., "When `goosed` sends a notification event, the Desktop App shall invoke tauri-plugin-notification within N ms and the notification shall be observable by the OS notification center").

**D5 (major). spec.md:L132 — REQ-DK-011 (biometric gating) has no corresponding acceptance criterion.** Biometric auth is a security-sensitive feature without test coverage. Severity: **major**. Fix: add AC covering "When OS supports biometrics and user enables gating, accessing Preferences without authentication shall be blocked" and "When OS does not support biometrics, the offer shall not appear".

**D6 (major). spec.md:L133 — REQ-DK-012 (native macOS menu bar) has no corresponding acceptance criterion.** Severity: **major**. Fix: add AC covering standard menu items (File/Edit/View/Window/Help) and their shortcuts on macOS only.

**D7 (major). spec.md:L266 — AC-DK-012 (cross-platform CI build) references no REQ and has no REQ mandating this behavior.** §3.1 item 9 (L84) lists packaging targets but is not declared as a normative REQ with an REQ-DK-XXX id. Severity: **major**. Fix: add an ubiquitous REQ like "The Desktop App CI pipeline shall produce signed artifacts for macOS (x64, arm64), Linux (x64, arm64), and Windows (x64)" and link AC-DK-012 to it. Also reconcile whether Windows ARM64 should be in scope (D11).

**D8 (major). spec.md:L113 — REQ-DK-001 contains HOW leak: "bundle a `goosed` daemon binary and start it automatically on launch when no daemon is detected on the configured gRPC port."** The "configured gRPC port" implementation detail is fine, but "bundle a daemon binary" prescribes the packaging strategy (bundled vs download), which §7 research open-issue #5 (research.md:L164) admits is unresolved. REQ fixes the answer without justification. Severity: **major**. Fix: either (a) defer packaging strategy to a later SPEC and rephrase REQ-DK-001 as "shall ensure a goosed daemon is running when the Desktop App is active (bundled or separately installed)"; or (b) explicitly note that bundling is the v0.1 strategy with a migration path.

**D9 (major). spec.md:L137 — REQ-DK-013 (tamper warning) prescribes signature verification without defining the verification contract.** "expected public key" — where is the public key stored? How is it distributed? Research.md open-issue #1 (L160) says this is unresolved. A security-critical REQ with unspecified key-distribution mechanism is not implementable unambiguously. Severity: **major**. Fix: reference a separate SPEC or appendix defining the public-key distribution strategy (embed in binary? fetch from well-known URL?), or inline a concrete mechanism.

**D10 (major). spec.md:L238 and L254 — Weasel words in ACs.** AC-DK-005 uses "즉시" (immediately) without numeric bound; AC-DK-009 says "서명 실패 메시지가 표시되고" without specifying message channel (dialog? toast? log-only?) or content contract. Severity: **major**. Fix: replace "즉시" with a numeric threshold (e.g., "within 500ms of process start"). Specify the error-channel contract for signature failures.

**D11 (minor). spec.md:L84, L266 — Platform matrix omits Windows ARM64 without justification.** macOS ARM64 is in scope but Windows ARM64 (increasingly common with Copilot+ PCs) is not. This is a scope gap that should be explicitly excluded with rationale, not silently omitted. Severity: **minor**. Fix: either add Windows ARM64 to the matrix or add it to §9 Exclusions with a rationale.

**D12 (minor). spec.md:L87-L93 vs L279-L286 — Redundancy between §3.2 OUT OF SCOPE and §9 Exclusions.** Both sections list "Voice input / wake word", "Store 등록", "QR code generation". If they are identical, one is redundant; if they are intentionally different scopes (IN-PHASE vs ALL-TIME exclusions), the distinction must be stated. Severity: **minor**. Fix: clarify the distinction or consolidate.

**D13 (minor). spec.md:L114 — REQ-DK-002 "GOOSE's current mood (calm, active, learning, alert)".** The mood source is not specified — is it computed by Desktop from session state, streamed from `goosed`, or both? Without this, implementation ambiguity remains. Severity: **minor**. Fix: add a subclause or reference to where mood is authoritatively computed.

**D14 (minor). spec.md:L4 — `status: Planned` uses a non-canonical value.** The standard enum is {draft, active, implemented, deprecated}. "Planned" is not in the enum. Severity: **minor**. Fix: align with the canonical enum or document the project's custom enum in a separate status-taxonomy section.

**D15 (minor). spec.md:L8 — `priority: P0` uses P0/P1/P2-style labels instead of the canonical {critical, high, medium, low} enum.** Severity: **minor**. Fix: map P0→critical or declare a custom enum.

**D16 (minor). spec.md:L262 — AC-DK-011 references "⌘W (macOS) or Ctrl+W (other)" but the REQ-DK-015 specifies the same.** The AC copy implies a dialog appears but does not define the dialog's decision options (Cancel / Continue / Force-close) or what happens on each. Severity: **minor**. Fix: enumerate dialog outcomes and their post-conditions.

**D17 (minor). spec.md:L220-L266 — AC section lacks any AC for the HAPPY path of auto-update (REQ-DK-007: release notes prompt before download).** AC-DK-009 covers the failure case. No AC covers: "user accepts update → download begins → progress shown → install on next launch." Severity: **minor**. Fix: add AC-DK-014 for the happy-path auto-update flow.

---

## Chain-of-Verification Pass (M6)

Second-pass re-read of each section:

- **§1 Overview / §2 Background (L25-L64)**: non-normative, no new defects.
- **§3 Scope (L67-L93)**: Re-verified item 9 "macOS `.dmg` + 코드사인, Linux `.deb`/`.rpm`/AppImage, Windows `.msi` + 코드사인" — these are packaging targets but NOT captured as a REQ. Confirmed D7 (orphan AC-DK-012).
- **§4 Dependencies (L97-L106)**: Library version list is fine for architecture-level SPEC but commits to specific minor versions (react 19.x, Tauri 2.x, tailwindcss 4.x). Not a defect per se — noted as design decision.
- **§5 Requirements (L109-L142)**: Re-verified all 15 REQs end-to-end. Confirmed REQ numbering is gap-free. Confirmed D4/D5/D6 (REQ-DK-006/011/012 have no AC).
- **§6 Types (L146-L214)**: Interfaces are well-formed. Rust Tauri commands (L205-L214) show only 2 commands while §5 REQs imply many more (tray update, shortcut register, window toggle, update install, etc.). The §6 listing is a *sample*, not exhaustive. Minor gap — not elevated to defect because §6 is illustrative per SPEC convention.
- **§7 Acceptance Criteria (L218-L266)**: Re-verified every single AC — confirmed 0/12 are in EARS form. D1 confirmed as critical.
- **§8 TDD Strategy (L270-L275)**: Non-normative, OK.
- **§9 Exclusions (L279-L286)**: 6 specific exclusions, sufficient. D12 (redundancy with §3.2) confirmed minor.
- **Frontmatter (L1-L13)**: Re-verified field-by-field. `created:` ≠ `created_at:` (D2). `labels` absent (D3). Confirmed.
- **Internal contradictions sweep**: §3.2 OUT OF SCOPE "QR code generation" vs §5 which never claims Desktop generates QR — consistent. §2.3 CLI relationship vs §3 Scope — consistent. No hard contradictions found on re-read.

New defects from second pass: **D17 identified in second pass** (missing happy-path auto-update AC). First-pass reviewer had noted D4 (REQ-DK-006) but missed that REQ-DK-007 also lacks a happy-path AC (only the failure-case AC-DK-009 exists).

Second-pass confirmed: audit was otherwise thorough; no missed structural defects.

---

## Regression Check

Not applicable — iteration 1.

---

## Recommendation

**Verdict: FAIL.** Three must-pass criteria (MP-2, MP-3, MP-3) fail. MP-1 passes; MP-4 is N/A.

### Fix instructions for manager-spec (priority order)

1. **[critical, MP-3] Frontmatter repair (spec.md:L1-L13)**:
   - Rename `created:` to `created_at:` (L5).
   - Add `labels:` field, e.g. `labels: [desktop, tauri, ui, phase-6, cross-platform]`.
   - Optional: align `status: Planned` to `draft` or declare custom enum; align `priority: P0` to `critical` or declare custom enum.

2. **[critical, MP-2] Rewrite §7 acceptance criteria in EARS form (spec.md:L218-L266)**:
   - Convert all 12 BDD scenarios to EARS statements. Examples:
     - AC-DK-001 → "When the Desktop App launches and no `goosed` daemon is detected on the configured gRPC port, the Desktop App shall spawn `goosed` and establish a successful gRPC Ping response within 5 seconds, then display the main window."
     - AC-DK-008 → "When the user clicks the main window close button, the Desktop App shall hide the window to the system tray and preserve the running process."
     - AC-DK-013 (Unwanted Behavior pattern) → "If the bundled `goosed` signature does not match the expected ed25519 public key, then the Desktop App shall not spawn the daemon and shall display a tamper-warning dialog."
   - If the author intends the current §7 content as test scenarios (not ACs), rename §7 to "Test Scenarios (BDD)" and elevate §5 EARS Requirements to dual-role (requirement + acceptance contract) with explicit statement.

3. **[major] Add missing ACs for orphan REQs**:
   - REQ-DK-006 (notification) → AC-DK-013.
   - REQ-DK-011 (biometric) → AC-DK-014.
   - REQ-DK-012 (macOS menu bar) → AC-DK-015.
   - REQ-DK-007 happy-path (auto-update prompt + download + install) → AC-DK-016.

4. **[major] Add REQ for cross-platform CI build and link AC-DK-012**:
   - New ubiquitous REQ-DK-016 covering the 5-platform signed-artifact matrix.
   - Reconsider Windows ARM64 inclusion/exclusion (D11).

5. **[major] Resolve HOW leaks and underspecified security contracts**:
   - REQ-DK-001 (L113) — relax "bundle a goosed binary" or explicitly scope to v0.1.
   - REQ-DK-013 (L137) — reference public-key distribution mechanism (research.md:L160 open-issue #1 must be resolved before merge).

6. **[major] Tighten weasel-word ACs**:
   - AC-DK-005 (L238) "즉시" → numeric bound (e.g., ≤500ms from app launch).
   - AC-DK-009 (L254) — specify error-channel (dialog vs toast) and minimum message content.

7. **[minor] Scope hygiene**:
   - D11 Windows ARM64 decision.
   - D12 consolidate §3.2 OUT OF SCOPE and §9 Exclusions or clarify distinction.
   - D13 specify mood authoritative source.
   - D16 specify ⌘W dialog outcomes.

Target for iteration 2: all MP-* must pass; Traceability ≥ 0.80; Testability ≥ 0.80.
