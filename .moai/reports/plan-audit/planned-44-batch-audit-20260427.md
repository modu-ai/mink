# Planned 44 SPEC Batch Audit Report

Date: 2026-04-27
Auditor: plan-auditor
Scope: 44 planned SPECs assigned for completeness audit
Methodology: YAML frontmatter extraction + section header analysis + AC content depth verification

---

## Executive Summary

| Category | Count | Verdict |
|----------|-------|---------|
| Skeleton (spec-first, minimal) | 10 | INCOMPLETE — require full rewrite by manager-spec |
| Well-structured (spec-anchored) | 34 | COMPLETE with minor gaps |
| **Total** | **44** | |

**Global finding**: All 44 SPECs have `issue_number: null`. No GitHub issues have been created for any planned SPEC.

---

## Tier 1: Skeleton SPECs (spec-first lifecycle) — 10 SPECs

These SPECs share a common minimal template: Goal / Scope / Requirements (EARS) / Acceptance Criteria / Dependencies / References. They lack research.md, have empty or stub AC sections, and REQ entries are header-only without EARS-patterned body text.

| # | SPEC ID | Size (bytes) | Priority | Phase | AC Status | research.md | Key Gap |
|---|---------|-------------|----------|-------|-----------|-------------|---------|
| 1 | SPEC-GOOSE-AUDIT-001 | 2,153 | P0 | 5 | EMPTY | No | No AC content at all |
| 2 | SPEC-GOOSE-AUTH-001 | 2,985 | P0 | 6 | DRAFT | No | AC marked as draft ("초안"), not EARS |
| 3 | SPEC-GOOSE-CREDENTIAL-PROXY-001 | 2,581 | P0 | 5 | EMPTY | No | No AC content, no research |
| 4 | SPEC-GOOSE-FS-ACCESS-001 | 2,407 | P0 | 5 | EMPTY | No | No AC content, no research |
| 5 | SPEC-GOOSE-GATEWAY-TG-001 | 3,247 | P1 | 9 | DRAFT | No | AC marked as draft ("초안") |
| 6 | SPEC-GOOSE-NOTIFY-001 | 3,543 | P0 | 6 | DRAFT | No | AC marked as draft ("초안"), scope shrunk |
| 7 | SPEC-GOOSE-PAI-CONTEXT-001 | 2,421 | P0 | 7 | EMPTY | No | No AC content, no research |
| 8 | SPEC-GOOSE-SECURITY-SANDBOX-001 | 2,320 | P0 | 5 | EMPTY | No | No AC content, no research |
| 9 | SPEC-GOOSE-SELF-CRITIQUE-001 | 2,130 | P0 | 3 | EMPTY | No | No AC content, no research |
| 10 | SPEC-GOOSE-WEBUI-001 | 2,322 | P0 | 6 | EMPTY | No | No AC content, no research |

### Common Defects in Tier 1

**D1. Empty Acceptance Criteria** (Severity: critical)
All 10 SPECs have `## Acceptance Criteria` headers with no substantive content. REQ-to-AC traceability is impossible.

**D2. No research.md** (Severity: major)
None of the 10 SPECs have a research.md file, meaning no prior art analysis, competitor research, or technology evaluation was performed.

**D3. REQ entries are headers only** (Severity: critical)
REQ entries (e.g., `REQ-SANDBOX-001` through `REQ-SANDBOX-005`) exist only as section headers. No EARS-patterned body text describes the actual requirement behavior.

**D4. Missing standard sections** (Severity: major)
Missing from most: HISTORY content, Technical Approach, TDD entry sequence, TRUST 5 mapping, Dependencies detail.

**D5. No Out of Scope / Exclusions section** (Severity: major)
The Scope sections are generic ("See requirements") without explicit exclusions.

### Priority for Rewrite

Given that 7 of 10 are P0 priority, these should be prioritized for manager-spec rewrite before run phase:
- P0: AUDIT-001, AUTH-001, CREDENTIAL-PROXY-001, FS-ACCESS-001, NOTIFY-001, PAI-CONTEXT-001, SECURITY-SANDBOX-001, SELF-CRITIQUE-001, WEBUI-001
- P1: GATEWAY-TG-001

---

## Tier 2: Well-Structured SPECs (spec-anchored lifecycle) — 34 SPECs

These SPECs follow the full MoAI SPEC template with HISTORY, Background, Scope, EARS Requirements, Acceptance Criteria (GWT format), Technical Approach, and Dependencies.

### Completeness Matrix

