---
id: SPEC-MINK-BRAND-RENAME-001
version: 0.1.1
status: completed
supersedes: SPEC-GOOSE-BRAND-RENAME-001
created_at: 2026-05-12
updated_at: 2026-05-12
author: manager-spec
priority: P1
phase: meta
size: 대(L)
lifecycle: spec-anchored
labels: [brand, meta, cross-cutting, rename, breaking]
issue_number: null
---

# SPEC-MINK-BRAND-RENAME-001 — GOOSE → MINK 전역 rename

## HISTORY

| Version | Date | Author | Change |
|---------|------|--------|--------|
| 0.1.0 | 2026-05-12 | manager-spec | 초안 작성. IDEA-002 결정 (브랜드 MINK / Made IN Korea, 단타 LLM-bot GooseBot 분리) + 사용자 8개 확정사항 (scope = MINK-001만, supersede policy, plan/SPEC-MINK-BRAND-RENAME-001 단일 branch + squash PR, --solo mode, SPEC-MINK-* prefix 신규 / SPEC-GOOSE-* 88개 보존, immutable HISTORY/CHANGELOG/git history 보존, Go module/repo/binary/proto 일괄 rename, brand 자산 재사용 retarget) 반영. SPEC-GOOSE-BRAND-RENAME-001 supersede 명시 (immutable body 정책 + frontmatter status 전환은 별도 후속 commit). 12 IN-scope items + 10 OUT-scope items + 27 EARS 요구사항 + 17 AC + 8 phase 구현 계획. |
| 0.1.1 | 2026-05-12 | claude(orchestrator) | plan-auditor v1 결과 반영 (.moai/reports/plan-audit/SPEC-MINK-BRAND-RENAME-001-2026-05-12.md). 6 line-level corrections 적용: (C-1) REQ-MINK-BR-020 + §7.5 step 5 에 append-only HISTORY 행 추가 허용 명시 (예비 SPEC supersede 행 추가 시 self-block 회피); (C-2) AC-MINK-BR-015 step 1 을 vacuously true 로 재정의 (downstream `SPEC-MINK-USERDATA-MIGRATE-001` scope 명시); (H-1) §1.3 에 brand-runtime split window NOTE 추가 (`GOOSE_*` env + `./.goose/` path deferral 명시 문서화); (H-2) AC-MINK-BR-013 step 4 README badge URL `${{ github.repository }}` parameterization 권장; (H-3) AC-MINK-BR-002 step 6 escape hatch 제거 (`-tags integration` + `-short` 만 허용); (H-5) AC-MINK-BR-018 신설 (CHANGELOG entry 무변경 + 신규 entry brand 통일 — REQ-MINK-BR-023 orphan 해소). 총 17 AC → 18 AC, 27 EARS 그대로. |
| 0.1.2 | 2026-05-14 | MoAI orchestrator | Drift correction: status planned → completed. 8-phase atomic rename 완료 (commit f0f02e4). Go module / GitHub repo / CLI binary / proto package 전역 rename + brand 자산 retarget 완료. 선행 SPEC-GOOSE-BRAND-RENAME-001 body immutable 유지. |

---

## 1. Overview

### 1.1 Scope Clarity

본 SPEC은 **메타 SPEC** (cross-cutting, 횡단 적용)이다. 단일 모듈/도메인의 기능을 정의하지 않으며, 프로젝트 전체에서 brand identifier 와 code identifier 를 모두 `MINK` / `mink` 로 전면 rename한다. 선행 SPEC `SPEC-GOOSE-BRAND-RENAME-001` (v0.1.1, status: completed)이 OUT scope으로 명시했던 12개 항목을 본 SPEC에서 **모두 IN scope으로 뒤집어** 실행한다.

본 SPEC은 다음을 명시한다:
- 12개 IN-scope item 명세 (§3.1) — 선행 SPEC §3.2의 인버전
- 10개 OUT-scope item 명세 (§3.2) — immutable 이력 / downstream 분리 항목
- 27개 EARS 요구사항 (§4)
- 17개 binary-verifiable Acceptance Criteria (§5)
- 8-phase 구현 계획 (§6) — atomicity 제약 + commit/PR 정책 포함
- MINK Brand Style Guide canonical draft (§7) — `.moai/project/brand/style-guide.md` 전면 재작성용 reference
- 12개 risk + mitigation (§9)
- 11개 명시 제외 항목 (§11)

### 1.2 Goal

`AI.GOOSE` 프로젝트의 brand 와 code identifier 를 모두 `MINK` (Made IN Korea, 산문) / `mink` (식별자·slug)로 통일한다. Go module path 변경 (`github.com/modu-ai/goose` → `github.com/modu-ai/mink`), GitHub repo rename (`modu-ai/goose` → `modu-ai/mink`), proto package rename (`goose.v1` → `mink.v1`), CLI binary rename (`goose`/`goosed`/`goose-proxy` → `mink`/`minkd`/`mink-proxy`)을 단일 SPEC PR squash로 합쳐 실행한다. immutable 이력 (88개 SPEC-GOOSE-* 디렉토리, 모든 SPEC HISTORY rows, 기존 CHANGELOG entries, git commit messages, PR titles)은 변경 금지.

### 1.3 Non-Goals

- 선행 SPEC `SPEC-GOOSE-BRAND-RENAME-001` 의 body 내용 수정 — body는 immutable, frontmatter status / HISTORY 1 row 추가는 **별도 후속 commit** (orchestrator 담당)
- 88개 `SPEC-GOOSE-*` 디렉토리 이름 변경 — preserve (cross-ref 무결성 + git log 검색성)
- 모든 `## HISTORY` 표 rows 변경 (status 무관, location 무관)
- 과거 CHANGELOG entries 변경 (이미 발행된 release section)
- git history 변경 (commit message, PR title, tag — immutable)
- `.claude/agent-memory/**` 변경 (각 subagent 의 persistent memory; 시점 기록 archive)
- MINK domain 실제 등록 (`mink.dev`, `mink.io` 등)
- MINK trademark filing
- logo / visual asset rewrite (text-only style guide만)
- block / goose distancing statement (별도 SPEC `SPEC-MINK-DISTANCING-STATEMENT-001`)
- product.md v7.0 (별도 SPEC `SPEC-MINK-PRODUCT-V7-001`)
- User-data 실제 마이그레이션 코드 (`./.goose/` → `./.mink/` 자동 마이그레이션 logic — 별도 SPEC `SPEC-MINK-USERDATA-MIGRATE-001`). 본 SPEC은 정책만 정의.
- M1+ feature (RITUAL bookend 등)

> **[NOTE] Brand-runtime split window 인정 (H-1 audit 반영)**: 본 SPEC PR merge 와 downstream `SPEC-MINK-ENV-MIGRATE-001` + `SPEC-MINK-USERDATA-MIGRATE-001` 머지 사이 기간 동안, binary `mink` 의 brand-position 출력은 모두 MINK 이지만 runtime 에서는 (a) `GOOSE_*` env vars 21개 (824 occurrence) 를 읽고 (b) `./.goose/` / `~/.goose/` workspace path 117 occurrence 를 사용한다. **이는 기능적으로 backward-compatible (no runtime regression)** 이며, 단지 일시적 brand-runtime inconsistency window 다. 완화: §8.4 downstream SPEC 들이 pre-planned 상태이고, 본 SPEC CHANGELOG entry 가 window 를 명시 문서화한다. R8/R9 참조.

### 1.4 Supersedes

본 SPEC은 `SPEC-GOOSE-BRAND-RENAME-001` (status: completed, 2026-04-27)을 **supersede**한다.

선행 SPEC은 user-facing 산문의 brand 표기를 `AI.GOOSE`로 통일했으나, code identifier / repo / Go module path / SPEC ID prefix / binary / proto package 는 **명시적으로 보존** (§3.2 items 1-12)했다. 본 SPEC은 그 OUT-scope decision 을 뒤집어 **모든 layer**를 `MINK` / `mink`로 rename한다.

선행 SPEC의 body 와 HISTORY 는 **immutable** 로 유지된다 — 본 SPEC PR 의 squash commit 은 선행 SPEC body 의 어떤 byte도 건드리지 않는다. 선행 SPEC frontmatter `status: completed` → `status: superseded` 전환과 `## HISTORY` 표에 1개 row 추가 (`Superseded by SPEC-MINK-BRAND-RENAME-001 on 2026-05-12`)는 본 SPEC merge 후 orchestrator 가 별도 commit 으로 처리한다 (R7 참조).

선행 SPEC 이 구축한 자산 4개 (`scripts/check-brand.sh`, `.github/workflows/brand-lint.yml`, `Makefile` brand-lint target, `.moai/project/brand/style-guide.md`)는 본 SPEC 에서 **scaffold 재사용**하고 내용만 retarget한다.

---

## 2. Background

### 2.1 Why MINK

`MINK` (Made IN Korea) 는 IDEA-002 에서 결정된 새 brand identifier 이다. 핵심 동기:
- **국적 정체성**: AI 영역에서 "Made in Korea" 명시는 ChatGPT / Claude / Gemini 등 미국 회사 대비 차별화 요소. 한국 사용자를 1차 타겟으로 명시.
- **짧고 외래어 호환**: 영문 4글자, 한글 표기 "민크" 일관, brand-position 발음 분쟁 minimal.
- **AI context disambiguation**: "mink" 의 모피 산업 연상은 AI 도메인 컨텍스트로 자연 해소 — 마치 `Bun` (JS runtime) 이 빵을 가리키지 않는 것과 동일.
- **6-month success metric**: "self + 1 other daily user" — minimum-lovable brand 단계라 SEO / WAU 등 외부 지표보다 brand 정합성을 우선시.

Tagline:
- KR: "매일 아침, 매일 저녁, 너의 MINK."
- EN: "Your AI that says good morning, every morning."

### 2.2 Why now (v0.2.x pre-0.1.0-public window)

`CLAUDE.local.md §1.3` 에 따르면 본 프로젝트는 **0.1.0 public 전환 이전**이다. 이 시점에 rename 하는 결정의 근거:

1. **외부 consumer 부재**: §research §4.3에서 확인된 대로 본 프로젝트를 import 하는 외부 Go module 은 부재. Go module proxy 캐시 영향도 minimal.
2. **88개 SPEC-GOOSE-* 누적 부담**: 시간이 갈수록 cross-ref 가 더 늘어 rename 비용이 비선형으로 증가. 88 + 더 → 100 + 더 가 되기 전에 break-point 설정.
3. **0.1.0 public 직전 brand 통일**: public 노출 시 일관된 첫인상 확보. README, GitHub repo 이름, badge URL 모두 단일 brand.
4. **선행 SPEC 자산 활용 시점**: `scripts/check-brand.sh` 등 brand-lint 인프라가 이미 구축되어 있어 비용이 낮음.

### 2.3 Predecessor SPEC limitation that forced this redesign

