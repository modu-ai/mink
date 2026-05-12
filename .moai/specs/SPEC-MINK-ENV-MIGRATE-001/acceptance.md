# Acceptance Criteria — SPEC-MINK-ENV-MIGRATE-001

> 본 문서는 spec.md §5 의 10 AC 를 Given-When-Then 전수 시나리오 + 검증 명령으로 확장한다.
> 각 AC 는 plan.md 의 phase 와 1:1 매핑되며, phase 의 atomic commit 이 통과 조건이다.

## AC Coverage Matrix

| AC | REQ trace | Phase | Severity | Test type |
|----|-----------|-------|----------|----------|
| AC-MINK-EM-001 | REQ-MINK-EM-001, REQ-MINK-EM-002 | Phase 1 | Critical | unit + structural grep |
| AC-MINK-EM-002 | REQ-MINK-EM-004 | Phase 2, 6 | Critical | unit + integration |
| AC-MINK-EM-003 | REQ-MINK-EM-005 | Phase 2, 6 | Critical | unit + integration |
| AC-MINK-EM-004 | REQ-MINK-EM-006 | Phase 2, 6 | Critical | unit + integration |
| AC-MINK-EM-005 | REQ-MINK-EM-003, REQ-MINK-EM-007 | Phase 1 | Critical | unit (sync.Once, race detector) |
| AC-MINK-EM-006 | REQ-MINK-EM-008 | Phase 1 | High | unit (nil logger) |
| AC-MINK-EM-007 | (test consistency) | Phase 4 | High | grep automation |
| AC-MINK-EM-008 | REQ-MINK-EM-002 | Phase 1, 6 | High | table-driven + race |
| AC-MINK-EM-009 | (env scrub backward+forward compat) | Phase 4 | High | unit (table-driven) |
| AC-MINK-EM-010 | (prose migration) | Phase 5 | Medium | grep + visual review |

## AC-MINK-EM-001 — alias loader 패키지 + API 일치

**Given**: feature/SPEC-MINK-ENV-MIGRATE-001 branch (Phase 1 commit 적용 후)

**When**:
```bash
cd /Users/goos/.moai/worktrees/goose/SPEC-MINK-ENV-MIGRATE-001
ls internal/envalias/
grep -nE "func.*\(.*Loader.*\).*Get|^type EnvSource " internal/envalias/loader.go
go test -race ./internal/envalias
```

**Then**:
- `ls` 결과: `doc.go`, `keys.go`, `loader.go`, `loader_test.go` (최소 4 file)
- grep 결과: `func (l *Loader) Get(newKey string) (value string, source EnvSource, ok bool)` 라인 매칭
- grep 결과: `type EnvSource int` 라인 매칭
- `go test -race` PASS (race condition 없음)

**Negative case**:
- `internal/envalias/` 디렉토리 부재 → FAIL
- `Get` 함수 시그니처가 다름 (e.g., parameter 추가) → FAIL
- race detector report 가 있음 → FAIL (warnedMu mutex 보호 검증)

## AC-MINK-EM-002 — GOOSE_X only 시나리오

**Given**:
- `internal/envalias/` 패키지 + Phase 2 envOverlay migration 적용 후
- 환경: `unset MINK_LOG_LEVEL; export GOOSE_LOG_LEVEL=debug`

**When**:
- unit: `internal/envalias/loader_test.go` 의 `TestGet_GooseOnly_ReturnsValueAndWarnsOnce`
- integration: `cmd/minkd/integration_test.go` 의 `TestMain_EnvAlias_GooseHomeOnly` (`GOOSE_HOME=<tmp>`, `MINK_HOME` unset → minkd 기동 → stderr capture)

**Then**:
- unit 결과: `loader.Get("LOG_LEVEL")` → `("debug", SourceGoose, true)`
- unit 결과: observer logger 에 1개 log entry, fields `{"old":"GOOSE_LOG_LEVEL", "new":"MINK_LOG_LEVEL"}`
- integration 결과:
   - minkd 정상 기동 (exit code 0 after shutdown)
   - stderr line: `{"level":"warn", ..., "msg":"deprecated env var, please rename", "old":"GOOSE_HOME", "new":"MINK_HOME", "spec":"SPEC-MINK-ENV-MIGRATE-001"}`
   - 동일 process 내 GOOSE_HOME warning 라인 = 정확히 1개

