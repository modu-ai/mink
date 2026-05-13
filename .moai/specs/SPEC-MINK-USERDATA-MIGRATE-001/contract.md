# SPEC-MINK-USERDATA-MIGRATE-001 — Sprint Contract

## HISTORY

| Version | Date | Author | Change |
|---------|------|--------|--------|
| 0.1.0 | 2026-05-13 | manager-strategy | 초안. evaluator-active Phase 2.0 / harness=thorough 용. 모든 gate 는 binary verifiable. |

---

## 1. Done Criteria (machine-checkable)

각 줄은 본 SPEC 의 squash merge 직전 main branch 에서 실행 가능해야 한다.

### 1.1 Path-resolver enforcement (AC-005, REQ-006/014)

```
# Production .goose literal = 0 (legacy.go 제외)
grep -rEn '"\.goose' --include='*.go' --exclude='*_test.go' . \
  | grep -v '^./internal/userpath/legacy\.go:' \
  | wc -l
# → 0

# legacy.go = 정확히 1건
grep -c '"\.goose' internal/userpath/legacy.go
# → 1

# filepath.Join + .goose pattern = 0 (legacy.go 제외)
grep -rEn 'filepath\.Join\([^)]*"\.goose"' --include='*.go' --exclude='*_test.go' . \
  | grep -v '^./internal/userpath/legacy\.go:' \
  | wc -l
# → 0

# 직접 os.UserHomeDir() 호출 = 0 (userpath.go 제외)
grep -rEn 'os\.UserHomeDir\(\)' --include='*.go' --exclude='*_test.go' . \
  | grep -v '^./internal/userpath/userpath\.go:' \
  | wc -l
# → 0
```

### 1.2 Test marker enforcement (AC-007, REQ-016)

```
grep -rEn '"\.goose' --include='*_test.go' . \
  | grep -v '^./internal/userpath/legacy_test\.go:' \
  | grep -v 'MINK migration fallback test' \
  | wc -l
# → 0
```

### 1.3 Tmp prefix (AC-006, REQ-004)

```
# 신규 prefix `.mink-` 단일 정의 = internal/userpath/userpath.go 의 TempPrefix() 함수만
grep -rEn '"\.mink-' --include='*.go' --exclude='*_test.go' . \
  | head -1
# → internal/userpath/userpath.go:NN: ... TempPrefix() ...
# 옛 prefix `.goose-` 미사용
grep -rEn '"\.goose-' --include='*.go' --exclude='*_test.go' . \
  | grep -v '^./internal/userpath/legacy\.go:' \
  | wc -l
# → 0
```

### 1.4 Build + Test + Race + Vet + Lint

```
go build ./...                       # exit 0
go vet ./...                         # exit 0
golangci-lint run                    # exit 0
go test ./... -race                  # exit 0
```

### 1.5 Coverage thresholds

```
# 전체 프로젝트 coverage ≥ 85% (회귀 0)
go test ./... -coverprofile=cover.out
go tool cover -func=cover.out | grep total | awk '{print $3}'
# → ≥ 85.0%

# internal/userpath/** coverage ≥ 90% (strict)
go test ./internal/userpath/... -coverprofile=u.out
go tool cover -func=u.out | grep total | awk '{print $3}'
# → ≥ 90.0%
```

### 1.6 Brand-lint (REQ-006)

```
bash scripts/check-brand.sh
# → exit 0
```

### 1.7 LSP gate (run phase, plan.md)

- max_errors = 0
- max_type_errors = 0
- max_lint_errors = 0
- no regression vs baseline (Phase 0.5 capture)

### 1.8 Documentation completeness (Phase 6)

- `README.md` contains `## Migration from .goose` section (`grep -c '^## Migration from .goose' README.md` ≥ 1)
- `CHANGELOG.md` `[Unreleased]` section contains `BREAKING` token referencing this SPEC
- `.moai/project/structure.md` lists `internal/userpath` package
- `.moai/project/codemaps/internal-userpath.md` exists (non-empty)

---

## 2. Edge Cases (must all be tested)