`SPEC-GOOSE-BRAND-RENAME-001 §3.2` 가 OUT-scope 으로 명시한 12개 항목은 brand 통일을 "표면적 layer (산문) 만" 수행하게 했다. 결과적으로:
- 산문은 `AI.GOOSE` 인데 Go module 은 `github.com/modu-ai/goose`
- 산문은 `AI.GOOSE` 인데 README h1 은 `# 🪿 GOOSE`
- SPEC 본문은 `AI.GOOSE` 인데 SPEC ID 는 `SPEC-GOOSE-XXX-NNN`

이런 split-brand 상태가 외부 노출 직전 단계에서 brand 인식 통일성을 다시 해친다. 본 SPEC 은 그 split 를 종결한다.

---

## 3. Scope

### 3.1 IN Scope (12 items — 선행 SPEC §3.2 OUT items 의 인버전)

[HARD] 다음 12개 항목을 본 SPEC PR 범위 안에서 일괄 rename한다. 각 item 은 선행 SPEC §3.2 의 같은 번호 item 의 **인버전**이며, item 1~7 은 선행 SPEC 의 1~7 과 1:1 대응한다.

1. **Go module path**: `github.com/modu-ai/goose` → `github.com/modu-ai/mink`. `go.mod` 첫 줄 + 모든 import statement 958 라인 (456 파일) 일괄 갱신. **단일 commit/atomic transaction** 필수.
2. **Go package 이름**: 실제로 `package goose` / `package goosed` 선언은 0건 (§research §2.2). 단, 생성 코드 `package goosev1` (`internal/transport/grpc/gen/goosev1/`)는 proto rename 시 자동 변경. 본 항목은 미래에 `package goose` 가 도입될 경우를 대비한 정책 선언 + 현 생성 디렉토리 (`goosev1` → `minkv1`) 변경 포함.
3. **Type/struct/func/var 식별자**: `GooseHome` → `MinkHome`, doc-comment 의 brand-position `Goose` → `Mink`. 총 80개 Goose 토큰 변경 (`GooseHome` 64건 + `Goose` 16건 of doc-comment / test fixture). 식별자 변경은 `gofmt -r` 또는 `goimports` 안전 도구로 수행 — naive `sed` 금지.
4. **CLI binary 이름**: `cmd/goose/` → `cmd/mink/`, `cmd/goosed/` → `cmd/minkd/`. `cmd/goose-proxy/` 는 현재 미구현 (README 계획 단계)이므로 디렉토리 rename 대상 외 — 정책만 명시 (`goose-proxy` → `mink-proxy` 신규 binary 신설 시).
5. **SPEC ID 네이밍 규약 (신규)**: `SPEC-MINK-XXX-NNN` prefix 사용. 기존 88개 `SPEC-GOOSE-*` 디렉토리는 **변경 금지** (§3.2 item 1 참조).
6. **GitHub repo**: `modu-ai/goose` → `modu-ai/mink`. `gh repo rename mink --repo modu-ai/goose` + 로컬 remote 갱신 (`git remote set-url origin https://github.com/modu-ai/mink.git`) + GitHub 자동 redirect 검증.
7. **proto package**: `goose.v1` → `mink.v1`. 4개 `.proto` 파일 변경 + 디렉토리 `proto/goose/v1/` → `proto/mink/v1/` + `buf.gen.yaml` `out` 경로 변경 + 생성 디렉토리 `internal/transport/grpc/gen/goosev1/` → `internal/transport/grpc/gen/minkv1/` + 생성 코드 재생성.
8. **코드 내 user-facing 문자열 (Go)**: log message, error message, CLI help text, doc-comment 의 brand-position 토큰 정정. `AI.GOOSE` (27건, §research §2.12) → `MINK`. brand-position `Goose` 단어 (~5건 test fixture 포함) → `Mink`.
9. **루트 핵심 문서**: README.md (h1, h2, badges, install snippets 39건), CLAUDE.md (brand 도입 인사말 도입 검토), CHANGELOG.md (앞으로 작성될 entry 만 — 기존 entry 는 immutable, §3.2 item 5 참조), CLAUDE.local.md (h1 4건), SECURITY.md (binary 3종 명시 + storage path), CONTRIBUTING.md, CODE_OF_CONDUCT.md.
10. **`.moai/project/**` brand-position 토큰**: product.md (93), branding.md (61, brand 자산 전면 재작성), tech.md (54), learning-engine.md (49), structure.md (44), migration.md (43), ecosystem.md (21), adaptation.md (20), token-economy.md (10), research/*.md (28~36 × 4 파일), codemaps/*.md (3 파일).
11. **`.claude/**` brand-position 토큰**: `.claude/agents/`, `.claude/skills/`, `.claude/commands/`, `.claude/rules/` 는 선행 SPEC 정정 결과 0건. `.claude/settings.local.json` (사용자 로컬) 만 선택적 검토. `.claude/agent-memory/**` 는 **OUT scope** (§3.2 item 8).
12. **Brand 자산 retarget**: `.moai/project/brand/style-guide.md` 전면 재작성 (§7 canonical draft 기반). `scripts/check-brand.sh` 위반 패턴 / exemption zone retarget. `.github/workflows/brand-lint.yml` 변경 0건 (script 호출만). `Makefile` brand-lint target 변경 0건. 추가: env var `MINK_*` 신설 + `GOOSE_*` deprecated alias loader 추가 (§3.1 item 13a)는 별도 SPEC 으로 분리 검토.

### 3.2 OUT Scope (반드시 보존 — immutable history + downstream 분리)

[HARD] 다음 항목은 변경 금지. 위반 시 무결성 손상 / git history 손상 / downstream SPEC 침해.

1. **기존 88개 `SPEC-GOOSE-*` 디렉토리 이름**: preserve (cross-ref 무결성 + git log 검색성, §research §6 참조). brand-lint exemption zone 으로 처리.
2. **모든 SPEC `## HISTORY` 표 rows**: status 무관 (planned/draft/implemented/closed/completed/superseded), location 무관 (`SPEC-GOOSE-*` / `SPEC-MINK-*` / `SPEC-AGENCY-*` / flat-file). 총 ~300 rows 보존. 선행 SPEC §3.2 item 7 verbatim 계승.
3. **과거 git commit messages**: immutable (force-push 차단으로 자연 보장, `CLAUDE.local.md §3.1`).
4. **이미 발행된 CHANGELOG entries**: release section header 이전 기존 entry (현 시점 모든 release section). 신규 entry 부터만 `MINK` 사용.
5. **PR/issue titles (closed 또는 merged)**: GitHub 자체가 title rewrite를 제한 + immutable archive 정책.
6. **`.moai/specs/SPEC-GOOSE-*/**/*.md` body content**: immutable archives. brand-lint exemption zone.
7. **`.moai/brain/IDEA-*/**`**: ideation history archive (현재 본 worktree 에 없으나 미래 도입 시 exemption zone 적용).
8. **`.claude/agent-memory/**`**: 각 subagent persistent memory 의 시점 기록. brand-lint exemption zone.
9. **`SPEC-AGENCY-ABSORB-001` 등 non-`SPEC-GOOSE-` 기존 SPEC body**: 이 SPEC들은 본 SPEC 범위 외 — brand-position 토큰이 있어도 본 SPEC PR 에서 정정하지 않음. 단, 새 SPEC 작성 시 작성자가 신경쓸 영역.
10. **CMDCTX-DEPENDENCY-ANALYSIS.md / IMPLEMENTATION-ORDER.md / ROADMAP.md / SPEC-DOC-REVIEW-2026-04-21.md (flat-file SPEC)**: 정정 대상 여부는 case-by-case — 본 SPEC 은 정책만 명시하고 실제 정정은 작성자 판단. ROADMAP.md 는 미래 작업 기록이므로 신규 entry 부터 `MINK` 사용.

---

## 4. EARS Requirements

본 SPEC은 27개 EARS 요구사항을 정의한다. 각 요구사항은 spec-anchored 이며 §5 AC 와 1:1 또는 N:1 매핑된다.

### 4.1 Ubiquitous (보편 — 항상 성립)

- **REQ-MINK-BR-001 [Ubiquitous]** The project **shall** use `MINK` as the official brand identifier in all NEW user-facing prose (산문 / 마케팅 / README / CHANGELOG 신규 entry / CLAUDE.md / SECURITY.md / CONTRIBUTING.md).
- **REQ-MINK-BR-002 [Ubiquitous]** The project **shall** use `mink` (lowercase) as the canonical short identifier in code identifiers, package paths, binary names, and domain terms (`mink CLI`, `minkd daemon`).
- **REQ-MINK-BR-003 [Ubiquitous]** The Go module path **shall** be `github.com/modu-ai/mink` after Phase 2 completes.
- **REQ-MINK-BR-004 [Ubiquitous]** The GitHub repository **shall** be `modu-ai/mink` after Phase 7 completes; the predecessor `modu-ai/goose` URL **shall** redirect to the new repo via GitHub's automatic redirect mechanism.
- **REQ-MINK-BR-005 [Ubiquitous]** New SPECs **shall** use the `SPEC-MINK-XXX-NNN` directory prefix.
- **REQ-MINK-BR-006 [Ubiquitous]** The Brand Style Guide at `.moai/project/brand/style-guide.md` **shall** be rewritten for MINK rules with `id: brand-style-guide-mink` and `classification: FROZEN_REFERENCE`.
- **REQ-MINK-BR-007 [Ubiquitous]** Binary directory names **shall** be `cmd/mink/` and `cmd/minkd/` after Phase 5 completes; `cmd/goose/` and `cmd/goosed/` directories **shall not** exist.
- **REQ-MINK-BR-008 [Ubiquitous]** The proto package **shall** be `mink.v1` after Phase 4 completes; the generated Go package directory **shall** be `internal/transport/grpc/gen/minkv1/`.
- **REQ-MINK-BR-009 [Ubiquitous]** The Go struct field `GooseHome` and all references **shall** be renamed to `MinkHome` after Phase 3 completes.
- **REQ-MINK-BR-010 [Ubiquitous]** The predecessor SPEC `SPEC-GOOSE-BRAND-RENAME-001` body content **shall** remain immutable across all phases of this SPEC; only frontmatter `status` transition and HISTORY row append are permitted via a separate orchestrator commit (not part of this SPEC's PR scope).

### 4.2 Event-Driven (트리거 발생 시)

- **REQ-MINK-BR-011 [Event-Driven]** **When** a SPEC author creates a new SPEC document, the SPEC template **shall** include a reference link to `.moai/project/brand/style-guide.md` and the directory **shall** use `SPEC-MINK-XXX-NNN` prefix.
- **REQ-MINK-BR-012 [Event-Driven]** **When** the brand-lint script (`scripts/check-brand.sh`) runs against a user-facing markdown file outside exemption zones, **and** the file contains any of {`goose 프로젝트`, `Goose 프로젝트`, `GOOSE 프로젝트`, `goose project`, `Goose project`, `GOOSE-AGENT`, `AI.GOOSE`} outside backticks/code-block, the script **shall** flag the line as a brand violation and **shall** exit with code 1.
- **REQ-MINK-BR-013 [Event-Driven]** **When** a Pull Request is opened or updated against `main`, the GitHub Actions workflow `.github/workflows/brand-lint.yml` **shall** run `scripts/check-brand.sh` and **shall** block merge on non-zero exit code.
- **REQ-MINK-BR-014 [Event-Driven]** **When** a new CHANGELOG entry is added, the entry **shall** use `MINK` for brand references; prior CHANGELOG entries (existing release sections at the time of this SPEC) **shall not** be edited.
- **REQ-MINK-BR-015 [Event-Driven]** **When** Phase 2 (Go module rename) commit lands, `go build ./...` `go vet ./...` `go test ./...` **shall** all exit 0 within the same commit's tree state.

### 4.3 State-Driven (상태 조건)

- **REQ-MINK-BR-016 [State-Driven]** **While** any file path matches `.moai/specs/SPEC-GOOSE-*/**` or `.moai/brain/IDEA-*/**` or `.claude/agent-memory/**`, the brand-lint script **shall** skip brand-position validation for that path.
- **REQ-MINK-BR-017 [State-Driven]** **While** a line is inside a markdown `## HISTORY` section (between `^## HISTORY` and the next `^## ` heading), the brand-lint script **shall** skip the line.
- **REQ-MINK-BR-018 [State-Driven]** **While** a string is enclosed in backticks (``\`...\``) or fenced code blocks, the brand-lint script **shall** treat the content as code identifier or domain term and skip brand-naming validation.
- **REQ-MINK-BR-019 [State-Driven]** **While** the existing 88 `SPEC-GOOSE-*` directories exist on disk, the new SPEC `SPEC-MINK-BRAND-RENAME-001` **shall not** trigger any rename or removal of those directories.

### 4.4 Unwanted (금지 행동)

- **REQ-MINK-BR-020 [Unwanted]** **If** any commit attempts to modify **an existing row at a row-index that was present at baseline** in any existing SPEC's `## HISTORY` table (status irrespective: planned/draft/implemented/closed/completed/superseded), **then** the brand-lint CI gate **shall** reject the change on immutable-history grounds. **Appending new rows after the last baseline row is explicitly permitted** (e.g., status-transition entries such as predecessor SPEC supersede annotations); this exemption resolves the contradiction with AC-MINK-BR-004 which requires appending one new row to `SPEC-GOOSE-BRAND-RENAME-001/## HISTORY` post-merge.
- **REQ-MINK-BR-021 [Unwanted]** **If** `go.mod`'s module path differs between its declaration and the import statements referenced by `.go` source files within the same commit's tree state, **then** `go build ./...` **shall** fail and CI **shall** block merge. (Atomicity guard: Phase 2 commit must contain go.mod + all import statements in lock-step.)
- **REQ-MINK-BR-022 [Unwanted]** **If** any commit attempts to alter or remove an existing `SPEC-GOOSE-*` directory name, **then** CI **shall** reject the change. SPEC-GOOSE-* directories are immutable archives.
- **REQ-MINK-BR-023 [Unwanted]** **If** any commit attempts to edit a published CHANGELOG entry (release section header older than today, 2026-05-12, at the time of this SPEC), **then** the brand-lint CI gate **shall** reject the change on immutable-history grounds.
- **REQ-MINK-BR-024 [Unwanted]** **If** any test or runtime path constructs a default user-data directory using the literal `./.goose/` or `~/.goose/` in NEW code after Phase 5, **then** the code review **shall** reject it. (Old paths are permitted only inside `_test.go` fixtures that verify the deprecation/migration fallback, with explicit `// MINK migration fallback` comment.)

### 4.5 Optional (해당 시)

- **REQ-MINK-BR-025 [Optional]** **Where** a future domain is registered for the project, the domain slug **shall** use `mink` (e.g., `mink.dev`, `mink.io`, `docs.mink.dev`). Actual domain registration is OUT of scope (§11 item 1).
- **REQ-MINK-BR-026 [Optional]** **Where** a developer's local environment is configured with pre-commit hooks, the brand-lint script **shall** be wired as a pre-commit hook so that `git commit` fails before the commit object is created on a brand violation.
- **REQ-MINK-BR-027 [Optional]** **Where** an environment variable migration is added (downstream SPEC), the runtime **shall** read both `MINK_*` (preferred) and `GOOSE_*` (deprecated) env var keys with the new key taking precedence, and **shall** emit a stderr deprecation warning when only the old key is set. Actual env var migration is OUT of scope of this SPEC (§3.1 item 12 footnote).

---

## 5. Acceptance Criteria

각 AC는 Given / When / Then 형식. 모든 AC 는 binary 검증 가능 (byte-level diff, exit code, file existence, command output equality).

### AC-MINK-BR-001 — Brand Style Guide 전면 재작성

**Given** SPEC-MINK-BRAND-RENAME-001 Phase 1 commit 이 완료된 상태에서
**When** `.moai/project/brand/style-guide.md` 파일을 확인하면
**Then** 파일이 존재하며 다음이 모두 명문화되어 있다:
- frontmatter `id: brand-style-guide-mink`, `version: 2.0.0` (선행 SPEC v1.0.0 supersede), `status: frozen`, `classification: FROZEN_REFERENCE`, `spec: SPEC-MINK-BRAND-RENAME-001`
- §1 표기 규범 3 영역 표: brand=`MINK` / 식별자=`mink` / slug=`mink` (REQ-MINK-BR-001, REQ-MINK-BR-002)
- §2 Dual Representation 원칙 (산문 vs 코드)
- §3 한국어/영어 예시 4쌍 이상 (한 쌍 = "MINK 프로젝트" ↔ "the MINK project" 형태)
- §4 후속 SPEC 작성자 참조 가이드
- §5 brand-lint 알고리즘 명세
- §6 Out of Scope 표
- §7 URL slug / 미래 도메인 정책 (`mink.dev` 예시)

REQ 매핑: REQ-MINK-BR-001, REQ-MINK-BR-002, REQ-MINK-BR-006, REQ-MINK-BR-025

### AC-MINK-BR-002 — Go module path rename + 컴파일 무결성

**Given** Phase 2 commit 직전 baseline 으로 `head -1 go.mod` = `module github.com/modu-ai/goose` 가 캡처된 상태에서
**When** Phase 2 commit 이 적용된 후 다음을 검증하면:
1. `head -1 go.mod` = `module github.com/modu-ai/mink`
2. `grep -rln 'github.com/modu-ai/goose' --include='*.go' | wc -l` = 0
3. `grep -rln 'github.com/modu-ai/mink' --include='*.go' | wc -l` = 456 (baseline 카운트)
4. `go build ./...` exit 0
5. `go vet ./...` exit 0
6. `go test ./...` exit 0 — exemptions: `-tags integration` 또는 외부 서비스 (network/DB) 필요 테스트만 `-short` 플래그로 skip 가능. 일반 prose rationale 로 skip 금지.

**Then** 6개 조건 모두 만족한다 (binary).

REQ 매핑: REQ-MINK-BR-003, REQ-MINK-BR-015, REQ-MINK-BR-021

### AC-MINK-BR-003 — GitHub repo rename + redirect 검증

**Given** Phase 2 ~ Phase 6 까지의 commit 이 main 에 merge 된 상태에서
**When** Phase 7 작업으로 다음이 실행되면:
1. `gh repo rename mink --repo modu-ai/goose` 실행 → exit 0
2. `git remote set-url origin https://github.com/modu-ai/mink.git` (로컬 갱신)
3. `git push origin main` → 200 응답

**Then** 다음 모두 만족한다:
- `git ls-remote https://github.com/modu-ai/mink.git HEAD` → main HEAD SHA 반환
- `git ls-remote https://github.com/modu-ai/goose.git HEAD` → 같은 SHA 반환 (GitHub redirect)
- `gh repo view modu-ai/mink` → metadata 반환 (visibility, default_branch 보존)
- 옛 PR URL (예: `https://github.com/modu-ai/goose/pull/162`) → 새 URL `https://github.com/modu-ai/mink/pull/162` 로 redirect (HTTP 301)

REQ 매핑: REQ-MINK-BR-004

### AC-MINK-BR-004 — 선행 SPEC body 무변경 + supersede 마커 표시 (별도 commit 검증)

**Given** Phase 1 시작 시점에 선행 SPEC `.moai/specs/SPEC-GOOSE-BRAND-RENAME-001/spec.md` 의 byte-level snapshot 이 캡처된 상태에서
**When** 본 SPEC PR squash merge 후 orchestrator 가 별도 commit 으로 다음을 실행하면:
1. 선행 SPEC frontmatter `status: completed` → `status: superseded` 변경
2. 선행 SPEC `## HISTORY` 표에 1행 추가: `| 0.1.2 | 2026-05-12 | manager-spec | Superseded by SPEC-MINK-BRAND-RENAME-001 — all OUT-scope items (Go module / repo / SPEC ID / binary / proto / code identifiers) now flipped to IN-scope. Body content remains immutable. |`

**Then** 다음 모두 만족한다:
- 선행 SPEC body section 1~11 (frontmatter / HISTORY 제외) byte-level 변경 0건
- 선행 SPEC frontmatter `status` 필드만 변경 (다른 필드는 동일)
- 선행 SPEC `## HISTORY` 표는 1행만 추가됨 (기존 0.1.0, 0.1.1 row 보존)

REQ 매핑: REQ-MINK-BR-010

### AC-MINK-BR-005 — 기존 88개 SPEC-GOOSE-* 디렉토리 byte-identical

**Given** Phase 1 시작 시점에 다음 baseline 이 캡처된 상태에서:
- `find .moai/specs -maxdepth 1 -type d -name 'SPEC-GOOSE-*' | sort` → 88-line snapshot
- 각 `SPEC-GOOSE-*/spec.md` 의 SHA-256 hash → 88-entry hash list

**When** 본 SPEC PR squash merge 후 동일 명령을 재실행하면

**Then**:
1. 디렉토리 목록이 baseline 과 byte-identical (88 디렉토리, 이름 변경 0건, 삭제/추가 0건)
2. **(단, AC-MINK-BR-004 의 별도 supersede commit 이 적용된 SPEC-GOOSE-BRAND-RENAME-001/spec.md 는 예외)** 87 개 SPEC-GOOSE-* spec.md 의 SHA-256 hash 가 baseline 과 일치

REQ 매핑: REQ-MINK-BR-019, REQ-MINK-BR-022

### AC-MINK-BR-006 — 모든 SPEC HISTORY rows immutable

**Given** Phase 1 시작 시점에 baseline 으로 다음이 캡처된 상태에서:
- 모든 `.moai/specs/**/spec.md` 파일 (108 파일, ~300 rows)에서 `## HISTORY` 섹션 (between `^## HISTORY` and next `^## `)만 추출한 단일 concatenated snapshot 파일

