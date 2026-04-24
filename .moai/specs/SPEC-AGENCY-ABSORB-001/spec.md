---
id: AGENCY-ABSORB-001
version: 1.0.0
title: /agency를 /moai design으로 흡수·통합
status: completed
lifecycle_level: spec-first
created_at: 2026-04-20
updated_at: 2026-04-24
completed: 2026-04-24
author: GOOS행님
priority: P1
issue_number: null
labels: 
  - refactoring
  - skill-system
  - design-workflow
  - characterization
---

# SPEC-AGENCY-ABSORB-001: /agency를 /moai design으로 흡수·통합

## HISTORY

- 2026-04-24: Characterization SPEC 회고 작성 (Level 1 spec-first). 이미 구현·커밋된 흡수 작업을 EARS 형식으로 재구성하여 `.moai/specs/`에 등록. 본 SPEC은 코드 변경을 유발하지 않으며 단지 기존 구현을 문서화한다.
- 2026-04-20 (M1): `.claude/rules/agency/constitution.md`(v3.2.0)을 `.claude/rules/moai/design/constitution.md`(v3.3.0)로 이전 완료. 원본 파일은 redirect stub으로 유지.
- 2026-04-20 (M2): 6개 신규 design 스킬 추가 (moai-domain-brand-design, moai-domain-copywriting, moai-workflow-design-context, moai-workflow-design-import, moai-workflow-gan-loop, moai-workflow-pencil-integration).
- 2026-04-20 (M3-M5): /moai design 라우터·.moai/design/ 표준화·/agency 명령 8개 deprecation redirect 적용.

---

## Purpose

MoAI-ADK는 SPEC-First DDD 워크플로우를 단일 오케스트레이터(/moai)로 통합하는 방향으로 진화해 왔다. 그러나 별도 진화하던 `/agency` (AI Agency 창작 생산 파이프라인)는 다음 문제를 야기했다.

1. **명령 체계 분기**: `/moai`와 `/agency`가 서로 다른 헌법(`.claude/rules/moai/` vs `.claude/rules/agency/`)·서로 다른 에이전트 풀(planner/builder/evaluator/learner vs manager-spec/manager-ddd 등)을 운영하여, 사용자가 디자인 작업과 일반 개발을 오가며 두 가지 멘탈 모델을 유지해야 했다.
2. **스킬·에이전트 중복**: agency planner의 BRIEF 생성 로직이 manager-spec과 90% 이상 겹치고, agency builder의 expert-frontend 호출 패턴이 /moai run의 ddd/tdd 매니저와 동일한 구조였다.
3. **거버넌스 분기**: TRUST 5, 헌법 검증, evaluator-active, harness 라우팅 등 MoAI 본체의 품질 인프라를 agency에서는 별도로 재구현해야 했다.
4. **유지보수 부담**: agency-design-system, agency-copywriting 같은 스킬을 별도 트리로 관리하면서 `moai update` 시 동기화 비용이 증가했다.

본 SPEC은 이 분기를 종식하고, agency가 가진 **창작 생산 도메인 전문성**(브랜드, 카피, 디자인 토큰, GAN Loop)을 /moai 오케스트레이터 하위의 **수직 도메인**으로 흡수하여 다음을 달성한다.

- 단일 명령 체계: 디자인 작업도 `/moai design`을 통해 일반 개발 워크플로우와 동일한 오케스트레이션 인프라(AskUserQuestion, 헌법, harness, evaluator-active, MX 태그)를 사용한다.
- 단일 에이전트 풀: copywriter/designer는 도메인 스킬(`moai-domain-copywriting`, `moai-domain-brand-design`)로 흡수되고, planner/builder/evaluator/learner는 manager-spec, expert-frontend, evaluator-active, manager-strategy 등 기존 MoAI 에이전트로 매핑된다.
- 단일 헌법: agency constitution은 `.claude/rules/moai/design/constitution.md`로 이전되어 MoAI rule 로딩 체계에 편입된다.
- 하이브리드 경로 유지: Claude Design import(path A)와 코드 기반 brand design(path B)을 모두 지원하여 사용자의 구독 등급·작업 성격에 따른 선택권을 보존한다.