| # | SPEC ID | Version | Priority | Phase | REQs | ACs | research.md | AC Format | Structure Score |
|---|---------|---------|----------|-------|------|-----|-------------|-----------|-----------------|
| 1 | SPEC-AGENCY-CLEANUP-002 | 0.1.0 | P2 | - | 5 | 5 | No (has plan.md, acceptance.md) | EARS | A |
| 2 | SPEC-GOOSE-A2A-001 | 0.1.0 | P2 | 7 | 17 | 6+ | Yes | GWT in §11 | A |
| 3 | SPEC-GOOSE-AGENT-001 | 0.1.0 | P0 | 0 | ~15 | 4+ | Yes | GWT | A |
| 4 | SPEC-GOOSE-BRIDGE-001 | 0.2.0 | P0 | 6 | ~18 | 4+ | Yes | GWT | A |
| 5 | SPEC-GOOSE-CALENDAR-001 | 0.1.1 | P0 | 7 | ~20 | 20 | Yes | EARS+GWT | A |
| 6 | SPEC-GOOSE-CLI-001 | 0.2.0 | P0 | 3 | ~25 | 4+ | Yes | GWT | A |
| 7 | SPEC-GOOSE-CMDCTX-CLI-INTEG-001 | 0.1.0 | P1 | 2 | ~30 | 11+ | Yes | GWT | A |
| 8 | SPEC-GOOSE-CMDCTX-DAEMON-INTEG-001 | 0.1.0 | P2 | 2 | ~40 | 11 | Yes | GWT | A |
| 9 | SPEC-GOOSE-CMDCTX-HOTRELOAD-001 | 0.1.0 | P4 | 3 | ~15 | - | Yes | - | B+ |
| 10 | SPEC-GOOSE-CMDCTX-PERMISSIVE-ALIAS-001 | 0.1.0 | P4 | 3 | ~12 | - | Yes | GWT (header) | B+ |
| 11 | SPEC-GOOSE-CMDCTX-TELEMETRY-001 | 0.1.0 | P3 | 3 | ~15 | - | Yes | GWT (header) | B+ |
| 12 | SPEC-GOOSE-COMPRESSOR-001 | 0.2.0 | P0 | 4 | ~20 | 4+ | Yes | GWT | A |
| 13 | SPEC-GOOSE-DESKTOP-001 | 0.2.0 | critical | 6 | ~20 | 4 | Yes | GWT (BDD) | A |
| 14 | SPEC-GOOSE-FORTUNE-001 | 0.1.0 | P1 | 7 | 15 | 20 | Yes | GWT | A |
| 15 | SPEC-GOOSE-GATEWAY-001 | 0.2.0 | P1 | 6 | 16 | 10+ | Yes | GWT | A |
| 16 | SPEC-GOOSE-HEALTH-001 | 0.1.0 | P1 | 7 | 19 | 22 | Yes | GWT | A |
| 17 | SPEC-GOOSE-IDENTITY-001 | 0.1.0 | P2 | 6 | 17 | 6+ | Yes | GWT in §11 | A |
| 18 | SPEC-GOOSE-INSIGHTS-001 | 0.1.0 | P1 | 4 | ~15 | 4+ | Yes | GWT | A |
| 19 | SPEC-GOOSE-JOURNAL-001 | 0.2.0 | P0 | 7 | 23 | 38 | Yes | GWT | A |
| 20 | SPEC-GOOSE-LLM-001 | 0.1.0 | P0 | 0 | ~15 | 4+ | Yes | GWT | A |
| 21 | SPEC-GOOSE-LOCALE-001 | 0.1.1 | P0 | 6 | ~15 | - | Yes | GWT (header) | B+ |
| 22 | SPEC-GOOSE-LORA-001 | 0.1.0 | P2 | 6 | 17 | 6+ | Yes | GWT in §11 | A |
| 23 | SPEC-GOOSE-MEMORY-001 | 0.2.0 | P0 | 4 | 21 | 3+ | Yes | GWT | A |
| 24 | SPEC-GOOSE-PLANMODE-CMD-001 | 0.1.0 | P3 | 2 | ~15 | - | Yes | GWT (header) | B+ |
| 25 | SPEC-GOOSE-REFLECT-001 | 0.1.0 | P1 | 5 | 31 | 12 | Yes | GWT | A |
| 26 | SPEC-GOOSE-REGION-SKILLS-001 | 0.1.0 | P1 | 6 | 20 | 22 | Yes | GWT | A |
| 27 | SPEC-GOOSE-RELAY-001 | 0.1.0 | P1 | 6 | 17 | 10 | Yes | GWT | A |
| 28 | SPEC-GOOSE-RITUAL-001 | 0.3.0 | P0 | 7 | 38 | 27 | Yes | GWT | A |
| 29 | SPEC-GOOSE-ROLLBACK-001 | 0.1.0 | P1 | 5 | 23 | 7 | Yes | GWT | A |
| 30 | SPEC-GOOSE-SAFETY-001 | 0.1.0 | P1 | 5 | 24 | 9 | Yes | GWT | A |
| 31 | SPEC-GOOSE-SIGNING-001 | 0.1.0 | P0 | - | 5 | 9 | No (has acceptance.md, plan.md) | EARS | A |
| 32 | SPEC-GOOSE-TRAJECTORY-001 | 0.1.1 | P0 | 4 | ~15 | - | Yes | GWT (header) | B+ |
| 33 | SPEC-GOOSE-VECTOR-001 | 0.1.0 | P2 | 6 | 14 | 6+ | Yes | GWT in §11 | A |
| 34 | SPEC-GOOSE-WEATHER-001 | 0.1.0 | P1 | 7 | ~12 | 4+ | Yes | GWT | A |