**When** 본 SPEC PR squash merge 후 동일 추출을 재실행하면 (AC-MINK-BR-004 별도 supersede commit 의 1 행 추가는 예외 처리)

**Then** baseline snapshot 과 post snapshot 의 diff 가 다음 만 보인다:
- 선행 SPEC HISTORY 표에 1행 추가 (AC-MINK-BR-004 의 결과)
- 본 SPEC HISTORY 표 신규 (Version 0.1.0 row, 본 SPEC 자체 신설로 추가)
- 그 외 모든 기존 HISTORY row 변경 0건 (byte-identical)

REQ 매핑: REQ-MINK-BR-017, REQ-MINK-BR-020

### AC-MINK-BR-007 — brand-lint retarget + CI 통과

**Given** Phase 6 commit 이 적용된 상태에서
**When** 다음을 모두 검증하면:
1. `scripts/check-brand.sh` 의 violation patterns 가 `goose 프로젝트`, `Goose 프로젝트`, `GOOSE-AGENT`, `goose project`, `Goose project`, `AI.GOOSE` (산문) 6개로 retarget 됨
2. exemption zones: `.moai/specs/SPEC-GOOSE-*/`, `.moai/brain/IDEA-*/`, `.claude/agent-memory/`, `## HISTORY` 섹션, fenced code block, inline code span
3. `bash scripts/check-brand.sh` 가 본 SPEC merge 후 main 에서 exit 0 반환
4. 의도적으로 brand 위반 (`GOOSE 프로젝트` 추가) 도입한 시험 PR 이 brand-lint check 실패로 merge 차단