---

## Scope

### In Scope

본 SPEC이 문서화하는 흡수 범위:

1. **명령 체계 흡수**: `/agency` 및 7개 서브커맨드(brief, build, review, profile, resume, learn, evolve)를 `/moai` 산하의 `/moai design`(신규) 및 기존 서브커맨드로 매핑.
2. **헌법 이전**: `.claude/rules/agency/constitution.md`(v3.2.0)을 `.claude/rules/moai/design/constitution.md`(v3.3.0)로 verbatim 이전. FROZEN/EVOLVABLE zone 정의 보존, Section 3 분리 확장(Brand Context / Design Brief / Relationship), HISTORY 기록.
3. **스킬 신설/흡수**: 6개 신규 스킬 추가:
   - `moai-domain-brand-design` (← agency-design-system v1.0.0)
   - `moai-domain-copywriting` (← agency-copywriting v3.2.0)
   - `moai-workflow-design-context`
   - `moai-workflow-design-import`
   - `moai-workflow-gan-loop` (← agency constitution Section 11/12)
   - `moai-workflow-pencil-integration`
4. **워크플로우 신설**: `.claude/skills/moai/workflows/design.md` 추가, Phase 0(pre-flight) → Phase 1(route selection) → Phase A(import) / Phase B(code-based) → Phase C(quality gate) 정의.
5. **설정 통합**: `.moai/config/sections/design.yaml` 신설 (gan_loop, evolution, adaptation, brand_context, design_docs, claude_design 섹션).
6. **디자인 브리프 디렉터리 표준화**: `.moai/design/`(README, research.md, system.md, spec.md, wireframes/, screenshots/) 도입. 자동 로딩 우선순위 정의 및 reserved filename 보호.
7. **Deprecation**: 기존 `/agency` 명령 8개를 redirect stub으로 변환, CLAUDE.md §3·§4 갱신.

### Out of Scope (Non-Goals)

다음 항목은 명백히 본 SPEC의 책임 범위를 벗어나며, 별도 트랙으로 분리한다:

- **DB 메타 관리 통합**: `moai-domain-db-docs` 스킬, `.moai/config/sections/db.yaml`, `.moai/project/db/`는 별도 SPEC(예: `SPEC-DB-SYNC-RELOC-001`)에 속한다. 본 SPEC의 검증 대상이 아니다.
- **GOOSE 브랜드 파운데이션 수립**: 커밋 d02f512 (2026-04-23 이전)에서 수행된 `.moai/project/brand/`의 GOOSE 페르소나 정립은 본 SPEC의 선행 조건이지 결과물이 아니다.
- **agency-migration CLI 구현**: `moai migrate agency` 명령 자체의 코드 구현은 별도 트랙. 본 SPEC은 그 명령이 `/moai design` workflow에서 호출되어야 한다는 사실만 규정한다.
- **Pencil MCP 자체의 안정성**: `moai-workflow-pencil-integration`이 호출하는 MCP 서버의 가용성·버전 관리는 외부 의존성으로 간주.

---

## Exclusions (What NOT to Build)

본 SPEC은 회고적 characterization SPEC이므로 신규 구현 산출물을 만들지 않는다. 다음 사항은 명시적으로 제외한다:

- **신규 코드 작성 금지**: 본 SPEC은 이미 commit된 변경을 문서화할 뿐 추가 구현을 트리거하지 않는다.
- **기존 파일 수정 금지**: 헌법 이전 후의 redirect stub(`.claude/rules/agency/constitution.md`)도, deprecated /agency 명령 8개도 수정 대상이 아니다. 본 SPEC 작성 과정에서 이들을 손대지 않는다.
- **agency 디렉터리 즉시 삭제 금지**: `.claude/agents/agency/`, `.claude/rules/agency/`, `.claude/commands/agency/` 등 legacy 트리는 REQ-DEPRECATE-003에 따라 2 minor version cycle 동안 유지된다. 본 SPEC 시점에 즉시 삭제하면 안 된다.
- **브랜드 파일 재작성 금지**: `.moai/project/brand/{brand-voice,target-audience,visual-identity}.md`의 _TBD_ 마커 정리는 brand interview의 책임이며 본 SPEC의 acceptance 조건이 아니다.
- **DB 관련 변경 비포함**: `moai-domain-db-docs`, `db.yaml`, `.moai/project/db/`는 본 SPEC 검증·acceptance 대상에서 제외한다.
- **/agency 명령의 즉시 제거 금지**: REQ-DEPRECATE-003은 deprecation window를 명시한다. 본 SPEC 시점에 redirect stub만 존재하면 충족되며, 삭제 자체는 후속 SPEC의 책임이다.

