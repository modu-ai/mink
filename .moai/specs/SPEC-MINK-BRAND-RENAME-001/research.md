# Research — SPEC-MINK-BRAND-RENAME-001 (GOOSE → MINK 전역 rename)

> 본 research 문서는 SPEC-MINK-BRAND-RENAME-001의 의사결정 근거자료다. spec.md §10 References에서 본 문서를 참조한다. 본 SPEC은 SPEC-GOOSE-BRAND-RENAME-001 (status: completed, 2026-04-27)을 **supersede**한다.

작성일: 2026-05-12
작성자: manager-spec
Status: planned
Supersedes: SPEC-GOOSE-BRAND-RENAME-001

---

## 1. 문제 정의 (Problem Restatement)

선행 SPEC `SPEC-GOOSE-BRAND-RENAME-001` (v0.1.1, completed 2026-04-27)은 user-facing 산문의 brand 표기를 `AI.GOOSE`로 통일하는 데 성공했다. 그러나 §3.2 Scope OUT으로 다음 12개 항목을 **명시적으로 제외**했다:

1. Go module path (`github.com/modu-ai/goose`)
2. Go package 이름 (`package goose` 등 — 실제로는 packages 모두 기능명 기반이라 미발견, §2.5 참조)
3. Type/struct/func/var 식별자 (`GooseHome` 등)
4. CLI binary 이름 (`goose`, `goosed`, `goose-cli`, `goose-proxy`)
5. SPEC ID 네이밍 규약 (`SPEC-GOOSE-XXX-NNN`)
6. Git remote / GitHub repo (`modu-ai/goose` — 선행 SPEC §3.2 item 6는 `modu-ai/goose-agent`로 기록했으나 2026-04-27 시점 이미 `modu-ai/goose`로 운영 중. 본 SPEC은 후자 기준)
7. proto package (`goose.v1`)
8. **immutable 이력** (CHANGELOG entries, git commit messages, 모든 SPEC `## HISTORY` 표 — status 무관)
9. 도메인 인용 형태 (`` `goose CLI` ``, `` `goosed daemon` `` 등 백틱 인용)
10. logo/visual identity
11. branding.md 본문 내용
12. config 키 (확인 결과 `.moai/config/` 에 `goose` 키 0건 — §2.6 참조)

이제 사용자 의사결정(IDEA-002 / GooseBot 한국시장 LLM-bot 별도 프로젝트 분리 + 본 AI.GOOSE 컴패니언의 새 brand 결정)에 따라, **MINK** (Made IN Korea)로 전면 rename한다.

핵심 변화:
- Brand: `AI.GOOSE` → `MINK` (uppercase 우선, lowercase `mink` 허용)
- 약칭: `goose` → `mink`
- URL slug: `ai-goose` → `mink`
- Go module path: `github.com/modu-ai/goose` → `github.com/modu-ai/mink`
- GitHub repo: `modu-ai/goose` → `modu-ai/mink`
- proto package: `goose.v1` → `mink.v1`
- Binary 3종: `goose` / `goosed` / `goose-proxy` → `mink` / `minkd` / `mink-proxy`
- SPEC ID prefix (신규): `SPEC-MINK-*` (기존 88개 `SPEC-GOOSE-*`는 immutable archive)

본 SPEC은 선행 SPEC의 §3.2 OUT 항목들을 **전부 IN scope으로 뒤집어** 실행한다. 단, immutable 이력(item 7)은 그대로 유지한다.

---

## 2. 현황 조사 (Current State Inventory — concrete grep counts)

조사 시점: 2026-05-12, branch `plan/SPEC-MINK-BRAND-RENAME-001` (forked main HEAD `e76febe`).

### 2.1 Go module path & import 분포

| 항목 | 카운트 | 비고 |
|---|---|---|
| `module github.com/modu-ai/goose` 선언 | 1 | `go.mod` 첫 줄 |
| `github.com/modu-ai/goose` 가 등장하는 `.go` 파일 수 | **456** | `grep -rln 'github.com/modu-ai/goose' --include='*.go'` |
| `.go` 파일 내 총 등장 라인 수 | **958** | `grep -rn 'github.com/modu-ai/goose' --include='*.go'` |
| go.sum 내 `github.com/modu-ai/goose` 등장 | 0 | self-referencing 없음 (Go 표준) |
| `go.mod` replace directives | 0 | replace 없음 — 단순 module path 교체만으로 충분 |

서브패키지별 import path 분포 (상위 15개):

| 패키지 import | 카운트 |
|---|---|
| `github.com/modu-ai/goose/internal/llm` | 292 |
| `github.com/modu-ai/goose/internal/tools` | 132 |
| `github.com/modu-ai/goose/internal/message` | 102 |
| `github.com/modu-ai/goose/internal/query` | 63 |
| `github.com/modu-ai/goose/internal/command` | 54 |
| `github.com/modu-ai/goose/internal/cli` | 48 |
| `github.com/modu-ai/goose/internal/permission` | 41 |
| `github.com/modu-ai/goose/internal/audit` | 39 |
| `github.com/modu-ai/goose/internal/learning` | 30 |
| `github.com/modu-ai/goose/internal/transport` | 24 |
| `github.com/modu-ai/goose/internal/messaging` | 19 |
| `github.com/modu-ai/goose/internal/observability` | 17 |
| `github.com/modu-ai/goose/internal/permissions` | 15 |
| `github.com/modu-ai/goose/internal/memory` | 11 |
| `github.com/modu-ai/goose/internal/hook` | 10 |

### 2.2 Go package 선언