**Then** 4개 조건 모두 만족 (binary).

REQ 매핑: REQ-MINK-BR-012, REQ-MINK-BR-013, REQ-MINK-BR-016, REQ-MINK-BR-017, REQ-MINK-BR-018

### AC-MINK-BR-008 — Binary 디렉토리 rename + Makefile 동작

**Given** Phase 5 commit 직전 baseline 으로 `ls cmd/` 출력 = `goose\ngoosed\n` 가 캡처된 상태에서
**When** Phase 5 commit 적용 후 다음을 검증하면:
1. `ls cmd/` 출력 = `mink\nminkd\n` (단 두 디렉토리, 옛 디렉토리 0건)
2. `cmd/mink/main.go` 가 존재하며 import path 는 `github.com/modu-ai/mink/internal/cli` (Phase 2 결과 일관성)
3. `cmd/minkd/main.go` 가 존재하며 doc-comment 가 `MINK` brand 표기 사용 (`AI.GOOSE` / `GOOSE` 0건)
4. `make build` 또는 `go build ./cmd/...` 가 두 binary (`mink`, `minkd`) 생성

**Then** 4개 조건 모두 만족 (binary).

REQ 매핑: REQ-MINK-BR-007

### AC-MINK-BR-009 — Proto package mink.v1 + 재생성 코드 무결성

**Given** Phase 4 commit 직전 baseline:
- `proto/goose/v1/{tool,config,agent,daemon}.proto` 4 파일 존재
- 각 파일 첫 줄 `// Package goose.v1 ...`
- 각 파일 L5 `package goose.v1;`
- 각 파일 L7 `option go_package = "...goosev1;goosev1";`

**When** Phase 4 commit 적용 후 다음을 검증하면:
1. `find proto/ -name '*.proto'` 출력이 `proto/mink/v1/{tool,config,agent,daemon}.proto`
2. 각 `.proto` 파일 `package mink.v1;`
3. 각 `.proto` 파일 `option go_package = "github.com/modu-ai/mink/internal/transport/grpc/gen/minkv1;minkv1";`
4. `internal/transport/grpc/gen/goosev1/` 디렉토리 부재
5. `internal/transport/grpc/gen/minkv1/` 디렉토리 존재 + 생성 `.pb.go` 파일들 `package minkv1` 선언
6. `make proto-generate` 실행 시 exit 0
7. `go build ./internal/transport/...` exit 0

**Then** 7개 조건 모두 만족 (binary).

REQ 매핑: REQ-MINK-BR-008, REQ-MINK-BR-021

### AC-MINK-BR-010 — Go 식별자 GooseHome → MinkHome rename

**Given** Phase 3 commit 직전 baseline:
- `grep -rEohw 'GooseHome' --include='*.go' | wc -l` = 64
- `grep -rEohw 'Goose' --include='*.go' | wc -l` = 80 (`GooseHome` 64 + bare `Goose` 16)

**When** Phase 3 commit 적용 후 다음을 검증하면:
1. `grep -rEohw 'GooseHome' --include='*.go' | wc -l` = 0
2. `grep -rEohw 'MinkHome' --include='*.go' | wc -l` ≥ 64 (= baseline 64 이상, gofmt -r 등 자동 도구가 동일 토큰 변경 보장)
3. brand-position `Goose` (test fixture 제외) 0건: `grep -rn '"You are Goose' --include='*.go' | wc -l` = 0 또는 `Mink` 로 치환됨
4. `go vet ./...` exit 0
5. `go test ./internal/config/...` exit 0 (`GooseHome` → `MinkHome` 영향이 가장 큰 패키지)

**Then** 5개 조건 모두 만족 (binary).

REQ 매핑: REQ-MINK-BR-009

### AC-MINK-BR-011 — User-facing 코드 문자열 brand 통일

**Given** Phase 8 commit (코드 user-facing 문자열) 적용 후
**When** 다음을 검증하면:
1. `grep -rn 'AI\.GOOSE' --include='*.go' | wc -l` = 0
2. `grep -rn '\bMINK\b' --include='*.go' | wc -l` ≥ 27 (baseline 27 이상)
3. `./mink --help` 출력 첫 줄에 `MINK` 등장, `AI.GOOSE` / `GOOSE` / `Goose` brand-position 0건
4. `./minkd --version` 출력에 `MINK daemon` 또는 `minkd` 등장

**Then** 4개 조건 모두 만족.

REQ 매핑: REQ-MINK-BR-001, REQ-MINK-BR-002

### AC-MINK-BR-012 — Documentation cross-reference 무결성

**Given** Phase 7 (문서 sweep) commit 적용 후
**When** 다음을 검증하면:
1. `grep -rEln 'github.com/modu-ai/goose' --include='*.md' --exclude-dir='.moai/specs/SPEC-GOOSE-*' --exclude-dir='.moai/brain/IDEA-*' --exclude-dir='.claude/agent-memory'` 출력에서 매치되는 파일 중, 매치 라인이 모두 `## HISTORY` 섹션 내부이거나 fenced code block 내부일 것 (baseline 비교: exemption zone 밖 라인 0건)
2. `grep -rEln '\bAI\.GOOSE\b' --include='*.md' --exclude-dir='.moai/specs/SPEC-GOOSE-*' --exclude-dir='.moai/brain/IDEA-*' --exclude-dir='.claude/agent-memory'` 출력 0 파일 (산문에서 `AI.GOOSE` 잔존 0건)
3. README.md h1 = `# MINK` 또는 동등 변형, h2 = `## What is MINK?` 패턴

**Then** 3개 조건 모두 만족.

REQ 매핑: REQ-MINK-BR-001, REQ-MINK-BR-002

### AC-MINK-BR-013 — CI workflow 파일 retarget

**Given** Phase 6 (CI/workflow) commit 적용 후
**When** 다음을 검증하면:
1. `.github/workflows/*.yml` 중 `modu-ai/goose` 하드코딩 0건 (Phase 4.3 baseline 확인된 대로 원래도 0건)
2. `.github/PULL_REQUEST_TEMPLATE.md` 가 `modu-ai/mink` 참조 (또는 `${{ github.repository }}` 같은 parameterized 방식)
3. `.github/ISSUE_TEMPLATE/*.yml` 4개 파일이 `modu-ai/mink` 참조
4. `README.md` badge URL 4개가 `${{ github.repository }}` 또는 동등 parameterization 으로 작성됨 (literal `modu-ai/mink` 도 허용하나, post-merge `gh repo rename` 까지 badge 404 window 발생 — parameterization 권장)

**Then** 4개 조건 모두 만족.

REQ 매핑: REQ-MINK-BR-001, REQ-MINK-BR-004

### AC-MINK-BR-014 — `.claude/agent-memory/**` immutable

**Given** Phase 1 시작 시점에 baseline 으로 `find .claude/agent-memory -type f -name '*.md' -exec sha256sum {} +` 출력 캡처
**When** 본 SPEC PR squash merge 후 동일 명령 재실행
**Then** baseline 과 byte-identical (`.claude/agent-memory/**` 변경 0건).

REQ 매핑: REQ-MINK-BR-016

### AC-MINK-BR-015 — User-data path 정책 명시 (실제 마이그레이션은 OUT, vacuously true)

**Given** Phase 8 (코드 user-facing 문자열) commit 적용 후
**When** 다음을 검증하면:
1. 본 SPEC Phase 8 commit 은 default user-data path 변경 작업을 **포함하지 않으며**, 이는 downstream `SPEC-MINK-USERDATA-MIGRATE-001` 의 scope 이다 (§1.3 Non-Goal item, §11 item 10 참조). 본 step 은 vacuously true (no new path additions in this PR).
2. 단, 본 SPEC 의 Phase 1–8 commit 시퀀스가 새 default path 추가를 도입한 경우라면 `./.mink/` 또는 `~/.mink/` 사용 (안전 가드: 실제로는 도입 없음).
3. `_test.go` 내 fixture 가 옛 `./.goose/` 경로를 사용하는 경우 `// MINK migration fallback test` 주석 또는 동등 표시 (manual 검토).

**Then** step 1 은 PR diff inspection 으로 binary 확인, step 2/3 는 조건부.

REQ 매핑: REQ-MINK-BR-024 (정책 명시만; 실제 path migration 은 downstream SPEC 에서 검증)

### AC-MINK-BR-016 — README + CHANGELOG + CLAUDE.md brand 통일

**Given** Phase 7 (문서 sweep) commit 적용 후
**When** 다음을 검증하면:
1. README.md 의 brand-position `goose` / `Goose` / `GOOSE` / `AI.GOOSE` 0건 (단, 백틱 인용 `` `mink CLI` `` 등 code identifier 는 제외; install snippet `go build ./cmd/mink` 는 정확히 그대로 노출)
2. CHANGELOG.md 신규 entry (본 SPEC merge 의 commit 이 추가하는 entry) 에 `MINK` 사용. 기존 entry 변경 0건 (AC-MINK-BR-006 의 부분 검증).
3. CLAUDE.md 의 brand-position 토큰 0건 (CLAUDE.md 는 self-instructions 위주라 brand-position 부재 → 0건 만족 가능)
4. CLAUDE.local.md h1 = `# CLAUDE Local Instructions — MINK Project` (또는 동등)

**Then** 4개 조건 모두 만족.

REQ 매핑: REQ-MINK-BR-001, REQ-MINK-BR-014

### AC-MINK-BR-017 — `.moai/project/**` brand-position 토큰 정정