---

## Dependencies

| Dependency | Type | Reference |
|------------|------|-----------|
| Thin Command Pattern | 선행(필수) | SPEC-THIN-CMDS-001 — `/moai design` 명령 파일이 routing wrapper(<20 LOC) 패턴을 따르려면 사전 정의 필요 |
| GOOSE 브랜드 파운데이션 | 선행(권장) | 커밋 d02f512 — `.moai/project/brand/` 구조 표준화 |
| DB 메타 관리 흡수 | 병렬 트랙 | SPEC-DB-SYNC-RELOC-001 (별도) — 동시기 진행되었으나 본 SPEC과 독립 |
| Claude Code v2.1.110+ | 런타임 | `effortLevel`, `disableBypassPermissionsMode`, Bash timeout 정책 등 디자인 워크플로우의 Opus 4.7 효율 활용을 위한 baseline |
| Pencil MCP | 외부 옵션 | Phase B2.6 활성화 시에만 필요. 부재 시 graceful skip |

---

## EARS Requirements

### Category 1: Command Routing & Migration (REQ-ABSORB)

**REQ-ABSORB-001 (Ubiquitous)**
The system SHALL retain the `/agency` command and its seven subcommands (brief, build, review, profile, resume, learn, evolve) as deprecation redirect wrappers that route invocations to the corresponding `/moai` subcommand.
- 증거: `.claude/commands/agency/{agency,brief,build,review,profile,resume,learn,evolve}.md` 8개 파일 모두 frontmatter `description`이 "(Deprecated)"로 시작하고 본문은 `Use Skill("moai")` 또는 `Use Skill("moai-workflow-research")` 라우팅으로 끝난다.

**REQ-ABSORB-002 (Event-Driven)**
WHEN a user invokes `/agency brief`, THE system SHALL route the call to `Skill("moai")` with `plan` subcommand and forward `$ARGUMENTS` unchanged.
- 매핑 표 (agency.md migration table 기준):

| /agency 서브커맨드 | /moai 매핑 |
|---|---|
| brief | plan |
| build | design |
| review | e2e |
| profile | project |
| resume | run |
| learn | (no direct equivalent) → moai-workflow-research |
| evolve | (no direct equivalent) → moai-workflow-research |

**REQ-ABSORB-003 (Event-Driven)**
WHEN a user invokes `/agency learn` or `/agency evolve`, THE system SHALL output `AGENCY_SUBCOMMAND_UNSUPPORTED` error code with migration guide URL and route to `Skill("moai-workflow-research")` instead of failing silently.
- 증거: `evolve.md`, `learn.md` 모두 `> ERROR: AGENCY_SUBCOMMAND_UNSUPPORTED — ... has no direct equivalent.` 라인 포함.

**REQ-ABSORB-004 (Ubiquitous)**
The system SHALL provide a new `/moai design` command at `.claude/commands/moai/design.md` as the canonical entry point for the hybrid design workflow (Claude Design path A or code-based path B).
- 증거: 파일 존재 + frontmatter `description: Hybrid design workflow — Claude Design import (path A) or code-based brand design (path B)`.

### Category 2: Skill Absorption (REQ-ABSORB cont.)

**REQ-ABSORB-005 (Ubiquitous)**
The system SHALL absorb the agency-copywriting capability into a new skill `moai-domain-copywriting` that preserves the v3.2.0 brand voice, anti-AI-slop rules, and JSON section structure (hero, features, social_proof, cta, footer).
- 증거: `.claude/skills/moai-domain-copywriting/SKILL.md` line 35 — "Absorbed from agency-copywriting (v3.2.0)".