**Verification command**:
```bash
go test -race -v -run TestGet_GooseOnly_ReturnsValueAndWarnsOnce ./internal/envalias
go test -race -v -run TestMain_EnvAlias_GooseHomeOnly ./cmd/minkd
```

## AC-MINK-EM-003 — MINK_X only 시나리오

**Given**: 환경 `unset GOOSE_LOG_LEVEL; export MINK_LOG_LEVEL=debug`

**When**:
- unit: `TestGet_MinkOnly_ReturnsValueNoWarn`
- integration: `TestMain_EnvAlias_MinkHomeOnly`

**Then**:
- unit 결과: `loader.Get("LOG_LEVEL")` → `("debug", SourceMink, true)`
- unit 결과: observer logger 에 0개 log entry (warning 없음)
- integration 결과:
   - minkd 정상 기동
   - stderr 에 `"GOOSE_HOME"` 단어 포함 라인 = 0개
   - 사용자에게 "your env is up to date" 의 silent UX

**Verification command**:
```bash
go test -race -v -run TestGet_MinkOnly_ReturnsValueNoWarn ./internal/envalias
go test -race -v -run TestMain_EnvAlias_MinkHomeOnly ./cmd/minkd
```

## AC-MINK-EM-004 — 동시 설정 시나리오 (NEW > OLD)

**Given**: 환경 `export GOOSE_LOG_LEVEL=debug; export MINK_LOG_LEVEL=info`

**When**:
- unit: `TestGet_BothSet_PrefersMinkAndWarnsConflictOnce`
- integration: `TestMain_EnvAlias_BothSet_PrefersMink`

**Then**:
- unit 결과: `loader.Get("LOG_LEVEL")` → `("info", SourceMink, true)`
- unit 결과: observer logger 의 1개 log entry, fields `{"msg":"both legacy and new env var set; using new key", "new":"MINK_LOG_LEVEL", "old":"GOOSE_LOG_LEVEL", "value_source":"MINK_LOG_LEVEL"}`
- integration 결과: minkd log level == info (debug 가 아님 — MINK_LOG_LEVEL 가 우선)
- stderr 에 "both legacy and new" 라인 1개 + `"new":"MINK_HOME"` 명시

**Verification command**:
```bash
go test -race -v -run TestGet_BothSet_PrefersMinkAndWarnsConflictOnce ./internal/envalias
go test -race -v -run TestMain_EnvAlias_BothSet_PrefersMink ./cmd/minkd
```

## AC-MINK-EM-005 — sync.Once 검증

**Given**: 환경 `export GOOSE_LOG_LEVEL=debug` 단일 process

**When**: `TestSyncOncePerKey` — loader.Get("LOG_LEVEL") 를 동일 process 내에서 1000회 호출 (table-driven, parallel goroutines 100 × sequential 10)

**Then**:
- 모든 1000회 호출이 동일 값 ("debug") 반환
- observer logger 에 deprecation warning 정확히 1개 entry
- race detector 통과

**Verification command**:
```bash
go test -race -v -run TestSyncOncePerKey ./internal/envalias
```

**Edge case sub-tests**:
- `TestSyncOncePerKey_Conflict_AlsoOnce` — 동시 설정 상황에서 conflict warning 도 1회만
- `TestSyncOncePerKey_DistinctKeysDistinctOnces` — `LOG_LEVEL` 와 `LOCALE` 각각 별개 sync.Once

## AC-MINK-EM-006 — logger nil safety

**Given**: `Loader` 가 `Options{Logger: nil, EnvLookup: ...}` 으로 생성됨, `GOOSE_LOG_LEVEL=debug`

**When**: `TestNilLoggerSafety` — `loader.Get("LOG_LEVEL")` 100회 호출 (panic 검증)

**Then**:
- panic 발생 안 함
- 모든 호출이 `("debug", SourceGoose, true)` 반환
- warning emit 은 silent skip (logger nil 이면 no-op)

**Verification command**:
```bash
go test -race -v -run TestNilLoggerSafety ./internal/envalias
```

**Negative case**:
- 코드가 `l.opts.Logger.Warn(...)` 를 nil check 없이 호출 → panic → FAIL