**Given** Phase 7 (문서 sweep) commit 적용 후
**When** 다음을 검증하면:
1. `grep -rEln '\bAI\.GOOSE\b' .moai/project/` 출력 0 파일 (산문 brand 정정 완료, brand-style-guide-mink 신설 후 옛 표기 잔존 0)
2. `grep -rEln '\bGOOSE-AGENT\b' .moai/project/` 출력 0 파일
3. `bash scripts/check-brand.sh .moai/project/` exit 0
4. `.moai/project/brand/style-guide.md` = AC-MINK-BR-001 의 결과물

**Then** 4개 조건 모두 만족.

REQ 매핑: REQ-MINK-BR-001, REQ-MINK-BR-012

### AC-MINK-BR-018 — CHANGELOG 기존 entry 무변경 + 신규 entry brand 통일

**Given** Phase 1 시작 시점에 baseline 으로 다음이 캡처된 상태에서:
- `CHANGELOG.md` 의 모든 release section header (예: `## [0.0.x] - YYYY-MM-DD`) 와 그 본문의 SHA-256 hash (2026-05-12 기준 모든 entry)

**When** 본 SPEC PR squash merge 후 동일 추출을 재실행하면

**Then**:
1. baseline 모든 release section header + 본문 SHA-256 hash 가 보존됨 (byte-identical)
2. 신규 entry (본 SPEC merge 가 추가하는 `## [Unreleased]` 또는 `SPEC-MINK-BRAND-RENAME-001` 참조 라인) 에 `MINK` brand 사용
3. 의도적으로 baseline release section 의 brand-position 토큰 (`AI.GOOSE`, `GOOSE-AGENT`, `goose 프로젝트` 등) 을 수정한 시험 PR 이 brand-lint check 실패로 merge 차단됨

REQ 매핑: REQ-MINK-BR-014, REQ-MINK-BR-023

---

## 6. Technical Approach (Phased Implementation)

본 SPEC 은 8개 phase 로 나누어 구현한다. 각 phase 는 독립 commit 으로 분리하되 **단일 PR (base=main, squash merge per CLAUDE.local.md §1.4)** 안에 포함한다. 단, Phase 7 (GitHub repo rename) 은 GitHub 직접 작업 (`gh repo rename`)이라 PR 외 단독 작업.

| Phase | 작업명 | 위험도 | Atomicity | 의존성 |
|---|---|---|---|---|
| Phase 1 | Baseline capture + style-guide 신설 + check-brand.sh retarget | 낮 | 독립 | 없음 |
| Phase 2 | Go module path rename (`go.mod` + 458 파일 import) | **매우 높** | **단일 commit** | Phase 1 |
| Phase 3 | Go 식별자 (`GooseHome` → `MinkHome` 등) | 중 | 단일 commit 권장 | Phase 2 |
| Phase 4 | proto package + 생성 코드 재생성 | 높 | **단일 commit** | Phase 2 |
| Phase 5 | CLI binary 디렉토리 rename (`cmd/goose` → `cmd/mink` 등) | 중 | 단일 commit | Phase 2, Phase 3 |
| Phase 6 | CI/workflow + brand-lint sanity check | 낮 | 독립 commit | Phase 1 |
| Phase 7 | 문서 sweep (README, .moai/project/, CLAUDE.md, etc.) | 낮 | 분할 가능 commits | Phase 5 |
| Phase 8 | 코드 내 user-facing 문자열 (log, error, CLI help, doc-comment) | 중 | 단일 commit | Phase 2 ~ Phase 5 |
| (Post-merge) | GitHub repo rename + remote 갱신 + 선행 SPEC supersede commit | 중 | 단독 작업 | 본 PR merge 후 |

### Phase 1 — Baseline + style-guide + brand-lint retarget

**대상**:
- `.moai/specs/SPEC-MINK-BRAND-RENAME-001/{spec.md, research.md}` (본 SPEC 자체)
- `.moai/specs/SPEC-MINK-BRAND-RENAME-001/migration-log.md` (Phase 1 baseline 캡처 — go.mod, package counts, ls cmd/, ls .moai/specs/, CHANGELOG snapshot, SPEC HISTORY snapshot, agent-memory hash 등)
- `.moai/project/brand/style-guide.md` 전면 재작성 (§7 canonical 기반)
- `scripts/check-brand.sh` 위반 패턴 + exemption zone retarget

**도구**: Edit / Write tool

**검증**:
- `bash scripts/check-brand.sh` exit 0
- `cat .moai/project/brand/style-guide.md | head -10` 의 frontmatter `id: brand-style-guide-mink`

**Commit type/message template** (CLAUDE.local.md §2.2):
```
docs(brand): SPEC-MINK-BRAND-RENAME-001 Phase 1 — style-guide 재작성 + brand-lint retarget + baseline 캡처

- .moai/project/brand/style-guide.md v2.0.0 (MINK 규범)
- scripts/check-brand.sh 위반 패턴 retarget: AI.GOOSE, GOOSE-AGENT, Goose 프로젝트
- exemption zone 확장: SPEC-GOOSE-*, IDEA-*, agent-memory, ## HISTORY, fenced code
- migration-log.md Baseline 캡처: go.mod, package counts, ls cmd/, agent-memory SHA-256 hash

SPEC: SPEC-MINK-BRAND-RENAME-001
REQ:  REQ-MINK-BR-001, REQ-MINK-BR-006, REQ-MINK-BR-012, REQ-MINK-BR-016
AC:   AC-MINK-BR-001, AC-MINK-BR-007
```

**Rollback**: `git reset --hard HEAD~1` (이 phase 만 reset, 다음 phase 가 의존하지 않으면 안전).

**Risk**: 낮 — 신규 파일 + script retarget만, 기존 컴파일 영향 0.

### Phase 2 — Go module path rename (ATOMIC)

[HARD] 본 phase 는 반드시 **단일 commit / atomic transaction**으로 수행한다. Partial rename 은 컴파일 break.

**대상**:
- `go.mod` 첫 줄: `module github.com/modu-ai/goose` → `module github.com/modu-ai/mink`
- `.go` 파일 458개 (test 포함, vendor 제외)의 `github.com/modu-ai/goose` import statements 958 라인 일괄 치환

**도구 (안전 순)**:
1. **`go mod edit -module=github.com/modu-ai/mink`** — go.mod 만 안전하게 수정
2. **`gofmt -r 'github.com/modu-ai/goose -> github.com/modu-ai/mink'`** — Go AST 인식 안 함 (gofmt -r 은 표현식 rewrite용), fallback to step 3
3. **`find . -type f -name '*.go' -not -path './vendor/*' -exec sed -i.bak 's|"github.com/modu-ai/goose|"github.com/modu-ai/mink|g; s|"github.com/modu-ai/goose"|"github.com/modu-ai/mink"|g' {} +`** — 안전한 패턴: import 문 시작의 `"github.com/modu-ai/goose` 만 매치
4. **`find . -name '*.go.bak' -delete`** — backup 파일 정리
5. **`go mod tidy`** — go.sum 재정렬 (필요 시)

**검증** (commit 전 필수):
- `head -1 go.mod` = `module github.com/modu-ai/mink`
- `grep -rln 'github.com/modu-ai/goose' --include='*.go' | wc -l` = 0
- `grep -rln 'github.com/modu-ai/mink' --include='*.go' | wc -l` = 456 (baseline 카운트)
- `go build ./...` exit 0
- `go vet ./...` exit 0
- `gofmt -l .` 출력 빈 줄

**Commit**:
```
refactor(module): SPEC-MINK-BRAND-RENAME-001 Phase 2 — Go module path 변경 (goose → mink, atomic)

- go.mod: module github.com/modu-ai/mink
- 458 .go 파일 import 일괄 치환 (958 라인)
- go.sum 변경 0건 (self-module은 sum 미등록)
- go build/vet/test 모두 통과

[HARD] 본 commit 은 atomic — partial rename은 컴파일 break. revert 시 단일 commit 전체 revert.

SPEC: SPEC-MINK-BRAND-RENAME-001
REQ:  REQ-MINK-BR-003, REQ-MINK-BR-015, REQ-MINK-BR-021
AC:   AC-MINK-BR-002
```

**Rollback**: `git revert <Phase2-commit>` — atomic revert 가능 (단일 commit).

**Risk**: 매우 높 (atomicity 위반 시 컴파일 break). Mitigation: commit 전 6개 검증 명령 통과 강제.

### Phase 3 — Go 식별자 rename

**대상**:
- `GooseHome` → `MinkHome` (64건, 주로 `internal/config/config.go` struct field + 모든 test setup)
- doc-comment `Goose` brand-position → `Mink` (16건, 주로 `internal/agent/`, `internal/learning/trajectory/`, `internal/credproxy/`, `internal/messaging/telegram/`)
- test fixture `"You are Goose..."` → `"You are Mink..."` (2건, `internal/learning/trajectory/{coverage_test.go, redact/rules_test.go}`)

**도구 (안전 순)**:
1. **`gofmt -r 'GooseHome -> MinkHome'`** 또는 `gopls rename` — AST 인식 도구 우선
2. fallback to sed: `find . -name '*.go' -exec sed -i.bak 's/\bGooseHome\b/MinkHome/g' {} +`
3. doc-comment 정정: 수동 Edit (각 파일 review)
4. `go vet ./...` + `go test ./internal/config/...`

**검증**:
- `grep -rEohw 'GooseHome' --include='*.go' | wc -l` = 0
- `grep -rEohw 'MinkHome' --include='*.go' | wc -l` ≥ 64
- `grep -rn '"You are Goose' --include='*.go' | wc -l` = 0
- `go build ./...` exit 0, `go vet ./...` exit 0
- `go test ./internal/config/...` exit 0

**Commit**:
```
refactor(identifiers): SPEC-MINK-BRAND-RENAME-001 Phase 3 — Go 식별자 rename (GooseHome → MinkHome 등)

- GooseHome → MinkHome (64건, struct field + 모든 test setup)
- doc-comment Goose → Mink (16건)
- test fixture "You are Goose..." → "You are Mink..." (2건)
- gofmt -r 우선, sed fallback

SPEC: SPEC-MINK-BRAND-RENAME-001
REQ:  REQ-MINK-BR-009
AC:   AC-MINK-BR-010
```

**Rollback**: `git revert <Phase3-commit>`. 단, GooseHome ↔ MinkHome 식별자는 외부 참조 없으므로 안전.

**Risk**: 중 — gofmt -r 도구 사용 시 안전. sed fallback 시 단어 경계 (`\b`) 확실히 사용해야 false match 방지 (예: `GooseHomes` 같은 누락된 식별자가 있다면 검증 단계에서 미수정 잔존 가능 — Phase 3 검증 명령에 grep 잔존 0 확인 강제).

### Phase 4 — proto package + generated code

[HARD] 본 phase 도 **단일 commit / atomic** — proto 변경 + 생성 코드 + import statement 일치.