| 패턴 | 카운트 | 비고 |
|---|---|---|
| `^package goose$` | **0** | 선언 0건 — 선행 SPEC §3.2 item 2의 가정과 다름 |
| `^package goosed` | 0 | `cmd/goosed/main.go` 는 `package main` |
| `^package goose` (모든 형태) | 0 | `goosev1` (생성 코드)는 별도 |

총 unique package 선언은 `adapter` / `agent` / `audit` / `bridge` / `builtin` / `cli` / `cmd` / `core` / `credential` / `credproxy` / 등 **기능명 기반**이며 `goose` prefix는 사용하지 않는다. 따라서 `package goose` → `package mink` 변환 대상은 **0건**이다.

생성 코드 `package goosev1` (위치: `internal/transport/grpc/gen/goosev1/`)는 proto의 `option go_package` 지시 결과이며 proto rename 시 자동 변경된다 (§2.7 참조).

### 2.3 Go 식별자 (Goose* CamelCase)

| 패턴 | 카운트 | 비고 |
|---|---|---|
| `^type Goose` (top-level) | **0** | 선행 SPEC §3.2 item 3의 `type Goose*` 가정은 실제 부재 |
| `Goose` (모든 토큰 형태) | **80** | `GooseHome` (64) + 단어 `Goose` (16) |
| `^func Goose` | 0 | top-level Goose-prefixed func 없음 |
| `^func .*Goose` | 7 | receiver method `(x *X) Goose...()` 형태 — 대부분 `GooseHome` 관련 |
| `^var Goose` / `^const Goose` | 0 | top-level Goose 변수 없음 |

가장 일관되게 등장하는 Goose-prefixed 식별자: **`GooseHome`** (config field, env var key 모두)
- struct field: `internal/config/config.go:164` `GooseHome string`
- env var key: `GOOSE_HOME` (§2.4 참조)
- test setup: `GooseHome: gooseHome,` 패턴 64회

기타 brand-position `Goose` 단어 사용:
- `internal/learning/trajectory/*` doc-comment: "Goose self-evolution pipeline"
- `internal/agent/chat.go` doc-comment: "Chat with the Goose agent"
- `internal/credproxy/keyring.go` doc-comment: "credential proxy for the Goose agent runtime"
- `internal/messaging/telegram/*` doc-comment: "Goose AI backend", "Goose-side identifier"
- 그 외 test fixtures: `"You are Goose. Email support@goose.ai for help."` 2건 (테스트 prompt)

### 2.4 GOOSE_* 환경 변수

총 unique env var 키 **21개** (모두 brand-position):

```
GOOSE_AUTH_         (prefix root)
GOOSE_AUTH_REFRESH
GOOSE_AUTH_TOKEN
GOOSE_CONFIG_STRICT
GOOSE_GRPC_BIND
GOOSE_GRPC_MAX_RECV_MSG_BYTES
GOOSE_GRPC_PORT
GOOSE_GRPC_REFLECTION
GOOSE_HEALTH_PORT
GOOSE_HISTORY_SNIP
GOOSE_HOME
GOOSE_HOOK_NON_INTERACTIVE
GOOSE_HOOK_TRACE
GOOSE_KIMI_REGION
GOOSE_LEARNING_ENABLED
GOOSE_LOCALE
GOOSE_LOG_LEVEL
GOOSE_METRICS_ENABLED
GOOSE_QWEN_REGION
GOOSE_SHUTDOWN_TOKEN
GOOSE_TELEGRAM_BOT_TOKEN
```

이 21개 키가 등장하는 총 라인 수: **824** (`grep -rEohw 'GOOSE[A-Z_]*' --include='*.go' | wc -l`).

env var 변경은 **breaking change** — 운영 중인 개발자 환경의 `~/.bashrc` / `~/.zshrc` / `.env` 파일에 영향. 마이그레이션 정책 결정 필요 (§7 R8 참조).

### 2.5 `.go` 파일 규모

| 항목 | 카운트 |
|---|---|
| 총 `.go` 파일 (vendor 제외) | 912 |
| 비-test `.go` | 496 |
| `_test.go` | 416 |
| `internal/` 서브디렉토리 (패키지) | 139 |
| `cmd/` 서브디렉토리 | 2 (`goose/`, `goosed/`) |

`cmd/goose-proxy/` 디렉토리는 **현재 부재** — 선행 SPEC 작성 시점에 binary 3종 계획이 있었으나 (README L63, SECURITY.md L49) 아직 미구현. 본 SPEC은 미래 binary까지 포함하여 rename 정책을 수립하되 실제 디렉토리 rename은 현존하는 `cmd/goose/` + `cmd/goosed/` 2개에 한정한다.

### 2.6 .moai/config/ 키

`grep -rln 'goose\|Goose\|GOOSE' .moai/config/` → **0건**.

선행 SPEC §3.2 item 12의 "config 파일의 `goose` 키" 우려는 실제로는 부재. config rename 작업 없음.

### 2.7 proto 파일

| 항목 | 카운트 | 비고 |
|---|---|---|
| `.proto` 파일 수 | 4 | `proto/goose/v1/{tool,config,agent,daemon}.proto` |
| `package goose.v1;` 선언 | 4 (파일당 1회) | |
| `option go_package = "github.com/modu-ai/goose/internal/transport/grpc/gen/goosev1;goosev1";` | 4 | |
| 생성 코드 디렉토리 | `internal/transport/grpc/gen/goosev1/` | 10+ `.pb.go` / `_grpc.pb.go` 파일 |

