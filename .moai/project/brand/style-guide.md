---
id: brand-style-guide-mink
version: 2.0.0
status: frozen
created_at: 2026-05-12
spec: SPEC-MINK-BRAND-RENAME-001
supersedes: SPEC-GOOSE-BRAND-RENAME-001 (style-guide v1.0.0)
classification: FROZEN_REFERENCE
---

# MINK Brand Style Guide

> **FROZEN REFERENCE** — 이 문서는 SPEC-MINK-BRAND-RENAME-001 §7 (spec.md v0.1.1)의 캐노니컬 사본이다.
> 변경 시 spec.md §7 과 이 파일을 같은 PR 안에서 동시 수정해야 하며, brand-lint workflow 가 §1 표의 byte-level 동일성을 검증한다 (선행 SPEC R8 의 dual-source drift 방지 정책 계승).
> 후속 SPEC 작성자 및 manager-spec agent 는 brand 표기 결정 시 반드시 이 문서를 참조한다.

---

## 1. 표기 규범 (3 영역)

| 컨텍스트 | 표기 | 적용 영역 | 예시 |
|---------|------|----------|------|
| 공식 브랜드명 / user-facing prose | `MINK` | 모든 문서, README 제목, CHANGELOG 신규 entry, CLAUDE.md 인사말, CLI 환영 메시지, 에러 메시지 도입부 | "MINK는 너의 매일을 기억하는 AI다." |
| 짧은 약칭 / 코드 식별자 / 도메인 용어 | `mink` | `mink CLI`, `minkd daemon`, type/func/var 이름 (Go 식별자) | `package mink`, `type MinkRuntime`, `cmd/mink` |
| URL slug / GitHub repo / 도메인 | `mink` | 미래 도메인 (`mink.dev` 등), URL slug | `mink.dev`, `https://docs.mink.dev/`, `github.com/modu-ai/mink` |

> 주의: 선행 SPEC 의 dual-representation 은 brand (대문자 + 점) / 식별자 (소문자) / slug (kebab) 3-way 분리였으나, MINK 는 brand=`MINK` / 식별자=`mink` / slug=`mink` 의 **2-way 분리** (대소문자만 차이). 표기 결정이 단순해진다.

---

## 2. Dual Representation 원칙