**REQ-ABSORB-006 (Ubiquitous)**
The system SHALL absorb the agency-design-system capability into a new skill `moai-domain-brand-design` that preserves hero-first chaining, WCAG 2.1 AA contrast, and design token output.
- 증거: `.claude/skills/moai-domain-brand-design/SKILL.md` line 35 — "Absorbed from agency-design-system (v1.0.0)".

**REQ-ABSORB-007 (Ubiquitous)**
The system SHALL provide a new workflow skill `moai-workflow-gan-loop` that absorbs the Builder-Evaluator GAN Loop logic (Section 11) and Evaluator Leniency Prevention (Section 12) from the agency constitution, reading all loop parameters from `.moai/config/sections/design.yaml` (no hardcoded thresholds).
- 증거: `.claude/skills/moai-workflow-gan-loop/SKILL.md` line 35 — "Absorbed from agency constitution Section 11 and Section 12. Integrates Sprint Contract Protocol, 4-dimension scoring, stagnation detection, and Evaluator Leniency Prevention." 및 line 38 — "All loop parameters are read from `.moai/config/sections/design.yaml`. Do not hardcode thresholds."

**REQ-ABSORB-008 (Ubiquitous)**
The system SHALL provide three additional workflow skills to support the design pipeline:
- `moai-workflow-design-context` (auto-load `.moai/design/` briefs into context)
- `moai-workflow-design-import` (parse Claude Design handoff bundle)
- `moai-workflow-pencil-integration` (Pencil MCP batch operations, conditional)
- 증거: 세 디렉터리·SKILL.md 파일 존재 (각 user-invocable=false, category="workflow", updated="2026-04-20").

**REQ-ABSORB-009 (Ubiquitous)**
The system SHALL preserve the `copywriter` and `designer` agent definitions (`.claude/agents/agency/{copywriter,designer}.md`) as fallback path B references during the deprecation window. They are NOT removed in M5.
- 증거: CLAUDE.md §4 — "Agency Agents (2) — copywriter and designer retained as fallback path B skills" + "copywriter (absorbed into moai-domain-copywriting skill), designer (absorbed into moai-domain-brand-design skill)".

### Category 3: Constitution Migration (REQ-CONST)

**REQ-CONST-001 (Ubiquitous)**
The system SHALL relocate `.claude/rules/agency/constitution.md` (v3.2.0) to `.claude/rules/moai/design/constitution.md` (v3.3.0) preserving FROZEN zone, EVOLVABLE zone, and Safety Architecture (Section 5) verbatim.
- 증거: 새 파일 line 6 — "Relocated from `.claude/rules/agency/constitution.md` (v3.2.0) ... No content changes. FROZEN zone and EVOLVABLE zone definitions are preserved verbatim." 및 footer "REQ coverage: REQ-CONST-001, REQ-CONST-002, REQ-CONST-003, REQ-CONST-004".

**REQ-CONST-002 (Ubiquitous)**
The system SHALL extend Section 3 of the design constitution into a tripartite structure: 3.1 Brand Context (constitutional parent), 3.2 Design Brief (execution scope), 3.3 Relationship (conflict resolution). FROZEN zone SHALL be extended to cover each subsection individually.
- 증거: `.claude/rules/moai/design/constitution.md` line 5 — "Section 3 expanded to tripartite structure (3.1/3.2/3.3). Version 3.2.0 → 3.3.0 (v3.3.0). FROZEN zone extended to cover each subsection individually."

**REQ-CONST-003 (Ubiquitous)**
The system SHALL replace the original `.claude/rules/agency/constitution.md` with a redirect stub that points to the new location and notes the relocation date and SPEC reference.
- 증거: `.claude/rules/agency/constitution.md` 21줄 stub. line 5 — "This file has been relocated to `.claude/rules/moai/design/constitution.md` as part of SPEC-AGENCY-ABSORB-001 M1 (2026-04-20)." line 13 — "This stub is retained for backward compatibility ... It will be removed when the /agency command is fully removed (2 minor version cycles after this release, per REQ-DEPRECATE-003)."