| # | Edge case | Test mechanism |
|---|-----------|----------------|
| EC-1 | `~/.goose` is symlink | T-007 unit test asserts `os.Lstat` branch + graceful error |
| EC-2 | Concurrent `mink` + `minkd` first-run (lock contention) | T-006 unit test with two goroutines + `sync.Once` style verification |
| EC-3 | Mid-copy crash → stale lock recovery | T-006 unit test: pre-create `.migration.lock` with dead PID + partial `~/.mink/` → next-run cleanup + retry |
| EC-4 | Read-only filesystem (`chmod 0400 /tmp/ro-mink && MINK_HOME=/tmp/ro-mink`) | T-002 unit test asserts `ErrPermissionDenied` or `ErrReadOnlyFilesystem` |
| EC-5 | Cross-filesystem rename (`EXDEV`) → copy fallback | T-005 unit test with `os.Rename` mock returning `syscall.EXDEV` |
| EC-6 | `MINK_HOME=""` (empty) | T-001 + T-002: `ErrMinkHomeEmpty` typed error |
| EC-7 | `MINK_HOME=$HOME/.goose` (legacy path policy violation) | T-001 + T-002: `ErrMinkHomeIsLegacyPath` typed error |
| EC-8 | `MINK_HOME=/tmp/../etc/foo` (path traversal) | T-001 + T-002: `ErrMinkHomePathTraversal` typed error (raw `..` reject before `filepath.Clean`) |
| EC-9 | Brand marker absent in `~/.goose/` | T-007 unit test: stderr warning + `brand_verified: false` in marker file |
| EC-10 | Project-local lazy migration when user enters cwd with `./.goose/` | T-014 unit test for `internal/qmd` config |
| EC-11 | Both `~/.goose/` + `~/.mink/` exist (dual existence) | T-007 unit test: `~/.mink/` wins + 1-time stderr warning |
| EC-12 | Idempotent re-run (alarm count = 0 on second invocation) | T-004 unit test using `sync.Once` + marker file check |

---

## 3. Hard Thresholds (do not weaken)

| Threshold | Value | Rationale |
|-----------|-------|-----------|
| `internal/userpath/**` coverage | ≥ 90% | plan.md §5 strict threshold (foundational package, no fallback) |
| Project-wide coverage regression | 0 (no decrease vs baseline) | quality.yaml `test_coverage_target: 85` |
| `go test ./... -race` failures | 0 | race detector clean (lock + sync.Once + goroutine paths) |
| LSP errors | 0 | run phase gate |
| LSP type errors | 0 | run phase gate |
| LSP lint errors | 0 | run phase gate |
| `golangci-lint run` warnings | 0 | existing project rule |
| brand-lint violations | 0 | REQ-006 gate, AC-005 #5 |
| Production `"\.goose"` literal count (legacy.go 제외) | 0 | AC-005 #1 |
| Production `os.UserHomeDir()` direct call (userpath.go 제외) | 0 | AC-005 #4 |
| Test files violating `.goose` marker rule | 0 | AC-007 #1 |
| Mode bits weakening (0600 source → > 0600 dest) | 0 occurrences | REQ-019 / AC-009 |
| `MINK_HOME` 4 negative cases (empty / legacy / traversal / non-writable) → silent fallback | 0 occurrences | REQ-018 / AC-008b |
| Data loss (verify-before-remove failure passing) | 0 occurrences | REQ-015 / R2 |
| Pass threshold floor (do not lower) | 0.60 | design constitution §11 (FROZEN) — applied to evaluator-active scoring |

---

## 4. Security Gates (REQ-019 + AC-008b + path traversal)

### 4.1 File mode bits (REQ-019, AC-009, R13)

Copy fallback 경로 (T-005) 에서 다음 4 sensitive file 의 dest mode ≤ source mode:

- `~/.mink/permissions/grants.json` — 0600 (source) → ≤ 0600 (dest)
- `~/.mink/messaging/telegram.db` — 0600 → ≤ 0600
- `~/.mink/mcp-credentials/<provider>.json` — 0600 → ≤ 0600
- `~/.mink/ritual/schedule.json` — 0600 → ≤ 0600

