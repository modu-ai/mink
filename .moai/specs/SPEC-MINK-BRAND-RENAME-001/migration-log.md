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

### Phase 1 commit hash — pending

(commit 직후 채움)

---

## Phase 2 Baseline — pending

(Phase 2 진입 직전 캡처)

---

## Phase 2 Baseline — pending

(Phase 2 진입 직전 캡처)