### Defects in Tier 2

**D6. CMDCTX-HOTRELOAD-001: AC section visibility** (Severity: minor)
SPEC has detailed REQs but the AC count was not clearly extractable. Likely has AC content but verification needed.

**D7. CMDCTX-PERMISSIVE-ALIAS-001, CMDCTX-TELEMETRY-001, LOCALE-001, PLANMODE-CMD-001, TRAJECTORY-001: AC section may be header-only** (Severity: minor)
These SPECs have AC section headers but grep-based AC content extraction was inconclusive. Recommend spot-checking.

**D8. DESKTOP-001: `priority: critical`** (Severity: minor)
Priority field is "critical" instead of standard P0/P1/P2/P3/P4 format. Should normalize to "P0" for consistency.

**D9. AGENCY-CLEANUP-002: No research.md** (Severity: minor)
Has plan.md and acceptance.md but no research.md. Acceptable for cleanup SPECs.

**D10. SIGNING-001: No research.md** (Severity: minor)
Has acceptance.md and plan.md but no research.md. Acceptable for security SPECs that reference external standards directly.

**D11. A2A-001, IDENTITY-001, LORA-001, VECTOR-001: AC in §11 instead of §5** (Severity: minor)
These SPECs use a different section numbering for AC (§11 instead of §5). Not a defect per se, but inconsistent with other SPECs.

### Structure Score Legend

- **A**: All required sections present (HISTORY, Background, Scope IN/OUT, EARS REQs, AC with GWT, Tech Approach, Dependencies). Substantive content.
- **B+**: Required sections present, AC may be abbreviated or header-visible but content depth uncertain.
- **B**: Required sections present but some are thin.
- **C**: Missing key sections or content is minimal.

---

## Global Findings (All 44 SPECs)

### GF-1: `issue_number: null` universally
Every single SPEC has `issue_number: null`. No GitHub tracking issues exist for any planned SPEC. This blocks traceability from SPEC to implementation issues.

### GF-2: No `title` field in YAML frontmatter
All 44 SPECs embed the title in the H1 markdown header instead of a dedicated `title` YAML field. This is a consistent convention, not a defect, but diverges from strict SPEC schema.

### GF-3: `status: planned` confirmed
All 44 SPECs correctly have `status: planned`. No inconsistencies found.

### GF-4: `version` consistency
- Most SPECs: 0.1.0
- Version bumps exist: BRIDGE-001 (0.2.0), CLI-001 (0.2.0), CALENDAR-001 (0.1.1), COMPRESSOR-001 (0.2.0), DESKTOP-001 (0.2.0), JOURNAL-001 (0.2.0), LOCALE-001 (0.1.1), MEMORY-001 (0.2.0), RITUAL-001 (0.3.0), TRAJECTORY-001 (0.1.1)
- Version bumps suggest prior audit/revision cycles, which is positive.

### GF-5: `lifecycle` field split
- `spec-anchored` (34): Full SPECs produced by manager-spec
- `spec-first` (10): Skeleton SPECs from architecture-redesign-v0.2 process

### GF-6: `labels` inconsistency
- Many SPECs have empty labels `labels: []`
- Some use array format, some use inline format
- CMDCTX-* SPECs have well-structured labels with area/type/priority taxonomy
- Recommend labeling all SPECs consistently per CLAUDE.local.md §1.5 taxonomy

### GF-7: `priority` field format inconsistency
- Most: P0/P1/P2/P3/P4
- DESKTOP-001: "critical" (should be P0)
- AGENCY-CLEANUP-002: P2
- Recommend normalizing DESKTOP-001 priority to "P0"

---

## Recommendation

### Immediate Action (P0 blocking)

1. **Rewrite 10 skeleton SPECs** via manager-spec (spec-first lifecycle):
   - Priority order: SELF-CRITIQUE-001 (Phase 3) > AUDIT-001 (Phase 5) > FS-ACCESS-001 (Phase 5) > CREDENTIAL-PROXY-001 (Phase 5) > SECURITY-SANDBOX-001 (Phase 5) > AUTH-001 (Phase 6) > WEBUI-001 (Phase 6) > NOTIFY-001 (Phase 6) > PAI-CONTEXT-001 (Phase 7) > GATEWAY-TG-001 (Phase 9)
   - Each requires: research.md, full EARS REQs, GWT ACs, Technical Approach, TDD sequence