Directory mode:
- `~/.mink/permissions/` — 0700 → ≤ 0700

Verification:
```
stat -c '%a' ~/.mink/permissions/grants.json   # Linux → 600 or stricter
stat -f '%Lp' ~/.mink/permissions/grants.json  # macOS → 600 or stricter
```

### 4.2 MINK_HOME boundary (REQ-018, AC-008b)

4 negative case 모두 typed error 로 reject. silent fallback 0건:

| Case | Input | Expected typed error |
|------|-------|----------------------|
| 1 | `MINK_HOME=""` | `ErrMinkHomeEmpty` |
| 2 | `MINK_HOME=$HOME/.goose` | `ErrMinkHomeIsLegacyPath` |
| 3 | `MINK_HOME="/tmp/../etc/foo"` (raw `..` segment) | `ErrMinkHomePathTraversal` |
| 4 | `MINK_HOME=/tmp/ro-mink` (chmod 0400) | `ErrPermissionDenied` or `ErrReadOnlyFilesystem` |

Path traversal 검증 규칙:
- raw input 에 `..` segment 가 있으면 `filepath.Clean` 호출 전에 reject (OWASP Path Traversal mitigation)
- `filepath.Clean` 결과만 보면 `..` 가 사라지므로 traversal 시도가 무음 흡수될 수 있음 → raw input 검증 필수

### 4.3 Data integrity (REQ-009 / REQ-015 / R2)

Copy fallback 경로에서:
1. source 파일 → dest 파일 `io.Copy`
2. SHA-256 hash 계산 (source + dest)
3. hash 불일치 → dest 삭제 + `~/.goose/` source 그대로 + error 반환
4. hash 일치 → source 삭제 (verified)

mid-copy crash 시:
- partial `~/.mink/` 존재 + `~/.goose/` 그대로 → next run 에서 `.migration.lock` PID 검사 → stale 이면 cleanup + retry

---

## 5. Evaluator Scoring Dimensions (harness=thorough)

evaluator-active 가 본 SPEC 의 implementation 을 4 dimension 으로 평가한다:

| Dimension | Weight | Pass condition (must-pass criteria) |
|-----------|--------|-------------------------------------|
| Functionality | 30% | §1.1 ~ §1.8 모든 gate 통과 + §2 의 12 edge case 모두 자동 또는 manual smoke test 통과 |
| Security | 25% | §4 의 모든 security gate 통과 + AC-008b 4 negative case + AC-009 mode bits 검증 |
| Craft | 25% | TRUST 5 (Tested / Readable / Unified / Secured / Trackable) 모든 dimension 통과, `internal/userpath` coverage ≥ 90%, MX tag plan 적용 (plan.md §4) |
| Consistency | 20% | 19 EARS REQ × 16 AC 의 100% traceability (tasks.md §2.1 + §2.2), envalias reference pattern 준수 |

Pass threshold: 0.75 (harness=thorough default) — FROZEN floor 0.60 absolute minimum.

Must-pass criteria (cannot be compensated by other dimensions):
- §1.5 Coverage `internal/userpath/**` ≥ 90% (strict, no exception)
- §4.1 Mode bits 보존 (security boundary)
- §4.2 MINK_HOME 4 negative case (security boundary)
- §4.3 Data integrity verify-before-remove (data loss prevention)
- §1.6 brand-lint exit 0 (brand identity guarantee)

---

## 6. Acceptance Stamp

본 contract 의 모든 줄이 main branch 에서 통과하면 evaluator-active 가 SPEC-MINK-USERDATA-MIGRATE-001 을 `status: completed` 로 승격.

manager-strategy 가 본 contract draft 를 작성. expert-frontend / manager-tdd 가 RED phase 진입 전 본 contract 의 §1 ~ §4 를 review 하고 (a) accept-as-is 또는 (b) request adjustment (BRIEF 위반 시 propose alternative) 응답해야 한다 (design constitution §11 sprint contract protocol 의 정신을 backend implementation 에 적용).

---

Version: 0.1.0
Status: draft
Last Updated: 2026-05-13