## AC-MINK-EM-007 — in-tree test migration 완료

**Given**: Phase 4 commit 적용 후

**When**:
```bash
cd /Users/goos/.moai/worktrees/goose/SPEC-MINK-ENV-MIGRATE-001
grep -rn 't\.Setenv("GOOSE_' --include="*.go" . \
   | grep -v "/vendor/" \
   | grep -v "envalias/loader_test.go" \
   | wc -l
```

**Then**: 결과 = 0

**Allowed exception (1 file)**: `internal/envalias/loader_test.go` — alias 동작 검증 전용 test 에서 `t.Setenv("GOOSE_LOG_LEVEL", ...)` 사용 의도된 backward compat 검증

**Edge case verification**:
- `internal/envalias/loader_test.go` 에서도 동시 설정 시나리오는 `t.Setenv("MINK_X", ...)` + `t.Setenv("GOOSE_X", ...)` 둘 다 사용 (의도)
- `os.Setenv` 사용은 0건 (production-wide setenv 금지, 모두 `t.Setenv` 사용):
   ```bash
   grep -rn 'os\.Setenv("GOOSE_\|os\.Setenv("MINK_' --include="*.go" . \
      | grep -v "/vendor/" | grep -v "_example" | wc -l
   # 목표: 0
   ```

## AC-MINK-EM-008 — alias 동작 검증 test 존재

**Given**: Phase 1 + Phase 4 commit 적용 후

**When**: `go test -v ./internal/envalias`

**Then**: 최소 다음 sub-test 들 PASS:
- `TestEnvSourceString` — enum 메서드
- `TestNew_DefaultsEnvLookupToOsGetenv` — defaults
- `TestGet_UnregisteredKey_ReturnsDefault` — REQ-MINK-EM-009 (strict mode off)
- `TestGet_MinkOnly_ReturnsValueNoWarn` — REQ-MINK-EM-005
- `TestGet_GooseOnly_ReturnsValueAndWarnsOnce` — REQ-MINK-EM-004
- `TestGet_BothSet_PrefersMinkAndWarnsConflictOnce` — REQ-MINK-EM-006
- `TestSyncOncePerKey` — REQ-MINK-EM-003, REQ-MINK-EM-007
- `TestSyncOncePerKey_Conflict_AlsoOnce` — REQ-MINK-EM-007
- `TestSyncOncePerKey_DistinctKeysDistinctOnces` — REQ-MINK-EM-003
- `TestNilLoggerSafety` — REQ-MINK-EM-008
- `TestAllKeysRegistered` — REQ-MINK-EM-002 (22-key mapping 검증)
- `TestStrictMode_UnknownKey_Logs` — REQ-MINK-EM-009 (optional)

**Verification command**:
```bash
go test -race -v ./internal/envalias | grep "PASS:" | wc -l
# 목표: 10+ PASS lines
```

## AC-MINK-EM-009 — env scrub deny-list 확장

**Given**: Phase 4 commit 적용 후

**When**:
- code grep: `grep -n "MINK_AUTH_\|GOOSE_AUTH_" internal/hook/isolation_*.go`
- test: `go test -v -run TestScrubEnv_DenyList ./internal/hook`

**Then**:
- grep 결과: 두 file 모두 다음 패턴 포함:
   ```go
   if strings.HasPrefix(upper, "MINK_AUTH_") || strings.HasPrefix(upper, "GOOSE_AUTH_") {
       return true
   }
   ```
- test 결과:
   - `MINK_AUTH_TOKEN=zzz` 가 scrubEnv 결과에서 제외됨 (assert.NotContains)
   - `MINK_AUTH_REFRESH=ref` 도 동일
   - `GOOSE_AUTH_TOKEN=zzz`, `GOOSE_AUTH_REFRESH=ref` 도 backward compat 유지 (assert.NotContains)

**Verification command**:
```bash
grep -E 'HasPrefix.*MINK_AUTH_|HasPrefix.*GOOSE_AUTH_' \
   internal/hook/isolation_unix.go internal/hook/isolation_other.go | wc -l
# 목표: 최소 4 (each file 2 line: MINK + GOOSE prefix)
go test -race -v -run TestScrubEnv_DenyList ./internal/hook
```

## AC-MINK-EM-010 — 산문/주석/error message migration 완료