**대상**:
- `proto/goose/` → `proto/mink/` (디렉토리 rename, `git mv`)
- 4개 `.proto` 파일: `package goose.v1;` → `package mink.v1;`
- 4개 `.proto` 파일: `option go_package = "github.com/modu-ai/mink/internal/transport/grpc/gen/minkv1;minkv1";` (module path 는 Phase 2 결과 일관성)
- `buf.gen.yaml`: `out: internal/transport/grpc/gen/goosev1` → `out: internal/transport/grpc/gen/minkv1` (3 plugin 모두)
- `buf.yaml`: 변경 없음 (`modules.path: proto` 그대로)
- 생성 코드 재생성: `make proto-generate` → `internal/transport/grpc/gen/minkv1/` 신설
- 옛 디렉토리 `internal/transport/grpc/gen/goosev1/` 삭제 (`git rm -r`)
- Phase 2 결과로 이미 import path 는 `github.com/modu-ai/mink/...` 인데 가리키는 끝 prefix `goosev1` 부분은 별도 sed: `find . -name '*.go' -exec sed -i.bak 's|/internal/transport/grpc/gen/goosev1|/internal/transport/grpc/gen/minkv1|g; s|\bgoosev1\b|minkv1|g' {} +`

**도구 순서**:
1. `git mv proto/goose proto/mink`
2. `sed -i.bak 's/package goose\.v1/package mink.v1/g' proto/mink/v1/*.proto`
3. `sed -i.bak 's|gen/goosev1;goosev1|gen/minkv1;minkv1|g' proto/mink/v1/*.proto`
4. Edit `buf.gen.yaml` (3 plugin out)
5. `git rm -r internal/transport/grpc/gen/goosev1`
6. `make proto-generate`
7. `find . -name '*.go' -exec sed -i.bak 's|/gen/goosev1|/gen/minkv1|g; s|\bgoosev1\b|minkv1|g' {} +`
8. `go build ./...` + `go vet ./...`

**검증**:
- `find proto/ -name '*.proto'` 출력 = `proto/mink/v1/{agent,config,daemon,tool}.proto`
- `grep '^package' proto/mink/v1/*.proto` 모두 `package mink.v1;`
- `ls internal/transport/grpc/gen/` 출력 `minkv1` (옛 `goosev1` 부재)
- `head -3 internal/transport/grpc/gen/minkv1/agent.pb.go` 에 `package minkv1`
- `go build ./...` exit 0

**Commit**:
```
refactor(proto): SPEC-MINK-BRAND-RENAME-001 Phase 4 — proto package goose.v1 → mink.v1 (atomic)

- proto/goose/ → proto/mink/ (git mv)
- 4 .proto 파일: package mink.v1; option go_package=...minkv1;minkv1
- buf.gen.yaml out: ...gen/minkv1
- internal/transport/grpc/gen/goosev1/ → minkv1/ (재생성 + 옛 디렉토리 삭제)
- 모든 .go import의 goosev1 → minkv1 치환

[HARD] atomic commit — proto + generated + import 동시 일치 필수.

SPEC: SPEC-MINK-BRAND-RENAME-001
REQ:  REQ-MINK-BR-008, REQ-MINK-BR-021
AC:   AC-MINK-BR-009
```

**Rollback**: `git revert <Phase4-commit>` — atomic revert. 단, 생성 코드 재생성은 revert 후 `make proto-generate` 재실행 필요 (또는 revert에 generated files 포함 시 자동).

**Risk**: 높 — proto wire-format은 영향 0이나 (R10), 생성 코드 / import / buf config 의 4-way 일관성 확인 필요.

### Phase 5 — CLI binary 디렉토리 rename

**대상**:
- `cmd/goose/` → `cmd/mink/` (`git mv`)
- `cmd/goosed/` → `cmd/minkd/` (`git mv`)
- `cmd/mink/main.go`, `cmd/minkd/main.go` 내 doc-comment + import path 정정 (Phase 2 결과 `github.com/modu-ai/mink/...` 는 이미 적용된 상태)
- `cmd/minkd/wire.go`, `cmd/minkd/integration_test.go`, `cmd/minkd/wire_test_helpers_test.go`: 동일

**도구**:
1. `git mv cmd/goose cmd/mink`
2. `git mv cmd/goosed cmd/minkd`
3. `find cmd/mink cmd/minkd -name '*.go' -exec sed -i.bak 's|\bgoosed\b|minkd|g; s|GOOSE\b|MINK|g' {} +` (cmd 디렉토리 내 brand 토큰 정정, doc-comment 위주)
4. `find cmd -name '*.go.bak' -delete`
5. `go build ./cmd/...`

**검증**:
- `ls cmd/` 출력 = `mink\nminkd`
- `cat cmd/mink/main.go` import = `github.com/modu-ai/mink/internal/cli`
- `cat cmd/minkd/main.go` doc-comment: `// minkd는 MINK의 핵심 데몬 프로세스다.` 와 같은 패턴
- `go build ./cmd/mink` 가 `mink` binary 생성
- `go build ./cmd/minkd` 가 `minkd` binary 생성

**Commit**:
```
refactor(cmd): SPEC-MINK-BRAND-RENAME-001 Phase 5 — CLI binary 디렉토리 rename

- cmd/goose/ → cmd/mink/ (git mv)
- cmd/goosed/ → cmd/minkd/ (git mv)
- cmd/{mink,minkd}/*.go: brand 토큰 정정 (goosed → minkd, GOOSE → MINK, doc-comment)

SPEC: SPEC-MINK-BRAND-RENAME-001
REQ:  REQ-MINK-BR-007
AC:   AC-MINK-BR-008
```

**Rollback**: `git revert <Phase5-commit>`.

**Risk**: 중 — git mv 후 import path 미일관 시 build 실패. Phase 2 + Phase 5 검증 명령으로 catch.

### Phase 6 — CI/workflow + brand-lint sanity

**대상**:
- `.github/workflows/{ci,brand-lint,release-drafter}.yml`: `modu-ai/goose` 하드코딩 0건 — 변경 0건 (Phase 4.3 baseline 일치)
- `.github/PULL_REQUEST_TEMPLATE.md`: `modu-ai/goose` → `modu-ai/mink`
- `.github/ISSUE_TEMPLATE/{bug_report.yml,config.yml,feature_request.yml}`: URL 갱신 (`https://github.com/modu-ai/mink/...`)
- `Makefile` brand-lint target: 변경 0건
- 본 phase 의 commit 자체가 `bash scripts/check-brand.sh` exit 0 보장

**도구**: Edit (manual review per file)

**검증**:
- `grep -rn 'modu-ai/goose' .github/workflows/` = 0건
- `grep -rn 'modu-ai/goose' .github/ISSUE_TEMPLATE/` = 0건
- `bash scripts/check-brand.sh` exit 0 (Phase 6 commit 후)
- 의도적 brand 위반 시험 PR 으로 CI gate 차단 확인 (Phase 6 commit 이후 별도 시험 commit, AC-MINK-BR-007 점검)

**Commit**:
```
chore(ci): SPEC-MINK-BRAND-RENAME-001 Phase 6 — CI workflows + brand-lint sanity

- .github/PULL_REQUEST_TEMPLATE.md: modu-ai/mink
- .github/ISSUE_TEMPLATE/{bug_report,config,feature_request}.yml: modu-ai/mink URLs
- workflows/*.yml: 변경 0건 (이미 ${{ github.repository }} 사용)

SPEC: SPEC-MINK-BRAND-RENAME-001
REQ:  REQ-MINK-BR-013
AC:   AC-MINK-BR-013, AC-MINK-BR-007
```

**Rollback**: `git revert <Phase6-commit>`.

**Risk**: 낮.

### Phase 7 — 문서 sweep

**대상**:
- README.md (39 brand 토큰), SECURITY.md, CONTRIBUTING.md, CODE_OF_CONDUCT.md, CHANGELOG.md (header — 신규 entry만 MINK 표기)
- CLAUDE.md (brand 도입 인사말 신설 가능), CLAUDE.local.md (h1 정정)
- `.moai/project/*.md` (9 파일, brand-position 토큰 정정)
- `.moai/project/brand/{README.md,brand-voice.md,target-audience.md,visual-identity.md}` (brand 산문 갱신)
- `.moai/project/brand/logo/*.md` (3 파일)
- `.moai/project/codemaps/*.md` (3 파일)
- `.moai/project/research/*.md` (4 파일)
- `.claude/settings.local.json` (사용자 로컬, 선택 검토)

**도구**: Edit (manual per file, 도메인 용어 보존 위해 자동 sed 금지). 단, `AI.GOOSE` → `MINK` 같은 명확한 brand-position 토큰은 batch sed 가능: `grep -rEln '\bAI\.GOOSE\b' .moai/project/ | xargs sed -i.bak 's/\bAI\.GOOSE\b/MINK/g'`.

**검증**:
- `bash scripts/check-brand.sh` exit 0
- `grep -rEln '\bAI\.GOOSE\b' .moai/project/` 출력 0 파일
- `grep -rEln '\bGOOSE-AGENT\b' .moai/project/` 출력 0 파일
- README.md h1 `# MINK` (또는 동등)

**Commit**:
```
docs(brand): SPEC-MINK-BRAND-RENAME-001 Phase 7 — 문서 sweep (README, .moai/project, CLAUDE, etc.)

- README.md: h1 MINK, badges modu-ai/mink, install snippets cmd/mink
- SECURITY.md, CONTRIBUTING.md, CODE_OF_CONDUCT.md: brand 정정
- CLAUDE.local.md: h1 MINK Project
- .moai/project/*.md (9 파일): AI.GOOSE → MINK
- .moai/project/brand/* (4 파일): 산문 갱신
- .moai/project/codemaps/*, research/*: brand 토큰 정정

SPEC: SPEC-MINK-BRAND-RENAME-001
REQ:  REQ-MINK-BR-001, REQ-MINK-BR-002, REQ-MINK-BR-014
AC:   AC-MINK-BR-012, AC-MINK-BR-016, AC-MINK-BR-017
```

**Rollback**: `git revert <Phase7-commit>`.

**Risk**: 낮 — 산문만 변경, 컴파일 영향 0.

### Phase 8 — 코드 내 user-facing 문자열

**대상**:
- `.go` 파일 27건의 `AI.GOOSE` 토큰 → `MINK`
- log message, error message, CLI help text, doc-comment 의 brand-position
- `internal/command/builtin/clear.go`, `internal/command/*.go`, `internal/plugin/errors.go`, `internal/skill/schema.go`, `internal/mcp/types.go`, `internal/tools/web/doc.go` 등 27개 파일

**도구**:
1. `grep -rln 'AI\.GOOSE' --include='*.go' | xargs sed -i.bak 's/\bAI\.GOOSE\b/MINK/g'`
2. 도메인 용어 (예: `// Package command implements the slash command system for MINK.`)는 자동 결과 검토
3. `go vet ./...` + `go test ./...`

**검증**:
- `grep -rn 'AI\.GOOSE' --include='*.go' | wc -l` = 0
- `grep -rn '\bMINK\b' --include='*.go' | wc -l` ≥ 27
- `go vet ./...` exit 0