**REQ-CONST-004 (Ubiquitous)**
The system SHALL declare brand context (`.moai/project/brand/`) as a constitutional constraint that flows through every design pipeline phase via `[HARD]` rules in Section 3.1, binding manager-spec, moai-domain-copywriting, moai-domain-brand-design, expert-frontend, and evaluator-active.
- 증거: 신 헌법 §3.1 — 5개 [HARD] 룰 존재 (manager-spec MUST load, copywriting MUST adhere to brand voice, brand-design MUST use palette, expert-frontend MUST implement design tokens, evaluator-active MUST score brand consistency).

### Category 4: Pipeline & Workflow (REQ-ROUTE / REQ-FALLBACK / REQ-BRIEF / REQ-DETECT / REQ-PENCIL)

**REQ-ROUTE-001 (State-Driven)**
WHILE `.moai/project/brand/` is missing any of the three brand files (brand-voice.md, visual-identity.md, target-audience.md) OR the present files contain `_TBD_` markers, THE `/moai design` workflow SHALL skip route selection and propose the brand interview instead.
- 증거: `.claude/skills/moai/workflows/design.md` Phase 0 Check 2.

**REQ-ROUTE-002 (Event-Driven)**
WHEN brand context preconditions are satisfied, THE `/moai design` workflow SHALL present two paths via AskUserQuestion: Option 1 (Recommended) Claude Design import, Option 2 code-based brand design.
- 증거: design.md Phase 1 — AskUserQuestion 옵션 정의.

**REQ-ROUTE-003 (Ubiquitous)**
The system SHALL place Claude Design import as the recommended option by default, marked "(Recommended)", as it carries the lowest implementation cost when a Pro/Max/Team/Enterprise subscription is available.
- 증거: design.md Phase 1 — "Option 1 (Recommended): Claude Design import".

**REQ-ROUTE-004 (Event-Driven)**
WHEN the user selects path A, THE system SHALL guide the user to claude.ai/design, collect a local handoff bundle file path, validate the path ends in `.zip` or `.html`, and invoke `moai-workflow-design-import` with the bundle.
- 증거: design.md Phase A Steps A1-A3.

**REQ-ROUTE-005 (Event-Driven)**
WHEN the user selects path B, THE system SHALL load `moai-domain-copywriting`, `moai-domain-brand-design`, `moai-workflow-gan-loop`, read the three brand files, and proceed through Phase B2.5 (design context loading), conditional Phase B2.6 (Pencil), Phase B3 (BRIEF generation by manager-spec), and Phase B4 (expert-frontend delegation).
- 증거: design.md Phase B Steps B1-B5.

**REQ-ROUTE-006 (State-Driven)**
WHILE `subscription.tier: "pro-or-below"` is declared in `.moai/config/sections/user.yaml` OR the user explicitly states they do not have Claude Design access, THE system SHALL reverse the option order so that the code-based path becomes Option 1 (Recommended), without disabling the Claude Design option.
- 증거: design.md Phase 1 — "Subscription override (REQ-ROUTE-006)".

**REQ-ROUTE-007 (Unwanted Behavior)**
IF the user does not select an option after AskUserQuestion is presented, THEN the system SHALL re-present the question up to 3 times. After 3 failed attempts, THE system SHALL output "Selection not confirmed. Resume with `/moai design` when ready." and stop without closing the session.
- 증거: design.md Phase 1 — "No-response handling (REQ-ROUTE-007)".

**REQ-ROUTE-008 (Event-Driven)**
WHEN either path A or path B produces design artifacts, THE system SHALL invoke `moai-workflow-gan-loop` (Phase C) which executes Builder-Evaluator iterations up to `gan_loop.max_iterations` (5) until `gan_loop.pass_threshold` (0.75) is met.
- 증거: design.md Phase C Step C1.

**REQ-FALLBACK-001 (Event-Driven)**
WHEN `moai-workflow-design-import` fails (invalid bundle, unsupported format, parsing error), THE system SHALL surface the structured error code and offer path B as fallback via AskUserQuestion.
- 증거: design.md Phase A Step A5.

