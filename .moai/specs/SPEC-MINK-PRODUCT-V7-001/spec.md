---
id: SPEC-MINK-PRODUCT-V7-001
version: 0.1.0
status: completed
created_at: 2026-05-12
updated_at: 2026-05-12
author: manager-spec
priority: High
labels: [brand-strategy, vision, meta, product-doc]
issue_number: null
phase: meta
size: 중(M)
lifecycle: spec-anchored
related_specs: [SPEC-MINK-BRAND-RENAME-001]
---

# SPEC-MINK-PRODUCT-V7-001 — product.md v7.0 (1인 dev personal ritual companion)

## HISTORY

| Version | Date | Author | Change |
|---------|------|--------|--------|
| 0.1.0 | 2026-05-12 | manager-spec | Initial draft. IDEA-002 brain decision Wave 3 §4.1 item 2 구현. product.md v7.0 신설 (1인 dev personal ritual companion 비전) + 기존 v6.0 (38665 bytes, "Daily Companion Edition", multi-tenant agent platform 야망) 를 `.moai/project/product-archive/product-v6.0-2026-04-27.md` 로 archive. 코드 변경 없음, 문서만. SPEC-MINK-BRAND-RENAME-001 (downstream child) 의 constitutional parent / vision anchor 역할. 19 EARS 요구사항 + 12 binary-verifiable AC + 7 명시 제외 항목. |
| 0.1.1 | 2026-05-14 | MoAI orchestrator | Drift correction: status draft → completed. `.moai/project/product.md` v7.0 배포 완료. |

---

## 1. Overview

### 1.1 Scope Clarity

본 SPEC 은 **vision-document SPEC** 이다. 단일 모듈 / 기능 / 코드 변경을 정의하지 않으며, 프로젝트의 product vision document (`.moai/project/product.md`) 의 v7.0 본문을 신설하고 기존 v6.0 본문을 archive 한다. 코드 변경 0건. 산출물은 markdown 문서 2개:

1. `.moai/project/product.md` 재작성 (v7.0 본문)
2. `.moai/project/product-archive/product-v6.0-2026-04-27.md` 신설 (v6.0 byte-identical 보존)

본 SPEC 은 downstream SPEC 들의 **vision anchor (constitutional parent)** 역할을 한다:
- `SPEC-MINK-BRAND-RENAME-001` (planned): brand identifier rename — v7.0 의 MINK 비전이 brand rename 의 정당화.
- `SPEC-MINK-DISTANCING-STATEMENT-001` (proposed Wave 3): distancing detail document — v7.0 §Distancing 의 4 단락이 detail SPEC 의 시작점.
- 향후 Wave 3 SPEC 들 (FORTUNE / HEALTH / CALENDAR / RITUAL / BRIEFING): v7.0 의 ritual flow 가 use case 정의 근거.

### 1.2 Goal

`.moai/project/product.md` 를 v6.0 (multi-tenant agent platform / 글로벌 1M user 야망 / freemium tier / Linux Foundation 가입 목표) 에서 v7.0 (**1인 dev 의 매일 ritual 을 함께하는 personal AI companion**) 으로 vision reset 한다. IDEA-002 brain decision 의 6개월 success metric ("본인 매일 + 본인 외 1명 daily user, 외부 지표 무관") 을 anti-goal 명시 형태로 본문에 포함한다. Hermes / Replika / Routinery / OpenClaw / block/goose 의 5개 product 와 niche 차별화를 명시한다.

### 1.3 Non-Goals

본 SPEC 의 직접 산출물 외 다음은 **본 SPEC scope 외**:

- `.moai/project/product.md` 외의 모든 sibling `.moai/project/*.md` 파일 정정 (tech.md / structure.md / ecosystem.md / migration.md / adaptation.md / learning-engine.md / token-economy.md / branding.md — 별도 후속 audit/SPEC)
- `.moai/project/brand/*` 정정 (visual-identity.md / brand-voice.md / target-audience.md — 별도 brand interview 완료 후 정정)
- 코드 변경 (Go module path / binary / proto / 식별자 등 — downstream `SPEC-MINK-BRAND-RENAME-001` 담당)
- distancing detail document 의 본문 작성 (별도 `SPEC-MINK-DISTANCING-STATEMENT-001`. 본 SPEC §Distancing 의 4 단락은 시작점 reference 만)
- Wave 3 신규 SPEC (FORTUNE / HEALTH / CALENDAR / RITUAL / BRIEFING) plan 작성
- v6.0 archive 외 다른 historical 문서 archive
- product.md 의 영문 번역본 작성

---

## 2. Background

### 2.1 IDEA-002 brain decision (2026-05-12)

사용자 IDEA-002 brain decision 의 8 확정사항 핵심:

1. **brand**: AI.MINK → MINK (Made IN Korea)
2. **6m success metric**: 본인 매일 + 1명 daily user. 외부 stars/WAU/revenue 무관 (anti-goal)
3. **scope reset**: multi-tenant SaaS / marketplace / enterprise tier 야망 폐기
4. **target persona**: 1인 dev (Korean, MoAI/Claude Code user)
5. **Wave 3 후속 SPEC**: PRODUCT-V7 (본 SPEC) / BRAND-RENAME / DISTANCING / USERDATA-MIGRATE / ENV-MIGRATE
6. **language**: 한국어 1차, 영어 i18n 후순위
7. **distancing 4종**: block/goose, Hermes, Replika, Routinery, OpenClaw (5개)
8. **immutable history**: 기존 v6.0 본문은 archive 로 보존 (byte-identical)

