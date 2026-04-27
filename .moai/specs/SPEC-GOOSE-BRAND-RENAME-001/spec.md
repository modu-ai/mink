---
id: SPEC-GOOSE-BRAND-RENAME-001
version: 0.1.1
status: implemented
created_at: 2026-04-26
updated_at: 2026-04-26
author: manager-spec
priority: P1
issue_number: null
phase: meta
size: 중(M)
lifecycle: spec-anchored
labels: [brand, meta, cross-cutting]
---

# SPEC-GOOSE-BRAND-RENAME-001 — AI.GOOSE 브랜드 통일

## HISTORY

| Version | Date | Author | Change |
|---------|------|--------|--------|
| 0.1.0 | 2026-04-26 | manager-spec | 초안 작성. 사용자 합의 결정사항(Brand Style Guide, Scope IN/OUT, 6 phase 구현 계획) 반영. 12개 AC + 18 EARS 요구사항 정의. |
| 0.1.1 | 2026-04-26 | claude(orchestrator) | plan-auditor v1 결과 반영: CRITICAL 2건(D4-1 SPEC 수 모순, D2-1 orphan REQ 3건) + HIGH 6건(D1-1/2, D2-2, D3-1, D5-1, D6-1, D6-2, D7-1) + MEDIUM 8건 + LOW 4건 일괄 정정. REQ-BR-019 신설(brand-lint CI gate 강제화). AC-BR-001/008/010 매핑 + Then 절 보강. §3.2 item 7 / REQ-BR-009 / AC-BR-011 / Phase 4 정책을 "모든 SPEC HISTORY 행 status 무관 보존"으로 broaden. §7.5 brand-lint 알고리즘 명시 추가. R8 신설 + R3/R4 보강. |

---

## 1. Overview

### 1.1 Scope Clarity

본 SPEC은 **메타 SPEC** (cross-cutting, 횡단 적용)이다. 단일 모듈/도메인의 기능을 정의하지 않으며, 프로젝트 전체의 user-facing 텍스트에서 **공식 브랜드 표기를 `AI.GOOSE`로 통일**한다.

본 SPEC은 다음을 명시한다.
- 공식 브랜드명, 코드 식별자, URL slug 의 분리 규범 (§7 Brand Style Guide)
- Scope IN/OUT 의 명확한 경계 (§3)
- 6-phase 구현 계획 (§6)
- 후속 SPEC 작성자가 표기 규범을 자동 참조할 수 있는 reference 위치 (§7.4)
- brand-lint 검증 알고리즘 명세 (§7.5)

### 1.2 Goal

`goose` 프로젝트의 공식 브랜드명을 `AI.GOOSE`로 통일하되, 코드 식별자(Go module path, package, struct, binary, SPEC ID)와 URL slug는 별도 정책으로 분리한다. 표기 규범을 SPEC + style-guide.md로 명문화하여 후속 SPEC 작성자가 헷갈리지 않게 한다.

### 1.3 Non-Goals

- 코드 식별자(`package goose`, `type Goose*`, `cmd/goose`) 변경 — **절대 금지**
- Go module path 변경 — **절대 금지**
- GitHub repo 이름 변경 (`modu-ai/goose-agent` 그대로)
- 과거 commit message / 종료된 SPEC HISTORY entry / 과거 CHANGELOG entry 변경 — **immutable history 원칙**
- 새 도메인(`ai-goose.dev` 등) 실제 등록 — 본 SPEC은 표기 규범만 수립
- branding.md 등 기존 문서의 본문 재작성 (brand 표기 정정 외의 내용 변경)

---

## 2. Background

### 2.1 현재 상황

프로젝트 전체에서 `goose`, `Goose`, `GOOSE`, `GOOSE-AGENT` 가 혼용되고 있다 (research.md §2 현황 조사 참조). 한 문서 안에서도 표기가 일관되지 않으며, 다음 문제를 일으킨다.

- 외부 노출(README, 문서, 추후 도메인) 시 brand 인식 통일성 부재
- 후속 SPEC 작성자가 매번 표기 결정에 시간 소모
- 새 사용자/기여자가 "이 프로젝트의 공식 이름은 무엇인가" 혼란

### 2.2 왜 지금인가

- v0.1.0 public 전환 전에 brand 표기를 통일해야 외부 노출 시 일관된 첫인상 확보
- SPEC 70여개가 이미 작성되어 일괄 정정 부담이 점점 커지는 시점
- `.moai/project/brand/` 디렉토리가 이미 존재하므로 frozen reference 추가 비용이 낮음

### 2.3 산업 관행

`Next.js` ↔ `next` 패키지, `Mistral AI` ↔ `mistral.ai` 도메인 등 **brand 표기와 식별자 분리는 표준 관행**이다 (research.md §4 참조). 본 SPEC은 이를 따른다.

---

## 3. Scope

### 3.1 IN Scope (정정 대상)

1. **`.moai/project/` 내 user-facing .md 파일**
   - product.md, migration.md, ecosystem.md, tech.md, structure.md, branding.md, learning-engine.md, adaptation.md, token-economy.md
   - brand 디렉토리 내 README.md 및 logo 하위 .md