**REQ-FALLBACK-002 (State-Driven)**
WHILE Phase B is active and a structured Pencil error code (`PENCIL_MCP_UNAVAILABLE`, `PENCIL_CONNECTION_FAILED`, `PENCIL_PLAN_SYNTAX_ERROR`, `PENCIL_BATCH_FAILED`) is returned by `moai-workflow-pencil-integration`, THE system SHALL log the error and continue to Phase B3 instead of returning to Phase 1 route selection.
- 증거: design.md Phase B2.6 — "Do NOT abort the overall `/moai design` workflow. Continue to Phase B3 immediately."

**REQ-FALLBACK-003 (Event-Driven)**
WHEN `moai-domain-brand-design` is invoked but `.moai/project/brand/visual-identity.md` is missing or contains `_TBD_` markers, THE skill SHALL stop and request brand interview completion before generating design output.
- 증거: `.claude/skills/moai-domain-brand-design/SKILL.md` Quick Reference Entry Conditions + footer "REQ coverage: ... REQ-FALLBACK-003".

**REQ-BRIEF-001 (Ubiquitous)**
The BRIEF document generated by manager-spec for design tasks SHALL include three required sections: `## Goal`, `## Audience`, `## Brand`. If any section is empty, manager-spec SHALL return `BRIEF_SECTION_INCOMPLETE`.
- 증거: design.md "BRIEF Section Requirements (REQ-BRIEF-001)".

**REQ-BRIEF-002 (State-Driven)**
WHILE the Brand section is empty in a BRIEF generation request, THE system SHALL auto-inject key content from the three brand files with source citation lines (`> source: .moai/project/brand/<filename>`).
- 증거: design.md Phase B3 — "If Brand section is empty: auto-inject key content from the three brand files with source citation".

**REQ-BRIEF-003 (Unwanted Behavior)**
IF the brand files are missing when manager-spec attempts BRIEF generation, THEN manager-spec SHALL halt with `BRIEF_SECTION_INCOMPLETE` and request brand interview.
- 증거: design.md Phase B3.

**REQ-DETECT-003 (State-Driven)**
WHILE `.agency/` directory exists AND `.moai/project/brand/` does not exist, THE `/moai design` workflow SHALL output a warning before route selection: "agency data detected — run `moai migrate agency` to migrate your brand context first." and continue to route selection without blocking.
- 증거: design.md Phase 0 Check 1.

### Category 5: Configuration & Brief Directory (REQ-CONFIG / REQ-DESIGN-DOCS)

**REQ-CONFIG-001 (Ubiquitous)**
The system SHALL provide a new configuration section `.moai/config/sections/design.yaml` containing all design pipeline parameters: `gan_loop`, `evolution`, `adaptation`, `brand_context`, `design_docs`, `claude_design`, `figma`, `default_framework`.
- 증거: 파일 57줄, 모든 키 존재 확인 완료.

**REQ-CONFIG-002 (Ubiquitous)**
The system SHALL define `gan_loop.pass_threshold` default at 0.75, `max_iterations` at 5, `escalation_after` at 3, `improvement_threshold` at 0.05; these values SHALL NOT be hardcoded inside skill files.
- 증거: design.yaml lines 43-47 + GAN Loop SKILL.md line 38 — "Do not hardcode thresholds".

**REQ-CONFIG-003 (Ubiquitous)**
The Sprint Contract Protocol SHALL be required when harness level is `thorough` (`required_harness_levels: [thorough]`) and optional when level is `standard` (`optional_harness_levels: [standard]`), with artifacts stored in `.moai/sprints/`.
- 증거: design.yaml lines 48-55.

**REQ-DESIGN-DOCS-001 (Ubiquitous)**
The system SHALL standardize `.moai/design/` as the design brief directory with auto-load priority `spec > system > research > pencil-plan` and a default token budget of 20000 (from `design.yaml design_docs.token_budget`).
- 증거: design.yaml lines 19-27 + `.moai/design/README.md` "Auto-load priority" 라인.

**REQ-DESIGN-DOCS-002 (Ubiquitous)**
The system SHALL reserve the following filenames in `.moai/design/` as auto-generated artifacts: `tokens.json`, `components.json`, `import-warnings.json`, `brief/BRIEF-*.md`. Human-authored files SHALL NOT collide with these names.
- 증거: README.md Reserved Filenames 섹션 + 헌법 §3.2 reserved file paths.

