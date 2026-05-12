---
id: SPEC-MINK-DISTANCING-STATEMENT-001
version: "0.1.0"
status: draft
created_at: 2026-05-12
updated_at: 2026-05-12
author: manager-spec
priority: Medium
labels: [brand-strategy, vision, meta, brand-distancing, product-doc]
issue_number: null
phase: meta
size: 중(M)
lifecycle: spec-anchored
parent: SPEC-MINK-PRODUCT-V7-001
related_specs: [SPEC-MINK-PRODUCT-V7-001, SPEC-MINK-BRAND-RENAME-001]
---

# SPEC-MINK-DISTANCING-STATEMENT-001 — `.moai/project/distancing.md` (5-product brand distancing statement)

## HISTORY

| Version | Date | Author | Change |
|---------|------|--------|--------|
| 0.1.0 | 2026-05-12 | manager-spec | Initial draft. PRODUCT-V7-001 §6 detail document. parent=SPEC-MINK-PRODUCT-V7-001 (vision anchor) / sibling=SPEC-MINK-BRAND-RENAME-001 (cross-cutting rename, body disjoint). 5-product distancing statement (block/goose, Hermes, Replika, Routinery, OpenClaw) — vision-level 1 단락 starting point 를 detail 4-단락 표준 (Identity / MINK is NOT / How MINK differs / 선택 Note on coexistence) 으로 확장. 단일 markdown 산출물 `.moai/project/distancing.md` 신설. 15 EARS 요구사항 + 11 binary-verifiable AC + 7 명시 제외 항목. 코드 변경 0건. plan-auditor 불필요 (markdown-only, low-risk vision-supporting doc). |

---

## 1. Overview

### 1.1 Scope Clarity

본 SPEC 은 **vision-supporting document SPEC** 이다. 단일 모듈 / 기능 / 코드 변경을 정의하지 않으며, 프로젝트의 brand distancing statement document (`.moai/project/distancing.md`) 본문을 v1.0.0 으로 신설한다. 코드 변경 0건. 산출물은 markdown 문서 1개:

1. `.moai/project/distancing.md` (신설, ~600-900 lines, v1.0.0)

본 SPEC 의 plan artifacts (이 spec.md + research.md) 는 별도 산출물.

본 SPEC 은 parent SPEC `SPEC-MINK-PRODUCT-V7-001 §6 Distancing` 의 5-product starting point 를 detail statement 로 확장한다 — vision-level 의 1 단락 정의를 detail-level 의 4-단락 표준 (Identity / MINK is NOT / How MINK differs / 선택 Note on coexistence) 으로 확장한다. 본 SPEC 은 product vision 을 새로 정의하지 않으며 PRODUCT-V7-001 §6 의 5 product set 을 그대로 채택한다.

### 1.2 Goal

`.moai/project/distancing.md` 를 v1.0.0 으로 신설하여, MINK 가 다음 5 product 와 명시적으로 어떻게 다른지 (categorical 차이 + factual basis + anti-FUD 약속) brand identity statement 로 정립한다:

1. block/goose (multi-tenant agent platform)
2. Hermes (AI girlfriend / emotional companion 카테고리)
3. Replika (long-term emotional AI)
4. Routinery (habit tracker app)
5. OpenClaw (agent framework, Together AI)

본 문서는 다음 사용 시나리오의 single source of truth 역할:

- README brand section 의 distancing 인용
- public Q&A / 블로그 / 외부 brand 충돌 대응
- 향후 SPEC 작성자의 anti-goal 점검 canonical reference

본 SPEC 은 위 5 product 중 어느 하나의 약점을 부각하지 않는다 (anti-FUD). MINK 의 niche 가 어디서 그들과 갈라지는지 사실만 기술한다.

### 1.3 Non-Goals

본 SPEC 의 직접 산출물 외 다음은 **본 SPEC scope 외**:

- `.moai/project/distancing.md` 외의 sibling `.moai/project/*.md` 파일 정정 (branding.md / ecosystem.md / migration.md / brand/* 등 — 별도 후속 audit / SPEC)
- PRODUCT-V7-001 의 §6 Distancing 본문 변경 — vision-level starting point 는 변경 안 함, 본 SPEC 은 detail 확장만
- BRAND-RENAME-001 의 brand-lint regex pattern / exemption rule 추가 — 본 SPEC 의 distancing.md 가 brand-lint 위반 flag 발생 시 별도 후속 SPEC 으로 exemption 추가 (BRAND-RENAME-001 §11 OUT-scope 와 일관)
- 코드 변경 (Go module path / binary / proto / 식별자 — sibling SPEC-MINK-BRAND-RENAME-001 담당)
- 5 product 의 logo / 시각 자산 / marketing material 인용
- distancing.md 영문 번역본 작성 (i18n 후순위)
- 6번째 이상의 product 추가 (PRODUCT-V7-001 §6 의 5 product subset 위반 금지, superset 도 본 SPEC scope 외)
- MINK 의 visual identity / logo / brand 시각자산 정의 (별도 brand interview 후속)

---

## 2. Background

### 2.1 PRODUCT-V7-001 §6 의 5 product starting point

parent SPEC `SPEC-MINK-PRODUCT-V7-001` 의 §6 Distancing 섹션은 5 product 와의 카테고리 차이를 **단락 1개씩** 정의한다 (vision-document 의 한 sub-section, ~5-8 sentences per product). 이는 vision-level brand identity 의 starting point.

PRODUCT-V7-001 §6 의 본문 자체는 vision-document 의 일부이며 detail brand identity statement 까지 담기에는 분량 / 문맥이 적절하지 않다. 본 SPEC 은 그 5 단락을 별도 문서 `.moai/project/distancing.md` 로 확장하여 product detail statement 의 single source of truth 로 정립한다.

### 2.2 detail SPEC 의 의미

vision-level 1 단락 vs detail-level 4 단락의 차이:

| layer | 분량 / product | 정보 layer | 사용처 |
|---|---|---|---|
| vision (PRODUCT-V7 §6) | ~5-8 sentences | 카테고리 차이만 | vision-document sub-section |
| detail (distancing.md) | ~20-40 sentences (3-5 단락) | Identity (사실) + MINK is NOT (anti-goal) + How MINK differs (factual differentiator) + Note on coexistence (선택) | README / public Q&A / brand 충돌 reference |

이 detail layer 는 다음 사용 시나리오를 위해 필요:

1. **외부 brand 충돌 대응**: block 사 / luka.ai 등이 brand 인접성을 표시할 경우 (가능성 매우 낮으나) 본 프로젝트의 명시적 분리 statement 가 reference 자료
2. **public Q&A**: "MINK 가 Replika 같은 emotional AI 인가?" 같은 질문에 인용 가능한 단일 statement
3. **anti-goal canonical reference**: 향후 SPEC 작성자가 새 기능 추가 시 "이 기능이 Replika / Routinery 영역으로 흘러가지 않는가?" 점검의 canonical reference

### 2.3 사용자 IDEA-002 결정의 §7 distancing 4종

사용자 IDEA-002 brain decision (2026-05-12, brain dir 별도 보존) 의 §7 distancing 결정은 다음 4종 (실제로는 5개) 을 distancing 대상으로 명시:

- block/goose (multi-tenant agent platform)
- Hermes (AI girlfriend / 친구 LLM-bot)
- Replika (long-term emotional AI)
- Routinery (habit tracker)
- OpenClaw (agent framework)

본 SPEC 은 이 5 product 의 detail statement 작성을 단일 mission 으로 한다.

### 2.4 brand-voice (self-derived)

`.moai/project/brand/brand-voice.md` 가 현재 `_TBD_` stub 인 점을 감안, 본 SPEC distancing.md 의 brand-voice 는 다음 sources 에서 self-derive:

- CLAUDE.local.md §2.5 (code 영어 / 사용자 문서 한국어 1차 정책)
- PRODUCT-V7-001 §1.1 §3.4 §4 anti-goal 명시 voice
- CLAUDE.md §10 Web Search Protocol (anti-hallucination)

핵심:

- 1인 dev → 1인 dev tone, 한국어 1차
- factual + sourced (각 product 의 운영 주체 / 라이선스 / 1차 사용 시나리오는 verified 정보)
- anti-AI-slop: "혁신적", "차세대", "최고", "유일", "혁명적" 등 마케팅 용어 금지
- anti-FUD: 경쟁 product 의 약점 부각 / 비교우월 표현 금지

자세히는 research.md §3 brand-voice 매칭 분석 참조.

---

## 3. Scope

### 3.1 IN Scope

본 SPEC PR 범위 안에서 다음을 산출한다:

1. **`.moai/specs/SPEC-MINK-DISTANCING-STATEMENT-001/research.md` 신설** (이 SPEC plan artifact, ~500 lines)
2. **`.moai/specs/SPEC-MINK-DISTANCING-STATEMENT-001/spec.md` 신설** (이 SPEC, v0.1.0, ~500 lines)
3. **`.moai/project/distancing.md` 신설** (v1.0.0, ~600-900 lines):
   - frontmatter: version 1.0.0, status published, classification BRAND_DISTANCING, parent SPEC-MINK-PRODUCT-V7-001, spec SPEC-MINK-DISTANCING-STATEMENT-001
   - HISTORY 1 row (v1.0.0 initial)
   - Preamble + 5 product detail sections + Out of comparison scope + Closing statement + References
   - 각 product section 은 4-단락 표준 (Identity / MINK is NOT / How MINK differs / 선택 Note on coexistence)

### 3.2 OUT Scope (반드시 보존 — sibling 파일 / 코드)

[HARD] 다음 항목은 변경 금지:

1. `.moai/project/distancing.md` 외의 sibling `.moai/project/*.md` 모든 파일 (product.md / tech.md / structure.md / branding.md / ecosystem.md / migration.md / adaptation.md / learning-engine.md / token-economy.md / brand/*)
2. `.moai/project/codemaps/*.md` (3 파일)
3. `.moai/project/research/*.md` (4 파일)
4. 코드 (`*.go`, `*.proto`, `cmd/`, `internal/` — sibling SPEC-MINK-BRAND-RENAME-001 담당)
5. 다른 SPEC 본문 (`.moai/specs/SPEC-*/spec.md`) — 본 SPEC plan artifacts (`.moai/specs/SPEC-MINK-DISTANCING-STATEMENT-001/*`) 만 신설
6. CHANGELOG / README / git history / `.claude/agent-memory/` (immutable archive)
7. PRODUCT-V7-001 §6 Distancing 본문 — vision-level starting point 는 보존, 본 SPEC 은 detail 확장만

---

## 4. EARS Requirements

본 SPEC 은 15 EARS 요구사항을 정의한다. 각 요구사항은 spec-anchored 이며 §5 AC 와 1:1 또는 N:1 매핑된다.

### 4.1 Ubiquitous (보편 — 항상 성립)

- **REQ-MINK-DST-FILE-001 [Ubiquitous]** The `.moai/project/distancing.md` v1.0.0 본문 **shall** include YAML frontmatter with fields: `version: 1.0.0`, `status: published`, `created_at: 2026-05-12`, `classification: BRAND_DISTANCING`, `parent: SPEC-MINK-PRODUCT-V7-001`, `spec: SPEC-MINK-DISTANCING-STATEMENT-001`.

- **REQ-MINK-DST-FILE-002 [Ubiquitous]** The `.moai/project/distancing.md` v1.0.0 본문 **shall** start its `## HISTORY` table with exactly one data row (`| 1.0.0 | 2026-05-12 | manager-spec | Initial draft. PRODUCT-V7-001 §6 detail document. |` 또는 동등 문언).

- **REQ-MINK-DST-STRUCT-001 [Ubiquitous]** The `.moai/project/distancing.md` v1.0.0 본문 **shall** include the following sections in order: (1) Preamble, (2) vs block/goose, (3) vs Hermes, (4) vs Replika, (5) vs Routinery, (6) vs OpenClaw, (7) Out of comparison scope, (8) Closing statement, (9) References.

- **REQ-MINK-DST-STRUCT-002 [Ubiquitous]** Each of the 5 product sections (block/goose, Hermes, Replika, Routinery, OpenClaw) **shall** follow the 4-paragraph standard structure: (a) Identity, (b) MINK is NOT, (c) How MINK differs, (d) optional Note on coexistence.

- **REQ-MINK-DST-BLOCK-001 [Ubiquitous]** The `## vs block/goose` section **shall** include the following factual identity: block (Stripe spinoff), Apache-2.0 license, github.com/block/goose repo, multi-tenant agent platform category.

- **REQ-MINK-DST-HERMES-001 [Ubiquitous]** The `## vs Hermes` section **shall** describe Hermes as a Korean-market "AI girlfriend / friend LLM-bot" category (multiple commercial vendors, not a single product) and **shall not** name or attack any specific commercial vendor instance.

- **REQ-MINK-DST-REPLIKA-001 [Ubiquitous]** The `## vs Replika` section **shall** include the following factual identity: Luka, Inc. (luka.ai) 운영, freemium SaaS, long-term emotional AI companion / friendship simulation 카테고리.

- **REQ-MINK-DST-ROUTINERY-001 [Ubiquitous]** The `## vs Routinery` section **shall** include the following factual identity: routinery.app, 한국제 startup, habit tracker / daily routine mobile app, gamification (streak) 기반.

- **REQ-MINK-DST-OPENCLAW-001 [Ubiquitous]** The `## vs OpenClaw` section **shall** include the following factual identity: Together AI (together.ai) 운영, Apache-2.0 license, agent framework / tool-use orchestration toolkit, developer 가 agent app 을 build 하는 도구.

- **REQ-MINK-DST-PATTERN-001 [Ubiquitous]** Each of the 5 product sections **shall** include the phrase "MINK is NOT" (or 한국어 등가 "MINK 는 ... 가 아니다") at least once and the phrase "MINK differs" (or 한국어 등가 "MINK 는 ... 차이가 있다" / "MINK 는 ... 다르다") at least once — these mark the anti-goal 단락 + factual differentiator 단락.

- **REQ-MINK-DST-VOICE-001 [Ubiquitous]** The `.moai/project/distancing.md` v1.0.0 본문 **shall** follow the self-derived brand-voice (research.md §3.2): 1인 dev → 1인 dev tone, 한국어 1차, factual + sourced, anti-AI-slop, anti-FUD.

- **REQ-MINK-DST-ANTIGOAL-001 [Ubiquitous]** The `.moai/project/distancing.md` v1.0.0 본문 **shall not** include comparative-superiority vocabulary (한국어: "보다 우수", "보다 낫", "최고", "유일"; English: "superior", "inferior", "outdated", "obsolete", "best", "only"), market-share / dominance claims, or AI-slop marketing vocabulary (한국어: "혁신적", "차세대"; English: "innovative", "next-generation", "disruptive", "game-changing", "revolutionary").

- **REQ-MINK-DST-REFERENCES-001 [Ubiquitous]** The `## References` section of `.moai/project/distancing.md` **shall** include: (a) parent SPEC SPEC-MINK-PRODUCT-V7-001 §6, (b) sibling SPEC SPEC-MINK-BRAND-RENAME-001, (c) 5 product 공식 URL (github.com/block/goose, replika.ai, routinery.app, together.ai; Hermes 는 카테고리이므로 single URL 부재 명시).

### 4.2 Event-Driven (트리거 발생 시)

- **REQ-MINK-DST-PR-001 [Event-Driven]** **When** this SPEC's run-phase PR is created, the PR diff **shall** include exactly the following changed files: (a) `.moai/project/distancing.md` (new), (b) optionally this SPEC's plan artifacts under `.moai/specs/SPEC-MINK-DISTANCING-STATEMENT-001/` if not already merged in plan PR. **No** other file in `.moai/project/`, `.moai/specs/SPEC-*/` (except this SPEC's own dir), `.go`, `.proto`, `.yaml`, `.json` **shall** be modified.

### 4.3 Unwanted (금지 행동)

- **REQ-MINK-DST-SCOPE-001 [Unwanted]** **If** this SPEC's run-phase PR diff includes modification to any `.moai/project/*.md` file other than `distancing.md`, **then** the PR **shall** be rejected as scope violation.

- **REQ-MINK-DST-SCOPE-002 [Unwanted]** **If** this SPEC's run-phase PR diff includes modification to any non-markdown file (`.go`, `.proto`, `.yaml`, `.json`, etc. outside `.moai/specs/SPEC-MINK-DISTANCING-STATEMENT-001/`), **then** the PR **shall** be rejected as scope violation.

---

## 5. Acceptance Criteria

각 AC 는 Given / When / Then 형식. 모든 AC 는 binary 검증 가능 (file existence, grep match count, single shell command). 11개 AC.

### AC-001 — distancing.md 신설 + frontmatter + HISTORY

**Given** 본 SPEC run-phase PR squash merge 후 main HEAD state 에서
**When** `head -30 .moai/project/distancing.md` 를 실행하면
**Then** 출력에 다음이 모두 포함된다:
- 첫 줄 `---` (frontmatter 시작)
- `version: 1.0.0` (또는 `version: "1.0.0"`) 라인 존재
- `status: published` 라인 존재
- `classification: BRAND_DISTANCING` 라인 존재
- `parent: SPEC-MINK-PRODUCT-V7-001` 라인 존재
- `spec: SPEC-MINK-DISTANCING-STATEMENT-001` 라인 존재
- `## HISTORY` 섹션 header 존재
- HISTORY 표에 `| 1.0.0 | 2026-05-12 |` 행 1개 존재

검증 명령:
```bash
test -f .moai/project/distancing.md && echo OK
head -30 .moai/project/distancing.md | grep -c '^version: \(1\.0\.0\|"1\.0\.0"\)'   # ≥ 1
head -30 .moai/project/distancing.md | grep -c '^classification: BRAND_DISTANCING'    # ≥ 1
head -30 .moai/project/distancing.md | grep -c '^parent: SPEC-MINK-PRODUCT-V7-001'    # ≥ 1
head -30 .moai/project/distancing.md | grep -c '^spec: SPEC-MINK-DISTANCING-STATEMENT-001'  # ≥ 1
grep -c '^| 1\.0\.0 | 2026-05-12 |' .moai/project/distancing.md   # ≥ 1
```

REQ 매핑: REQ-MINK-DST-FILE-001, REQ-MINK-DST-FILE-002

### AC-002 — vs block/goose section grep ≥ 3

**Given** 본 SPEC run-phase PR squash merge 후
**When** `.moai/project/distancing.md` 의 본문에서 `block/goose` 또는 `block goose` 토큰을 grep 하면
**Then** 매치 라인 수 ≥ 3 (vs PRODUCT-V7 §6 의 ≥ 1).

검증 명령:
```bash
grep -ciE 'block/goose|block goose' .moai/project/distancing.md   # ≥ 3
```

추가 검증 (Identity 단락 사실 확인):
```bash
grep -ciE 'Apache-2\.0|apache 2\.0' .moai/project/distancing.md           # ≥ 1 (block/goose Identity)
grep -ciE 'github\.com/block/goose|block, Inc|Stripe' .moai/project/distancing.md  # ≥ 1
```

REQ 매핑: REQ-MINK-DST-BLOCK-001, REQ-MINK-DST-STRUCT-002

### AC-003 — vs Hermes section grep ≥ 3

**Given** 본 SPEC run-phase PR squash merge 후
**When** `.moai/project/distancing.md` 의 본문에서 `Hermes` 토큰을 grep 하면
**Then** 매치 라인 수 ≥ 3.

검증 명령:
```bash
grep -cw 'Hermes' .moai/project/distancing.md   # ≥ 3
```

추가 검증 (Hermes 카테고리 정의 + anti-FUD: 특정 vendor 비난 부재):
```bash
grep -ciE 'AI 여친|AI 친구|emotional companion|emotional engagement' .moai/project/distancing.md   # ≥ 1
```

REQ 매핑: REQ-MINK-DST-HERMES-001, REQ-MINK-DST-STRUCT-002

### AC-004 — vs Replika section grep ≥ 3

**Given** 본 SPEC run-phase PR squash merge 후
**When** `.moai/project/distancing.md` 의 본문에서 `Replika` 토큰을 grep 하면
**Then** 매치 라인 수 ≥ 3.

검증 명령:
```bash
grep -cw 'Replika' .moai/project/distancing.md   # ≥ 3
```

추가 검증 (Replika Identity):
```bash
grep -ciE 'replika\.ai|luka\.ai|Luka, Inc' .moai/project/distancing.md   # ≥ 1
grep -ciE 'emotional|long-term|companion|friendship simulation' .moai/project/distancing.md   # ≥ 1
```

REQ 매핑: REQ-MINK-DST-REPLIKA-001, REQ-MINK-DST-STRUCT-002

### AC-005 — vs Routinery section grep ≥ 3

**Given** 본 SPEC run-phase PR squash merge 후
**When** `.moai/project/distancing.md` 의 본문에서 `Routinery` 토큰을 grep 하면
**Then** 매치 라인 수 ≥ 3.

검증 명령:
```bash
grep -cw 'Routinery' .moai/project/distancing.md   # ≥ 3
```

추가 검증 (Routinery Identity + gamification anti-goal):
```bash
grep -ciE 'routinery\.app|habit tracker|habit formation|streak|gamification' .moai/project/distancing.md   # ≥ 1
```

REQ 매핑: REQ-MINK-DST-ROUTINERY-001, REQ-MINK-DST-STRUCT-002

### AC-006 — vs OpenClaw section grep ≥ 3

**Given** 본 SPEC run-phase PR squash merge 후
**When** `.moai/project/distancing.md` 의 본문에서 `OpenClaw` 토큰을 grep 하면
**Then** 매치 라인 수 ≥ 3.

검증 명령:
```bash
grep -cw 'OpenClaw' .moai/project/distancing.md   # ≥ 3
```

추가 검증 (OpenClaw Identity):
```bash
grep -ciE 'together\.ai|Together AI|agent framework|tool-use orchestration' .moai/project/distancing.md   # ≥ 1
```

REQ 매핑: REQ-MINK-DST-OPENCLAW-001, REQ-MINK-DST-STRUCT-002

### AC-007 — 각 product section 의 "MINK is NOT" + "MINK differs" 패턴

**Given** 본 SPEC run-phase PR squash merge 후
**When** `.moai/project/distancing.md` 의 본문에서 anti-goal 패턴과 differentiator 패턴을 grep 하면
**Then** 다음이 모두 성립:
- "MINK is NOT" 또는 한국어 등가 "MINK 는 ... 가 아니다" 패턴 매치 ≥ 5 (5 product 각각 최소 1회)
- "MINK differs" 또는 한국어 등가 "MINK 는 ... 다르다" / "MINK 는 ... 차이" 패턴 매치 ≥ 5 (5 product 각각 최소 1회)

검증 명령:
```bash
# "MINK is NOT" 패턴 (영어 + 한국어 등가)
grep -ciE 'MINK is NOT|MINK 는 .* (이|가) 아니|MINK 는 .* 아니' .moai/project/distancing.md   # ≥ 5

# "MINK differs" 패턴 (영어 + 한국어 등가)
grep -ciE 'MINK differs|MINK 는 .* 다르|MINK 는 .* 차이' .moai/project/distancing.md   # ≥ 5
```

REQ 매핑: REQ-MINK-DST-PATTERN-001, REQ-MINK-DST-STRUCT-002

### AC-008 — anti-FUD / anti-AI-slop / 시간 추정 부재

**Given** 본 SPEC run-phase PR squash merge 후
**When** `.moai/project/distancing.md` 의 본문에서 비교우월 / 약점 부각 / AI-slop / 시간 추정 키워드를 grep 하면
**Then** 매치 0건:

검증 명령:
```bash
# 비교우월 / 약점 부각 (anti-FUD)
grep -ciE '보다 우수|보다 낫|보다 우월|superior to|inferior to|outdated|obsolete|단점|약점' .moai/project/distancing.md   # 0

# AI-slop 마케팅 용어
grep -ciE '혁신적|차세대|혁명적|next-generation|disruptive|game-chang|revolution|cutting-edge' .moai/project/distancing.md   # 0

# 시장 점유 / 우월성 주장
grep -ciE '최고|유일|시장 점유|market share|market dominance|leading|industry-leading' .moai/project/distancing.md   # 0

# 시간 추정
grep -ciE '\b(1주|2주|1개월|3개월|6개월|1년|Q[1-4]|Year [1-4]|2027|2028|다음 달|이번 주)\b' .moai/project/distancing.md   # 0
```

위 4 grep 모두 결과 0. (단, "6개월" 이 PRODUCT-V7 metric 인용 형태로 등장 시 false positive 가능 — 인용 형태는 manual review 로 허용)

REQ 매핑: REQ-MINK-DST-VOICE-001, REQ-MINK-DST-ANTIGOAL-001

### AC-009 — References section + 5 product 공식 URL

**Given** 본 SPEC run-phase PR squash merge 후
**When** `.moai/project/distancing.md` 의 References 섹션을 grep 하면
**Then** 다음이 모두 존재:
- `## References` 섹션 header
- parent SPEC 인용: `SPEC-MINK-PRODUCT-V7-001` 매치 ≥ 1
- sibling SPEC 인용: `SPEC-MINK-BRAND-RENAME-001` 매치 ≥ 1
- 5 product 공식 URL 각각 매치 ≥ 1 (또는 Hermes 의 경우 "카테고리, 단일 URL 부재" 명시)

검증 명령:
```bash
grep -c '^## References' .moai/project/distancing.md         # ≥ 1
grep -c 'SPEC-MINK-PRODUCT-V7-001' .moai/project/distancing.md   # ≥ 1
grep -c 'SPEC-MINK-BRAND-RENAME-001' .moai/project/distancing.md  # ≥ 1
grep -c 'github\.com/block/goose' .moai/project/distancing.md     # ≥ 1
grep -ciE 'replika\.ai|luka\.ai' .moai/project/distancing.md      # ≥ 1
grep -c 'routinery\.app' .moai/project/distancing.md              # ≥ 1
grep -ciE 'together\.ai|OpenClaw' .moai/project/distancing.md     # ≥ 1
```

REQ 매핑: REQ-MINK-DST-REFERENCES-001

### AC-010 — 5 product set 이 PRODUCT-V7 §6 와 동일 (subset / superset 금지)

**Given** 본 SPEC run-phase PR squash merge 후, parent SPEC PRODUCT-V7-001 의 §6 Distancing 의 5 product 가 set { block/goose, Hermes, Replika, Routinery, OpenClaw }
**When** `.moai/project/distancing.md` 에서 등장하는 product 이름 set 을 확인하면
**Then**:
- 위 5 product 가 모두 distancing.md 의 main section header (`## vs <product>` 형태) 로 등장 (5 product 모두 H2 section 보유)
- 6번째 이상의 product 가 main section header 로 등장 안 함 (단, §Out of comparison scope 섹션의 "비교 안 함" list 에서 ChatGPT / Claude / Gemini 등 단순 mention 은 허용)

검증 명령:
```bash
# 5 product main section header 존재
grep -cE '^## vs (block/goose|Hermes|Replika|Routinery|OpenClaw)' .moai/project/distancing.md   # = 5

# 그 외 main section header 가 다른 product 비교용으로 사용 안 됨 (수동 review)
grep -cE '^## vs ' .moai/project/distancing.md   # = 5 (정확히 5개)
```

REQ 매핑: REQ-MINK-DST-STRUCT-001

### AC-011 — scope violation 점검 (코드 변경 0건 + sibling 파일 무변경)

**Given** 본 SPEC run-phase PR squash merge 후
**When** PR diff 의 changed file list 를 확인하면 (`gh pr view <PR-N> --json files`)
**Then**:
- changed files 가 다음 prefix 중 하나로만 시작:
  - `.moai/specs/SPEC-MINK-DISTANCING-STATEMENT-001/` (본 SPEC plan artifacts)
  - `.moai/project/distancing.md` (단일 신설 markdown)
- `.go`, `.proto`, `.yaml`, `.json` 파일 변경 0건
- 다른 `.moai/specs/SPEC-*` 디렉토리 변경 0건
- 다른 `.moai/project/*` 파일 변경 0건

검증 명령:
```bash
gh pr view <PR> --json files --jq '.files[].path' \
  | grep -vE '^(\.moai/specs/SPEC-MINK-DISTANCING-STATEMENT-001/|\.moai/project/distancing\.md$)'
# 결과 0줄
```

REQ 매핑: REQ-MINK-DST-PR-001, REQ-MINK-DST-SCOPE-001, REQ-MINK-DST-SCOPE-002

---

## 6. Technical Approach (Phased Implementation)

본 SPEC 은 markdown-only 변경이므로 phase 분리는 단순하다. 단일 PR (base=main, squash merge per CLAUDE.local.md §1.4) 안에 다음 2-step:

| Step | 작업 | 산출물 | 검증 |
|---|---|---|---|
| Step 1 | 5 product factual sourcing 점검 | (write 전 점검 결과) | research.md §4 사실 + run phase 시점 공식 URL 재확인 |
| Step 2 | distancing.md v1.0.0 작성 | `.moai/project/distancing.md` (~600-900 lines) | AC-001 ~ AC-011 grep 검증 |

### Step 1 — 5 product factual sourcing 점검

**도구**: WebFetch (선택), 또는 research.md §4 의 사실 인용

**대상**: 5 product 의 공식 URL / 운영 주체 / 라이선스 / pricing / 1차 사용 시나리오 — 본 SPEC plan phase 시점 (2026-05-12) 기준 사실 확인.

**점검 명령** (선택, run phase 작성자가 수행):

```bash
# (선택) WebFetch 로 공식 URL 사실 재확인
# - github.com/block/goose
# - replika.ai
# - routinery.app
# - together.ai (OpenClaw 페이지)
# - Hermes 는 카테고리이므로 단일 URL 부재 — 한국 시장 일반 인지로 충분
```

**Commit**: 본 step 은 별도 commit 부재 — Step 2 의 distancing.md 본문에 References 로 포함.

**Risk**: 매우 낮 — research.md §4 의 사실은 PRODUCT-V7-001 research.md §5 와 일관, run phase 작성자가 재확인 만으로 충분.

### Step 2 — distancing.md v1.0.0 작성

**도구**: Write tool

**대상**: `.moai/project/distancing.md` 신설 (현재 부재)

**v1.0.0 본문 sections** (target outline, research.md §5.2 와 동일):

1. frontmatter (REQ-MINK-DST-FILE-001)
2. `# MINK — Brand Distancing Statement` (제목)
3. `## HISTORY` (1.0.0 단일 row, REQ-MINK-DST-FILE-002)
4. `## 1. Preamble` (목적 + parent 인용 + anti-FUD 약속 + 사용처 + brand voice 명시)
5. `## 2. vs block/goose` (Identity / MINK is NOT / How MINK differs / 선택 Note)
6. `## 3. vs Hermes` (같은 구조)
7. `## 4. vs Replika` (같은 구조)
8. `## 5. vs Routinery` (같은 구조)
9. `## 6. vs OpenClaw` (같은 구조)
10. `## 7. Out of comparison scope` (ChatGPT / Claude / Gemini / Apple Intelligence / Cursor 등 비교 거부)
11. `## 8. Closing statement` (niche 한 줄 + brand-identity 약속)
12. `## 9. References` (parent SPEC + sibling SPEC + 5 product 공식 URL)

**분량 target**: 약 600~900 lines.

**작성 가이드**: research.md §6 (5 product 단락 작성 가이드) 의 4-단락 표준 + brand-voice §3.2 사양 준수.

**Commit type/message** (CLAUDE.local.md §2.2):

```
docs(distancing): SPEC-MINK-DISTANCING-STATEMENT-001 — distancing.md v1.0.0 신설

- PRODUCT-V7-001 §6 의 5 product starting point 를 detail 4-단락 구조로 확장
- block/goose / Hermes / Replika / Routinery / OpenClaw 각 Identity + MINK is NOT
  + How MINK differs + (선택) Note on coexistence
- brand-voice: 1인 dev tone, 한국어 1차, factual + sourced, anti-AI-slop, anti-FUD
- 시간 추정 부재, 비교우월 / 약점 부각 / 시장 점유 주장 부재
- References: parent SPEC + sibling SPEC + 5 product 공식 URL

SPEC: SPEC-MINK-DISTANCING-STATEMENT-001
REQ:  REQ-MINK-DST-FILE-001, REQ-MINK-DST-FILE-002, REQ-MINK-DST-STRUCT-*,
      REQ-MINK-DST-BLOCK-001, REQ-MINK-DST-HERMES-001, REQ-MINK-DST-REPLIKA-001,
      REQ-MINK-DST-ROUTINERY-001, REQ-MINK-DST-OPENCLAW-001,
      REQ-MINK-DST-PATTERN-001, REQ-MINK-DST-VOICE-001, REQ-MINK-DST-ANTIGOAL-001,
      REQ-MINK-DST-REFERENCES-001
AC:   AC-001 ~ AC-011
```

**Rollback**: `git revert <Step2-commit>` — distancing.md 신설 commit revert.

**Risk**: 낮 — markdown 만 변경, 컴파일 영향 0. 본문 quality 는 reviewer 의 판단 영역 + AC grep 검증 + anti-FUD grep 검증.

### PR 정책

- 단일 PR (CLAUDE.local.md §1.4 feature branch → main, squash merge with `--delete-branch`)
- branch 이름: `plan/SPEC-MINK-DISTANCING-STATEMENT-001` (CLAUDE.local.md §1.2 feature branch naming) — 본 SPEC 의 plan PR. run PR 은 별도 branch.
- PR title: `docs(distancing): SPEC-MINK-DISTANCING-STATEMENT-001 plan — 5-product brand distancing statement` (plan PR), `docs(distancing): SPEC-MINK-DISTANCING-STATEMENT-001 — distancing.md v1.0.0 신설` (run PR)
- PR body: §1 Overview + §4 EARS + §5 AC summary + AC-011 scope check 결과
- label: `type/docs` + `priority/p2-medium` + `area/docs`
- merge: squash + auto-delete branch

---

## 7. Dependencies

### 7.1 선행 결정

- **PRODUCT-V7-001 §6 Distancing** (parent / vision anchor, merge 완료 가정): 본 SPEC 의 5 product set + 카테고리 차이 starting point. 본 SPEC plan PR base 가 PRODUCT-V7-001 merge 후 main HEAD 이거나 그 후속.
- **IDEA-002 brain decision** (2026-05-12, brain dir 별도 보존): §7 distancing 4 product (실제 5개) 가 본 SPEC 의 source.
- **사용자 8 확정사항** (orchestrator-collected, 2026-05-12): 본 SPEC 의 brand-voice + anti-FUD + Wave 3 후속 SPEC roadmap 의 정당화.

### 7.2 본 SPEC 의 sibling (병행)

- **SPEC-MINK-BRAND-RENAME-001** (planned v0.1.1, sibling): brand identifier rename. body 변경 영역 disjoint — BRAND-RENAME 은 코드 + go.mod + .proto 등 변경, 본 SPEC 은 `.moai/project/distancing.md` 1 파일 markdown 변경. merge order 무관. brand-lint exemption logic 의 base 인용.

### 7.3 본 SPEC 의 downstream

본 SPEC 머지 후 다음의 reference 자료로 활용:

- README brand section 의 distancing 인용 (별도 후속 README 정정 SPEC)
- public Q&A / 블로그 / 외부 brand 충돌 대응 (외부 작업)
- 향후 Wave 3 SPEC (FORTUNE / HEALTH / CALENDAR / RITUAL / BRIEFING) 의 anti-goal 점검 reference

### 7.4 외부 의존

- 없음. 본 SPEC 은 markdown only.

---

## 8. Risks & Mitigations

| # | 리스크 | 가능성 | 영향 | Mitigation |
|---|--------|--------|------|------|
| R1 | distancing.md 의 5 product 묘사 부정확 (예: Replika pricing 변경, Routinery 기술 stack 추측) | 중 | 중 | run phase 시점 (2026-05-12) 에 5 product 공식 URL 사실 재확인. distancing.md References section 에 정보 캡처 시점 명시. 사실 변경 시 후속 sync PR. |
| R2 | anti-FUD 의도 외에 경쟁 product 비난으로 읽힐 위험 | 중 | 중 | brand-voice §3.2 의 avoided terms grep ≥ 0 강제 (AC-008). 4-단락 표준의 "Note on coexistence" 단락이 neutrality 강화. |
| R3 | "1인 dev" persona 의 좁은 정의가 distancing rationale 을 약화 | 낮 | 낮 | distancing.md preamble 에서 PRODUCT-V7-001 §3 (success metric anti-goal) 인용. 정당화는 parent SPEC vision anchor. |
| R4 | 5 product 중 일부가 향후 pivot / 기능 변화 → 정확성 손상 | 중 | 낮 | distancing.md frontmatter 의 `updated_at: 2026-05-12` 가 timestamp. 사실 변경 시 별도 sync PR 로 정정. 본 SPEC 은 v1.0.0 initial draft 만 책임. |
| R5 | brand-lint regex pattern (BRAND-RENAME-001 §7) 이 distancing.md 본문의 factual citation 토큰 ("block/goose", "Goose") 을 위반으로 flag | 중 | 낮 | factual citation 영역은 brand-lint exemption (BRAND-RENAME-001 §10 immutable preserve zone exemption logic 과 동일 원칙). 본 SPEC scope 외. run phase 작성자가 `scripts/check-brand.sh` 로 사전 점검 권장. flag 발생 시 후속 SPEC 으로 exemption 추가. |
| R6 | distancing.md 가 향후 marketing material 로 사용되어 anti-marketing 정책과 충돌 | 낮 | 낮 | distancing.md §1 Preamble 이 "brand-identity statement 이지 marketing material 이 아니다" 명시. anti-FUD / anti-AI-slop voice 가 자동으로 marketing 톤 차단. |
| R7 | distancing.md 가 너무 길어 README 인용 시 부담 | 낮 | 낮 | 600-900 lines 분량은 reference document 로 적정. README 인용은 preamble + niche 한 줄만 인용, detail 은 link. |
| R8 | 본 SPEC 머지 시점에 PRODUCT-V7-001 §6 본문이 아직 머지 안 됨 | 매우 낮 | 높 | 본 SPEC plan PR 머지 전 PRODUCT-V7-001 plan + run 완료 확인. orchestrator 머지 순서 담당. |
| R9 | 본 SPEC 의 5 product set 이 PRODUCT-V7 §6 의 5 product set 과 불일치 | 매우 낮 | 중 | AC-010 강제: 5 product 모두 main section header 보유 + 6번째 section 부재. |
| R10 | distancing.md 본문에 시간 추정 / Q3 / 다음 달 등 표현 등장 | 매우 낮 | 낮 | AC-008 grep 강제. brand-voice 가 시간 추정 부재 정책 (PRODUCT-V7-001 §4 REQ-MINK-PV7-ROADMAP-001 과 동일). |

---

## 9. References

### 9.1 본 SPEC 산출물

- `.moai/specs/SPEC-MINK-DISTANCING-STATEMENT-001/spec.md` (이 문서, v0.1.0)
- `.moai/specs/SPEC-MINK-DISTANCING-STATEMENT-001/research.md` (현황 + decision trail, ~500 lines)
- `.moai/project/distancing.md` (v1.0.0 본문, Step 2 산출물)

### 9.2 parent / sibling SPEC

- `.moai/specs/SPEC-MINK-PRODUCT-V7-001/spec.md` v0.1.0 (**parent / vision anchor**, merge 완료 가정): §6 Distancing 의 5 product 1 단락 starting point 가 본 SPEC 의 input.
- `.moai/specs/SPEC-MINK-PRODUCT-V7-001/research.md` (§5 Competitive landscape (Distancing) §5.1 의 5 product 분석 source).
- `.moai/specs/SPEC-MINK-BRAND-RENAME-001/spec.md` v0.1.1 (**sibling**, body 변경 영역 disjoint, merge order 무관).
- `.moai/specs/SPEC-MINK-BRAND-RENAME-001/research.md` (§3 분류 매트릭스 의 immutable preserve zone 의 brand-lint exemption logic base 인용).

### 9.3 결정 trail

- **IDEA-002 brain decision** (2026-05-12, brain dir: `/Users/goos/Projects/GooseBot/.moai/brain/IDEA-002/`): 8 확정사항 중 §7 distancing 4 product (실제 5개) 가 본 SPEC 의 source.
- **사용자 8 확정사항** (orchestrator-collected, 2026-05-12): scope / 6m metric / brand / persona / language / Wave 3 후속 SPEC roadmap / distancing 5 product.

### 9.4 본 프로젝트 기존 자료

- `.moai/project/branding.md` (v4.0 GLOBAL EDITION + v5.0 Tamagotchi, 24939 bytes, 2026-04-27): 옛 GOOSE 자료. v7.0 vision reset 이후 sibling 정정 후속 SPEC 대상. 본 SPEC 의 brand-voice 는 branding.md v4.0 의 GOOSE / 거위 / 다마고치 메타포 인용 안 함.
- `.moai/project/brand/{brand-voice,visual-identity,target-audience}.md`: 모두 `_TBD_` stub (brand interview 미완). 본 SPEC 의 brand-voice 사양은 self-derived (research.md §3.2).
- `CLAUDE.md` §10 Web Search Protocol (anti-hallucination): 5 product 사실 확인 의무 근거.
- `CLAUDE.local.md` §1.4 (squash merge), §2.2 (한국어 commit body), §2.5 (code comment 영어 / 사용자 문서 한국어 1차 정책).

### 9.5 외부 참조 (5 product 공식 URL)

- **block/goose**: <https://github.com/block/goose> (multi-tenant agent framework)
- **Hermes**: 한국 AI 여친 / 친구 LLM-bot 카테고리 (single canonical URL 부재, multiple vendors)
- **Replika**: <https://replika.ai> (Luka, Inc. 운영, luka.ai)
- **Routinery**: <https://routinery.app> (한국제 startup, habit tracker)
- **OpenClaw**: <https://together.ai> (Together AI 운영, agent framework)

---

## 10. Exclusions (What NOT to Build)

본 SPEC 은 다음을 **명시적으로 제외**한다. 후속 SPEC 작성자가 본 SPEC 범위 외 작업을 본 PR 에 추가하지 못하도록 차단한다.

1. **sibling `.moai/project/*.md` 파일 정정** (product.md / branding.md / ecosystem.md / migration.md / brand/* 등) — 별도 후속 audit / SPEC
2. **PRODUCT-V7-001 의 §6 Distancing 본문 변경** — vision-level starting point 는 보존, 본 SPEC 은 detail 확장만
3. **BRAND-RENAME-001 의 brand-lint regex / exemption rule 추가** — 본 SPEC scope 외, 별도 후속 SPEC 으로 처리
4. **코드 변경** (Go module path / binary / proto / 식별자 — sibling SPEC-MINK-BRAND-RENAME-001 담당)
5. **5 product 의 logo / 시각 자산 / marketing material 인용** — 본 SPEC 은 text-only statement
6. **distancing.md 영문 번역본 작성** — i18n 후순위, 별도 후속 SPEC
7. **6번째 이상의 product 추가** (PRODUCT-V7-001 §6 5 product subset 위반 금지, superset 도 본 SPEC scope 외 — distancing 대상 product 확장은 PRODUCT-V7 vision-level 변경 후 본 SPEC v1.1.0 으로 처리)

---

Version: 0.1.0
Status: draft
Last Updated: 2026-05-12