**Given**: Phase 5 commit 적용 후

**When**:
```bash
cd /Users/goos/.moai/worktrees/goose/SPEC-MINK-ENV-MIGRATE-001
grep -rn "GOOSE_" --include="*.go" . | grep -v "/vendor/" \
   | grep -v "envalias/" \
   | grep -v "_test.go" \
   | grep -v 'isolation_.*GOOSE_AUTH_' \
   | wc -l
```

**Then**: 결과 = 0

**Allowed exceptions**:
- `internal/envalias/keys.go` — alias mapping 정의
- `internal/envalias/loader.go` — deprecation warning format string ("old": "GOOSE_X")
- `internal/envalias/doc.go` — package documentation
- `internal/hook/isolation_unix.go`, `internal/hook/isolation_other.go` — `GOOSE_AUTH_` prefix glob 유지 (backward compat)
- `_test.go` files — alias 동작 검증 test 의 의도된 인용 + alias 동작 검증
- 본 SPEC 의 `.moai/specs/SPEC-MINK-ENV-MIGRATE-001/*.md` — 의도된 인용

**Additional visual review checklist**:
- error message: `messaging_telegram.go:68, 138` 의 `"GOOSE_TELEGRAM_BOT_TOKEN"` 표기 → `"MINK_TELEGRAM_BOT_TOKEN (legacy: GOOSE_TELEGRAM_BOT_TOKEN)"` 형식 update
- 한국어 주석: `qwen/client.go`, `kimi/client.go` 의 `// GOOSE_QWEN_REGION 환경변수 키` → `// MINK_QWEN_REGION 환경변수 키 (legacy alias: GOOSE_QWEN_REGION)`
- @MX:SPEC tag: `internal/config/env.go:18` 의 `@MX:SPEC: SPEC-GOOSE-CONFIG-001 §6.2` 가 본 SPEC reference 추가됨 (`+ SPEC-MINK-ENV-MIGRATE-001 §7`)

**Verification command**:
```bash
# 자동 검증
grep -rn "GOOSE_" --include="*.go" . | grep -v "/vendor/" \
   | grep -v "envalias/" | grep -v "_test.go" \
   | grep -v 'isolation_.*GOOSE_AUTH_' | tee /tmp/goose-residue.txt
test ! -s /tmp/goose-residue.txt && echo "AC-MINK-EM-010 PASS"
```

---

## Definition of Done (전체 SPEC)

본 SPEC 의 6 phase 6 commit (또는 squash 1 PR) 이 모두 적용된 상태에서:

- [ ] AC-MINK-EM-001 ~ AC-MINK-EM-010 전수 PASS
- [ ] `go build ./...` PASS
- [ ] `go vet ./...` PASS
- [ ] `gofmt -l . | (! grep .)` PASS (no formatting drift)
- [ ] `go test -race ./...` PASS
- [ ] `golangci-lint run --timeout 5m` PASS (installed only)
- [ ] CHANGELOG.md 의 unreleased section 에 본 SPEC entry 추가
- [ ] PR description 에 22-key migration 표 + verification result 포함
- [ ] frontmatter `status: draft` → `status: approved` (plan-auditor PASS 후)
- [ ] frontmatter `issue_number` 채움 (GH Issue 생성 후)

---

## Cumulative Open Question (acceptance 단계)

| OQ # | Question | 결정 시점 |
|------|----------|----------|
| OQ-AC-1 | sync.Once 검증의 goroutine 수 (100 × 10) 가 충분한가? race detector 가 잡아낼 보장? | Phase 1 에서 ulimit 한도 내 goroutine 수 결정 — 100 × 10 = 1000 호출이 합리적 (대다수 race 시나리오 cover). |
| OQ-AC-2 | integration test 의 stderr capture 가 stderr/stdout 혼선 위험? | `cmd/minkd/integration_test.go` 의 기존 wire test 가 동일 패턴 사용 — 검증된 방법, 동일 패턴 채택. |
| OQ-AC-3 | `golangci-lint` 가 본 CI 에 강제인가 optional 인가? | research: CI workflow 미확인 (해당 .github/ 부재 가능성). 본 SPEC 에서는 optional 명시. plan-auditor 가 CI workflow 검토 시 hard 로 승격 가능. |

End of acceptance.md.
