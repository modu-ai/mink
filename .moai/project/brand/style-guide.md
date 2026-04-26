---
id: brand-style-guide
version: 1.0.0
status: frozen
created_at: 2026-04-26
spec: SPEC-GOOSE-BRAND-RENAME-001
classification: FROZEN_REFERENCE
---

# AI.GOOSE Brand Style Guide

> **FROZEN REFERENCE** — 이 문서는 SPEC-GOOSE-BRAND-RENAME-001 §7 (spec.md v0.1.1)의 캐노니컬 사본이다.
> 변경 시 spec.md §7과 이 파일을 동시에 수정해야 하며, brand-lint workflow가 §7.1 표의 byte-level 동일성을 검증한다.
> 후속 SPEC 작성자 및 manager-spec agent는 brand 표기 결정 시 반드시 이 문서를 참조한다.

---

## 1. 표기 규범 (3 영역)

| 컨텍스트 | 표기 | 적용 영역 | 예시 |
|---------|------|----------|------|
| 공식 브랜드명 / user-facing prose | `AI.GOOSE` | 모든 문서, README 제목, CHANGELOG 신규 entry, CLAUDE.md 인사말, CLI 환영 메시지, 에러 메시지 도입부 | "AI.GOOSE는 Daily Companion AI입니다." |
| 짧은 약칭 / 코드 식별자 / 도메인 용어 | `goose` | `goose CLI`, `goosed daemon`, `goose agent loop`, type/func/var 이름 (Go 식별자) | `package goose`, `type GooseRuntime` |
| URL slug / GitHub repo / 도메인 | `ai-goose` | 미래 도메인(`ai-goose.dev` 등), URL slug, 검색 친화 표기 | `ai-goose.dev`, `https://docs.ai-goose.dev/` |

---

## 2. Dual Representation 원칙

산문에서 brand로 가리킬 때는 `AI.GOOSE`, 코드/식별자로 가리킬 때는 `goose`를 사용한다. 백틱(`)으로 감싼 식별자는 brand-naming 규칙에서 제외된다.

예:
- 좋은 예: `` AI.GOOSE는 `goose CLI`로 실행됩니다. ``
- 나쁜 예: `` Goose는 goose CLI로 실행됩니다. `` (brand 표기 잘못, 식별자 백틱 누락)

---

## 3. 한/영 표기 예시 (i18n 일반 정책 포함)

| 한국어 | 영어 |
|--------|------|
| `AI.GOOSE 프로젝트` | `the AI.GOOSE project` |
| `AI.GOOSE는 ...입니다.` | `AI.GOOSE is ...` |
| `` `goose CLI` 명령어 `` | `` the `goose CLI` command `` |
| `Welcome to AI.GOOSE` | `Welcome to AI.GOOSE` |

i18n 일반 정책: 다른 언어(일본어, 중국어 등)가 추후 도입되더라도 동일 dual representation 원칙(brand=`AI.GOOSE` / 식별자=`goose` / slug=`ai-goose`)을 그대로 적용한다. 신규 언어 추가 자체는 본 SPEC OUT scope이며, 별도 SPEC에서 처리한다.

---

## 4. 후속 SPEC 작성자 참조

후속 SPEC을 작성하는 manager-spec agent / 인간 작성자는 다음을 의무 준수한다:

- SPEC template 또는 plan 단계에서 `.moai/project/brand/style-guide.md` 참조 링크 포함
- brand 표기 시 `AI.GOOSE` 사용 (REQ-BR-001)
- 코드 식별자/도메인 용어는 `goose` (REQ-BR-002)
- URL/도메인 slug는 `ai-goose` (REQ-BR-003)

---

## 5. brand-lint 검증 알고리즘 (`scripts/check-brand.sh` 동작 명세)

`scripts/check-brand.sh` 및 GitHub Actions workflow `brand-lint.yml`는 다음 알고리즘으로 동작한다:

1. **입력**: 검사 대상 .md 파일 목록 (default: `.moai/project/`, `.moai/specs/`, `.claude/`, `README.md`, `CHANGELOG.md`, `CLAUDE.md`)
2. **마크다운 파싱**: 각 파일을 inline code span 및 fenced code block 영역을 제외하고 검사
3. **위반 검출**: 코드 영역 외부에서 다음 패턴을 검출:
   - `goose 프로젝트`, `Goose 프로젝트`, `GOOSE 프로젝트`
   - `goose project`, `Goose project`
   - `GOOSE-AGENT` (백틱 외부, brand 위치)
4. **분류 (A) vs (D) 판정**: 코드 영역 내부 `` `goose CLI` ``, `` `goosed daemon` ``, `` `goose agent loop` `` 등은 도메인 용어(D)로 보존하고 위반에서 제외
5. **HISTORY 보존 검증**: 모든 SPEC의 `## HISTORY` 섹션 안에서 brand 패턴 변경이 발생했는지 baseline과 byte-level diff. 변경 발견 시 즉시 fail
6. **출력**: 위반이 0건이면 exit 0, 위반이 있으면 exit 1과 함께 file path/line number/offending pattern 보고
7. **언어 선택**: Python 스크립트 우선 (마크다운 파서 활용 용이). bash + ripgrep PCRE2 대안도 허용

---

## 6. Out of Scope (변경 금지 영역)

다음 항목은 brand 정규화 대상에서 **절대 제외**:

1. Go module path: `github.com/modu-ai/goose`
2. Go package 이름: `package goose`, `package goosed` 등
3. Type/struct/function/variable 이름: `type Goose*` 등 코드 식별자 일체
4. CLI binary 이름: `goose`, `goosed`, `goose-cli`
5. SPEC ID 네이밍: `SPEC-GOOSE-XXX-NNN`
6. Git remote / GitHub repo: `modu-ai/goose-agent`
7. 모든 SPEC `## HISTORY` 표 entries (status 무관)
8. 과거 CHANGELOG entry (이미 발행된 release section)
9. proto package / message 이름: `goose.v1` 등
10. 백틱으로 감싼 도메인 용어: `` `goose CLI` ``, `` `goosed daemon` `` 등

---

## 7. URL slug 및 미래 도메인 정책

미래 도메인 등록 시 `ai-goose` 케밥 케이스를 사용한다:

- 예시: `ai-goose.dev`, `ai-goose.io`, `docs.ai-goose.dev`
- GitHub repo는 현행 `modu-ai/goose-agent` 유지 (변경 OUT scope)

---

Version: 1.0.0
Classification: FROZEN_REFERENCE
Source: SPEC-GOOSE-BRAND-RENAME-001 §7 (v0.1.1)
Created: 2026-04-26
REQ coverage: REQ-BR-001, REQ-BR-002, REQ-BR-003, REQ-BR-004, REQ-BR-016, REQ-BR-017