proto package rename은 다음을 연쇄 변경시킨다:
- `proto/goose/v1/` → `proto/mink/v1/` (디렉토리)
- `package goose.v1;` → `package mink.v1;` (4 파일)
- `option go_package = "...goosev1;goosev1"` → `option go_package = "...minkv1;minkv1"`
- 생성 코드 디렉토리: `internal/transport/grpc/gen/goosev1/` → `internal/transport/grpc/gen/minkv1/`
- `package goosev1` → `package minkv1` (생성 .pb.go 헤더)
- `buf.gen.yaml` 의 `out: internal/transport/grpc/gen/goosev1` 변경
- 모든 import: `github.com/modu-ai/goose/internal/transport/grpc/gen/goosev1` → `github.com/modu-ai/mink/internal/transport/grpc/gen/minkv1` (이 변경은 §2.1 module-path 변경의 부수효과)

### 2.8 .moai/project/ user-facing 문서 (선행 SPEC 적용 후 현재 상태)

| 파일 | `goose/Goose/GOOSE` 등장 횟수 | `AI.GOOSE` 등장 |
|---|---|---|
| `.moai/project/product.md` | 93 | 다수 (선행 SPEC §6 Phase 2로 정정 완료) |
| `.moai/project/branding.md` | 61 | 다수 |
| `.moai/project/tech.md` | 54 | 다수 |
| `.moai/project/learning-engine.md` | 49 | 다수 |
| `.moai/project/structure.md` | 44 | 다수 |
| `.moai/project/migration.md` | 43 | 다수 |
| `.moai/project/ecosystem.md` | 21 | 다수 |
| `.moai/project/adaptation.md` | 20 | 다수 |
| `.moai/project/token-economy.md` | 10 | 일부 |
| `.moai/project/research/*.md` | 28~36 (4 파일) | 일부 |
| `.moai/project/brand/style-guide.md` | 33 (대부분 규범 설명용 인용) | 본 SPEC으로 전면 재작성 대상 |
| `.moai/project/brand/README.md` | 6 | 일부 |
| `.moai/project/brand/logo/*.md` | 9 (3 파일) | 일부 |
| `.moai/project/codemaps/*.md` (3 파일) | 미측정 | `AI.GOOSE` 일부 |

`AI.GOOSE` 형태가 이미 14개 파일에 존재 (선행 SPEC §6 Phase 2 결과). 본 SPEC은 이 14개 + 위 표의 모든 `goose/Goose/GOOSE` brand-position 토큰을 **MINK / mink**로 재정정한다.

### 2.9 핵심 루트 문서

| 파일 | 카운트 | 비고 |
|---|---|---|
| `README.md` | 39 (`goose/Goose/GOOSE` + `AI.GOOSE`) | h1 `# 🪿 GOOSE`, h2 `## What is GOOSE?`, badges, install snippets 다수 |
| `CHANGELOG.md` | 33 | **대부분 immutable** — 기존 release entry는 보존 (§4.2 OUT scope) |
| `CLAUDE.md` | 0 (brand 표기 없음) | self-instructions 위주, brand-position 부재 |
| `CLAUDE.local.md` | 4 | h1 `# CLAUDE Local Instructions — GOOSE Agent Project` 등 |
| `SECURITY.md` | ~5+ | binary 3종 명시 (`goosed`, `goose`, `goose-proxy`), storage paths `~/.goose/`, `./.goose/` |
| `CONTRIBUTING.md` | ~3 | 일부 |
| `CODE_OF_CONDUCT.md` | 1 | AI.GOOSE 한 군데 |

### 2.10 .claude/ 영역

`grep -rln 'goose\|Goose\|GOOSE\|AI\.GOOSE' .claude/` → **8 파일**:

| 파일 | 비고 |
|---|---|
| `.claude/settings.local.json` | 환경 / 권한 설정 (확인 필요) |
| `.claude/agent-memory/manager-docs/MEMORY.md` | agent persistent memory — **immutable archive** (시점 기록) |
| `.claude/agent-memory/manager-docs/project_goose_v5.md` | 같음 |
| `.claude/agent-memory/manager-tdd/MEMORY.md` | 같음 |
| `.claude/agent-memory/manager-tdd/project_goose_context.md` | 같음 |
| `.claude/agent-memory/manager-tdd/project_telegram_p3.md` | 같음 |
| `.claude/agent-memory/manager-tdd/project_goose_skills.md` | 같음 |
| `.claude/agent-memory/manager-tdd/project_goose_adapter.md` | 같음 |

`.claude/agents/`, `.claude/skills/`, `.claude/commands/`, `.claude/rules/` 에는 brand-position `goose` 토큰 **0건**. 선행 SPEC §6 Phase 3 정정이 이미 완료된 상태.

`.claude/agent-memory/` 는 각 subagent의 persistent memory file로 **시점 기록** 성격이 강함. 본 SPEC은 이를 immutable archive로 분류 (§4.2 OUT scope).

`.claude/settings.local.json` 은 사용자 로컬 권한 설정이며 brand-position token이 있다면 정정 대상이나 사용자 개인 파일이라 본 SPEC 적용 후 사용자가 직접 검토 (§4.1 IN scope, optional manual review).

### 2.11 기존 SPEC 본문 (88 디렉토리 + 3 기타 + 4 flat-file)