**REQ-DESIGN-DOCS-003 (State-Driven)**
WHILE invoked from `/moai design` Phase B2.5 and `design_docs.auto_load_on_design_command` is true, `moai-workflow-design-context` SHALL read candidate files in parallel (single batched tool-call set), filter out `_TBD_`-only files, and apply token budget enforcement using priority order with reverse truncation (drop pencil-plan first, then research, then system; always preserve spec).
- 증거: `moai-workflow-design-context/SKILL.md` Steps 4-6.

### Category 6: Deprecation Policy (REQ-DEPRECATE)

**REQ-DEPRECATE-001 (Ubiquitous)**
Every deprecated `/agency` command file SHALL contain a `> DEPRECATED:` notice in its body referencing SPEC-AGENCY-ABSORB-001.
- 증거: 8개 파일 모두 grep 확인 완료 (검색 결과: "DEPRECATED... SPEC-AGENCY-ABSORB-001").

**REQ-DEPRECATE-002 (Ubiquitous)**
The CLAUDE.md `/agency` reference SHALL be marked DEPRECATED with redirect notice and migration guide pointer.
- 증거: CLAUDE.md §3 — "### /agency (DEPRECATED — use /moai design)" 섹션 + "Migration guide: see .claude/commands/agency/agency.md".

**REQ-DEPRECATE-003 (Ubiquitous)**
The `/agency` command and the `.claude/rules/agency/constitution.md` redirect stub SHALL be removed in the next minor version (deprecation window: 2 minor version cycles after the SPEC release date 2026-04-20).
- 증거: agency.md line 24 — "This wrapper will be removed in the next minor version per SPEC-AGENCY-ABSORB-001 REQ-DEPRECATE-003." + agency/constitution.md line 14-15 — "It will be removed when the /agency command is fully removed (2 minor version cycles after this release, per REQ-DEPRECATE-003)."

**REQ-DEPRECATE-004 (Unwanted Behavior)**
IF a user invokes a deprecated `/agency` command without subcommand or with an unsupported subcommand (`learn`, `evolve`), THEN the system SHALL NOT silently fail; it SHALL output the migration table or `AGENCY_SUBCOMMAND_UNSUPPORTED` error code with the migration guide URL.
- 증거: agency.md migration table 표 + learn.md/evolve.md ERROR 본문.

### Category 7: Documentation Sync (REQ-DOC)

**REQ-DOC-001 (Ubiquitous)**
CLAUDE.md §4 (Agent Catalog) SHALL be updated to remove the agency planner/builder/evaluator/learner agents from the active catalog and to note that copywriter/designer are absorbed into moai-domain-* skills as path B fallback references.
- 증거: CLAUDE.md line 128-131 — "Agency Agents (2) — copywriter and designer retained as fallback path B skills" + "planner, builder, evaluator, learner removed in SPEC-AGENCY-ABSORB-001 M5".
- **확인 필요**: 실제 agent 파일들 (`.claude/agents/agency/{planner,builder,evaluator,learner}.md`)은 여전히 디스크상에 존재 (mtime 2026-04-21). CLAUDE.md의 "removed" 표현은 active catalog 등록 측면을 의미하며, 파일 시스템 차원의 즉시 삭제는 REQ-DEPRECATE-003 deprecation window까지 보류된 것으로 해석한다. SPEC 작성자는 이 모순을 후속 sync 작업의 정리 대상으로 권고.

**REQ-DOC-002 (Ubiquitous)**
CLAUDE.md §9 SHALL document the new design system configuration locations: `.moai/config/sections/design.yaml`, `.moai/project/brand/`, `.claude/rules/moai/design/constitution.md`.
- 증거: CLAUDE.md "Design System Configuration (absorbed from agency, SPEC-AGENCY-ABSORB-001)" 섹션 (line 408 부근).

**REQ-DOC-003 (Ubiquitous)**
CLAUDE.md SHALL note that legacy `.agency/` directories are archived via the `moai migrate agency` command.
- 증거: CLAUDE.md line 417 — "Legacy .agency/ directories are archived via `moai migrate agency` command."

---

## Acceptance Criteria Summary