산문에서 brand 로 가리킬 때는 `MINK`, 코드/식별자로 가리킬 때는 `mink` 를 사용한다. 백틱 (`)으로 감싼 식별자는 brand-naming 규칙에서 제외된다.

예:
- 좋은 예: `` MINK는 `mink CLI`로 실행됩니다. ``
- 나쁜 예: `` Mink는 mink CLI로 실행됩니다. `` (brand 표기 잘못 — Title case Mink 는 brand-position 에서 금지, 식별자 백틱 누락)

---

## 3. 한국어 / 영어 예시 (i18n 일반 정책 포함)

| 한국어 | 영어 |
|--------|------|
| `MINK 프로젝트` | `the MINK project` |
| `MINK는 ...입니다.` | `MINK is ...` |
| `` `mink CLI` 명령어 `` | `` the `mink CLI` command `` |
| `Welcome to MINK` | `Welcome to MINK` |
| `매일 아침, 매일 저녁, 너의 MINK.` | `Your AI that says good morning, every morning.` |

i18n 일반 정책: 다른 언어 (일본어, 중국어, 영어 외)가 추후 도입되더라도 동일 dual representation 원칙 (brand=`MINK` / 식별자=`mink` / slug=`mink`)을 그대로 적용한다. 신규 언어 추가 자체는 본 SPEC OUT scope.

---

## 4. 후속 SPEC 작성자 참조

후속 SPEC 을 작성하는 manager-spec agent / 인간 작성자는 다음을 의무 준수한다:

- SPEC template 또는 plan 단계에서 `.moai/project/brand/style-guide.md` 참조 링크 포함
- brand 표기 시 `MINK` 사용 (REQ-MINK-BR-001)
- 코드 식별자 / 도메인 용어는 `mink` (REQ-MINK-BR-002)
- URL / 도메인 slug 는 `mink` (REQ-MINK-BR-025)
- 신규 SPEC 디렉토리 이름은 `SPEC-MINK-XXX-NNN` (REQ-MINK-BR-005)
- 기존 `SPEC-GOOSE-XXX-NNN` 디렉토리에 cross-reference 시 그대로 (immutable archive)

---

## 5. brand-lint 검증 알고리즘 (`scripts/check-brand.sh` 동작 명세)

`scripts/check-brand.sh` 및 GitHub Actions workflow `brand-lint.yml` 은 다음 알고리즘으로 동작한다 (선행 SPEC §7.5 의 구조를 계승하되 패턴만 retarget):

1. **입력**: 검사 대상 .md 파일 목록 (default: 모든 `*.md` 단, exemption zone 제외)
2. **exemption zones** (검사 skip):
   - `.moai/specs/SPEC-GOOSE-*/**` 경로 (immutable archive)
   - `.moai/brain/IDEA-*/**` (ideation history)
   - `.claude/agent-memory/**` (subagent persistent memory)
   - `## HISTORY` 섹션 내부 (between `^## HISTORY` and next `^## ` heading)
   - fenced code blocks (between ```` ``` ```` and matching ```` ``` ````)
   - inline code spans (`` `...` ``)
   - 본 SPEC 자체 (`.moai/specs/SPEC-MINK-BRAND-RENAME-001/`) — 규범 설명 시 옛 표기 인용 필요
3. **위반 검출 (코드 영역 외부)**: 다음 패턴 검출:
   - `goose 프로젝트`, `Goose 프로젝트`, `GOOSE 프로젝트` (한국어 brand-position)
   - `goose project`, `Goose project` (영문 brand-position)
   - `GOOSE-AGENT` (옛 brand 약칭)
   - `\bAI\.GOOSE\b` (선행 SPEC 결과의 brand 표기 잔존)
4. **분류 (A) vs (D) 판정**: 코드 영역 내부 `` `mink CLI` ``, `` `minkd daemon` ``, `` `goose` `` (옛 binary 인용), `` `github.com/modu-ai/goose/...` `` (옛 module path 인용 in HISTORY/changelog) 등은 도메인 용어 (D) 또는 immutable archive 로 보존하고 위반에서 제외.
5. **HISTORY 보존 검증 (append-only)**: 모든 SPEC 의 `## HISTORY` 섹션 내부에서 baseline 행 의 byte-level 변경이 발생했는지 검증. baseline 행 이후에 새 행이 추가된 경우 (append-only) 는 허용 — predecessor SPEC supersede transition 등 status-event 기록을 위함. baseline 행의 cell 내용이 변경된 경우 즉시 fail.
6. **agent-memory 보존 검증**: `.claude/agent-memory/**` 의 SHA-256 hash 가 baseline 과 byte-identical (REQ-MINK-BR-016 + AC-MINK-BR-014).
7. **출력**: 위반이 0건이면 exit 0, 위반이 있으면 exit 1 + file path / line number / offending pattern 보고.
8. **언어 선택**: Python 스크립트 우선 (마크다운 파서 활용 용이 — 선행 SPEC 의 162 라인 Python-in-bash 스크립트 계승).

GitHub Actions workflow `.github/workflows/brand-lint.yml` 은 위 스크립트를 PR trigger 에서 실행하며, exit 1 시 merge 차단 (REQ-MINK-BR-013).

---

## 6. Out of Scope (변경 금지 영역)

다음 항목은 brand 정규화 대상에서 **절대 제외**:

1. 모든 `SPEC-GOOSE-*` 디렉토리 이름 (88 개) — immutable archive
2. 모든 SPEC 의 `## HISTORY` 표 baseline 행 — append-only 만 허용
3. 과거 CHANGELOG entry (이미 발행된 release section 이전) — preserve
4. `.claude/agent-memory/**` — byte-identical 보존
5. 선행 SPEC body content (`.moai/specs/SPEC-GOOSE-BRAND-RENAME-001/spec.md`) — frontmatter status / HISTORY append 만 허용
6. 백틱으로 감싼 도메인 용어: `` `goose` ``, `` `goosed` ``, `` `github.com/modu-ai/goose/...` `` (HISTORY / 인용 컨텍스트)
7. git commit message 과거 history (rewrite 금지)

---

## 7. URL slug 및 미래 도메인 정책

미래 도메인 등록 시 `mink` 단순 케이스를 사용한다:

- 예시: `mink.dev`, `mink.io`, `docs.mink.dev`
- GitHub repo: `modu-ai/mink` (본 SPEC Phase 7 post-merge 에서 `gh repo rename mink` 실행)

---

Version: 2.0.0
Classification: FROZEN_REFERENCE
Source: SPEC-MINK-BRAND-RENAME-001 §7 (v0.1.1)
Created: 2026-05-12
Supersedes: SPEC-GOOSE-BRAND-RENAME-001 §7 (style-guide v1.0.0)
REQ coverage: REQ-MINK-BR-001, REQ-MINK-BR-002, REQ-MINK-BR-005, REQ-MINK-BR-006, REQ-MINK-BR-016, REQ-MINK-BR-019, REQ-MINK-BR-020, REQ-MINK-BR-022, REQ-MINK-BR-023, REQ-MINK-BR-025