| 항목 | 카운트 |
|---|---|
| `SPEC-GOOSE-*` 디렉토리 | **88** |
| 그 외 `SPEC-*` 디렉토리 | 3 (`SPEC-AGENCY-ABSORB-001`, `SPEC-AGENCY-CLEANUP-002`, `SPEC-MINK-BRAND-RENAME-001` (본 SPEC, 곧 생성)) |
| flat-file SPEC | 4 (`SPEC-DOC-REVIEW-2026-04-21.md`, `CMDCTX-DEPENDENCY-ANALYSIS.md`, `IMPLEMENTATION-ORDER.md`, `ROADMAP.md`) |
| `## HISTORY` 섹션을 가진 SPEC 파일 | 108 |
| 모든 SPEC HISTORY 표의 데이터 row 수 (합계) | **300** |
| `goose`-mentioning 파일 in `.moai/specs/` (전체) | 297 |
| `goose`-mentioning 파일 in `SPEC-GOOSE-*/` 하위 (preserve zone) | 292 (= 거의 전부 `SPEC-GOOSE-*` 안) |
| `goose`-mentioning 파일 outside `SPEC-GOOSE-*` | 5 (flat-file + `SPEC-AGENCY-ABSORB-001/{spec,plan}.md`) |

**핵심 관찰**: 297개 `goose`-mentioning 파일 중 292개가 `SPEC-GOOSE-*` 디렉토리 내부 — 즉 본 SPEC의 immutable preserve zone이다. 외부에 있는 5개 파일은 정정 대상.

### 2.12 코드 내 user-facing 문자열 (Go)

| 패턴 | 카운트 | 비고 |
|---|---|---|
| Go 파일 내 `AI.GOOSE` 등장 | 27 | 선행 SPEC §6 Phase 5 정정 결과 — doc-comment 위주 |
| Go 파일 내 brand-position `Goose` (string literal) | ~5 | 대부분 test fixture/prompt (`"You are Goose..."`) |
| `./.goose/` workspace path 등장 in `.go` | **117** | fsaccess test/policy, audit log, 등 |
| `~/.goose/` user-home path 등장 in `.go` | ~5 | secret storage 위치 |
| `~/.config/goose/` legacy path | ~3 | fsaccess test |

**user-data 경로 (`./.goose/`, `~/.goose/`, `~/.config/goose/`)는 별도 처리 카테고리** — 이 경로들은 사용자의 disk-on-machine 데이터를 가리키므로 단순 코드 rename으로는 부족하고 **마이그레이션 정책**이 필요하다. (§7 R8 + §11 참조)

### 2.13 GitHub repo references

`.github/` 하위 `modu-ai/goose` 또는 `modu-ai/goose-agent` 등장:

| 파일 | 라인 | 형태 |
|---|---|---|
| `.github/PULL_REQUEST_TEMPLATE.md` | 1 | `PR template for AI.GOOSE (modu-ai/goose)` |
| `.github/ISSUE_TEMPLATE/bug_report.yml` | 2 | issue search URL + advisory URL |
| `.github/ISSUE_TEMPLATE/config.yml` | 3 | discussions / security / CoC URLs |
| `.github/ISSUE_TEMPLATE/feature_request.yml` | 2 | discussions + ROADMAP link |

**`.github/workflows/*.yml` 내 `modu-ai/goose` 하드코딩 0건** — CI workflows는 `actions/checkout@v6` + `${{ github.repository }}` 기반이라 repo rename에 자동 적응. workflow 측면 영향 minimal.

`README.md` 내 badge URL: `https://img.shields.io/github/actions/workflow/status/modu-ai/goose/ci.yml?...` 형태 — 4개 badge URL 모두 `modu-ai/goose` 하드코딩. repo rename 시 GitHub redirect로 자동 동작하나 brand 정합성 위해 명시적 갱신 권장.

---

## 3. 분류 매트릭스 (Classification Matrix)

선행 SPEC §3의 5-class 분류 ((A) Brand / (B) Module Path / (C) Code Identifier / (D) Domain Term / (E) Immutable Archive)를 계승하고, "MINK-001 IN/OUT" 컬럼을 추가하여 본 SPEC의 처리 정책을 명시한다.