2. **루트 핵심 문서**
   - README.md
   - CHANGELOG.md (앞으로 작성될 entry section부터. 기존 entry는 immutable)
   - CLAUDE.md (brand 표기 도입 문구 — 인사말, 도입부)
   - CLAUDE.local.md (선택, 사용자 개인 문서이므로 권장 수준)
3. **`.claude/` 하위 user-facing 표현**
   - `.claude/rules/**/*.md`, `.claude/agents/**/*.md`, `.claude/skills/**/*.md`, `.claude/commands/**/*.md` 중 "goose 프로젝트"를 가리키는 산문
   - 영향 추정: 4 파일 (research.md §6 Phase 3)
4. **코드 내 user-facing 문자열**
   - log message (예: `logger.Info("starting goose daemon...")`)
   - error message (예: `errors.New("goose: invalid config")` 의 prefix가 brand 표기로 보일 경우)
   - CLI help text (`cmd.Long`, `cmd.Short`)
   - doc-comment 의 brand 언급
5. **기존 SPEC 98개 본문** (research.md §2.4 grep 기준, §6 Phase 4와 정합)
   - "프로젝트 명칭"으로 goose를 언급한 부분만 선별 정정 (판정 절차는 research.md §3.2의 5단계 분류표를 inline 적용)
   - 결과는 `migration-log.md`에 기록 (변경된 SPEC 목록 + diff 카운트)

### 3.2 OUT Scope (반드시 보존)

[HARD] 다음 항목은 **변경 금지**. 위반 시 build/test/이력 손상 발생.

1. **Go module path**: `github.com/modu-ai/goose` 그대로
2. **Go package 이름**: `package goose`, `package goosed`, `package goosecli` 등
3. **Type/struct/function/variable 이름**: `type Goose*`, `func GooseRun()`, `var GooseConfig` 등 코드 식별자 일체
4. **CLI binary 이름**: `goose`, `goosed`, `goose-cli` (Makefile, 빌드 산출물, 설치 스크립트)
5. **SPEC ID 네이밍 규약**: `SPEC-GOOSE-XXX-NNN` — 기존 SPEC 디렉토리 이름 변경 금지
6. **Git remote / GitHub repo**: `modu-ai/goose-agent`
7. **Immutable 이력**:
   - 과거 CHANGELOG entry (이미 발행된 release section)
   - 과거 git commit message
   - **모든** 기존 SPEC `## HISTORY` 표 entries — status(planned/draft/implemented/closed) 무관, brand 정정 대상에서 제외 (이유: HISTORY는 chronological record이므로 retroactive 수정 시 기록이 왜곡됨)