### 2.2 v6.0 본문의 한계 (research.md §1.1 요약)

현재 `.moai/project/product.md` v6.0 (38665 bytes, 2026-04-22 작성) 은:
- 외부지표 지향 (GitHub stars 100K / 1M users / ARR $24M / Linux Foundation 가입)
- multi-tenant SaaS 구조 (Goose Cloud / Marketplace / Enterprise SLA)
- block/goose 와 brand 충돌 (`goose` identifier + 거위 메타포)
- 1인 dev personal tool 본질이 multi-tenant 야망 아래 묻혀 명시되지 않음
- 1인 dev (Korean, MoAI user) persona 명시적 정의 부재

v7.0 은 위 한계를 직접 reset.

### 2.3 v7.0 의 의미

v7.0 은 패러다임 진화표 (research.md §2.1) 의 6번째 피벗:
- v4.0 GLOBAL: 글로벌 오픈소스 (2026-04-21)
- v5.0 Tamagotchi: + 양육 메타포 (2026-04-21)
- v6.0 Daily Companion: + 일상 ritual (2026-04-22)
- **v7.0 MINK Personal Ritual Companion** (2026-05-12): 1인 dev personal tool 로 reset, multi-tenant 야망 폐기

---

## 3. Scope

### 3.1 IN Scope

본 SPEC PR 범위 안에서 다음을 산출한다:

1. **`.moai/project/product.md` v7.0 재작성**:
   - frontmatter (version: 7.0.0 / brand: MINK / status: published / classification: VISION_DOCUMENT / spec: SPEC-MINK-PRODUCT-V7-001)
   - HISTORY 표 (v7.0 row 1행)
   - 본문 sections: Vision / Persona / Success Metric / In-Scope Ritual Flow / Anti-Goals / Distancing / Wave Roadmap pointer
   - 분량: 약 400~600 lines (v6.0 의 1189 lines 대비 의도된 단순화)
2. **`.moai/project/product-archive/product-v6.0-2026-04-27.md` 신설**:
   - v6.0 본문 byte-identical 보존
   - archive 디렉토리 (`product-archive/`) 신설 (현재 부재)
   - 파일명 컨벤션: `product-v{version}-{mtime-date}.md`

### 3.2 OUT Scope (반드시 보존 — sibling 파일 / 코드)

[HARD] 다음 항목은 변경 금지:

1. `.moai/project/product.md` 외의 sibling `.moai/project/*.md` 9 파일 (tech.md / structure.md / ecosystem.md / migration.md / adaptation.md / learning-engine.md / token-economy.md / branding.md / brand/* 4 파일)
2. `.moai/project/codemaps/*.md` 3 파일
3. `.moai/project/research/*.md` 4 파일
4. 코드 (`*.go`, `*.proto`, `cmd/`, `internal/` — downstream SPEC-MINK-BRAND-RENAME-001 담당)
5. 다른 SPEC 본문 (`.moai/specs/SPEC-*/spec.md`)
6. CHANGELOG / README / git history / `.claude/agent-memory/` (immutable archive)
7. v6.0 본문 byte (archive 시 byte-identical 보존, 한 byte도 수정 금지)

---

## 4. EARS Requirements

본 SPEC 은 19 EARS 요구사항을 정의한다. 각 요구사항은 spec-anchored 이며 §5 AC 와 1:1 또는 N:1 매핑된다.

### 4.1 Ubiquitous (보편 — 항상 성립)

- **REQ-MINK-PV7-VISION-001 [Ubiquitous]** The `product.md` v7.0 본문 **shall** define MINK as "1인 dev 의 매일 ritual 을 함께하는 personal AI companion" in the Vision section.
- **REQ-MINK-PV7-VISION-002 [Ubiquitous]** The `product.md` v7.0 본문 **shall** include the Korean tagline "매일 아침, 매일 저녁, 너의 MINK." in the Vision section.
- **REQ-MINK-PV7-PERSONA-001 [Ubiquitous]** The `product.md` v7.0 본문 **shall** define the primary persona as a Korean-speaking 1인 dev (solo developer or small team senior dev, age 30-45, MoAI-ADK / Claude Code user) in the Persona section.
- **REQ-MINK-PV7-METRIC-001 [Ubiquitous]** The `product.md` v7.0 본문 **shall** define the 6-month success metric as "본인 (project owner) 매일 사용 + 본인 외 1명 daily user" in the Success Metric section.
- **REQ-MINK-PV7-METRIC-002 [Ubiquitous]** The `product.md` v7.0 본문 **shall** explicitly mark the following as anti-goals (외부 지표 추구 거부): GitHub stars, WAU/MAU/DAU, revenue (구독/marketplace commission), Linux Foundation 가입, public marketing channels.
- **REQ-MINK-PV7-SCOPE-001 [Ubiquitous]** The `product.md` v7.0 본문 **shall** enumerate the in-scope ritual flow use cases: morning brief, journal write + emotion tagging, long-term memory recall (1년 후 추억 / trend / search / summary), ambient LLM context inject, telegram bridge.
- **REQ-MINK-PV7-ANTIGOAL-001 [Ubiquitous]** The `product.md` v7.0 본문 **shall** declare the following as out-of-scope (anti-goals) for the product: multi-tenant SaaS, marketplace / plugin commission, enterprise tier (SLA / RBAC), mobile-first design, romance / persona role-play, gamification / streak, behavioral nudge optimization.
- **REQ-MINK-PV7-DIST-001 [Ubiquitous]** The `product.md` v7.0 본문 **shall** include a Distancing section with at least one paragraph each for: block/goose (multi-tenant agent platform), Hermes (AI girlfriend / emotional companion category), Replika (long-term emotional AI), Routinery (habit tracker app), OpenClaw (agent framework).
- **REQ-MINK-PV7-ARCHIVE-001 [Ubiquitous]** The v6.0 본문 **shall** be preserved byte-identical at `.moai/project/product-archive/product-v6.0-2026-04-27.md` after this SPEC merges.
- **REQ-MINK-PV7-FRONTMATTER-001 [Ubiquitous]** The `product.md` v7.0 본문 **shall** include YAML frontmatter with fields: `version: 7.0.0`, `brand: MINK`, `status: published`, `created_at: 2026-05-12`, `classification: VISION_DOCUMENT`, `spec: SPEC-MINK-PRODUCT-V7-001`.
- **REQ-MINK-PV7-HISTORY-001 [Ubiquitous]** The `product.md` v7.0 본문 **shall** start its `## HISTORY` table with exactly one data row (v7.0 신설 row), with the older v1.0~v6.0 history rows preserved in the archive file only.

### 4.2 Event-Driven (트리거 발생 시)

- **REQ-MINK-PV7-DOWNSTREAM-001 [Event-Driven]** **When** a downstream SPEC (e.g., `SPEC-MINK-BRAND-RENAME-001`, future Wave 3 SPECs FORTUNE / HEALTH / CALENDAR / RITUAL / BRIEFING) is authored, the SPEC **shall** reference `.moai/project/product.md` v7.0 as the vision anchor in its `§Background` or `§References` section.
- **REQ-MINK-PV7-ROADMAP-001 [Event-Driven]** **When** the v7.0 본문 includes a roadmap section, the roadmap **shall** be expressed as Wave-based ordering (Wave 1 / Wave 2 / Wave 3 / Wave 4) with no calendar time estimates (no "2026 Q3", no "in 6 months", no "next year").
- **REQ-MINK-PV7-ARCHIVE-002 [Event-Driven]** **When** this SPEC's PR is created, the PR diff **shall** include exactly two changed files in the `.moai/project/` tree: (a) `.moai/project/product.md` (modified, full rewrite to v7.0) and (b) `.moai/project/product-archive/product-v6.0-2026-04-27.md` (new file, byte-identical to v6.0 baseline).

### 4.3 State-Driven (상태 조건)

- **REQ-MINK-PV7-PRIORITY-001 [State-Driven]** **While** the v7.0 본문 references priorities or sequencing of work, the references **shall** use Priority labels (Priority High / Medium / Low) or Wave ordering — not calendar time estimates.
- **REQ-MINK-PV7-LANG-001 [State-Driven]** **While** the v7.0 본문 references language support, Korean **shall** be designated as the 1차 language and English / Japanese / Chinese as i18n 후순위 (anti-goal: English-only global user persona).

### 4.4 Unwanted (금지 행동)

- **REQ-MINK-PV7-IMMUTABLE-001 [Unwanted]** **If** the v6.0 archive file (`product-archive/product-v6.0-2026-04-27.md`) byte content differs from the v6.0 baseline (`.moai/project/product.md` at git HEAD before this SPEC's first commit), **then** the change **shall** be rejected.
- **REQ-MINK-PV7-SCOPE-002 [Unwanted]** **If** this SPEC's PR diff includes any modification to `.moai/project/*.md` files other than `product.md` and `product-archive/product-v6.0-2026-04-27.md`, **then** the PR **shall** be rejected as scope violation.
- **REQ-MINK-PV7-SCOPE-003 [Unwanted]** **If** this SPEC's PR diff includes any modification to non-markdown files (`.go`, `.proto`, `.yaml`, `.json`, etc. outside `.moai/specs/SPEC-MINK-PRODUCT-V7-001/`), **then** the PR **shall** be rejected as scope violation.

### 4.5 Optional (해당 시)

- **REQ-MINK-PV7-RECONCILE-001 [Optional]** **Where** the v7.0 본문 conflicts with statements in sibling `.moai/project/*.md` files (e.g., ecosystem.md still references marketplace tier from v6.0), the v7.0 본문 **shall** take precedence as the canonical vision; sibling file reconciliation is OUT scope of this SPEC and deferred to a follow-up audit.

---

## 5. Acceptance Criteria

각 AC 는 Given / When / Then 형식. 모든 AC 는 binary 검증 가능 (file existence, byte-level diff, single shell command, grep match count).

### AC-001 — product.md v7.0 frontmatter + HISTORY 신설

**Given** 본 SPEC PR squash merge 후 main HEAD state 에서
**When** `head -30 .moai/project/product.md` 를 실행하면
**Then** 출력에 다음이 모두 포함된다:
- 첫 줄 `---` (frontmatter 시작)
- `version: 7.0.0` (또는 `version: "7.0.0"`) 라인 존재
- `brand: MINK` 라인 존재
- `status: published` 라인 존재
- `spec: SPEC-MINK-PRODUCT-V7-001` 라인 존재
- `classification: VISION_DOCUMENT` 라인 존재
- `## HISTORY` 섹션 header 존재
- HISTORY 표에 `| 7.0.0 | 2026-05-12 |` 행 존재

검증 명령: `head -30 .moai/project/product.md | grep -c '^version: \(7\.0\.0\|"7\.0\.0"\)'` ≥ 1

REQ 매핑: REQ-MINK-PV7-FRONTMATTER-001, REQ-MINK-PV7-HISTORY-001

### AC-002 — v6.0 archive byte-identical 보존

**Given** 본 SPEC PR 의 first commit 직전 baseline 으로 `sha256sum .moai/project/product.md` 가 캡처된 상태에서 (baseline SHA `X`)
**When** 본 SPEC PR squash merge 후 archive 파일의 hash 를 확인하면
**Then**:
- `.moai/project/product-archive/product-v6.0-2026-04-27.md` 파일 존재 (`test -f` exit 0)
- `sha256sum .moai/project/product-archive/product-v6.0-2026-04-27.md` 의 hash 값 = baseline SHA `X` (byte-identical)
- baseline 의 first 3 lines (`# AI.MINK - 제품 문서 v4.0 GLOBAL EDITION` 등) 가 archive 파일의 first 3 lines 와 정확 일치

검증 명령:
```bash
test -f .moai/project/product-archive/product-v6.0-2026-04-27.md && echo OK
sha256sum .moai/project/product-archive/product-v6.0-2026-04-27.md  # baseline 과 비교
```

REQ 매핑: REQ-MINK-PV7-ARCHIVE-001, REQ-MINK-PV7-IMMUTABLE-001

### AC-003 — v7.0 vision 정의 + tagline 명시

**Given** 본 SPEC PR squash merge 후
**When** `.moai/project/product.md` 에서 vision 본문을 grep 하면
**Then** 다음 매치가 모두 존재 (각 명령 결과 ≥ 1):
- `grep -c '1인 dev' .moai/project/product.md` ≥ 1
- `grep -c 'personal\(.*\)ritual\(.*\)companion' .moai/project/product.md` ≥ 1 (or 한글 등가 표현)
- `grep -c '매일 아침, 매일 저녁, 너의 MINK' .moai/project/product.md` ≥ 1

REQ 매핑: REQ-MINK-PV7-VISION-001, REQ-MINK-PV7-VISION-002

### AC-004 — 6m success metric clause 명시

**Given** 본 SPEC PR squash merge 후
**When** `.moai/project/product.md` 에서 metric 본문을 grep 하면
**Then** 다음 매치가 모두 존재:
- `grep -c '본인 매일\|본인이 매일\|매일 본인' .moai/project/product.md` ≥ 1
- `grep -c '1명\|한 명' .moai/project/product.md` ≥ 1 (본인 외 1명 daily user clause)
- `grep -c 'stars\|WAU\|MAU\|revenue\|anti-goal' .moai/project/product.md` ≥ 3 (anti-goal 명시 cluster)

REQ 매핑: REQ-MINK-PV7-METRIC-001, REQ-MINK-PV7-METRIC-002

### AC-005 — persona 정의

**Given** 본 SPEC PR squash merge 후
**When** `.moai/project/product.md` 에서 persona 본문을 grep 하면
**Then** 다음 매치가 모두 존재:
- `grep -c '1인 dev\|solo developer' .moai/project/product.md` ≥ 1
- `grep -c 'Korean\|한국어' .moai/project/product.md` ≥ 1
- `grep -c 'MoAI\|Claude Code' .moai/project/product.md` ≥ 1

REQ 매핑: REQ-MINK-PV7-PERSONA-001

### AC-006 — in-scope ritual flow 5종 enumeration

**Given** 본 SPEC PR squash merge 후
**When** `.moai/project/product.md` 의 in-scope ritual flow 섹션을 확인하면
**Then** 다음 5개 use case 가 모두 명시적으로 enumerate 됨 (각 grep ≥ 1):
- morning brief (운세 + 날씨 + 일정 + mood trend)
- journal (write + emotion tagging + crisis word + long-term memory recall)
- weather / scheduler integration
- ambient LLM context (journal/weather/일정 결과를 LLM 응답에 inject)
- telegram bridge

검증: `grep -E 'morning brief|아침 brief|aim brief' .moai/project/product.md | wc -l` ≥ 1 (각 5개 use case 에 대해)

REQ 매핑: REQ-MINK-PV7-SCOPE-001

### AC-007 — anti-goal 항목 명시

**Given** 본 SPEC PR squash merge 후
**When** `.moai/project/product.md` 에서 anti-goal 본문을 확인하면
**Then** 다음 6개 anti-goal 항목 중 최소 5개 명시적으로 등장 (grep 매치 ≥ 1):
- multi-tenant SaaS / multi-tenant agent platform
- marketplace / plugin commission
- enterprise tier / SLA / RBAC
- mobile-first
- gamification / streak
- public marketing / Linux Foundation / community channels

검증: 6개 키워드 cluster 별로 `grep -c` 누계 ≥ 5

REQ 매핑: REQ-MINK-PV7-METRIC-002, REQ-MINK-PV7-ANTIGOAL-001

### AC-008 — Distancing 5 product 단락 존재

**Given** 본 SPEC PR squash merge 후
**When** `.moai/project/product.md` 의 Distancing 섹션을 확인하면
**Then** 5개 product 이름이 각각 본문 텍스트에 등장 (grep 매치 ≥ 1):
- `grep -c '\bblock/goose\b\|block goose' .moai/project/product.md` ≥ 1
- `grep -c '\bHermes\b' .moai/project/product.md` ≥ 1
- `grep -c '\bReplika\b' .moai/project/product.md` ≥ 1
- `grep -c '\bRoutinery\b' .moai/project/product.md` ≥ 1
- `grep -c '\bOpenClaw\b' .moai/project/product.md` ≥ 1

또는 본 SPEC 작성자 판단에 따라 distancing detail 을 `SPEC-MINK-DISTANCING-STATEMENT-001` 로 위임할 경우, 위임 marker 명시 (`grep -c 'SPEC-MINK-DISTANCING-STATEMENT-001' .moai/project/product.md` ≥ 1) — 단, 본 SPEC 본문에서 5 product 이름 중 최소 4개는 등장해야 함 (vision 차원 기본 distancing 의무).

REQ 매핑: REQ-MINK-PV7-DIST-001

### AC-009 — sibling .moai/project/ 파일 무변경

**Given** 본 SPEC PR 의 first commit 직전 baseline 으로 다음이 캡처된 상태에서:
- `find .moai/project -maxdepth 2 -type f -name '*.md' | grep -v 'product.md' | grep -v 'product-archive' | sort` 의 SHA-256 hash list

**When** 본 SPEC PR squash merge 후 동일 명령을 재실행하면

**Then** baseline 과 byte-identical (sibling 파일 변경 0건).

검증: 위 명령의 hash list 가 baseline 과 일치

REQ 매핑: REQ-MINK-PV7-SCOPE-002

### AC-010 — 비-markdown 파일 무변경 + 다른 SPEC 무변경

**Given** 본 SPEC PR squash merge 후
**When** PR diff 의 changed file list 를 확인하면 (`gh pr view <PR-N> --json files`)
**Then**:
- changed files 가 다음 prefix 중 하나로만 시작: `.moai/specs/SPEC-MINK-PRODUCT-V7-001/`, `.moai/project/product.md`, `.moai/project/product-archive/`
- `.go`, `.proto`, `.yaml`, `.json` 파일 변경 0건
- 다른 `.moai/specs/SPEC-*` 디렉토리 변경 0건

검증: `gh pr view <PR> --json files --jq '.files[].path' | grep -vE '^\.moai/(specs/SPEC-MINK-PRODUCT-V7-001|project/(product\.md|product-archive))'` 결과 0줄

REQ 매핑: REQ-MINK-PV7-SCOPE-003

### AC-011 — roadmap 시간 추정 부재

**Given** 본 SPEC PR squash merge 후
**When** `.moai/project/product.md` 에서 roadmap 본문을 확인하면
**Then** 다음 시간 추정 키워드 등장 0건 (또는 부정 문맥 / anti-goal 인용 문맥에서만 등장):
- `grep -E '\b(1주|2주|1개월|3개월|6개월|1년|2년|Q[1-4]|Year [1-4]|2027|2028)\b' .moai/project/product.md` 의 매치 중, anti-goal / "폐기" / "v6.0 비전" 같은 부정 문맥 외 양의 시간 약속이 0건
- Wave 기반 표현 ("Wave 1 / Wave 2 / Wave 3") 또는 Priority 라벨 사용

검증: 수동 review (PR description 에 reviewer 가 confirmation comment 작성)

REQ 매핑: REQ-MINK-PV7-ROADMAP-001, REQ-MINK-PV7-PRIORITY-001

### AC-012 — downstream SPEC 의 vision anchor 가능성 검증

**Given** 본 SPEC PR squash merge 후
**When** `.moai/specs/SPEC-MINK-BRAND-RENAME-001/spec.md` 의 §1 Background 또는 §References 에서 `.moai/project/product.md` 또는 "v7.0" 참조 가능성을 확인하면
**Then** 다음 중 하나가 성립:
- v7.0 본문이 후속 SPEC 의 vision anchor 로 인용 가능 형태 (Vision section 의 한 줄 정의 + persona + metric clause 가 self-contained 인용 가능)
- 본 SPEC 의 spec.md §10 References 에 SPEC-MINK-BRAND-RENAME-001 / 후속 Wave 3 SPEC 들과의 parent-child 관계가 명시되어 있음 (이미 §1.1 + §References 에 명시)

검증: 본 SPEC 자체의 §1.1 + §References 의 명시로 만족 (binary)

REQ 매핑: REQ-MINK-PV7-DOWNSTREAM-001

---

## 6. Technical Approach (Phased Implementation)

본 SPEC 은 markdown-only 변경이므로 phase 분리는 단순하다. 단일 PR (base=main, squash merge per CLAUDE.local.md §1.4) 안에 다음 3-step:

| Step | 작업 | 산출물 | 검증 |
|---|---|---|---|
| Step 1 | v6.0 baseline 캡처 + archive 신설 | `.moai/project/product-archive/product-v6.0-2026-04-27.md` (cp 결과) | `sha256sum` byte-identical |
| Step 2 | product.md v7.0 재작성 | `.moai/project/product.md` (v7.0 본문) | AC-001 ~ AC-008, AC-011 grep 검증 |
| Step 3 | scope 위반 점검 | sibling 파일 / 비-markdown 무변경 확인 | AC-009 + AC-010 |

### Step 1 — v6.0 archive

**도구**: Bash `mkdir` + `cp`

**명령**:
```bash
mkdir -p .moai/project/product-archive
cp .moai/project/product.md .moai/project/product-archive/product-v6.0-2026-04-27.md
```

**검증**:
- `sha256sum .moai/project/product.md .moai/project/product-archive/product-v6.0-2026-04-27.md` 두 hash 일치
- archive 디렉토리 신설 commit 의 첫 단계

**Commit type/message** (CLAUDE.local.md §2.2):
```
docs(product): SPEC-MINK-PRODUCT-V7-001 Step 1 — v6.0 본문 archive

- .moai/project/product-archive/ 신설
- product-v6.0-2026-04-27.md = product.md byte-identical copy
- v7.0 재작성 직전 baseline 보존

SPEC: SPEC-MINK-PRODUCT-V7-001
REQ:  REQ-MINK-PV7-ARCHIVE-001, REQ-MINK-PV7-IMMUTABLE-001
AC:   AC-002
```

**Rollback**: `rm -rf .moai/project/product-archive/` (archive 신설만 reset).

**Risk**: 매우 낮 — cp byte-identical 보장.

### Step 2 — product.md v7.0 재작성

**도구**: Write tool

**대상**: `.moai/project/product.md` 전면 재작성 (overwrite)

**v7.0 본문 sections** (target outline):

1. frontmatter (REQ-MINK-PV7-FRONTMATTER-001)
2. `# MINK — 1인 dev personal ritual companion`
3. `## HISTORY` (v7.0 단일 row, REQ-MINK-PV7-HISTORY-001)
4. `## 1. Vision` (REQ-MINK-PV7-VISION-001, REQ-MINK-PV7-VISION-002)
   - "MINK 는 1인 dev 의 매일 ritual 을 함께하는 personal AI companion" 한 줄 정의
   - Korean tagline + English tagline
   - 패러다임 진화표 (v0.2 ~ v7.0 6 rows, v6.0 archive 인용 형태로 inline)
5. `## 2. Primary Persona` (REQ-MINK-PV7-PERSONA-001)
   - 1인 dev (Korean, MoAI/Claude Code user, age 30-45)
   - anti-persona (enterprise team / marketplace plugin dev / English-only / mobile-first / casual chat)
6. `## 3. Success Metric (6-month)` (REQ-MINK-PV7-METRIC-001, REQ-MINK-PV7-METRIC-002)
   - 필수: 본인 매일
   - ceiling: 본인 외 1명 daily
   - anti-goals 6항목 cluster
   - 정당화 (IDEA-002 brain reasoning 인용)
7. `## 4. In-Scope Ritual Flow` (REQ-MINK-PV7-SCOPE-001)
   - morning brief
   - journal + emotion + crisis word
   - long-term memory (1년 후 추억 / trend / search / summary)
   - ambient LLM context inject
   - telegram bridge
   - (선택) MoAI dogfooding sub-section
8. `## 5. Anti-Goals` (REQ-MINK-PV7-ANTIGOAL-001)
   - multi-tenant SaaS / marketplace / enterprise tier / mobile-first / romance role-play / gamification / public marketing
9. `## 6. Distancing` (REQ-MINK-PV7-DIST-001)
   - vs block/goose (multi-tenant agent platform)
   - vs Hermes (AI girlfriend / emotional companion)
   - vs Replika (long-term emotional AI)
   - vs Routinery (habit tracker)
   - vs OpenClaw (agent framework)
   - (선택) "out of comparison scope" — ChatGPT / Cursor 등
10. `## 7. Wave Roadmap pointer` (REQ-MINK-PV7-ROADMAP-001)
    - Wave 1 (완료): foundation (CLI / journal / scheduler / weather)
    - Wave 2 (진행 중): integration (BRIEFING 등)
    - Wave 3 (계획): brand reset (BRAND-RENAME / DISTANCING / USERDATA-MIGRATE / ENV-MIGRATE / FORTUNE / HEALTH / CALENDAR / RITUAL)
    - 시간 추정 금지
    - Priority 라벨 사용
11. `## 8. References`
    - `.moai/specs/SPEC-MINK-PRODUCT-V7-001/spec.md` (본 SPEC)
    - `.moai/specs/SPEC-MINK-BRAND-RENAME-001/spec.md`
    - `.moai/project/product-archive/product-v6.0-2026-04-27.md` (이전 본문)
    - IDEA-002 brain decision (별도 프로젝트 brain dir)
12. footer (Version / Status / License)

**분량 target**: 약 400~600 lines (v6.0 의 1189 lines 대비 ~50% 단순화).

**Commit type/message**:
```
docs(product): SPEC-MINK-PRODUCT-V7-001 Step 2 — product.md v7.0 재작성

- 1인 dev personal ritual companion 비전 정립
- 6m success metric: 본인 매일 + 1명, 외부 지표 anti-goal
- persona: 1인 dev (Korean, MoAI/Claude Code user)
- in-scope ritual flow 5종: morning brief / journal / memory recall / ambient context / telegram
- anti-goals: multi-tenant / marketplace / enterprise / mobile-first / gamification / public marketing
- distancing 5 product: block/goose, Hermes, Replika, Routinery, OpenClaw
- Wave 기반 roadmap (시간 추정 부재)

SPEC: SPEC-MINK-PRODUCT-V7-001
REQ:  REQ-MINK-PV7-VISION-001, REQ-MINK-PV7-PERSONA-001, REQ-MINK-PV7-METRIC-*, REQ-MINK-PV7-SCOPE-001, REQ-MINK-PV7-ANTIGOAL-001, REQ-MINK-PV7-DIST-001, REQ-MINK-PV7-ROADMAP-001, REQ-MINK-PV7-FRONTMATTER-001, REQ-MINK-PV7-HISTORY-001
AC:   AC-001, AC-003 ~ AC-008, AC-011
```

**Rollback**: `git revert <Step2-commit>` — v7.0 재작성 만 revert, archive 보존.

**Risk**: 낮 — markdown 만 변경, 컴파일 영향 0. 본문 quality 가 reviewer 의 판단 영역.

### Step 3 — scope 위반 점검

**도구**: Bash + `git diff`

**대상**: PR 의 changed file list 점검

**검증 명령**:
```bash
# AC-009: sibling .moai/project/*.md 무변경
git diff origin/main --name-only -- .moai/project/ | grep -vE '^\.moai/project/(product\.md|product-archive/)'
# 출력 0줄 기대

# AC-010: 비-markdown / 다른 SPEC 무변경
git diff origin/main --name-only | grep -vE '^\.moai/(specs/SPEC-MINK-PRODUCT-V7-001|project/(product\.md|product-archive))'
# 출력 0줄 기대
```

**Commit**: 본 step 은 별도 commit 부재 — Step 1, 2 의 PR description / reviewer comment 에서 점검 결과만 명시.

**Risk**: 매우 낮 — pre-PR 자체 점검 후 PR 생성.

### PR 정책

- 단일 PR (CLAUDE.local.md §1.4 feature branch → main, squash merge with `--delete-branch`)
- branch 이름: `plan/SPEC-MINK-PRODUCT-V7-001` (CLAUDE.local.md §1.2 feature branch naming)
- PR title: `docs(product): SPEC-MINK-PRODUCT-V7-001 — product.md v7.0 (1인 dev personal ritual companion)`
- PR body: §1 Overview + §4 EARS + §5 AC summary + AC-009/AC-010 scope check 결과
- label: `type/docs` (D 73A 14A 카테고리) + `priority/p1-high` + `area/docs`
- merge: squash + auto-delete branch

---

## 7. Dependencies

### 7.1 선행 결정

- **IDEA-002 brain decision** (2026-05-12, 8 확정사항): 본 SPEC 의 모든 결정의 원천. 별도 프로젝트 brain dir (`/Users/goos/Projects/GooseBot/.moai/brain/IDEA-002/`) 보존.
- **사용자 8 확정사항** (orchestrator-collected, 2026-05-12): scope / metric / brand / persona / language / Wave 3 SPEC roadmap.

### 7.2 본 SPEC 의 downstream

본 SPEC 은 다음 SPEC 들의 parent / vision anchor:

- `SPEC-MINK-BRAND-RENAME-001` (planned v0.1.1, downstream child): brand identifier rename. 본 SPEC merge 후 §1 Background 또는 §References 에서 v7.0 인용.
- `SPEC-MINK-DISTANCING-STATEMENT-001` (proposed Wave 3): distancing detail document. 본 SPEC §Distancing 의 4 단락이 starting point.
- `SPEC-MINK-USERDATA-MIGRATE-001` (downstream Wave 3): `./.goose/` → `./.mink/` 마이그레이션. v7.0 의 local-first / single-user 가정이 base.
- `SPEC-MINK-ENV-MIGRATE-001` (downstream Wave 3): `MINK_*` → `MINK_*` env var. 같은 base.
- Wave 3 신규 ritual SPEC (FORTUNE / HEALTH / CALENDAR / RITUAL / BRIEFING): v7.0 §In-Scope §4 ritual flow 가 use case 정의 근거.

### 7.3 본 SPEC 의 sibling (병행)

- `SPEC-MINK-BRAND-RENAME-001` (planned v0.1.1): brand identifier rename. 본 SPEC 과 sibling — 두 SPEC merge order 무관 (각 SPEC 의 frontmatter 가 cross-reference 만 명시, body 변경 영역 disjoint).

### 7.4 외부 의존

- 없음. 본 SPEC 은 markdown only.

---

## 8. Risks & Mitigations

| # | 리스크 | 가능성 | 영향 | Mitigation |
|---|--------|--------|------|------|
| R1 | v7.0 본문이 너무 야망 축소되어 implementation 방향 보수 화 | 중 | 중 | "anti-goal" 명시는 self-imposed constraint (의도된 효과). 보수 인식 자체는 v7.0 본문의 §3 정당화 (IDEA-002 brain reasoning 인용) 로 해소. |
| R2 | downstream Wave 3 SPEC (FORTUNE/HEALTH/CALENDAR) 가 v7.0 ritual flow 와 misalign | 중 | 중 | v7.0 §4 ritual flow 가 downstream SPEC 의 acceptance criteria 근거. 본 SPEC merge 후 Wave 3 plan 작성 시 v7.0 §4 cross-reference 강제. REQ-MINK-PV7-DOWNSTREAM-001 명시. |
| R3 | v6.0 archive 본문의 license / IP 문제 | 매우 낮 | 낮 | v6.0 본문 자체가 Apache-2.0 (현재 §15). archive 도 동일 license. byte-identical 보존 영향 없음. |
| R4 | "본인 외 1명" 6m metric 이 너무 낮아 product 야망 부재로 인식 | 중 | 낮 | IDEA-002 brain reasoning (research.md §3.4) 을 v7.0 §3 본문에 inline 인용. "낮은 ceiling 은 anti-goal 명시의 직접 표현" 명문화. |
| R5 | "1인 dev" persona 가 너무 좁아 본인 외 1명 자체가 어려움 | 중 | 중 | persona 영역 close peer (Korean dev, similar workflow) 명시. "본인 외 1명" 의 1명은 random user 가 아니라 본인의 close peer. |
| R6 | block/mink distancing 이 legal/trademark 영역으로 확대 | 매우 낮 | 매우 낮 | distancing 은 vision 차원 카테고리 차별화 (multi-tenant agent platform vs personal ritual companion). trademark/legal claim 아님 — 본문 명시. |
| R7 | Hermes / Replika / Routinery 의 사용자 일부가 MINK 와 confusion | 낮 | 낮 | §Distancing 4 단락이 정체성 명시. 외부 marketing 안 하므로 confusion 자체 도달성 낮음. |
| R8 | v7.0 vision 변경 → sibling `.moai/project/*` (tech.md / structure.md / ecosystem.md / migration.md / token-economy.md 등) 의 일관성 손상 | 중 | 중 | 본 SPEC 은 product.md 1 파일만 변경. sibling reconciliation 은 별도 후속 audit (REQ-MINK-PV7-RECONCILE-001 optional). 본 SPEC §1.3 Non-Goals 에 명시. |
| R9 | v6.0 의 8년 vision (Linux Foundation / 1M users 등) 가 archive 에 보존되어 후속 작성자가 잘못 인용 | 낮 | 낮 | archive 파일명 prefix `product-v6.0-` 와 `product-archive/` 디렉토리 자체가 superseded 지표. v7.0 본문이 canonical vision. |

---

## 9. References

### 9.1 본 SPEC 산출물

- `.moai/specs/SPEC-MINK-PRODUCT-V7-001/spec.md` (이 문서, v0.1.0)
- `.moai/specs/SPEC-MINK-PRODUCT-V7-001/research.md` (현황 + decision trail, ~600 lines)
- `.moai/project/product.md` (v7.0 본문, Step 2 산출물)
- `.moai/project/product-archive/product-v6.0-2026-04-27.md` (v6.0 archive, Step 1 산출물)

### 9.2 관련 SPEC

- `.moai/specs/SPEC-MINK-BRAND-RENAME-001/spec.md` v0.1.1 (planned, downstream child of this SPEC). 본 SPEC merge 후 brand rename 의 vision 정당화 인용 가능.
- `.moai/specs/SPEC-MINK-BRAND-RENAME-001/research.md` (planned). brand rename 현황의 implementation evidence.

### 9.3 결정 trail

- **IDEA-002 brain decision** (2026-05-12, brain dir: `/Users/goos/Projects/GooseBot/.moai/brain/IDEA-002/`): 8 확정사항.
- **사용자 8 확정사항** (orchestrator-collected, 2026-05-12):
  1. brand: AI.MINK → MINK (Made IN Korea)
  2. 6m metric: 본인 매일 + 1명, 외부 지표 무관
  3. scope reset: multi-tenant / marketplace / enterprise anti-goal
  4. persona: 1인 dev (Korean, MoAI user)
  5. Wave 3 후속 SPEC: PRODUCT-V7 / BRAND-RENAME / DISTANCING / USERDATA / ENV
  6. language: 한국어 1차
  7. distancing 4종: block/goose, Hermes, Replika, Routinery, OpenClaw
  8. immutable archive: v6.0 byte-identical preservation

### 9.4 본 프로젝트 기존 자료

- `.moai/project/product.md` v6.0 (38665 bytes, 2026-04-22, "Daily Companion Edition"): 본 SPEC 의 archive 대상
- `.moai/project/brand/{visual-identity,brand-voice,target-audience}.md`: 현재 모두 `_TBD_` stub (brand interview 미완, 본 SPEC scope 외)
- `CLAUDE.md` §1 Core Identity (MoAI orchestrator 본질)
- `CLAUDE.local.md` §1 Git Flow (feature branch + squash PR)

### 9.5 외부 참조

- **block/goose**: <https://github.com/block/goose> (multi-tenant agent framework, distancing target)
- **Hermes** (한국 AI 여친 LLM-bot 시장 카테고리)
- **Replika**: <https://replika.ai> (emotional AI companion)
- **Routinery**: <https://routinery.app> (habit tracker)
- **OpenClaw / Together AI**: <https://together.ai> (agent framework)

---

## 10. Exclusions (What NOT to Build)

본 SPEC 은 다음을 **명시적으로 제외**한다. 후속 SPEC 작성자가 본 SPEC 범위 외 작업을 본 PR 에 추가하지 못하도록 차단한다.

1. **sibling `.moai/project/*.md` 파일 정정** (tech.md / structure.md / ecosystem.md / migration.md / adaptation.md / learning-engine.md / token-economy.md / branding.md / brand/* / codemaps/* / research/*) — 별도 후속 audit / SPEC
2. **코드 변경** (Go module path / binary / proto / 식별자 — downstream `SPEC-MINK-BRAND-RENAME-001` 담당)
3. **distancing detail document 본문 작성** — 별도 `SPEC-MINK-DISTANCING-STATEMENT-001` (Wave 3)
4. **Wave 3 신규 SPEC plan 작성** (FORTUNE / HEALTH / CALENDAR / RITUAL / BRIEFING — 각 별도 SPEC)
5. **v6.0 외 다른 historical 문서 archive** (예: ROADMAP.md / IMPLEMENTATION-ORDER.md 등 — 본 SPEC scope 외)
6. **product.md 영문 번역본 작성** — i18n 후순위, 본 SPEC scope 외
7. **시각 자산 / 로고 / website copy / brand interview 완료** — 별도 작업

---

Version: 0.1.0
Status: draft
Last Updated: 2026-05-12