| Class | 정의 | 대표 예시 | 선행 SPEC 정책 | **MINK-001 IN/OUT** | 사유 |
|---|---|---|---|---|---|
| (A) Brand | 외부 노출 산문 brand 표기 | "AI.GOOSE는 Daily Companion AI" | `AI.GOOSE`로 정정 | **IN** | `MINK`로 재정정 |
| (B) Module Path / Repo | Git 인프라 식별자 | `github.com/modu-ai/goose`, `modu-ai/goose` repo | **보존** | **IN** | `github.com/modu-ai/mink`, `modu-ai/mink`로 변경 (사용자 결정사항 #7) |
| (C) Code Identifier | Go package/struct/func/var/binary | `cmd/goose`, `GooseHome`, `goosed` | **보존** | **IN** | `cmd/mink`, `MinkHome`, `minkd` 등으로 변경 |
| (D) Domain Term | 시스템 컴포넌트 약칭 | `` `goose CLI` ``, `` `goosed daemon` `` | 백틱 인용 보존 | **IN** | `` `mink CLI` ``, `` `minkd daemon` `` (재명명 후 새 binary 이름 반영) |
| (E) Immutable Archive | 시점 기록 | 모든 SPEC HISTORY rows, CHANGELOG 기존 entry, git commit msg | 변경 금지 | **OUT (유지)** | 선행 SPEC §3.2 item 7 verbatim 계승 |
| (F) User-data Path | 디스크상 사용자 데이터 경로 | `./.goose/`, `~/.goose/`, `~/.config/goose/` | (선행 SPEC 미언급) | **IN with migration** | 새 코드는 `./.mink/`, `~/.mink/` 사용. 기존 데이터는 fallback 읽기 / 1회 마이그레이션 (§11 R8 참조) |
| (G) Env Var | 21개 `GOOSE_*` env var | `GOOSE_HOME`, `GOOSE_AUTH_TOKEN` | (선행 SPEC 미언급) | **IN with deprecation window** | `MINK_*` 신설 + `GOOSE_*` deprecated alias 1 release cycle 유지 (§11 R5 참조) |

class (F) (G)는 본 SPEC에서 신설된 분류로, 선행 SPEC이 다루지 않은 영역이며 단순 rename 외 마이그레이션 정책을 요한다.

---

## 4. Go module path 변경 영향 분석

### 4.1 Go 컴파일 의미론 (semantics)

Go module path는 다음과 결합되어 있다:
- `go.mod` 파일 첫 줄 `module <path>` 선언
- **모든** 동일 모듈 내 파일의 `import "<path>/..."` 문 (456 파일 × 평균 ~2.1 import = 958 라인)
- `option go_package = "<path>/internal/transport/grpc/gen/goosev1;goosev1"` (4 .proto 파일)
- `buf.gen.yaml` plugin output 경로 prefix: `internal/transport/grpc/gen/goosev1` (이 파일은 module path 직접 포함 X, 그러나 `module=github.com/modu-ai/goose/internal/transport/grpc/gen/goosev1` 옵션은 포함)

**원자성 (atomicity)**: Go에서 module path 변경은 **단일 commit 안에서 전부 일관되게** 적용되어야 한다. `go.mod` 의 module 선언만 바꾸고 import statement 일부를 누락하면 `go build` 단계에서 "cannot find module" 컴파일 에러가 발생한다 — partial rename = broken build.

**도구 권장**: `go mod edit -module=github.com/modu-ai/mink` 로 module 선언만 갱신한 뒤, 다음 명령으로 모든 import를 일괄 치환:
```bash
find . -type f -name '*.go' -not -path './vendor/*' \
  -exec sed -i.bak 's|github.com/modu-ai/goose|github.com/modu-ai/mink|g' {} +
```
또는 더 안전한 `gofmt -r` 또는 `goimports -local` 사용. 본 SPEC §6 Phase 2에서 안전성 검토.

### 4.2 go.sum / replace directives

- `go.sum`: self-module은 sum에 등록되지 않음. 외부 의존성만 등록됨. `goose`/`mink` rename은 go.sum 직접 영향 0건. 단, module path 변경 후 `go mod tidy` 실행 시 go.sum이 재정렬될 수 있음 (외부 deps 변경 없으므로 실질 변경은 없으나 line ordering 차이 가능).
- `replace` directives: `go.mod` 에 0건 → 영향 없음.

### 4.3 third-party 의존성 검사

`grep 'goose' go.mod | grep -v 'modu-ai/goose'` → **0건**. 외부 dep 중 `goose`가 들어간 라이브러리는 부재. (예: `pressly/goose` (DB migration), `honeycombio/...goose...` 등은 본 프로젝트에서 사용하지 않음)

### 4.4 GitHub repo rename의 redirect 동작

GitHub은 repo rename 시 다음을 자동 제공:
- **HTTP redirect**: 옛 `https://github.com/modu-ai/goose` → 새 `https://github.com/modu-ai/mink` (301)
- **git clone redirect**: `git clone https://github.com/modu-ai/goose` 도 동작 (URL redirect 후 clone)
- **Issue/PR URL retention**: 옛 issue/PR URL은 redirect로 새 URL에 도달
- **API endpoint redirect**: REST API도 일정 기간 redirect 작동

단:
- **Go module proxy 캐시**: `proxy.golang.org` 가 `github.com/modu-ai/goose` 의 modules 메타데이터를 캐시 중. repo rename 후 일정 시간(~수시간~24시간)동안 옛 module path가 proxy에 잔존 가능. 그러나 본 프로젝트는 **pre-public** (외부 consumer 부재)이라 proxy 캐시 issue 영향 최소. 본인 개발 환경에서는 `GOPROXY=direct` 또는 `go clean -modcache` 로 즉시 해결 가능.
- **third-party consumer**: 현 시점에서 본 프로젝트를 import하는 외부 모듈은 부재 (확인됨).

### 4.5 redirect 미흡 영역

- **README badge URLs**: `img.shields.io/github/actions/workflow/status/modu-ai/goose/ci.yml?...` 4개 — redirect 동작은 하나 brand 정합성 위해 명시적 갱신.
- **Issue/PR templates**: `.github/ISSUE_TEMPLATE/*.yml` 내 URL — 명시적 갱신.
- **이전 release tag / GitHub Releases page**: tag 자체는 보존 (immutable). release notes 본문도 immutable.

---

## 5. Repo rename 영향 분석

### 5.1 GitHub redirect

- Repo URL `https://github.com/modu-ai/goose` → `https://github.com/modu-ai/mink`: GitHub 자동 처리.
- Git remote: 본 프로젝트 로컬 clone들 (`/Users/goos/MoAI/AI-Goose`, `/Users/goos/moai/ai-goose`) 의 `git remote set-url origin https://github.com/modu-ai/mink.git` 수동 갱신 필요.

### 5.2 기존 PR / Issue URL 안정성

- 기존 merged PR URL (예: `https://github.com/modu-ai/goose/pull/162`): GitHub redirect로 새 URL 도달. 자동.
- 기존 issue URL: 같음. 자동.
- ROADMAP / SPEC body 내 PR/issue inline URL (`https://github.com/modu-ai/goose/...`): redirect 동작하나 brand 정합성 위해 점진적 갱신 (별도 PR로도 가능).

### 5.3 Branch protection / Rulesets

- 활성 branch protection (main, release/*) — repo rename 후에도 유지됨 (GitHub 보장). `CLAUDE.local.md §1.3`의 2026-04-27 상태 (Team plan + ruleset id `15491142`)는 rename으로 영향 없음.

### 5.4 workflows 영향

- `.github/workflows/*.yml` 내 `modu-ai/goose` 하드코딩 **0건** — repo rename에 자동 적응.
- `${{ github.repository }}` 사용 → 자동 새 repo 이름 반영.
- `actions/checkout@v6` → context 기반, 자동.

### 5.5 외부 shell alias / clone

- 사용자의 `~/.bashrc` / `~/.zshrc` 에 `alias goose-dev='cd ~/MoAI/AI-Goose'` 등이 있을 수 있음 — 사용자 자체 갱신 (notification 항목, AC 외).
- 로컬 clone 디렉토리 이름 (`/Users/goos/MoAI/AI-Goose`): 그대로 두거나 사용자가 수동 rename. 본 SPEC scope 외.

### 5.6 GitHub App / OAuth / Webhook 영향

- 본 repo에 연결된 외부 GitHub App (예: Dependabot, Release Drafter)은 repo rename 후에도 동작 (GitHub 보장).
- webhook URL은 redirect 대상이 아니나 본 프로젝트에 외부 webhook 미설치.

---

## 6. SPEC ID prefix 정책 결정 근거

사용자 결정사항 #5에 따라, 신규 SPEC은 `SPEC-MINK-*` prefix를 사용하고 기존 88개 `SPEC-GOOSE-*` 디렉토리는 **변경 금지**.

이유:
1. **Cross-reference 무결성**: 기존 SPEC 본문, 코드 주석 (`// SPEC-GOOSE-CORE-001 …`), commit messages, PR titles, GitHub issue references 등 수천 개 식별자가 옛 SPEC ID를 가리킨다. 일괄 rename 시:
   - Commit message는 immutable (rewrite 시 force-push 차단)
   - PR titles는 history 일부
   - Go 코드 주석은 변경 가능하나 SPEC 본문 내 cross-ref와 분리 작업 → 무결성 깨짐
2. **git log 검색성**: 개발자가 `git log --grep='SPEC-GOOSE-CLI-001'` 로 과거 작업 검색 가능해야 함. 옛 ID 보존이 검색성 보장.
3. **GitHub issue / PR title alignment**: 과거 88개 SPEC 관련 PR/issue들의 title이 `SPEC-GOOSE-*` 를 그대로 포함 — rename 시 title 자체는 immutable이라 mismatch 영구화.

대안 — 옛 SPEC 디렉토리 rename:
- 비용: 88개 디렉토리 + 모든 cross-ref (예상 1000+ 위치) 일괄 갱신, 검색성 영구 손상
- 이득: brand 일관성 (단, 88개 SPEC 모두 status=completed/closed라 미래 작성자가 신규 사용 X)
- 결론: 비용 >> 이득. **OUT scope 확정**.

기존 88개 `SPEC-GOOSE-*` 는 **immutable archive**로 분류 (class (E)). 신규 SPEC만 `SPEC-MINK-*` prefix 사용.

`brand-lint` (`check-brand.sh`) retarget 시 다음을 처리:
- `.moai/specs/SPEC-GOOSE-*/**` 경로 내부의 `goose/Goose/GOOSE` 토큰: brand-lint 검사 면제 (immutable archive exemption zone)
- `.moai/specs/SPEC-MINK-*/**` 경로 또는 그 외 위치의 `goose/Goose/GOOSE` 토큰: brand 위반으로 flag (단, 다음 경우 제외: HISTORY rows, fenced code, inline code, immutable archive 인용)

---

## 7. Brand 자산 재사용 결정 근거

선행 SPEC이 구축한 자산:
1. `.moai/project/brand/style-guide.md` — v1.0.0 / classification: FROZEN_REFERENCE / 110 라인
2. `scripts/check-brand.sh` — 162 라인 Python-in-bash 마크다운 파서 + 위반 패턴 검출
3. `.github/workflows/brand-lint.yml` — PR-triggered CI gate
4. `Makefile` brand-lint target

본 SPEC은 위 4개 자산을 **scaffold로 재사용**하고 내용만 retarget:

| 자산 | 변경 유형 | 변경 내용 |
|---|---|---|
| `style-guide.md` | 전면 재작성 | `AI.GOOSE` → `MINK` / `goose` → `mink` / `ai-goose` → `mink` (slug). 단, dual-representation 원칙(§7.2)은 그대로 계승. 한/영 예시 4쌍 갱신. |
| `check-brand.sh` | pattern retarget | 위반 패턴: `goose 프로젝트`, `Goose 프로젝트`, `GOOSE-AGENT`, `goose project`, `Goose project`, `AI.GOOSE` (산문에서 사용 시 — predecessor 의 정정 결과가 다시 위반이 됨) |
| `brand-lint.yml` | **변경 0건** | 동일 PR trigger / 동일 Python setup / 동일 script 호출 |
| `Makefile` brand-lint target | 변경 0건 | `bash scripts/check-brand.sh` 그대로 |

선행 SPEC 이 frozen-reference로 인용한 dual-representation 원칙 (산문=brand / 코드=식별자 / URL=slug)은 본 SPEC도 계승하되, MINK는 `mink` slug가 brand identifier 와 거의 동일 (대소문자 차이만)하므로 단순화된다 (§7.1 of spec.md 참조).

---

## 8. Risk-to-Mitigation Table (preview of spec.md §9)

| # | 리스크 | 가능성 | 영향 | Mitigation |
|---|---|---|---|---|
| R1 | Partial rename으로 인한 컴파일 break | 높 | 매우 높 | go.mod module 선언 + 모든 import statement를 **단일 commit/atomic transaction**으로 변경. `go build ./...` 이 commit 안에 통과해야 함. 별도 PR 분리 금지. |
| R2 | 개발자 로컬 clone drift | 중 | 낮 | 후속 README/CHANGELOG에 "post-rename checklist" 추가: `git remote set-url`, `go clean -modcache`, `~/.bashrc` alias 갱신 |
| R3 | third-party dep이 `github.com/modu-ai/goose` import 함 (가설) | 매우 낮 | 낮 | 사전 검증: `go.sum` 및 외부 의존성 검사 → **0건 확인됨** (§4.3) |
| R4 | CI workflow hardcoded refs 잔존 | 낮 | 낮 | 사전 검증: `.github/workflows/*.yml` 내 `modu-ai/goose` 하드코딩 **0건 확인됨** (§5.4) |
| R5 | Go module proxy `proxy.golang.org` 캐싱 stale | 낮 | 매우 낮 | pre-public 단계라 proxy consumer 부재. 본인 환경은 `go clean -modcache` 또는 `GOPROXY=direct`. |
| R6 | SPEC-GOOSE-* 디렉토리 경로 참조가 코드/docs 내 잔존 | 중 | 낮 (정상 동작) | 본 SPEC §3.2 OUT scope (preserve). brand-lint exemption zone으로 처리. |
| R7 | 예전 SPEC 보존 commit timing | 낮 | 낮 | 본 SPEC PR의 squash 커밋이 선행 SPEC body는 건드리지 않음. 선행 SPEC frontmatter status / 1개 HISTORY row 추가는 **별도 후속 commit**으로 분리 (orchestrator 담당). |
| R8 | User-data 경로 (`./.goose/`, `~/.goose/`) 마이그레이션 누락 | 중 | 중 | 1차 launch에서 새 코드는 `./.mink/`, `~/.mink/` 사용. 기존 데이터는 `./.goose/` 가 존재하면 fallback 읽기 + 첫 실행 시 1회 마이그레이션 alert. 본 SPEC 은 마이그레이션 정책 결정만 하고 실제 마이그레이션은 별도 SPEC `SPEC-MINK-USERDATA-MIGRATE-001` (downstream)으로 분리. (§11 R8 verbatim에서 결정 trail 명시) |
| R9 | env var (`GOOSE_*` 21개) 변경 → 개발자 환경 break | 중 | 중 | 새 `MINK_*` env var 신설 + 옛 `GOOSE_*` deprecated alias 로 1개 minor version cycle 유지. config loader 가 둘 다 읽고 옛 키 사용 시 stderr 경고. |
| R10 | proto package rename 시 wire-format 호환성 | 매우 낮 | 매우 높 | proto package 이름은 wire-format 영향 0 (wire는 field number 기반). package 이름은 generated Go 코드의 import path / type qualified name 만 영향 — 컴파일 break는 import statement 갱신으로 해결. 단, **단일 commit 안에서 proto + generated code + import 일괄 갱신** 필수. |
| R11 | brand "mink" 의 모피 산업 연상 | 매우 낮 | 매우 낮 | IDEA-002 risk assessment에서 AI context disambiguation으로 정리. logo direction 은 text-only minimalist (별도 SPEC). 본 SPEC scope 외. |
| R12 | 본 SPEC PR squash가 너무 거대해 review 어려움 | 높 | 중 | §6 Phase별 commit 분리 + 단일 squash PR (CLAUDE.local.md §1.4). Phase 2 (module path) + Phase 3 (identifiers) 은 atomicity 때문에 단일 commit이나 다른 phase는 분리 가능. 필요 시 multi-PR 분리 옵션 (Phase 7 repo-rename은 GitHub 직접 작업이므로 PR 외). |

---

## 9. 선행 작업 의존성

본 SPEC `/moai run` 진입 전 만족해야 할 조건:

1. **MINK 상표(trademark) 검색**: IDEA-002 §5에서 검토 완료, AI context "MINK" 사용 사례 부재. **본 SPEC scope 외** (별도 trademark filing은 OUT scope §11 item 4).
2. **외부 consumer 부재 확인**: §4.3에서 확인됨 (`go.sum` 외부 `goose` dep 0건, public Go module proxy 캐시 영향 무시 가능).
3. **선행 SPEC `SPEC-GOOSE-BRAND-RENAME-001` 상태 = completed**: 확인됨 (v0.1.1, 2026-04-27).
4. **Branch protection bypass 권한**: main branch 직접 commit은 admin bypass 가능 (CLAUDE.local.md §1.3) — 단, 본 SPEC은 feature branch + squash PR 경로 강제.
5. **Stashed self-dev sweep 미간섭**: `git stash list` 의 `stash@{0}` "self-dev sweep WIP (322 entries)" 는 본 SPEC 작업 중 건드리지 않음 (orchestrator 지시).
6. **MINK domain 미등록 확인 (informational only)**: domain registration은 OUT scope §11 item 1.
7. **Go module proxy 사용 정책 결정**: pre-public 단계라 proxy 영향 minimal — 본 SPEC 진입 후에도 `proxy.golang.org` 캐시 일시 stale 가능성 수용.

---

## 10. References

### 10.1 선행 SPEC
- `.moai/specs/SPEC-GOOSE-BRAND-RENAME-001/spec.md` v0.1.1 (2026-04-27, completed)
- `.moai/specs/SPEC-GOOSE-BRAND-RENAME-001/research.md` (2026-04-26)

### 10.2 사용자 결정 trail
- IDEA-002 / GooseBot 한국시장 단타 LLM-bot 분리 결정 (별도 프로젝트로 GooseBot 명칭 이관)
- 본 AI.GOOSE → MINK rename 결정 (orchestrator 8개 확정사항, 2026-05-12)
- Brand: MINK (Made IN Korea)
- Tagline KR: "매일 아침, 매일 저녁, 너의 MINK."
- Tagline EN: "Your AI that says good morning, every morning."
- 6-month success metric: self + 1 other daily user

### 10.3 외부 참조
- GitHub repo rename 동작 문서: <https://docs.github.com/en/repositories/creating-and-managing-repositories/renaming-a-repository>
- Go module path 변경 공식 가이드: <https://go.dev/ref/mod#go-mod-file-module>
- Buf proto generator 옵션: <https://buf.build/docs/configuration/v2/buf-gen-yaml>

### 10.4 본 프로젝트 자료
- `go.mod` (현재 module: `github.com/modu-ai/goose`)
- `.github/workflows/{ci,brand-lint,release-drafter}.yml`
- `.github/{PULL_REQUEST_TEMPLATE.md,ISSUE_TEMPLATE/*}`
- `CLAUDE.local.md` §1.3 (Team plan branch protection 활성 상태), §1.4 (merge strategy), §2.2 (commit convention), §2.5 (code comment language)
- `.moai/project/brand/style-guide.md` v1.0.0 (FROZEN_REFERENCE)
- `scripts/check-brand.sh` (162 라인)

### 10.5 grep 카운트 reproducibility

본 research 내 모든 카운트는 다음 명령으로 재현 가능 (2026-05-12 기준 HEAD `e76febe`):
```bash
# §2.1
grep -rln 'github.com/modu-ai/goose' --include='*.go' | wc -l       # 456
grep -rn 'github.com/modu-ai/goose' --include='*.go' | wc -l        # 958
grep -roh 'github.com/modu-ai/goose/internal/[a-z]*' --include='*.go' | sort | uniq -c | sort -rn

# §2.2
grep -rln '^package goose$' --include='*.go' | wc -l                # 0

# §2.3
grep -rEohw 'Goose[a-zA-Z]*' --include='*.go' | sort | uniq -c | sort -rn

# §2.4
grep -rEohw 'GOOSE_[A-Z_]*' --include='*.go' | sort -u | wc -l      # 21

# §2.11
ls -d .moai/specs/SPEC-GOOSE-* | wc -l                              # 88
total=0; for f in $(grep -rEl '^## HISTORY' .moai/specs/); do
  n=$(awk '/^## HISTORY/{p=1;next} /^##/{p=0} p && /^\|/' "$f" \
      | grep -vE '\|---|\| Version \| Date' | wc -l); total=$((total + n)); done
echo $total                                                          # 300

# §2.12
grep -rn '\.goose/' --include='*.go' | wc -l                        # 117
```

---

## 11. 결정 사항 요약 (Decision Snapshot)

본 research를 기반으로 spec.md에 다음 결정을 명문화한다:

1. **Brand**: `AI.GOOSE` → `MINK` (Made IN Korea, uppercase 산문 우선, lowercase `mink` slug/식별자)
2. **Module path**: `github.com/modu-ai/goose` → `github.com/modu-ai/mink` (single-commit atomic)
3. **Repo**: `modu-ai/goose` → `modu-ai/mink` (GitHub rename + 로컬 remote 갱신)
4. **Binary**: `goose` → `mink`, `goosed` → `minkd`, `goose-proxy` (미구현) → `mink-proxy` (계획)
5. **proto**: `goose.v1` → `mink.v1`, 생성 디렉토리 `goosev1` → `minkv1`
6. **SPEC ID prefix (신규)**: `SPEC-MINK-*`. 기존 88개 `SPEC-GOOSE-*` 보존 (immutable archive).
7. **immutable preserve zones**:
   - `.moai/specs/SPEC-GOOSE-*/**` 전체
   - 모든 `## HISTORY` 표 rows (status 무관, location 무관)
   - 기존 CHANGELOG entries
   - git history (commit messages, PR titles)
   - `.claude/agent-memory/**` (시점 기록)
8. **User-data path**: `./.goose/` → `./.mink/`, `~/.goose/` → `~/.mink/` (마이그레이션 정책은 본 SPEC에서 정책만 정의, 실제 코드는 downstream SPEC `SPEC-MINK-USERDATA-MIGRATE-001`)
9. **Env var**: `MINK_*` 신설 + `GOOSE_*` deprecated alias 1 minor version cycle 유지
10. **brand-lint retarget**:
    - 위반 패턴: `goose 프로젝트`, `Goose 프로젝트`, `GOOSE-AGENT`, `goose project`, `Goose project`, `AI.GOOSE` (산문)
    - exemption: `.moai/specs/SPEC-GOOSE-*/**`, `.moai/brain/IDEA-*/**`, `## HISTORY` rows, fenced code blocks, inline code spans, `.claude/agent-memory/**`
11. **Brand 자산**: `style-guide.md` 전면 재작성, `check-brand.sh` pattern retarget, `brand-lint.yml` 변경 0건

---

Version: 0.1.0
Status: planned
Last Updated: 2026-05-12