**Commit**:
```
docs(brand): SPEC-MINK-BRAND-RENAME-001 Phase 8 — Go 코드 내 user-facing 문자열 정정

- 27 .go 파일: AI.GOOSE → MINK (doc-comment 위주)
- log message, error message, CLI help, doc-comment brand-position 정정
- 식별자 (Phase 3) / module path (Phase 2) / binary (Phase 5) / proto (Phase 4) 미간섭

SPEC: SPEC-MINK-BRAND-RENAME-001
REQ:  REQ-MINK-BR-001, REQ-MINK-BR-002
AC:   AC-MINK-BR-011
```

**Rollback**: `git revert <Phase8-commit>`.

**Risk**: 중 — sed 자동 치환 결과의 false positive 가능성. mitigation: 검증 단계의 `go vet` 통과 강제.

### Post-merge — GitHub repo rename + 선행 SPEC supersede commit

이 작업은 본 SPEC PR squash merge **이후** 별도 orchestrator 단독 작업 (PR 외):

1. `gh repo rename mink --repo modu-ai/goose` — 1회 실행
2. 로컬 clone (`/Users/goos/MoAI/AI-Goose`, `/Users/goos/moai/ai-goose`) 의 `git remote set-url origin https://github.com/modu-ai/mink.git`
3. README badge URL 4개의 `modu-ai/goose` → `modu-ai/mink` (or `${{ github.repository }}` parameterized — 별도 PR로도 가능)
4. **선행 SPEC supersede commit** (orchestrator 담당):
   - `.moai/specs/SPEC-GOOSE-BRAND-RENAME-001/spec.md` frontmatter: `status: completed` → `status: superseded`
   - 동일 파일 `## HISTORY` 표에 1행 추가:
     ```
     | 0.1.2 | 2026-05-12 | manager-spec | Superseded by SPEC-MINK-BRAND-RENAME-001 — all OUT-scope items (Go module / repo / SPEC ID / binary / proto / code identifiers) now flipped to IN-scope. Body content remains immutable. |
     ```
   - body 다른 byte는 변경 0건
   - commit type `docs(spec): SPEC-GOOSE-BRAND-RENAME-001 supersede 표시`
5. AC-MINK-BR-003 + AC-MINK-BR-004 검증

### PR 분량 정책

- 기본: 8 phase 가 하나의 PR 안에 multi-commit 으로 합쳐지고, squash merge (CLAUDE.local.md §1.4 — feature branch 는 squash). 단일 squash commit 의 본문에 8 phase 의 commit message 요약을 포함.
- 분량 부담 시: Phase 2 (atomic, ~458 파일 변경) + Phase 4 (atomic) + Phase 3 (~80 변경) + Phase 5 (~10 파일 변경) + Phase 8 (27 파일) + Phase 7 (문서, 30+ 파일) 의 분포로 squash diff 가 ~600+ 파일. review 부담 시 다음 분할 옵션:
  - Option A (권장): 본 8 phase 를 1 PR squash merge. review 는 phase 별 commit 으로 따라가기.
  - Option B (분할): 1 PR = Phase 1-2 (style-guide + atomic module rename), 2 PR = Phase 3-5 (식별자 + proto + binary), 3 PR = Phase 6-8 (CI + 문서 + 코드 문자열). 단, Phase 2 commit 이 partial-rename break 위험을 다른 phase 와 격리하므로 분할 시에도 Phase 2 만은 단일 PR 안에 atomic 보장.

---

## 7. MINK Brand Style Guide (canonical reference draft)

본 §7 은 `.moai/project/brand/style-guide.md` v2.0.0 의 **canonical 사본**이다. 변경 시 본 §7 과 style-guide.md 를 같은 PR 안에서 동시 수정해야 하며, brand-lint workflow 가 §7.1 표의 byte-level 동일성을 검증한다 (선행 SPEC R8 의 dual-source drift 방지 정책 계승).

### 7.1 표기 규범 (3 영역)

| 컨텍스트 | 표기 | 적용 영역 | 예시 |
|---------|------|----------|------|
| 공식 브랜드명 / user-facing prose | `MINK` | 모든 문서, README 제목, CHANGELOG 신규 entry, CLAUDE.md 인사말, CLI 환영 메시지, 에러 메시지 도입부 | "MINK는 너의 매일을 기억하는 AI다." |
| 짧은 약칭 / 코드 식별자 / 도메인 용어 | `mink` | `mink CLI`, `minkd daemon`, type/func/var 이름 (Go 식별자) | `package mink`, `type MinkRuntime`, `cmd/mink` |
| URL slug / GitHub repo / 도메인 | `mink` | 미래 도메인 (`mink.dev` 등), URL slug | `mink.dev`, `https://docs.mink.dev/`, `github.com/modu-ai/mink` |

> 주의: 선행 SPEC 의 dual-representation 은 brand (대문자 + 점) / 식별자 (소문자) / slug (kebab) 3-way 분리였으나, MINK 는 brand=`MINK` / 식별자=`mink` / slug=`mink` 의 **2-way 분리** (대소문자만 차이). 표기 결정이 단순해진다.

### 7.2 Dual Representation 원칙