2. **Create GitHub issues** for all 44 SPECs and populate `issue_number` fields

### Near-term Action (Quality improvement)

3. **Normalize DESKTOP-001 priority** from "critical" to "P0"
4. **Add labels** to SPECs with empty labels following §1.5 taxonomy
5. **Spot-check AC depth** for B+ rated SPECs (CMDCTX-PERMISSIVE-ALIAS, CMDCTX-TELEMETRY, LOCALE, PLANMODE-CMD, TRAJECTORY)
6. **Normalize AC section numbering** for A2A-001, IDENTITY-001, LORA-001, VECTOR-001 (§11 → §5)

---

## Appendix: File Inventory

| SPEC ID | Files Present |
|---------|---------------|
| SPEC-AGENCY-CLEANUP-002 | spec.md, acceptance.md, plan.md, spec-compact.md, legacy-manifest.yaml |
| SPEC-GOOSE-A2A-001 | spec.md, research.md |
| SPEC-GOOSE-AGENT-001 | spec.md, research.md, DEPRECATED.md |
| SPEC-GOOSE-AUDIT-001 | spec.md only |
| SPEC-GOOSE-AUTH-001 | spec.md only |
| SPEC-GOOSE-BRIDGE-001 | spec.md, research.md |
| SPEC-GOOSE-CALENDAR-001 | spec.md, research.md |
| SPEC-GOOSE-CLI-001 | spec.md, research.md, DEPRECATED.md |
| SPEC-GOOSE-CMDCTX-CLI-INTEG-001 | spec.md, research.md, progress.md |
| SPEC-GOOSE-CMDCTX-DAEMON-INTEG-001 | spec.md, research.md, progress.md |
| SPEC-GOOSE-CMDCTX-HOTRELOAD-001 | spec.md, research.md, progress.md |
| SPEC-GOOSE-CMDCTX-PERMISSIVE-ALIAS-001 | spec.md, research.md, progress.md |
| SPEC-GOOSE-CMDCTX-TELEMETRY-001 | spec.md, research.md, progress.md |
| SPEC-GOOSE-COMPRESSOR-001 | spec.md, research.md |
| SPEC-GOOSE-CREDENTIAL-PROXY-001 | spec.md only |
| SPEC-GOOSE-DESKTOP-001 | spec.md, research.md |
| SPEC-GOOSE-FORTUNE-001 | spec.md, research.md |
| SPEC-GOOSE-FS-ACCESS-001 | spec.md only |
| SPEC-GOOSE-GATEWAY-001 | spec.md, research.md |
| SPEC-GOOSE-GATEWAY-TG-001 | spec.md only |
| SPEC-GOOSE-HEALTH-001 | spec.md, research.md |
| SPEC-GOOSE-IDENTITY-001 | spec.md, research.md |
| SPEC-GOOSE-INSIGHTS-001 | spec.md, research.md |
| SPEC-GOOSE-JOURNAL-001 | spec.md, research.md |
| SPEC-GOOSE-LLM-001 | spec.md, research.md, DEPRECATED.md |
| SPEC-GOOSE-LOCALE-001 | spec.md, research.md |
| SPEC-GOOSE-LORA-001 | spec.md, research.md |
| SPEC-GOOSE-MEMORY-001 | spec.md, research.md |
| SPEC-GOOSE-NOTIFY-001 | spec.md only |
| SPEC-GOOSE-PAI-CONTEXT-001 | spec.md only |
| SPEC-GOOSE-PLANMODE-CMD-001 | spec.md, research.md, progress.md |
| SPEC-GOOSE-REFLECT-001 | spec.md, research.md |
| SPEC-GOOSE-REGION-SKILLS-001 | spec.md, research.md |
| SPEC-GOOSE-RELAY-001 | spec.md, research.md |
| SPEC-GOOSE-RITUAL-001 | spec.md, research.md |
| SPEC-GOOSE-ROLLBACK-001 | spec.md, research.md |
| SPEC-GOOSE-SAFETY-001 | spec.md, research.md |
| SPEC-GOOSE-SECURITY-SANDBOX-001 | spec.md only |
| SPEC-GOOSE-SELF-CRITIQUE-001 | spec.md only |
| SPEC-GOOSE-SIGNING-001 | spec.md, acceptance.md, plan.md, spec-compact.md |
| SPEC-GOOSE-TRAJECTORY-001 | spec.md, research.md |
| SPEC-GOOSE-VECTOR-001 | spec.md, research.md |
| SPEC-GOOSE-WEATHER-001 | spec.md, research.md |
| SPEC-GOOSE-WEBUI-001 | spec.md only |