상세 검증 절차는 `acceptance.md`를 참조한다. 핵심 글로벌 수용 기준:

- AC-GLOBAL-1: 8개 `/agency` 명령 모두 redirect stub로 변환되어 호출 시 `Skill("moai")` 또는 `Skill("moai-workflow-research")`로 라우팅된다.
- AC-GLOBAL-2: `/moai design` 호출 시 Phase 0 → Phase 1 AskUserQuestion 경로 분기가 작동하고, 두 옵션이 description과 함께 제시된다.
- AC-GLOBAL-3: `.moai/design/` 브리프 자동 로딩이 design.yaml `design_docs.token_budget` 제약(기본 20000)을 준수하고 우선순위 truncation이 정확히 적용된다.
- AC-GLOBAL-4: `.claude/rules/moai/design/constitution.md` FROZEN zone (Section 2)이 명시적 보호 대상으로 유지되고 Learner의 자동 수정이 차단된다.

---

## Non-Goals (Confirmed)

이전 Scope 섹션의 Out of Scope를 재확인한다:

- DB 메타 관리·`moai-domain-db-docs`·`db.yaml`·`.moai/project/db/`는 본 SPEC의 acceptance 대상이 아니다.
- GOOSE 브랜드 페르소나 정립(d02f512)은 선행 트랙이며, 본 SPEC은 그 결과물(`.moai/project/brand/` 표준 구조)을 입력으로 사용할 뿐이다.
- agency-migration CLI 자체의 코드 구현은 별도 트랙.
- 본 SPEC은 신규 코드 작성을 트리거하지 않는다 (characterization 패턴).

---

## Open Items / 확인 필요

회고 작성 과정에서 발견된 모순 또는 후속 정리 권고 사항:

1. **agency agent 파일 잔존**: CLAUDE.md §4는 "planner, builder, evaluator, learner removed"라 명시하나, `.claude/agents/agency/{planner,builder,evaluator,learner}.md` 6개 파일이 디스크상에 여전히 존재한다. 본 SPEC에서는 "active catalog 등록 해제 = removed"로 해석하지만, 다음 minor version에서 REQ-DEPRECATE-003 cleanup과 함께 물리적 삭제 여부를 결정해야 한다.
2. **brand 파일 _TBD_ 미해소**: `.moai/project/brand/{brand-voice,visual-identity,target-audience}.md`에 _TBD_ 마커가 다수 잔존 (예: brand-voice.md line 12 `tone: _TBD_`). 본 SPEC의 책임은 아니나, `/moai design` 첫 실행 시 REQ-ROUTE-001에 따라 brand interview가 강제 트리거됨을 사용자에게 안내할 필요가 있다.
3. **구버전 design 스킬 잔존**: `.claude/skills/agency-design-system/`이 여전히 존재 (mtime 2026-04-10). 본 SPEC은 새 스킬(`moai-domain-brand-design`)이 흡수된 사실만 기록하며, 구 디렉터리 정리는 후속 sync 작업으로 권고.
4. **/agency learn·evolve 경로**: 두 명령이 `moai-workflow-research`로 라우팅되는데, 이는 직접 등가물이 아닌 "관련성 있는 스킬"이라는 절충이다. 향후 사용자 피드백에 따라 별도 SPEC으로 정식 매핑을 정의할지 검토.

---

REQ coverage: REQ-ABSORB-001~009, REQ-CONST-001~004, REQ-ROUTE-001~008, REQ-FALLBACK-001~003, REQ-BRIEF-001~003, REQ-DETECT-003, REQ-CONFIG-001~003, REQ-DESIGN-DOCS-001~003, REQ-DEPRECATE-001~004, REQ-DOC-001~003

총 EARS 요구사항 수: **41건** (ABSORB 9 + CONST 4 + ROUTE 8 + FALLBACK 3 + BRIEF 3 + DETECT 1 + CONFIG 3 + DESIGN-DOCS 3 + DEPRECATE 4 + DOC 3 — 외부 SPEC에서 정의된 REQ-PENCIL-001~016, REQ-SKILL-* 시리즈는 본 SPEC이 참조만 하며 재정의하지 않음)