산문에서 brand 로 가리킬 때는 `MINK`, 코드/식별자로 가리킬 때는 `mink` 를 사용한다. 백틱 (`)으로 감싼 식별자는 brand-naming 규칙에서 제외된다.

예:
- 좋은 예: `` MINK는 `mink CLI`로 실행됩니다. ``
- 나쁜 예: `` Mink는 mink CLI로 실행됩니다. `` (brand 표기 잘못 — Title case Mink 는 brand-position 에서 금지, 식별자 백틱 누락)

### 7.3 한국어 / 영어 예시 (i18n 일반 정책 포함)

| 한국어 | 영어 |
|--------|------|
| `MINK 프로젝트` | `the MINK project` |
| `MINK는 ...입니다.` | `MINK is ...` |
| `` `mink CLI` 명령어 `` | `` the `mink CLI` command `` |
| `Welcome to MINK` | `Welcome to MINK` |
| `매일 아침, 매일 저녁, 너의 MINK.` | `Your AI that says good morning, every morning.` |

i18n 일반 정책: 다른 언어 (일본어, 중국어, 영어 외)가 추후 도입되더라도 동일 dual representation 원칙 (brand=`MINK` / 식별자=`mink` / slug=`mink`)을 그대로 적용한다. 신규 언어 추가 자체는 본 SPEC OUT scope.

### 7.4 후속 SPEC 작성자 참조

후속 SPEC 을 작성하는 manager-spec agent / 인간 작성자는 다음을 의무 준수한다:

- SPEC template 또는 plan 단계에서 `.moai/project/brand/style-guide.md` 참조 링크 포함
- brand 표기 시 `MINK` 사용 (REQ-MINK-BR-001)
- 코드 식별자 / 도메인 용어는 `mink` (REQ-MINK-BR-002)
- URL / 도메인 slug 는 `mink` (REQ-MINK-BR-025)
- 신규 SPEC 디렉토리 이름은 `SPEC-MINK-XXX-NNN` (REQ-MINK-BR-005)
- 기존 `SPEC-GOOSE-XXX-NNN` 디렉토리에 cross-reference 시 그대로 (immutable archive)

### 7.5 brand-lint 검증 알고리즘 (`scripts/check-brand.sh` 동작 명세)

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
   - 선행 SPEC (`.moai/specs/SPEC-GOOSE-BRAND-RENAME-001/`) — immutable
3. **위반 검출 (코드 영역 외부)**: PCRE2 또는 Python regex 로 다음 패턴 검출:
   - `goose 프로젝트`, `Goose 프로젝트`, `GOOSE 프로젝트` (한국어 brand-position)
   - `goose project`, `Goose project` (영문 brand-position)
   - `GOOSE-AGENT` (옛 brand 약칭)
   - `\bAI\.GOOSE\b` (선행 SPEC 결과의 brand 표기 잔존)
4. **분류 (A) vs (D) 판정**: 코드 영역 내부 `` `mink CLI` ``, `` `minkd daemon` ``, `` `goose` `` (옛 binary 인용), `` `github.com/modu-ai/goose/...` `` (옛 module path 인용 in HISTORY/changelog) 등은 도메인 용어 (D) 또는 immutable archive 로 보존하고 위반에서 제외.
5. **HISTORY 보존 검증 (append-only)**: 모든 SPEC 의 `## HISTORY` 섹션 내부에서 **baseline 행** (row-index가 baseline 캡처 시점에 존재했던 행) 의 byte-level 변경이 발생했는지 검증. baseline 행 이후에 **새 행이 추가**된 경우 (append-only) 는 허용 — predecessor SPEC supersede transition 등 status-event 기록을 위함 (REQ-MINK-BR-020 의 modify 정의에서 제외). baseline 행의 cell 내용이 변경된 경우 즉시 fail.
6. **agent-memory 보존 검증**: `.claude/agent-memory/**` 의 SHA-256 hash 가 baseline 과 byte-identical (REQ-MINK-BR-016 + AC-MINK-BR-014).
7. **출력**: 위반이 0건이면 exit 0, 위반이 있으면 exit 1 + file path / line number / offending pattern 보고.
8. **언어 선택**: Python 스크립트 우선 (마크다운 파서 활용 용이 — 선행 SPEC 의 162 라인 Python-in-bash 스크립트 계승).

GitHub Actions workflow `.github/workflows/brand-lint.yml` 은 위 스크립트를 PR trigger 에서 실행하며, exit 1 시 merge 차단 (REQ-MINK-BR-013).

---

## 8. Dependencies

### 8.1 선행 SPEC

- **SPEC-GOOSE-BRAND-RENAME-001** (status: completed → superseded by 본 SPEC). 본 SPEC 이 supersede. 자산 4개 (style-guide.md / check-brand.sh / brand-lint.yml / Makefile target) 재사용.

### 8.2 외부 의존

- **GitHub repo rename API**: `gh repo rename` (Phase post-merge 작업)
- **Go module proxy** (`proxy.golang.org`): rename 후 캐시 stale ~수시간 가능. pre-public 상태라 무시 가능.
- **`buf` CLI**: proto generator. 본 프로젝트는 `make proto-generate` 가 `go run github.com/bufbuild/buf/cmd/buf` 호출 — 외부 protoc 설치 불필요.
- **`gh` CLI**: GitHub CLI, repo rename 실행.

### 8.3 Tooling

- **Go toolchain** (1.26+): `go build`, `go vet`, `go test`, `go mod edit`, `go mod tidy`, `gofmt -l`, `gofmt -r`
- **`gh` CLI**: repo rename (post-merge)
- **`make`**: brand-lint target, proto-generate target
- **`grep` / `ripgrep`**: inventory + verification
- **`sed`** (GNU 또는 BSD): macOS BSD sed 는 `-i.bak` 명시 필요 (CLAUDE.md §14 platform compatibility — `Edit tool over sed/awk` 권장이나 batch import path 치환은 sed 가 효율적)

### 8.4 Downstream-affected SPECs (informational, 본 SPEC scope 외)

- `SPEC-MINK-PRODUCT-V7-001` (next): product.md v7.0 (별도 SPEC, IDEA-002 §4.1 wave item 2)
- `SPEC-MINK-DISTANCING-STATEMENT-001` (next): block/goose distancing statement (IDEA-002 §4.1 wave item 3)
- `SPEC-MINK-USERDATA-MIGRATE-001` (downstream): `./.goose/` → `./.mink/` 실제 마이그레이션 logic. 본 SPEC §3.1 item 4 footnote 에서 분리 명시.
- `SPEC-MINK-ENV-MIGRATE-001` (downstream): `GOOSE_*` 21개 env var → `MINK_*` deprecation alias loader. 본 SPEC §3.1 item 12 footnote 에서 분리 명시.
- 모든 미래 SPEC: `SPEC-MINK-*` prefix 사용 (REQ-MINK-BR-005).

---

## 9. Risks & Mitigations

| # | 리스크 | 가능성 | 영향 | Mitigation |
|---|--------|--------|------|------|
| R1 | Partial rename (go.mod + import statement 의 일부 누락) 으로 인한 컴파일 break | 높 | 매우 높 | Phase 2 의 atomic commit 정책 (REQ-MINK-BR-021). commit 전 6개 검증 명령 (head -1 go.mod, grep counts, go build/vet/test) 강제. CI gate (`brand-lint` + `go build`) 차단. |
| R2 | 개발자 로컬 clone drift (옛 module path 캐시 / 옛 remote URL) | 중 | 낮 | post-rename checklist 를 README + 본 SPEC PR description 에 명시: (1) `git remote set-url`, (2) `go clean -modcache`, (3) `~/.bashrc` alias 갱신. |
| R3 | third-party dep 이 `github.com/modu-ai/goose` import 함 | 매우 낮 | 낮 | Pre-rename 검증 (§research §4.3): `grep 'goose' go.mod | grep -v 'modu-ai/goose'` → 0건. 외부 consumer 부재. |
| R4 | CI workflow hardcoded refs 잔존 | 낮 | 낮 | Pre-rename 검증 (§research §5.4): `grep -rn 'modu-ai/goose' .github/workflows/` → 0건. workflows 는 `${{ github.repository }}` 기반. |
| R5 | Go module proxy `proxy.golang.org` 캐싱 stale | 낮 | 매우 낮 | pre-public 단계라 외부 consumer 부재. 본인 환경: `go clean -modcache` 또는 `GOPROXY=direct`. |
| R6 | SPEC-GOOSE-* 디렉토리 경로 참조가 코드/docs 내 잔존 | 중 | 낮 (정상 동작) | 본 SPEC §3.2 item 1 OUT scope (preserve). brand-lint exemption zone (REQ-MINK-BR-016). 정상 동작은 영향 없음. |
| R7 | 선행 SPEC supersede commit timing 충돌 | 낮 | 낮 | 본 SPEC PR squash 가 선행 SPEC body 의 어떤 byte도 건드리지 않음 (AC-MINK-BR-004). frontmatter status / HISTORY 1 row 추가는 **별도 후속 commit** (orchestrator 담당). AC-MINK-BR-006 의 baseline-vs-post diff 가 검증. |
| R8 | User-data 경로 마이그레이션 누락 → 1차 launch 시 기존 사용자 데이터 손실 | 중 | 중 | 본 SPEC 은 정책만 정의 (§3.1 item 4 footnote, REQ-MINK-BR-024, AC-MINK-BR-015). 실제 fallback 읽기 + 1회 마이그레이션 logic 은 별도 SPEC `SPEC-MINK-USERDATA-MIGRATE-001`. 본 SPEC merge 전 downstream SPEC plan 작성 권장 (선행 의존). |
| R9 | env var (`GOOSE_*` 21개) 변경 → 개발자 환경 break | 중 | 중 | 새 `MINK_*` env var 신설 + 옛 `GOOSE_*` deprecated alias loader. 본 SPEC 은 정책만 (§3.1 item 12 footnote, REQ-MINK-BR-027). 실제 loader 는 별도 SPEC `SPEC-MINK-ENV-MIGRATE-001`. |
| R10 | proto package rename 시 wire-format 호환성 | 매우 낮 | 매우 높 | proto package 이름은 wire-format 영향 0 (wire 는 field number 기반). package 이름은 generated Go 코드의 import path / type qualified name 만 영향. Phase 4 의 atomic commit (REQ-MINK-BR-008 + REQ-MINK-BR-021) 으로 proto + generated + import 일치 강제. |
| R11 | brand "mink" 의 모피 산업 연상 | 매우 낮 | 매우 낮 | IDEA-002 risk assessment 에서 AI context disambiguation 으로 정리. logo direction 은 text-only minimalist (별도 SPEC). 본 SPEC scope 외 (§11 item 3). |
| R12 | 본 SPEC PR squash 가 거대해 review 부담 | 높 | 중 | §6 PR 분량 정책에 따라 8 phase 별 commit 분리 → review 는 phase 별로. 분량 부담 시 Option B (3 PR 분할) 옵션. 단 Phase 2 의 atomic 보장은 어떤 분할에서도 유지. |
| R13 | brand "mink" + 한글 표기 ("민크") drift | 중 | 낮 | §7.3 한국어/영어 예시 표에 "MINK" 라틴 표기 우선 명시. "민크" 한글 음역은 marketing 영역 (별도 결정), 본 SPEC 은 영문 표기만 다룸. |
| R14 | dual-source drift (§7 = style-guide.md) | 중 | 중 | 선행 SPEC R8 의 정책 계승: §7 변경 시 같은 PR 안에서 style-guide.md 도 동시 수정. brand-lint workflow 가 §7.1 표 byte-level 동일성 검증 옵션 (Phase 6 검증 단계에서 검토). |

---

## 10. References

### 10.1 본 SPEC 산출물

- `.moai/specs/SPEC-MINK-BRAND-RENAME-001/spec.md` (이 문서, v0.1.0)
- `.moai/specs/SPEC-MINK-BRAND-RENAME-001/research.md` (현황 조사 + 의사결정 근거, 100+ grep counts)
- `.moai/specs/SPEC-MINK-BRAND-RENAME-001/migration-log.md` (Phase 1 baseline + 각 phase 검증 결과 기록, 구현 시점 신설)
- `.moai/project/brand/style-guide.md` v2.0.0 (frozen reference, §7 canonical 사본 — Phase 1에서 신설)

### 10.2 선행 SPEC

- `.moai/specs/SPEC-GOOSE-BRAND-RENAME-001/spec.md` v0.1.1 (2026-04-27, completed → 본 SPEC merge 후 superseded). Body content 는 immutable.
- `.moai/specs/SPEC-GOOSE-BRAND-RENAME-001/research.md` (2026-04-26)

### 10.3 결정 trail

- **IDEA-002**: GooseBot 한국시장 단타 LLM-bot 분리 결정 + AI.GOOSE → MINK rename 결정 (별도 프로젝트 `/Users/goos/Projects/GooseBot/.moai/brain/IDEA-002/` 에 ideation/proposal/research 보존)
- **사용자 8개 확정사항** (orchestrator-collected, 2026-05-12):
  1. Scope: SPEC-MINK-BRAND-RENAME-001 only (PRODUCT-V7 / DISTANCING 분리)
  2. Supersede policy: state-of-the-art reset, predecessor body immutable
  3. Branch/PR: single `plan/SPEC-MINK-BRAND-RENAME-001`, squash merge
  4. Mode: --solo
  5. SPEC ID prefix: `SPEC-MINK-*` 신규, 88 `SPEC-GOOSE-*` preserve
  6. Immutable history: 전체 보존 (HISTORY / CHANGELOG / git commit)
  7. Module/repo target: `github.com/modu-ai/mink`, `modu-ai/mink`, `mink`/`minkd`, `mink.v1`
  8. Brand assets: scaffold 재사용 retarget

### 10.4 산업 사례 (research.md §10.3 참조)

- Next.js (Next.js / next / vercel/next.js — dual-representation 산업 표준)
- Mistral AI (Mistral AI / mistral / mistral.ai)
- Bun (Bun / bun / oven-sh/bun — 짧은 영문 brand 의 disambiguation 사례)
- GitHub repo rename mechanics: <https://docs.github.com/en/repositories/creating-and-managing-repositories/renaming-a-repository>

### 10.5 본 프로젝트 기존 자료

- `go.mod`, `buf.gen.yaml`, `buf.yaml`, `Makefile`
- `.github/workflows/{ci,brand-lint,release-drafter}.yml`
- `.github/{PULL_REQUEST_TEMPLATE.md,ISSUE_TEMPLATE/*}`
- `CLAUDE.local.md` §1.3 (Team plan branch protection 활성), §1.4 (merge strategy: feature squash / release merge / hotfix squash), §2.2 (commit message convention), §2.5 (code comment language)
- `scripts/check-brand.sh` 162 라인 (Phase 1에서 retarget)
- `.moai/project/brand/style-guide.md` v1.0.0 (Phase 1에서 v2.0.0 으로 재작성)

---

## 11. Exclusions (What NOT to Build)

본 SPEC 은 다음을 **명시적으로 제외**한다. 후속 SPEC 작성자가 본 SPEC 범위 외 작업을 본 SPEC PR 에 추가하지 못하도록 차단한다.

1. **MINK domain 실제 등록** — `mink.dev`, `mink.io` 등 도메인 구매·등록은 별도 작업 (REQ-MINK-BR-025 는 정책만 정의)
2. **MINK trademark filing / 법적 등록** — 별도 외부 절차
3. **Logo / visual asset rewrite** — text-only style guide만, 시각 자산은 별도 SPEC
4. **i18n 확장 (KR/EN 외)** — 일본어, 중국어 등 추가 언어 brand 표기 정책은 별도 SPEC
5. **기존 88개 `SPEC-GOOSE-*` 디렉토리 이름 변경** — preserve (§3.2 item 1, REQ-MINK-BR-019, REQ-MINK-BR-022)
6. **선행 SPEC body 내용 수정** — immutable (§3.2 item 6, REQ-MINK-BR-010)
7. **과거 CHANGELOG entry 변경** — 발행된 release section header 이전 모든 entry 보존 (§3.2 item 4, REQ-MINK-BR-023)
8. **`SPEC-MINK-DISTANCING-STATEMENT-001`** (block / goose distancing) — 별도 SPEC, IDEA-002 §4.1 wave item 3
9. **`SPEC-MINK-PRODUCT-V7-001`** (product.md v7.0) — 별도 SPEC, IDEA-002 §4.1 wave item 2
10. **`SPEC-MINK-USERDATA-MIGRATE-001`** (`./.goose/` → `./.mink/` 실제 fallback 읽기 + 1회 마이그레이션 logic) — 별도 SPEC. 본 SPEC 은 정책만 (§3.1 item 4 footnote, REQ-MINK-BR-024).
11. **`SPEC-MINK-ENV-MIGRATE-001`** (`GOOSE_*` 21개 env var → `MINK_*` deprecation alias loader) — 별도 SPEC. 본 SPEC 은 정책만 (§3.1 item 12 footnote, REQ-MINK-BR-027).

---

Version: 0.1.0
Status: planned
Last Updated: 2026-05-12
