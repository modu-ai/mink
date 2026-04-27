## SPEC-GOOSE-CMDCTX-HOTRELOAD-001 Progress

- Started: 2026-04-27 (plan phase)
- Status: planned
- Mode: TDD (quality.yaml development_mode=tdd 가정 — run phase 진입 시 재확인)
- Harness: standard (file_count<10 예상, 단일 Go domain, runtime/config 영역, 보안/결제 영역 아님)
- Scale-Based Mode: Standard
- Language: Go (moai-lang-go)
- Greenfield 여부: 부분적 — `internal/command/adapter/hotreload/` 신규, `internal/command/adapter/` 자체는 amendment 대상
- Branch base: `feature/SPEC-CMDCTX-FOLLOWUPS-batch-plan` (CMDCTX-001 followups batch 의 일환)

### 의존 SPEC 상태 확인

| SPEC | Status | 본 SPEC 와의 관계 |
|------|--------|----------------|
| SPEC-GOOSE-CMDCTX-001 | implemented v0.1.1 (FROZEN, PR #52 c018ec5) | **amendment 대상**. 본 SPEC implementation 시점에 v0.1.1 → v0.X.0 (X >= 2) 갱신. ContextAdapter struct (§6.2), ResolveModelAlias (§6.4 step 1-2), race 안전성 (§6.6), Exclusions #8 동시 변경. |
| SPEC-GOOSE-ALIAS-CONFIG-001 | planned (Batch A) | 데이터 소스. `aliasconfig.LoadDefault` / `aliasconfig.Validate` 를 watcher 가 호출. surface 변경 없음, read-only consume. **본 SPEC 의 implementation 은 ALIAS-CONFIG-001 implementation 후 진행 권장**. |
| SPEC-GOOSE-CMDCTX-PERMISSIVE-ALIAS-001 | planned (P4) | **amendment 충돌 가능**. 동일 CMDCTX-001 v0.X.0 amendment 대상. §6.4 본문 동시 수정 가능성. governance: research §7.3 의 직렬 머지 + 직렬 버전 bump 트랙. |
| SPEC-GOOSE-ROUTER-001 | implemented (FROZEN) | `*router.ProviderRegistry` API read-only 사용. **변경 없음**. |
| SPEC-GOOSE-COMMAND-001 | implemented (FROZEN) | `command.SlashCommandContext` 인터페이스 read-only 사용 (Optional `/reload aliases` builtin 등록 시 `command.Metadata` 활용). **변경 없음**. |

### Phase Log

- 2026-04-27 plan phase 시작
  - 부모 SPEC: CMDCTX-001 v0.1.1 §Exclusions #8 ("Hot-reload of registry / aliasMap") 의 명시적 후속 plan
  - 본 SPEC 산출물:
    - `research.md` — 8 design decisions, ContextAdapter v0.1.1 분석, fsnotify cross-platform 분석, debounce 정책, amendment 충돌 governance
    - `spec.md` — 19 REQ / 23 AC / EARS 5 카테고리 (Ubiquitous 6, Event 4, State 3, Unwanted 6, Optional 3) + 12 Exclusions + 13 Acceptance Summary
    - `progress.md` — 본 파일
  - 다음 단계 후보:
    - (a) plan-auditor 사이클 (independent SPEC 검토, EARS 컴플라이언스, 의존 SPEC 영향 확인, amendment governance 검증)
    - (b) 사용자 검토 → /moai run 분기 결정 (단, ALIAS-CONFIG-001 implementation 선행 권장)

### Open Questions (사용자 결정 필요)

research §9 Open Questions 와 동일:

1. **fsnotify build tag 분기**: default-on 채택 (단순성) vs `// +build fsnotify` optional 화. 권장: v0.1.0 default-on, optional 화는 후속 결정 (REQ-HOTRELOAD-041 stub).
2. **CMDCTX-001 amendment 버전 숫자**: PERMISSIVE-ALIAS-001 (P4) 와 본 SPEC (P4) 둘 다 v0.2.0 차지 시도. governance: 머지 순서에 따라 v0.2.0 / v0.3.0 직렬 bump. 사용자가 머지 순서 결정.
3. **ReloadRegistry 필요성**: registry hot-reload 호출자가 v0.1.0 에서는 부재 (dynamic provider registration SPEC 미존재). v0.1.0 에 surface 만 노출 vs aliasMap 만 hot-reload. 권장: 둘 다 atomic.Pointer 로 전환 + ReloadRegistry surface 노출 (future-proofing). 호출자는 후속 SPEC.
4. **active model invalidation 정책**: REQ-HOTRELOAD-040 Optional, v0.1.0 stub. 후속 SPEC 책임.
5. **다중 watcher 인스턴스**: v0.1.0 단일 ContextAdapter 가정 (CMDCTX-001 §Exclusions #9 와 동일).

### 산출 파일

- `/Users/goos/MoAI/AI-Goose/.moai/specs/SPEC-GOOSE-CMDCTX-HOTRELOAD-001/research.md`
- `/Users/goos/MoAI/AI-Goose/.moai/specs/SPEC-GOOSE-CMDCTX-HOTRELOAD-001/spec.md`
- `/Users/goos/MoAI/AI-Goose/.moai/specs/SPEC-GOOSE-CMDCTX-HOTRELOAD-001/progress.md`

### Next-step Hand-off Notes

- **선행 의존성 확인**: ALIAS-CONFIG-001 implementation 완료 후 본 SPEC run phase 진입 권장. ALIAS-CONFIG-001 의 `aliasconfig.LoadDefault` / `aliasconfig.Validate` surface 가 본 SPEC watcher 의 입력.
- **amendment governance**: 본 SPEC implementation 머지 직전에 PERMISSIVE-ALIAS-001 / TELEMETRY (가칭) SPEC 의 plan/run 상태 점검. 직렬 머지 순서 결정 후 CMDCTX-001 amendment version 숫자 fix.
- **Run phase 시 manager-tdd 가 본 SPEC 으로 진입할 때 다음 자료 우선 로딩**:
  - `internal/command/adapter/adapter.go` (ContextAdapter struct — amendment 대상)
  - `internal/command/adapter/alias.go` (resolveAlias 알고리즘 — step 1-2 read adaptation)
  - `internal/command/adapter/adapter_test.go` (race detector 패턴 — AC-CMDCTX-014 회귀 보존)
  - `.moai/specs/SPEC-GOOSE-CMDCTX-001/spec.md` (FROZEN reference — amendment 영역 식별)
  - `.moai/specs/SPEC-GOOSE-ALIAS-CONFIG-001/spec.md` (`aliasconfig` 패키지 surface — watcher 데이터 소스)
- **검증 필수 정적 체크**:
  - AC-HOTRELOAD-035: `go vet -copylocks ./internal/command/adapter/...` 0 violation
  - AC-HOTRELOAD-051: `go.mod` diff CI gate (fsnotify 1건만 신규 require)
  - AC-HOTRELOAD-005: 정적 grep — ResolveModelAlias 본문 내 atomic.Load() 1회만 호출
  - AC-HOTRELOAD-052: CMDCTX-001 의 19개 AC 회귀 PASS
- **신규 작성 파일 (예상)**:
  - `internal/command/adapter/hotreload/watcher.go`
  - `internal/command/adapter/hotreload/watcher_test.go`
  - `internal/command/adapter/hotreload/debounce.go`
  - `internal/command/adapter/hotreload/debounce_test.go`
  - `internal/command/builtin/reload.go` (Optional REQ-HOTRELOAD-013)
- **수정 예상 파일**:
  - `internal/command/adapter/adapter.go` — ContextAdapter struct + New(...) 초기화 + Reload* API 추가
  - `internal/command/adapter/alias.go` — resolveAlias step 1-2 read adaptation
  - `internal/command/adapter/adapter_test.go` — neue race + reload 테스트 추가
  - `internal/command/adapter/errors.go` — ErrNilAliasMap, ErrNilRegistry 추가
  - `cmd/goosed/main.go` 또는 등가 부트스트랩 — Watcher.Run(ctx) wiring 1회 추가
  - `.moai/specs/SPEC-GOOSE-CMDCTX-001/spec.md` — amendment governance 적용 (frontmatter version, HISTORY, §1, §6.2, §6.4, §6.6, §Exclusions)
  - `go.mod` / `go.sum` — fsnotify 1건 require 추가