8. **proto package / message 이름**: `goose.v1` 등
9. **도메인 용어 인용 형태**: 백틱(`)으로 감싼 `goose CLI`, `goosed daemon`, `goose agent loop` 등 — research.md §3 분류 (D) 도메인 용어로 보존

---

## 4. EARS Requirements

본 SPEC은 19개 EARS 요구사항을 정의한다. 각 요구사항은 spec-anchored이며 §5 AC와 1:1 또는 N:1 매핑된다.

### 4.1 Ubiquitous (보편 — 항상 성립)

- **REQ-BR-001 [Ubiquitous]** The project **shall** use `AI.GOOSE` as the official brand identifier in all user-facing prose.
- **REQ-BR-002 [Ubiquitous]** The project **shall** use `goose` (lowercase) as the canonical short identifier in code identifiers, package paths, binary names, and domain terms (`goose CLI`, `goosed daemon`).
- **REQ-BR-003 [Ubiquitous]** The project **shall** use `ai-goose` (kebab-case) for URL slugs, future domain names, and search-friendly identifiers.
- **REQ-BR-004 [Ubiquitous]** The Brand Style Guide **shall** be persisted at `.moai/project/brand/style-guide.md` as a frozen reference document for all subsequent SPEC authors.

### 4.2 Event-Driven (트리거 발생 시)

- **REQ-BR-005 [Event-Driven]** **When** a SPEC author creates a new SPEC document, the SPEC template **shall** include a reference link to `.moai/project/brand/style-guide.md`.
- **REQ-BR-006 [Event-Driven]** **When** the brand-lint script runs against a user-facing string (log, error, CLI help, doc-comment) that refers to the project as a brand and does not use `AI.GOOSE`, the script **shall** flag the line as a brand violation (covers `GOOSE`, `Goose`, `GOOSE-AGENT` brand-position usage outside backticks).
- **REQ-BR-007 [Event-Driven]** **When** a new CHANGELOG entry is added, the entry **shall** use `AI.GOOSE` for brand references; **prior** CHANGELOG entries are immutable and **shall not** be edited.
- **REQ-BR-008 [Event-Driven]** **When** the brand-lint script runs, it **shall** report violations as a non-zero exit code with file path, line number, and offending pattern.

### 4.3 State-Driven (상태 조건)

- **REQ-BR-009 [State-Driven]** **While** any `.moai/specs/SPEC-GOOSE-XXX-NNN/` directory exists (regardless of `status`: planned/draft/implemented/closed), the SPEC's `## HISTORY` table entries **shall not** be modified for brand normalization.
- **REQ-BR-010 [State-Driven]** **While** a string is enclosed in backticks (\`...\`) or fenced code blocks, the brand-lint script **shall** treat the content as code identifier or domain term and skip brand-naming validation.
- **REQ-BR-011 [State-Driven]** **While** a SPEC author is creating or saving a new SPEC document, the SPEC template **shall** present the style-guide reference link (`.moai/project/brand/style-guide.md`) at the top of the template before persistence completes.

### 4.4 Unwanted (금지 행동)

- **REQ-BR-012 [Unwanted]** **If** the brand-lint script detects `Goose 프로젝트`, `GOOSE-AGENT` (used as brand outside backticks), or `goose project` (English brand without backticks), **then** the script **shall** fail with exit code 1.
- **REQ-BR-013 [Unwanted]** **If** any commit or PR attempts to change `github.com/modu-ai/goose` (Go module path), `package goose` (Go package), or any `Goose*` type identifier, **then** CI **shall** fail with a brand-rule violation message and **shall** block merge. (Code review rejection is a complementary process note, not a system behavior.)
- **REQ-BR-014 [Unwanted]** **If** any commit attempts to alter a SPEC directory name from `SPEC-GOOSE-XXX-NNN` to `SPEC-AI-GOOSE-XXX-NNN`, **then** the brand-lint CI gate **shall** reject the change. SPEC ID naming is OUT of scope (§3.2 item 5).
- **REQ-BR-015 [Unwanted]** **If** any commit attempts to edit any existing SPEC's HISTORY entry (regardless of status) or a published CHANGELOG entry for brand normalization, **then** the brand-lint CI gate **shall** reject the change on immutable-history grounds.

### 4.5 Optional (해당 시)

- **REQ-BR-016 [Optional]** **Where** a future domain is registered for the project, the domain slug **shall** use `ai-goose` (e.g., `ai-goose.dev`, `ai-goose.io`).
- **REQ-BR-017 [Optional]** **Where** Korean and English mixed-language prose appears, the Brand Style Guide **shall** define at least 4 example pairs covering both `AI.GOOSE 프로젝트` (Korean) and `the AI.GOOSE project` (English) usage.
- **REQ-BR-018 [Optional]** **Where** a developer's local environment is configured with pre-commit hooks, the brand-lint script **shall** be wired as a pre-commit hook so that `git commit` fails before the commit object is created on a brand violation.

### 4.6 Mandatory CI Gate (강제 검증)

- **REQ-BR-019 [Event-Driven]** **When** a Pull Request is opened or updated against `main`, the GitHub Actions workflow `.github/workflows/brand-lint.yml` **shall** run `scripts/check-brand.sh` and **shall** block merge on non-zero exit code. (This requirement is mandatory and supersedes REQ-BR-018's optional pre-commit wiring.)

---

## 5. Acceptance Criteria

각 AC는 Given / When / Then 형식. 측정 가능한 검증 기준으로 작성.

### AC-BR-001 — Brand Style Guide 문서 존재

**Given** SPEC-GOOSE-BRAND-RENAME-001 구현이 완료된 상태에서
**When** `.moai/project/brand/style-guide.md` 파일을 확인하면
**Then** 파일이 존재하며 다음이 모두 명문화되어 있다.
- 공식 브랜드: `AI.GOOSE` (REQ-BR-001)
- 코드 식별자/약칭: `goose` 식별자 규칙 (REQ-BR-002)
- URL slug: `ai-goose` 케밥 규칙 + 미래 도메인 예시 (`ai-goose.dev`) (REQ-BR-003, REQ-BR-016)
- 한국어/영어 예시 4쌍 이상 (REQ-BR-017): `AI.GOOSE 프로젝트` ↔ `the AI.GOOSE project`, `AI.GOOSE는 ...입니다.` ↔ `AI.GOOSE is ...`, `` `goose CLI` 명령어 `` ↔ `` the `goose CLI` command ``, `Welcome to AI.GOOSE` (한/영 동일)

REQ 매핑: REQ-BR-001, REQ-BR-002, REQ-BR-003, REQ-BR-004, REQ-BR-016, REQ-BR-017

### AC-BR-002 — README/CHANGELOG/CLAUDE.md brand 통일

**Given** Phase 2 정정이 완료된 상태에서
**When** README.md, CHANGELOG.md(앞으로 작성될 entry section만), CLAUDE.md 를 검사하면
**Then** brand 표기로 사용된 모든 위치에 `AI.GOOSE`가 사용되며, `GOOSE-AGENT` 또는 `Goose 프로젝트` 형태의 brand 표기가 0건이다 (백틱 인용 형태 제외).

REQ 매핑: REQ-BR-001, REQ-BR-007

### AC-BR-003 — `.moai/project/` user-facing brand naming 위반 0건

**Given** Phase 2 정정이 완료된 상태에서
**When** `.moai/project/` 하위 user-facing .md 파일에 대해 `scripts/check-brand.sh`를 실행하면 (검증 알고리즘은 §7.5 참조: 마크다운 inline code 파서 또는 PCRE2 negative lookbehind 기반으로 백틱 외부 위반 패턴만 검출)
- 검출 대상 패턴: `goose 프로젝트`, `Goose 프로젝트`, `GOOSE 프로젝트`, `goose project`, `Goose project`
- 인용 제외: `` `goose` ``, `` `goosed` ``, `` `goose CLI` ``, fenced code block 내부, inline code span 내부
**Then** 위반 건수가 0이고 exit code가 0이다.

REQ 매핑: REQ-BR-001, REQ-BR-010

### AC-BR-004 — `.claude/` 하위 brand naming 위반 0건

**Given** Phase 3 정정이 완료된 상태에서
**When** `.claude/rules/`, `.claude/agents/`, `.claude/skills/`, `.claude/commands/` 하위 .md 파일을 검사하면
**Then** brand 표기 위반 0건. (예상 정정 대상은 4 파일, research.md §6 Phase 3 참조)

REQ 매핑: REQ-BR-001

### AC-BR-005 — 기존 SPEC 본문 정정 결과 기록

**Given** Phase 4 정정이 완료된 상태에서
**When** `.moai/specs/SPEC-GOOSE-BRAND-RENAME-001/migration-log.md` 파일을 확인하면
**Then** 다음을 모두 포함한다 (binary 검증):
- 변경된 SPEC 디렉토리 목록 (예: `SPEC-GOOSE-CLI-001/spec.md`, ...)
- 각 SPEC 별 diff 카운트 (변경된 line 수)
- spot-check QA: 변경 SPEC 중 무작위 5건(또는 전체 변경 SPEC의 20% 중 큰 값; 변경 SPEC이 5건 미만이면 전수)을 추출하여 §7.5 brand-lint 알고리즘 분류(A) 정정 판정과 100% 일치함을 확인 (= 5건 중 5건 일치, binary)

REQ 매핑: REQ-BR-009 (HISTORY 보존 검증 포함)

### AC-BR-006 — 코드 내 user-facing 문자열 brand 통일

**Given** Phase 5 정정이 완료된 상태에서
**When** 코드의 user-facing 출력(log, error, CLI help, doc-comment)을 테스트로 검증하면
**Then** brand 표기가 사용된 출력에서 `AI.GOOSE`가 사용된다.
- 예: `goosed --help` 출력 첫 줄이 `AI.GOOSE daemon — ...` 형태 (백틱 또는 코드 식별자 위치는 `goose`/`goosed` 그대로)

REQ 매핑: REQ-BR-006

### AC-BR-007 — Go module path 미변경 검증

**Given** Phase 1 시작 시점에 `go list -m` 출력이 `migration-log.md`의 `## Baseline` 섹션에 캡처된 상태에서
**When** Phase 6 검증 단계에서 동일 명령(`go list -m` 또는 `head -1 go.mod`)을 재실행하면
**Then** 출력이 정확히 `github.com/modu-ai/goose` 이고 baseline과 byte-level 일치한다.

REQ 매핑: REQ-BR-013

### AC-BR-008 — Go package/struct/binary 식별자 미변경 검증

**Given** Phase 1 시작 시점에 다음 명령 출력이 `migration-log.md`의 `## Baseline` 섹션에 캡처된 상태에서
- `grep -rh "^package goose" --include="*.go"` 카운트
- `grep -rh "^type Goose" --include="*.go"` 카운트
- `ls cmd/` 디렉토리 출력
**When** Phase 5 시작 시점 및 Phase 6 검증 단계에서 동일 명령을 재실행하고 baseline과 비교하면
**Then** 모든 카운트와 디렉토리 목록이 baseline과 일치한다 (변경 0건). `goose` 식별자 규칙(REQ-BR-002) 준수가 baseline 비교로 검증된다.

REQ 매핑: REQ-BR-002, REQ-BR-013

### AC-BR-009 — SPEC ID 네이밍 미변경

**Given** Phase 1 시작 시점에 `ls .moai/specs/ | grep "^SPEC-"` 출력이 `migration-log.md`의 `## Baseline` 섹션에 캡처된 상태에서
**When** Phase 4 정정 후 및 Phase 6 검증 단계에서 동일 명령을 재실행하고 baseline과 비교하면
**Then** SPEC 디렉토리 이름 목록이 baseline과 정확히 일치한다. `SPEC-AI-GOOSE-*` 형태의 새 디렉토리 0건, 기존 `SPEC-GOOSE-*` 디렉토리 이름 변경 0건.

REQ 매핑: REQ-BR-014

### AC-BR-010 — `make brand-lint` 통과 + GitHub Actions 강제 gate

**Given** Phase 6 검증 도구가 신설된 상태에서
**When** 다음을 모두 검증하면
1. `make brand-lint` 또는 `scripts/check-brand.sh` 실행 → exit code 0, 위반 0건
2. `.github/workflows/brand-lint.yml` 워크플로우 파일이 존재하고 PR trigger(`on: pull_request`)에 등록됨
3. 의도적으로 brand 위반(`Goose 프로젝트` 추가)을 도입한 시험 PR이 brand-lint check 실패로 merge가 차단됨
**Then** 세 조건 모두 만족한다 (binary). 단, REQ-BR-018(pre-commit hook wiring)은 [Optional]이므로 wiring 후 `git commit` 시 brand 위반에 한해 차단됨을 별도 시험으로 확인하면 통과로 인정한다.

REQ 매핑: REQ-BR-008, REQ-BR-012, REQ-BR-018, REQ-BR-019

### AC-BR-011 — Immutable history 보존

**Given** Phase 1 시작 시점에 다음 baseline이 `migration-log.md`의 `## Baseline` 섹션에 캡처된 상태에서
- 과거 CHANGELOG entry section (이미 발행된 release): byte-level snapshot
- **모든** 기존 SPEC `## HISTORY` 표 entries (status 무관: planned/draft/implemented/closed): byte-level snapshot
- 과거 git commit message: `git log --oneline` 출력 snapshot
**When** Phase 4/5 정정 후 동일 항목을 다시 추출하여 baseline과 비교하면
**Then** 모든 비교에서 변경 0건이다 (byte-level 일치).

REQ 매핑: REQ-BR-009, REQ-BR-015

### AC-BR-012 — SPEC template에 style-guide 자동 참조 link 추가

**Given** Phase 1 표기 규범 commit이 완료된 상태에서
**When** 새 SPEC을 작성하기 위한 template 또는 manager-spec agent의 동작을 확인하면
**Then** template/agent 출력에 `.moai/project/brand/style-guide.md` 참조 링크가 자동 포함된다.

REQ 매핑: REQ-BR-005

---

## 6. Technical Approach (Implementation Phasing)

본 SPEC은 6 phase로 나누어 구현한다. 각 phase는 별도 commit으로 분리하되 단일 PR(squash merge)에 포함한다.

### Phase 1 — 표기 규범 commit

- 본 SPEC 디렉토리 생성: `.moai/specs/SPEC-GOOSE-BRAND-RENAME-001/{spec.md, research.md}`
- `.moai/project/brand/style-guide.md` 신설 (frozen reference, §7 Brand Style Guide의 표를 박제)
- `migration-log.md`의 `## Baseline` 섹션에 baseline 캡처 (`go list -m`, package/type grep counts, `ls cmd/`, `ls .moai/specs/`, CHANGELOG snapshot, 모든 SPEC HISTORY snapshot, `git log --oneline` snapshot)
- commit: `docs(brand): SPEC-GOOSE-BRAND-RENAME-001 v0.1.0 초안 + style-guide.md 신설 + baseline 캡처`

### Phase 2 — 핵심 다큐먼트 일괄 정정

- README.md, CHANGELOG.md (header 추가 entry부터), CLAUDE.md
- `.moai/project/*.md` (product, tech, structure, branding, learning-engine, migration, ecosystem, adaptation, token-economy)
- `.moai/project/brand/README.md`, `.moai/project/brand/logo/*.md`
- 정정 도구: 수동 Edit (자동 sed 금지 — 도메인 용어 보존을 위해)
- commit: `docs(brand): SPEC-GOOSE-BRAND-RENAME-001 — Phase 2 핵심 문서 brand 통일`

### Phase 3 — Claude rules / agents / skills / commands

- 영향 파일 (research.md §6 Phase 3 참조): 4 파일, 6건
- 도메인 용어가 대부분이므로 정정 대상은 1~2건 수준 추정
- commit: `docs(brand): SPEC-GOOSE-BRAND-RENAME-001 — Phase 3 .claude/ user-facing 정정`

### Phase 4 — 기존 SPEC 본문 선별 정정

- 98개 SPEC 마크다운 중 brand 표기로 goose를 언급한 부분만 선별 (research.md §2.4 grep 카운트 기준)
- SPEC ID, **모든** SPEC HISTORY 표 entries (status 무관: planned/draft/implemented/closed), 코드 인용은 보존
- 결과를 `.moai/specs/SPEC-GOOSE-BRAND-RENAME-001/migration-log.md`에 기록
- spot-check QA: 변경 SPEC 중 무작위 5건(또는 전체 변경 SPEC의 20% 중 큰 값; 5건 미만이면 전수)을 §7.5 알고리즘 분류(A) 정정과 100% 일치 검증
- commit: `docs(brand): SPEC-GOOSE-BRAND-RENAME-001 — Phase 4 SPEC 본문 brand 정정 (migration-log 포함)`

### Phase 5 — 코드 내 user-facing 문자열

- log/error message, CLI help text, doc-comment 의 brand 언급
- 사전 baseline grep: `grep -rn "goose" --include="*.go" | grep -vE "package |type |func |var |import |//.*github.com"` 등
- 식별자(`type Goose*`, `package goose`, `cmd/goose`)는 절대 변경 금지 — AC-BR-008로 검증
- 분류 정책: 런타임 plaintext 출력의 "goose daemon" / "goose CLI" 등 brand 위치는 `AI.GOOSE daemon` / `AI.GOOSE CLI`로 정정. 단, 백틱 인용 코드 식별자(`` `goosed` ``, `` `goose` ``)는 보존 (§7.5 알고리즘 + research.md §3 분류 (A) vs (D) 참조)
- commit: `docs(brand): SPEC-GOOSE-BRAND-RENAME-001 — Phase 5 코드 내 user-facing 문자열 정정` (commit type을 `docs(brand)`로 통일: user-facing 문자열은 동작 변경 없이 표기만 정정하므로 `feat`가 아닌 `docs`로 분류. Phase 6의 `chore(brand)`와 더불어 §6 전체에서 `feat` 사용 0건 유지)

### Phase 6 — 검증 도구 + CI gate

- `scripts/check-brand.sh` 또는 `make brand-lint` target 신설
- 위반 패턴 grep: `Goose 프로젝트`, `GOOSE-AGENT` (백틱 외부), `goose project` (백틱 외부)
- AC-BR-007/008/009 baseline 비교 자동화 (선택)
- (선택) pre-commit hook wiring
- commit: `chore(brand): SPEC-GOOSE-BRAND-RENAME-001 — Phase 6 brand-lint 검증 도구 추가`

### Phase 의존성 및 PR 분량 정책

- Phase 1은 모든 phase의 선행 작업 (style-guide.md가 다른 phase의 reference로 작동, baseline도 Phase 1에서 캡처)
- Phase 2~5는 상호 독립적이므로 병렬 작업 가능 (다만 squash merge 시 순서 정렬 권장)
- Phase 6은 Phase 2~5 검증을 위해 마지막 수행
- **PR 분량 정책**: 기본은 단일 PR + multi-commit 구조이며 squash merge(CLAUDE.local.md §1.4) 시 모든 phase commit이 하나로 통합된다. 단, Phase 2(약 14파일) 또는 Phase 4(98 SPEC 분량)가 단일 squash로 review에 부담을 줄 만큼 비대해지면 해당 phase를 별도 squash PR로 분리할 수 있다. 분리 시 base branch는 `feature/SPEC-GOOSE-BRAND-RENAME-001-spec`을 그대로 사용하고, PR description에 split 사유와 후속 phase의 진행 순서를 명시한다.

---

## 7. Brand Style Guide (frozen reference 후보)

본 섹션은 `.moai/project/brand/style-guide.md`로 추출되어 frozen reference로 박제될 표기 규범이다.

### 7.1 표기 규범 (3 영역)

| 컨텍스트 | 표기 | 적용 영역 | 예시 |
|---------|------|----------|------|
| 공식 브랜드명 / user-facing prose | `AI.GOOSE` | 모든 문서, README 제목, CHANGELOG 신규 entry, CLAUDE.md 인사말, CLI 환영 메시지, 에러 메시지 도입부 | "AI.GOOSE는 Daily Companion AI입니다." |
| 짧은 약칭 / 코드 식별자 / 도메인 용어 | `goose` | `goose CLI`, `goosed daemon`, `goose agent loop`, type/func/var 이름 (Go 식별자) | `package goose`, `type GooseRuntime` |
| URL slug / GitHub repo / 도메인 | `ai-goose` | 미래 도메인(`ai-goose.dev` 등), URL slug, 검색 친화 표기 | `ai-goose.dev`, `https://docs.ai-goose.dev/` |

### 7.2 Dual Representation 원칙

산문에서 brand로 가리킬 때는 `AI.GOOSE`, 코드/식별자로 가리킬 때는 `goose`를 사용한다. 백틱(`)으로 감싼 식별자는 brand-naming 규칙에서 제외된다.

예:
- 좋은 예: `` AI.GOOSE는 `goose CLI`로 실행됩니다. ``
- 나쁜 예: `` Goose는 goose CLI로 실행됩니다. `` (brand 표기 잘못, 식별자 백틱 누락)

### 7.3 한/영 표기 예시 (i18n 일반 정책 포함)

| 한국어 | 영어 |
|--------|------|
| `AI.GOOSE 프로젝트` | `the AI.GOOSE project` |
| `AI.GOOSE는 ...입니다.` | `AI.GOOSE is ...` |
| `` `goose CLI` 명령어 `` | `` the `goose CLI` command `` |
| `Welcome to AI.GOOSE` | `Welcome to AI.GOOSE` |

i18n 일반 정책: 다른 언어(일본어, 중국어 등)가 추후 도입되더라도 동일 dual representation 원칙(brand=`AI.GOOSE` / 식별자=`goose` / slug=`ai-goose`)을 그대로 적용한다. 신규 언어 추가 자체는 본 SPEC OUT scope이며, 별도 SPEC에서 처리한다.

### 7.4 후속 SPEC 작성자 참조

후속 SPEC을 작성하는 manager-spec agent / 인간 작성자는 다음을 의무 준수한다.

- SPEC template 또는 plan 단계에서 `.moai/project/brand/style-guide.md` 참조 링크 포함
- brand 표기 시 `AI.GOOSE` 사용 (REQ-BR-001)
- 코드 식별자/도메인 용어는 `goose` (REQ-BR-002)
- URL/도메인 slug는 `ai-goose` (REQ-BR-003)

### 7.5 brand-lint 검증 알고리즘 (`scripts/check-brand.sh` 동작 명세)

`scripts/check-brand.sh` 및 GitHub Actions workflow `brand-lint.yml`는 다음 알고리즘으로 동작한다 (AC-BR-003 / AC-BR-010 검증의 근거).

1. **입력**: 검사 대상 .md 파일 목록 (default: `.moai/project/`, `.moai/specs/`, `.claude/`, `README.md`, `CHANGELOG.md`, `CLAUDE.md`)
2. **마크다운 파싱**: 각 파일을 마크다운 inline code 파서(예: Python의 `markdown-it-py` 또는 Go의 `goldmark`)로 파싱하여 inline code span 및 fenced code block 영역의 byte offset을 식별
3. **위반 검출**: 코드 영역 외부에서 다음 패턴을 검출 (PCRE2 negative lookbehind 대안 사용 시 `(?<!\`)goose 프로젝트(?!\`)` 형태)
   - `goose 프로젝트`, `Goose 프로젝트`, `GOOSE 프로젝트`
   - `goose project`, `Goose project`
   - `GOOSE-AGENT` (백틱 외부, brand 위치)
4. **분류 (A) vs (D) 판정**: 코드 영역 내부 `` `goose CLI` ``, `` `goosed daemon` ``, `` `goose agent loop` `` 등은 도메인 용어(D)로 보존하고 위반에서 제외
5. **HISTORY 보존 검증**: 모든 SPEC의 `## HISTORY` 섹션 (status 무관) 안에서 brand 패턴 변경이 발생했는지 baseline과 byte-level diff. 변경 발견 시 즉시 fail
6. **출력**: 위반이 0건이면 exit 0, 위반이 있으면 exit 1과 함께 file path/line number/offending pattern 보고
7. **언어 선택**: Python 스크립트 우선 (마크다운 파서 활용 용이). bash + ripgrep PCRE2 (`rg -P --pcre2`) 대안도 허용하되 lookbehind 한계로 fenced code block 처리는 제한적 → 가능하면 Python 우선

GitHub Actions workflow는 위 스크립트를 PR trigger에서 실행하며, exit 1 시 `merge_blocking` 상태로 등록되어 PR merge가 차단된다 (REQ-BR-019 강제화).

---

## 8. Dependencies

### 8.1 선행 SPEC

- **없음**. 본 SPEC은 메타 SPEC으로 모든 SPEC과 횡단 관계.

### 8.2 외부 의존

- **없음**. 외부 라이브러리/도메인 등록 등 의존 없음.

### 8.3 Tooling

- `grep` (또는 `ripgrep`) — 검증 스크립트용
- `make` — `make brand-lint` target 등록 시
- shell script (`bash` 또는 `sh`)

### 8.4 후속 영향 SPEC

- 향후 작성될 모든 SPEC이 본 SPEC의 §7 Brand Style Guide를 참조해야 함.
- 영향 강도: medium (브랜드 표기 한 줄 추가, 비즈니스 로직 영향 없음).

---

## 9. Risks & Mitigations

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|--------|--------|------|------|
| R1 | 코드 식별자 잘못 변경 (Go module path / package / type 손상) | 낮 | 매우 높 | AC-BR-007/008/009 baseline 비교 자동화. CI gate에서 차단. Phase 5 작업 시 수동 Edit만 허용, 자동 sed 금지. |
| R2 | 도메인 용어("goose agent loop")의 brand 오인식 | 중 | 중 | Brand Style Guide §7.2 Dual Representation 원칙 명시. 정정 시 인간 리뷰 필수. brand-lint는 백틱 인용을 자동 제외 (REQ-BR-010). |
| R3 | 98개 SPEC 본문 일괄 정정 시 누락/오변경 | 중 | 중 | migration-log.md로 추적 (AC-BR-005). spot-check QA 표본 크기 정당화: 변경 SPEC의 20% 또는 5건 중 큰 값(전체 변경 SPEC이 5건 미만이면 전수). 분류(A) 정정 100% 일치 binary 확인. |
| R4 | 진행 중인 모든 active feature/release branch에 대한 rebase 필요 (PR #24 등 merge 시 base 갱신) | 중 | 낮 | feature/SPEC-GOOSE-BRAND-RENAME-001-spec branch는 main 기반. 구현 시작 전 `git branch --list 'feature/*' 'release/*'`로 active branch 목록을 추출하고, brand 정정과 충돌 가능성이 있는 branch는 owner에게 main rebase 또는 본 SPEC merge 후 일괄 rebase 일정을 공지. PR #24는 1차 시범 사례. |
| R5 | i18n / 번역 본문(특히 한국어/영어 혼용)에서 일관성 균열 | 중 | 낮 | Brand Style Guide §7.3 한/영 표기 예시 section 추가 + i18n 일반 정책 명시. |
| R6 | brand-lint script가 false positive를 만들어 개발자 피로도 증가 | 중 | 낮 | 백틱 인용 자동 제외 (REQ-BR-010). 검증 패턴은 명백한 brand 위치(`Goose 프로젝트`, `GOOSE-AGENT` 외부)에 한정. §7.5 알고리즘에서 마크다운 파서 기반 정확한 코드 영역 식별. |
| R7 | SPEC ID `SPEC-GOOSE-XXX-NNN` 잔존이 brand 통일 대비 어색해 보임 | 낮 | 낮 | OUT scope (§3.2 item 5)로 명시. 후속 별도 논의 (v0.2.0+ 검토). |
| R8 | 표기 규범이 §7 Brand Style Guide(SPEC 본문)와 `.moai/project/brand/style-guide.md`(frozen reference) 두 곳에 존재하여 drift 발생 가능 | 중 | 중 | Phase 1에서 `.moai/project/brand/style-guide.md`를 §7.1~§7.5 표/내용의 캐노니컬 사본으로 박제. 본 SPEC §7은 "frozen reference 후보"임을 명시 (§7 도입부 참조). 후속 변경 시 양쪽을 동시 수정하는 PR 정책을 §10.1에 명시 + brand-lint workflow가 두 파일의 핵심 표(Section 7.1 표) byte-level 동일성을 추가 검증하는 옵션 검토. |

---

## 10. References

### 10.1 본 SPEC 산출물

- `.moai/specs/SPEC-GOOSE-BRAND-RENAME-001/spec.md` (이 문서, v0.1.1)
- `.moai/specs/SPEC-GOOSE-BRAND-RENAME-001/research.md` (현황 조사 + 의사결정 근거)
- `.moai/specs/SPEC-GOOSE-BRAND-RENAME-001/migration-log.md` (Phase 1 baseline + Phase 4 결과 기록, 구현 시점에 신설)
- `.moai/project/brand/style-guide.md` (frozen reference, §7 캐노니컬 사본). **Dual-source drift 방지 정책**: §7과 `style-guide.md` 중 하나가 변경되면 같은 PR 안에서 다른 쪽도 동시 수정해야 하며, brand-lint workflow가 §7.1 표의 byte-level 동일성을 검증한다 (R8).

### 10.2 산업 사례 (research.md §4 참조)

- Next.js (`Next.js` brand / `next` 패키지 / `vercel/next.js` repo)
- Mistral AI (`Mistral AI` brand / `mistral.ai` 도메인 / `mistralai/*` repo)
- Tailwind CSS (`Tailwind CSS` brand / `tailwindcss` 패키지)

### 10.3 본 프로젝트 기존 자료

- `.moai/project/branding.md` — 기존 branding 문서 (본 SPEC 적용 대상)
- `.moai/project/brand/README.md`, `.moai/project/brand/visual-identity.md`, `.moai/project/brand/brand-voice.md` — brand 자산 디렉토리

### 10.4 결정 trail

- 사용자 합의 결정사항: orchestrator가 위임한 prompt (2026-04-26) — Brand Style Guide 표 + Scope IN/OUT + Phase 분할 모두 합의 완료
- AskUserQuestion 호출 없음 (subagent 제약, 결정사항이 모두 prompt에 명시됨)

---

## 11. Exclusions (What NOT to Build)

본 SPEC은 다음을 **명시적으로 제외**한다. 후속 SPEC 작성자가 본 SPEC 범위 외 작업을 본 SPEC에 추가하지 못하도록 차단한다.

1. **Go module path 변경** — `github.com/modu-ai/goose` 그대로 (R1, AC-BR-007)
2. **Go package/type/func/var 식별자 변경** — `package goose`, `type Goose*` 그대로 (R1, AC-BR-008)
3. **CLI binary 이름 변경** — `goose`, `goosed`, `goose-cli` 그대로
4. **SPEC ID 네이밍 변경** — `SPEC-GOOSE-XXX-NNN` 그대로 (R7, AC-BR-009)
5. **GitHub repo 이름 변경** — `modu-ai/goose-agent` 그대로
6. **proto package 변경** — `goose.v1` 그대로
7. **과거 commit message / 종료 SPEC HISTORY / 발행된 CHANGELOG entry 변경** — immutable history 원칙 (AC-BR-011)
8. **새 도메인 실제 등록** — 본 SPEC은 표기 규범만 수립, `ai-goose.dev` 등 도메인 구매·등록은 별도 작업
9. **branding.md 본문 재작성** — brand 표기 정정만, 내용 변경 금지
10. **i18n 번역 추가** — 한/영 표기 예시는 Brand Style Guide §7.3에 한정, 신규 언어 지원은 OUT
11. **logo / visual-identity 변경** — 표기(text) 통일에 한정, 시각 자산은 OUT (별도 SPEC)
12. **config 파일의 `goose` 키 변경** — `.moai/config/sections/*.yaml` 등 식별자 위치는 OUT (R1)

---

Version: 0.1.1
Status: planned
Last Updated: 2026-04-26
