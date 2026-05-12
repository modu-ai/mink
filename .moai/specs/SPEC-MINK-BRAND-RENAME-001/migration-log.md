---
id: spec-mink-brand-rename-001-migration-log
spec: SPEC-MINK-BRAND-RENAME-001
status: in_progress
created_at: 2026-05-12
---

# SPEC-MINK-BRAND-RENAME-001 Migration Log

각 phase 의 baseline 캡처 + 검증 결과를 append-only 로 기록한다. 본 파일은 SPEC sync 시점의 audit trail 역할.

---

## Phase 1 Baseline (2026-05-12)

### Git state

| 항목 | 값 |
|------|-----|
| Branch | `feature/SPEC-MINK-BRAND-RENAME-001` |
| Base HEAD | `0ae00946d15711e85e9798d6992e697fddb80e20` (PR #167 머지 직후, main = 0ae0094) |
| Worktree path | `/Users/goos/.moai/worktrees/goose/SPEC-MINK-BRAND-RENAME-001` |

### Module / Package state

| 항목 | 값 |
|------|-----|
| `head -1 go.mod` | `module github.com/modu-ai/goose` |
| `go.mod` Go version | `go 1.26` |
| `.go` 파일 총수 (vendor 제외) | 912 |
| `github.com/modu-ai/goose` import 참조 .go 파일 수 | 456 |

### Filesystem layout

| 항목 | 값 |
|------|-----|
| `ls cmd/` | `goose`, `goosed` |
| `ls proto/` | `goose` |
| `.moai/specs/SPEC-GOOSE-*` 디렉토리 수 | 88 (immutable archive — 본 SPEC 으로 byte-identical 보존) |

### Immutable archive integrity (baseline)

| 항목 | SHA-256 |
|------|---------|
| `.claude/agent-memory/**` (recursive aggregate) | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` (빈 디렉토리 — empty SHA) |

> 본 hash 는 Phase 8 / Sync 시점에 재계산하여 byte-identical 검증 (AC-MINK-BR-014).

### CHANGELOG header (baseline)

`CHANGELOG.md` line 1: `# 변경 이력`

> 신규 entry 만 MINK 표기; 기존 release section header 이전 모든 entry 는 보존 (REQ-MINK-BR-023, AC-MINK-BR-018).

### 322 self-dev sweep stash

Phase 1 진입 직전 worktree 에 BRAND-RENAME 무관한 322개 변경 (114 M + 152 D + 56 ??) 가 떠 있어 atomic commit 위배 위험. `git stash push -u -m "self-dev-sweep-322-before-BRAND-RENAME-001-run-20260512-190843"` 로 보관 후 clean baseline 에서 진행. Sync 후 main session 으로 복구 검토.

---

## Phase 1 작업 결과 (2026-05-12)

### 산출물

| 파일 | 작업 | 검증 |
|------|------|------|
| `.moai/project/brand/style-guide.md` | v1.0.0 → v2.0.0 (전면 재작성, MINK 규범) | frontmatter `id: brand-style-guide-mink` ✅ |
| `scripts/check-brand.sh` | 패턴 retarget + exemption 확장 | 위반 패턴 4종 (`goose 프로젝트`, `goose project`, `GOOSE-AGENT`, `\bAI\.GOOSE\b`) 활성 ✅ |
| `.moai/specs/SPEC-MINK-BRAND-RENAME-001/migration-log.md` | 신설 | Phase 1 baseline + 결과 기록 ✅ |

### brand-lint 실행 결과 (Phase 1 시점)

`bash scripts/check-brand.sh` → **exit 1, 39 violations** 보고됨.

이는 **예상된 중간 상태**다. 위반 분포:
- `.moai/project/*.md` (16건 — branding/product/tech/learning-engine/migration/ecosystem/structure/codemaps 등)
- `.moai/specs/SPEC-MINK-PRODUCT-V7-001/{spec,research}.md` (3건 — 이전 SPEC body 가 brand rename 결정 trail 인용)
- `.moai/specs/{ROADMAP,IMPLEMENTATION-ORDER}.md` (2건)
- `.moai/state/NEXT-SESSION.md` (1건)
- `.moai/reports/plan-audit/...` (1건)
- `.github/PULL_REQUEST_TEMPLATE.md` (1건)
- `CODE_OF_CONDUCT.md`, `docs/cli/{README,getting-started}.md`, `internal/llm/provider/README.md` (4건)
- `.moai/project/codemaps/{architecture,modules/bridge,README}.md` (3건)
- `.moai/project/branding.md` 본문 자체 (8건)

해소 시점: **Phase 7 (문서 sweep)** 에서 일괄 `AI.GOOSE → MINK` 치환 후 exit 0 보장. PR 종결 시점 brand-lint CI gate 통과.

> spec.md §6 Phase 1 §verification "bash scripts/check-brand.sh exit 0" 항목은 PR 종결 시점 (Phase 7 후) 기준 의미로 해석. Phase 1 commit 시점에는 script/style-guide 의 정합성만 검증한다.

### Phase 1 commit hash

`bba61a8` on `feature/SPEC-MINK-BRAND-RENAME-001` (3 files changed, +214/-77, +1 new file)

---

## Phase 2 (2026-05-12)

### 산출물
- Commit: `ee26004` on `feature/SPEC-MINK-BRAND-RENAME-001` (461 files changed, +995/-988)
- Delegated to: `expert-refactoring` subagent

### 검증 (6개 중 5 PASS + 1 expected drift)
| # | 명령 | 기대 | 실제 | 상태 |
|---|------|------|------|------|
| 1 | `head -1 go.mod` | `module github.com/modu-ai/mink` | 일치 | ✅ |
| 2 | `grep goose imports .go` | 0 | 4 (pb.go raw descriptor) | ⚠️ → Phase 4 자동 해소 |
| 3 | `grep mink imports .go` | 456 | 451 (pb.go 4 제외) | ⚠️ → Phase 4 자동 해소 |
| 4 | `go build ./...` | exit 0 | exit 0 | ✅ |
| 5 | `go vet ./...` | exit 0 | exit 0 | ✅ |
| 6 | `gofmt -l .` | empty | empty | ✅ |

### 특이사항
- pb.go 4 raw descriptor binary string 의 `github.com/modu-ai/goose` 잔존: proto wire-format 인코딩 특성 — Phase 4 (`buf generate` 재생성) 시 자동 교체. Phase 2 에서 sed 직접 치환 시 wire-format 정합성 깨짐.
- `useragent.go` 주석 `// Override via ldflags: -X github.com/modu-ai/goose/...` 도 함께 수정 (ldflags 경로 정확성).
- `go mod tidy` 부산물: `github.com/sergi/go-diff v1.4.0` 신규 indirect dep 추가 (self-module 무관, transitive 풀이).

---

## Phase 3 (2026-05-12)

### 산출물
- Delegated to: `expert-refactoring` subagent
- 변경 파일: 22개 (21 .go + 1 migration-log.md)

### 검증 (4개 모두 PASS)
| # | 명령 | 기대 | 실제 | 상태 |
|---|------|------|------|------|
| 1 | `grep GooseHome \*.go \| wc -l` | 0 | 0 | ✅ |
| 2 | `grep MinkHome \*.go \| wc -l` | ≥64 | 64 | ✅ |
| 3 | `grep "You are Goose" \*.go \| wc -l` | 0 | 0 | ✅ |
| 4 | `go build/vet/test` | exit 0 | exit 0 | ✅ |

### 특이사항
- gofmt -r 'GooseHome -> MinkHome' 로 57건 처리 (struct field + 호출부)
- 나머지 7건: comments 내 GooseHome 언급 → 수동 Edit (MinkHome 로 정정)
- resolveGooseHome() 함수명, TestExpand_GooseHome 함수명은 Phase 5 대상 — Phase 3 에서 미수정
- doc-comment Goose brand-position 14건 정정 (internal/agent, internal/learning/*, internal/credproxy, internal/messaging/telegram, internal/command/adapter/aliasconfig)
- test fixture "You are Goose..." → "You are Mink..." 2건 (coverage_test.go, redact/rules_test.go)
- config_test.go의 TestLoad_GooseHome_Unset → TestLoad_MinkHome_Unset 함수명 정정 (struct field 참조 comment → MinkHome)
- Phase 3 commit hash: `b196dd3`

---

## Phase 4-8 + Hotfix (2026-05-12) — All Complete

| Phase | Commit | 요약 | 검증 |
|-------|--------|------|------|
| 4 | `bd47817` | proto package goose.v1 → mink.v1 atomic (proto/mink/v1/ + buf regen + 옛 goosev1/ 삭제 + .go import 일괄 치환). expert-refactoring 위임. | proto/=mink, gen/=minkv1, build/vet/gofmt 통과 |
| 5 | `a69830d` | CLI binary cmd/goose→mink, cmd/goosed→minkd, doc-comment 6건. expert-refactoring 위임. | mink+minkd 바이너리 빌드 성공 (36.8MB + 11.4MB) |
| 5-fix | `b5efaa5` | hotfix: cmd/minkd/main.go:1 의 invented `AI.MINK` → `MINK` (style-guide §1 위반 정정, main session 즉시 catch) | grep AI.MINK = 0 |
| 6 | `e39670b` | .github/{PULL_REQUEST_TEMPLATE.md, ISSUE_TEMPLATE/*.yml, dependabot.yml} modu-ai/mink + MINK 정정 (5 files). main session 직접 처리. | grep modu-ai/goose .github/ = 0 |
| 8 | `8ab36fe` | 27 .go 파일 user-facing string AI.GOOSE → MINK (batch sed). main session 직접 처리. | grep AI.GOOSE .go = 0, build/vet/gofmt 통과 |
| 7 | `261e605` | 36 .md 파일 docs sweep AI.GOOSE → MINK + check-brand.sh exemption 확장 (SPEC-MINK-* 모두) + branding.md 의미 정정 3 line. | brand-lint OK 0 violations |

### 진행 중 발견 / 정정

- **Phase 5 (a69830d) 직후**: expert-refactoring 이 cmd/minkd/main.go 의 brand-position 산문을 `AI.MINK` 로 잘못 변환. main session 의 즉시 grep 으로 catch, hotfix `b5efaa5` 적용. (lesson_subagent_analysis_verification 패턴 재확인 — subagent claims about broader-codebase need verification)
- **Phase 7 진행 중**: sibling SPEC-MINK-PRODUCT-V7-001 + DISTANCING-STATEMENT-001 의 `AI.GOOSE → MINK` 컨텍스트 산문이 sed 로 `MINK → MINK` 로 깨짐. `git checkout HEAD --` 으로 복구 + check-brand.sh exemption 확장 (SPEC-MINK-*) 적용. 두 SPEC 은 PR #166/#167 머지 완료된 immutable archive 로 처리.
- **LSP stale**: Phase 3 / Phase 4 직후 LSP 가 GooseHome BrokenImport / DuplicateDecl 가짜 에러 보고. main session 의 `go build ./...` 직접 verify 로 실제 정상 확인. lesson_lsp_stale_after_codegen 12회 재확인.

## 최종 검증 (HEAD = 261e605, 2026-05-12)

| 항목 | 기대 | 실제 | 상태 |
|------|------|------|------|
| `head -1 go.mod` | `module github.com/modu-ai/mink` | 일치 | ✅ |
| `ls cmd/` | `mink`, `minkd` | 일치 | ✅ |
| `ls proto/` | `mink` | 일치 | ✅ |
| `ls internal/transport/grpc/gen/` | `minkv1` | 일치 | ✅ |
| `go build ./...` | exit 0 | exit 0 | ✅ |
| `go vet ./...` | exit 0 | exit 0 | ✅ |
| `gofmt -l .` | empty | empty | ✅ |
| `bash scripts/check-brand.sh` | OK 0 violations | OK 0 violations | ✅ |
| `grep github.com/modu-ai/goose` (.go, vendor 제외) | 0 | 0 | ✅ |
| `grep GooseHome` (.go) | 0 | 0 | ✅ |
| `grep AI.GOOSE` (excl exemption) | 0 | 0 | ✅ |

## Branch state

`feature/SPEC-MINK-BRAND-RENAME-001` (9 commits ahead of `main` 0ae0094):

```
261e605 Phase 7 docs sweep
8ab36fe Phase 8 .go user-facing
e39670b Phase 6 CI templates
b5efaa5 Phase 5 hotfix AI.MINK→MINK
a69830d Phase 5 cmd/{mink,minkd}
bd47817 Phase 4 proto mink.v1
b196dd3 Phase 3 GooseHome→MinkHome
ee26004 Phase 2 Go module path
bba61a8 Phase 1 style-guide + brand-lint
```

## Post-merge 작업 (PR squash merge 후 별도 단독 작업)

1. `gh repo rename mink --repo modu-ai/goose` — GitHub repo rename (1회)
2. 로컬 clone remote URL 갱신: `git remote set-url origin https://github.com/modu-ai/mink.git`
3. 선행 SPEC supersede commit (orchestrator 단독, 별도 commit):
   - `.moai/specs/SPEC-GOOSE-BRAND-RENAME-001/spec.md` frontmatter `status: completed → superseded`
   - `## HISTORY` 표 1 row append (v0.1.2)
4. `/moai sync SPEC-MINK-BRAND-RENAME-001`
5. 322 stash 복구 검토 (main session, stash@{0})
6. 후속 SPEC plan 작성 권장: `SPEC-MINK-ENV-MIGRATE-001`, `SPEC-MINK-USERDATA-MIGRATE-001`
