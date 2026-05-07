---
id: SPEC-GOOSE-TOOLS-WEB-001
artifact: strategy
scope: M1 (Search & HTTP + common infra) only
version: 0.1.0
created_at: 2026-05-06
author: manager-strategy
operating_mode: ultrathink + thorough harness + TDD
---

# SPEC-GOOSE-TOOLS-WEB-001 — Implementation Strategy (M1)

본 문서는 manager-strategy 가 SPEC-GOOSE-TOOLS-WEB-001 의 **M1 (Search & HTTP + common infra)** 만을 대상으로
한 단일 실행 계획이다. M2~M4 는 후속 세션에서 별도 strategy 로 다룬다.

---

## 1. ASSUMPTIONS I'M MAKING

각 항목은 잘못 추정될 경우 실패 비용이 큰 순서로 정렬했다. 사용자가 설명 없이 진행에 동의했으므로
구현 직전 다시 손볼 가치가 있는 추정만 명시한다.

1. **jsonschema 라이브러리는 v6 를 재사용한다 (plan.md §7 의 v5 표기는 typo).**
   - 근거: `go.mod` 가 `github.com/santhosh-tekuri/jsonschema/v6 v6.0.2` 한 줄만 보유. v5 는 부재.
   - 확인 위치: `go.mod` 라인 (grep 결과). `internal/tools/registry.go:11` 에서 `"github.com/santhosh-tekuri/jsonschema/v6"` 사용.
   - 위험: 만약 사용자가 v5 를 의도했다면 plan.md 를 수정하도록 요청. 본 strategy 는 v6 채택을 전제로 한다.

2. **Schema validation 은 Tool 별 개별 책임이 아니라 `Executor.validateInput` 이 도맡는다.**
   - 근거: `internal/tools/executor.go:94-97, 141-170` — Executor 가 등록 시점 컴파일된 schema 를 사용해 입력을 검증.
   - 결과: `web_search.Call()` / `http_fetch.Call()` 은 schema-pre-validated 입력을 받는다. 도구 내부에서 별도 schema 검증은 중복.
   - 도구 본체는 enum/range 위반을 가정하지 않고 곧장 동작한다.

3. **Tool 등록 패턴은 builtin 의 init() 자동 등록 + RegisterBuiltin 패턴을 미러한다.**
   - 근거: `internal/tools/registry.go:407-418` (`globalBuiltins` 슬라이스), `internal/tools/builtin/builtin.go:9-11` (`Register` thin wrapper),
     `internal/tools/builtin/file/read.go:17-19` (각 도구 init() 호출).
   - 본 SPEC 은 `tools.RegisterWebTool` + `internal/tools/web/register.go` 에서 `WithWeb()` 옵션 노출.

4. **PERMISSION-001 통합은 `permission.Manager.Check()` 직접 호출이 아니라 `permissions.CanUseTool` (tool/Executor 경유) 가 부족하다.**
   - 근거: `Executor` 의 step 4 `canUseTool.Check(ctx, req.PermissionCtx)` (`executor.go:110-122`) 는 tool 실행 전 게이트.
     첫 호출 동의(Confirmer.Ask)는 `permission.Manager.Check()` 가 책임. 두 시스템은 별도.
   - 결과: web 도구는 `permission.Manager.Check(ctx, PermissionRequest{SubjectType: SubjectAgent, Capability: CapNet, Scope: <host>})` 를
     `Tool.Call()` 진입 시점에 직접 호출해야 한다. Executor 의 CanUseTool 은 별도 (tool execution gate, not first-call confirm).
   - **이 점은 plan.md / acceptance.md 가 묵시적으로 가정한 흐름이다. 본 strategy 는 web 도구 `Call()` 메서드가
     Permission Manager 를 의존성 주입(DI) 으로 받아 직접 호출하는 디자인을 채택한다.**

5. **AUDIT-001 의 web 호출 이벤트는 신규 EventType 을 도입한다.**
   - 근거: `internal/audit/event.go:15-40` 의 EventType 상수 목록에 `tool.web.invoke` 또는 `tool.web.sandbox_warning` 없음.
   - 결과: M1 에서 `EventTypeToolWebInvoke EventType = "tool.web.invoke"` 와
     `EventTypeToolWebSandboxWarning EventType = "tool.web.sandbox_warning"` 를 audit 패키지에 추가하는 PR(또는 본 SPEC 작업 일환).

6. **Subject ID 는 호출 chain 의 어디서 부여되는가?**
   - 추정: web 도구의 SubjectID 는 `"agent:<agent_name>"` 또는 `"skill:<skill_name>"` 형태이며, Executor 호출 chain 에서
     `req.PermissionCtx` 또는 별도 ctx value 로 전달된다.
   - **불확실**: 현재 ExecRequest 에는 명시적 SubjectID 필드가 없음. 본 strategy 는 "M1 기간에는 단일 SubjectID `"agent:goose"`
     를 web 패키지 init 에서 설정 가능한 변수로 노출하고, 호출 측에서 ctx 로 override 하는 인터페이스" 를 제안한다.
   - 후속 세션 검토 필요. M1 코드 작성 직전 manager-strategy 가 다시 읽어볼 항목.

7. **`~/.goose/cache/web/**` 는 FS-ACCESS-001 default seed 가 아직 비어 있다.**
   - 근거: `internal/fsaccess/` 디렉토리에 web 관련 path 미발견 (grep 결과 empty).
   - 결과: M1 의 cache layer 가 첫 write 시점에 fsaccess 의 `DecisionAsk` 결과를 받게 됨. 본 SPEC 은 default policy 에
     `~/.goose/cache/web/**` 를 추가하는 작업을 M1 에 포함한다 (plan.md §11 의 별도 PR 항목).

8. **Brave provider 의 quota 1회 호출 비용 ≥ 1 query.**
   - 결과: integration test 는 build tag `integration` + `BRAVE_SEARCH_API_KEY` 환경변수 부재 시 `t.Skip` — local CI 에서는 안전.
     하지만 PR 머지 시 nightly CI 가 매 PR 마다 1+ query 소비. 한 달 2,000 quota 를 surplus 로 가정.

9. **Playwright / readability / gofeed 는 M2~M4 에서만 사용. M1 에는 들어가지 않는다.**
   - 결과: M1 PR 에 추가될 외부 의존성은 4개만 — `temoto/robotstxt`, `bbolt`, `hashicorp/golang-lru/v2` (이미 등록됨), `errgroup` (golang.org/x/sync, 이미 등록 가능성 큼).

10. **Test 전략은 TDD (RED-GREEN-REFACTOR). brownfield 적용 (기존 패키지 옆에 신규 패키지 추가).**
    - 결과: 테스트 파일 먼저 작성 (RED), 이어서 최소 구현 (GREEN), 마지막에 리팩터 (REFACTOR).
    - manager-tdd 가 RED 단계마다 `go test ./internal/tools/web/...` 가 실패하는 것을 확인한 후 GREEN 으로 진입.

→ Correct me now or M1 implementation will proceed under these assumptions.

---

## 2. Existing Infrastructure Map

모든 인용은 `internal/...` relative path 기준.

### 2.1 Tool Interface — `internal/tools/tool.go`
- L13-22: `Tool` 인터페이스 4 메서드 (`Name() string`, `Schema() json.RawMessage`, `Scope() Scope`, `Call(ctx, input) (ToolResult, error)`).
- L25-32: `ToolResult{Content []byte, IsError bool, Metadata map[string]any}` — 도구가 반환하는 타입.
- 결과: web 도구 8종은 `ToolResult{Content: <표준응답shape JSON>, IsError: <ok==false 여부>, Metadata: nil 또는 cache_hit 등}` 형태로 반환.
  표준 응답 shape 의 outer JSON 은 `Content` 에 들어가고, ToolResult 의 `Metadata` 는 별도 (mixin 가능).

### 2.2 Registry — `internal/tools/registry.go`
- L72-83: `WithBuiltins() Option` — `globalBuiltins` 슬라이스를 순회하여 등록. **본 SPEC 의 `WithWeb()` 가 그대로 미러할 패턴.**
- L143-190: `Register(t Tool, src Source) error` — duplicate / schema invalid / strict_schema 검증.
- L290-298: `Resolve(name string) (Tool, bool)` — RWMutex 보호 lookup.
- L310-319: `ListNames() []string` — alphabetical sort (AC-WEB-001 검증 사용).
- L407-418: `globalBuiltins` 슬라이스 + `RegisterBuiltin(t Tool)` — 신규 `globalWebTools` + `RegisterWebTool(t Tool)` 추가 필요.

### 2.3 Executor — `internal/tools/executor.go`
- L78-139: `Run(ctx, req) ToolResult` — 5-step pipeline:
  - Step 1 Resolve, Step 2 Schema validate (`validateInput`), Step 3 Preapproved (settings.json allow), Step 4 CanUseTool gate, Step 5 Tool.Call.
- L141-170: `validateInput()` — 등록 시점 컴파일된 schema 로 검증. **web 도구는 schema validation 위임 OK.**
- 결과: web 도구 자체에서는 schema 재검증 불필요. AC-WEB-010 (method allowlist) 는 Executor 단계에서 자동 처리됨.

### 2.4 Scope — `internal/tools/scope.go`
- L4-14: `ScopeShared = 0`, `ScopeLeaderOnly`, `ScopeWorkerShareable`. **본 SPEC 의 8 도구 모두 `ScopeShared`.**

### 2.5 Permission Manager — `internal/permission/`
- `manager.go:166-236`: `Manager.Check(ctx, PermissionRequest) (Decision, error)`. 결정 흐름:
  1. registry 조회 (subject 등록 여부, REQ-PE-012)
  2. plugin integrity (REQ-PE-020)
  3. blocked_always (REQ-PE-009)
  4. manifest declares (REQ-PE-001)
  5. per-triple lock + Store.Lookup → first-call Confirmer.Ask
- `grant.go:33-48`: `PermissionRequest{SubjectID, SubjectType, ParentSubjectID, InheritGrants, Capability, Scope, RequestedAt}`.
- `grant.go:81-91`: `Decision{Allow bool, Choice DecisionChoice, ExpiresAt *time.Time, Reason string}`.
- `grant.go:11-19`: `DecisionAlwaysAllow / DecisionOnceOnly / DecisionDeny`.
- `manifest.go:5-16`: `Capability` enum: `CapNet / CapFSRead / CapFSWrite / CapExec`. **web 도구는 CapNet 사용.**
- `store.go:20-37`: `Store` 인터페이스 (`Open / Lookup / Save / Revoke / List / GC / Close`).
- `manager.go:106-117`: `Manager.Register(subjectID string, manifest Manifest) error` — **subject 가 manifest 에 net hosts 를 선언해야 Check 통과.**
  - 본 SPEC 의 web 도구는 호출 직전 `Manager.Register("agent:goose-web-search", Manifest{NetHosts: []string{"api.search.brave.com"}})` 처럼
    동적으로 manifest 를 등록하는 것이 최소-변경 디자인. **또는 web 도구 첫 호출 시점에 자동 register 하는 helper 함수 도입.**

### 2.6 Permission Tool Matcher — `internal/tools/permission/`
- `config.go:6-16`: `Config{Allow []string, Deny []string, AdditionalDirectories []string}` — settings.json permissions allowlist.
- `matcher.go:11-31`: `Matcher.Preapproved(toolName, input, cfg) (bool, string)` — Executor step 3.
- 결과: settings.json 의 `permissions.allow: ["web_search(*)", "http_fetch(https://*)"]` 같은 패턴이 있으면 first-call confirm 우회.
  M1 는 default 로 web 도구 패턴을 allow 에 추가하지 **않는다** (PERMISSION-001 동의 흐름이 동작하는지 검증 우선).

### 2.7 Audit — `internal/audit/`
- `event.go:15-40`: `EventType` 상수. **본 SPEC 은 `EventTypeToolWebInvoke = "tool.web.invoke"`,
  `EventTypeToolWebSandboxWarning = "tool.web.sandbox_warning"` 추가 필요.**
- `event.go:94-107`: `AuditEvent{Timestamp, Type, Severity, Message, Metadata, PrevHash}`.
- `event.go:112-120`: `NewAuditEvent(timestamp, eventType, severity, message, metadata)` — 생성 helper.
- `writer.go:32-53`: `NewFileWriter(path) (*FileWriter, error)` — append-only JSON Lines.
- `writer.go:61-91`: `(w *FileWriter).Write(event AuditEvent) error` — 동시성 안전.
- `dual.go:39-70`: `NewDualWriter(config DualWriterConfig)` — global + local 동시 기록 (DualWriter 권장 사용).
- 결과: M1 의 audit 통합은 DualWriter 인스턴스를 web 도구 패키지 변수 또는 DI 로 받아 `dw.Write(audit.NewAuditEvent(...))` 호출.

### 2.8 RateLimit Tracker — `internal/llm/ratelimit/`
- `tracker.go:55-79`: `New(opts TrackerOptions) (*Tracker, error)` — Parsers/Observers/ThresholdPct/WarnCooldown 설정.
- `tracker.go:86-130`: `(t *Tracker).Parse(provider string, headers map[string]string, now time.Time) error` — 4-bucket 갱신 + threshold 평가.
- `tracker.go:193-203`: `(t *Tracker).State(provider string) RateLimitState` — copy 반환.
- `bucket.go:9-15`: `RateLimitBucket{Limit, Remaining, ResetSeconds, CapturedAt}`.
- `bucket.go:29-34`: `(b RateLimitBucket).UsagePct() float64` — exhausted 검출용 (>=100).
- `event.go:7-10`: `BucketRequestsMin / BucketRequestsHour / BucketTokensMin / BucketTokensHour` 문자열 상수.
- 결과: web 도구는 호출 전 `tracker.State(provider).RequestsMin.UsagePct() >= 100` 검사하여 `ratelimit_exhausted` 반환. 응답 후
  `tracker.Parse(provider, respHeadersAsMap, time.Now())` 호출.
- **주의**: Brave / Tavily / Exa 의 헤더 포맷에 맞는 `Parser` 구현이 필요. 현재 `parser_anthropic.go` / `parser_openai.go` /
  `parser_openrouter.go` 만 등록됨. **M1 task 에 `parser_brave.go` (또는 web 패키지 내부 parser) 추가 필요.**

### 2.9 fsaccess seed — `internal/fsaccess/`
- `policy.go:21-30`: `SecurityPolicy{WritePaths []string, ReadPaths []string, BlockedAlways []string}`.
- 결과: `~/.goose/cache/web/**` 를 default `SecurityPolicy.WritePaths` 에 추가하는 작업이 M1 task list 에 포함된다.
  template 파일 위치는 `internal/template/templates/.moai/config/sections/security.yaml.tmpl` 추정 (확인은 M1 RED 단계에서).

### 2.10 sandbox — `internal/sandbox/`
- `landlock.go:45`: `newLandlockSandbox(cfg Config) (Sandbox, error)` — Linux only entry. **M1 에서는 호출 안 함 (Playwright 미사용).**
- M2 에서 `web_browse` 가 활용. M1 strategy 범위 외.

### 2.11 외부 라이브러리 in go.sum
- `github.com/santhosh-tekuri/jsonschema/v6 v6.0.2` ✓ (이미 사용 중)
- `github.com/hashicorp/golang-lru/v2 v2.0.7` ✓ (이미 등록)
- `github.com/temoto/robotstxt` ✗ (M1 에서 신규 추가)
- `go.etcd.io/bbolt` ✗ (M1 에서 신규 추가)
- `golang.org/x/sync` (errgroup) — 별도 grep 필요 (M2 web_rss 에서 사용, M1 에서는 미사용 가능)

---

## 3. M1 Architecture Overview

### 3.1 패키지 레이아웃 (M1 only — 17 파일)

```
internal/tools/web/
├── doc.go                      # package overview
├── register.go                 # WithWeb() option, RegisterWebTool helper, globalWebTools slice
├── search.go                   # web_search Tool 구현
├── search_test.go              # web_search RED tests
├── http.go                     # http_fetch Tool 구현
├── http_test.go                # http_fetch RED tests
├── ratelimit_brave_parser.go   # Brave 응답 헤더 → ratelimit.Parser 구현
└── common/
    ├── response.go             # 표준 응답 wrapper {ok,data|error,metadata}
    ├── response_test.go        # AC-WEB-012 일관성 검증
    ├── useragent.go            # goose-agent/{version} 빌드
    ├── cache.go                # bbolt TTL 캐시
    ├── cache_test.go           # AC-WEB-007
    ├── robots.go               # robots.txt fetch + 24h LRU 캐시 (temoto/robotstxt)
    ├── robots_test.go          # AC-WEB-004
    ├── safety.go               # blocklist + redirect cap + size cap (LimitReader)
    ├── safety_test.go          # AC-WEB-005, AC-WEB-006, AC-WEB-009
    └── deps.go                 # dependency injection: Confirmer/Auditor/Tracker DI struct
```

### 3.2 도구 호출 시퀀스 (web_search.Call() / http_fetch.Call() 공통)

엄격한 순서. 각 단계 실패 시 즉시 표준 error response 반환.

```
1. blocklist lookup (common.safety.IsHostBlocked(host))
   → fail: {ok:false, error:{code:"host_blocked"}} + audit "tool.web.invoke" outcome=denied reason=host_blocked
   → PERMISSION 단계 진입 전. AC-WEB-009 충족.

2. robots.txt fetch + check (common.robots.IsAllowed(host, path, userAgent))
   - provider API endpoint (api.search.brave.com / api.tavily.com / api.exa.ai) 는 skip — REQ-WEB-005 단서.
   → fail: {ok:false, error:{code:"robots_disallow"}} + audit outcome=denied reason=robots_disallow
   → AC-WEB-004 충족.

3. permission Manager.Check (CapNet, scope=host)
   - SubjectID = ctx 에서 추출 (default "agent:goose").
   - first-call → Confirmer.Ask 호출 (orchestrator 가 prompt 처리).
   - Choice=Deny → {ok:false, error:{code:"permission_denied"}} + (Manager 가 audit 처리)
   → AC-WEB-003 충족.

4. ratelimit Tracker.State 검사
   - tracker.State(provider).RequestsMin.UsagePct() >= 100 또는 RequestsHour >= 100 시
   → {ok:false, error:{code:"ratelimit_exhausted", retry_after_seconds: int(state.RequestsMin.RemainingSecondsNow(now)), retryable:true}}
   → AC-WEB-008 충족.

5. cache lookup (common.cache.Get(key)) — key = SHA256(toolName + canonicalInput).
   → hit: {ok:true, data:..., metadata:{cache_hit:true, duration_ms:int}}
   → miss: 다음 단계로.

6. outbound http.Client.Do(req) with:
   - User-Agent: "goose-agent/{version}" (REQ-WEB-003)
   - CheckRedirect: 5회 cap (max_redirects 0..10 override) — common.safety.RedirectGuard
   - response body: io.LimitReader(body, 10*1024*1024 + 1) — 10MB hard cap (REQ-WEB-012)
   → redirect 6회: {ok:false, error:{code:"too_many_redirects"}} (AC-WEB-005)
   → size > 10MB: {ok:false, error:{code:"response_too_large"}} (AC-WEB-006)

7. response normalize (provider-specific) → 표준 data shape

8. cache write (TTL 24h or override from web.yaml)

9. ratelimit Tracker.Parse(provider, respHeadersAsMap, time.Now()) — 비파괴적, REQ-WEB-007.

10. audit DualWriter.Write(NewAuditEvent("tool.web.invoke", SeverityInfo, "...", metadata{tool, host, method, status_code, cache_hit, duration_ms, outcome:"ok"}))
    → AC-WEB-018 충족.

11. return {ok:true, data:..., metadata:{cache_hit:false, duration_ms:int}}
```

### 3.3 `WithWeb()` Option Signature (mirror of WithBuiltins)

```go
// internal/tools/web/register.go (계획)
package web

import "github.com/modu-ai/goose/internal/tools"

// globalWebTools 는 init() 에서 등록되는 web tool 목록 (mirror of registry.globalBuiltins).
var (
    globalWebToolsMu sync.Mutex
    globalWebTools   []tools.Tool
)

// RegisterWebTool 은 web 패키지 init() 에서 호출하여 globalWebTools 에 추가.
func RegisterWebTool(t tools.Tool) {
    globalWebToolsMu.Lock()
    defer globalWebToolsMu.Unlock()
    globalWebTools = append(globalWebTools, t)
}

// WithWeb 는 globalWebTools 를 Registry 에 등록하는 Option 을 반환.
// 호출 측: tools.NewRegistry(tools.WithBuiltins(), web.WithWeb())
func WithWeb() tools.Option {
    return func(r *tools.Registry) {
        for _, t := range globalWebTools {
            if err := r.Register(t, tools.SourceBuiltin); err != nil {
                panic(fmt.Sprintf("web tool registration failed for %q: %v", t.Name(), err))
            }
        }
    }
}
```

각 web 도구 파일은 `func init() { web.RegisterWebTool(NewWebSearch(deps)) }` 형식. `deps` 는 패키지 변수
또는 lazy initialization 으로 Confirmer/Auditor/Tracker 를 받음. **명시적 DI struct 권장 (init() 에서는 nil 인스턴스
등록하고, `WithWeb(WithDeps(...))` 옵션 chain 으로 실 인스턴스 주입).**

### 3.4 init() 자동 등록 vs 명시적 list

- **추천: init() 자동 등록** (builtin 패턴 미러). 이유:
  - builtin 6 종이 동일 패턴 사용 — 일관성.
  - 도구 추가 시 init() 한 줄만 추가하면 되어 유지비용 낮음.
- **단점**: 테스트에서 두 번째 `WithWeb()` 호출이 duplicate panic — `register_test.go` 에서 `t.Cleanup` 으로 globalWebTools 리셋 또는 `tools.NewRegistry()` 사용 시 두 번째 옵션 호출 금지로 대응 (AC-WEB-001 edge case).

### 3.5 Confirmer / Auditor / Tracker DI 패턴

```go
// internal/tools/web/common/deps.go
type Deps struct {
    PermMgr      *permission.Manager  // 실제 타입 import
    AuditWriter  AuditWriter           // interface (DualWriter 또는 FileWriter)
    RateTracker  *ratelimit.Tracker
    Clock        func() time.Time      // testable clock — AC-WEB-007 cache TTL 테스트용
    Cwd          string                // cache directory 위치 결정
}

type AuditWriter interface { Write(audit.AuditEvent) error }
```

각 web 도구 struct 는 `*Deps` 를 보유. `NewWebSearch(deps *Deps) tools.Tool` 형식.

---

## 4. M1 Detailed Task List (TDD ordered)

| Task ID | File (relative) | LOC est | Deps | AC mapped | Test file | RED-GREEN order |
|---|---|---|---|---|---|---|
| T-001 | internal/tools/web/common/response.go | 60 | — | AC-WEB-012 | common/response_test.go | RED → T-001-test, GREEN → T-001 |
| T-002 | internal/tools/web/common/response_test.go | 90 | T-001 (struct shape only) | AC-WEB-012 | self | RED first |
| T-003 | internal/tools/web/common/useragent.go | 30 | — | (REQ-WEB-003 covered indirectly via T-008/T-013 tests) | inline test in T-008 | minimal — no separate RED |
| T-004 | internal/tools/web/common/safety.go | 140 | — | AC-WEB-005, AC-WEB-006, AC-WEB-009 | common/safety_test.go | RED → T-005, GREEN → T-004 |
| T-005 | internal/tools/web/common/safety_test.go | 200 | T-004 (signatures) | AC-WEB-005/006/009 | self | RED first — 3 sub-tests |
| T-006 | internal/tools/web/common/robots.go | 110 | golang-lru/v2, temoto/robotstxt | AC-WEB-004 | common/robots_test.go | RED → T-007, GREEN → T-006 |
| T-007 | internal/tools/web/common/robots_test.go | 150 | T-006 (signatures) | AC-WEB-004 | self | RED first |
| T-008 | internal/tools/web/common/cache.go | 180 | go.etcd.io/bbolt | AC-WEB-007 | common/cache_test.go | RED → T-009, GREEN → T-008 |
| T-009 | internal/tools/web/common/cache_test.go | 180 | T-008 (signatures) | AC-WEB-007 | self | RED first — TTL 시간 조작은 injected clock |
| T-010 | internal/tools/web/common/deps.go | 50 | permission/audit/ratelimit | — (helper) | none | no test (struct only) |
| T-011 | internal/tools/web/register.go | 80 | T-010 | AC-WEB-001 | register_test.go | RED → T-012, GREEN → T-011 |
| T-012 | internal/tools/web/register_test.go | 70 | T-011 | AC-WEB-001 | self | RED first |
| T-013 | internal/tools/web/http.go | 280 | T-001~T-011 | AC-WEB-005, 006, 009, 010, 012, 018 (부분) | http_test.go | RED → T-014, GREEN → T-013 |
| T-014 | internal/tools/web/http_test.go | 320 | T-013 (signatures) | AC-WEB-005/006/009/010/012 | self | RED first — table-driven |
| T-015 | internal/tools/web/ratelimit_brave_parser.go | 90 | ratelimit.Parser interface | AC-WEB-008 (precondition) | inline (T-017 통합) | minimal RED |
| T-016 | internal/tools/web/search.go | 320 | T-001~T-011, T-015 | AC-WEB-003, 008, 012, 017, 018 (부분) | search_test.go | RED → T-017, GREEN → T-016 |
| T-017 | internal/tools/web/search_test.go | 400 | T-016 | AC-WEB-003/008/012/017/018 | self | RED first — multi-fixture |
| T-018 | internal/tools/web/permission_integration_test.go | 220 | T-013, T-016 | AC-WEB-003 (deep) | self | RED first — integration |
| T-019 | internal/tools/web/audit_integration_test.go | 180 | T-013, T-016 | AC-WEB-018 | self | RED first — 4 calls |
| T-020 | internal/tools/web/ratelimit_integration_test.go | 160 | T-016 | AC-WEB-008 | self | RED first |
| T-021 | internal/tools/web/schema_test.go | 80 | T-013, T-016 | AC-WEB-002 | self | RED-once meta-test |
| T-022 | template/.../security.yaml.tmpl 패치 | 5 lines | none | (FS-ACCESS seed) | manual smoke | adoption only |
| T-023 | audit/event.go: Add EventTypeToolWebInvoke + Sandbox | 8 lines | none | enabling AC-WEB-018 | — | minimal extension |

총 23 task, 17 + 6 (test/integration/seed) = ~17 production files + ~6 helper/integration. Plan.md §2.1 기준
17 파일과 일치.

**TDD 사이클 운영 규칙 (manager-tdd 가 따를 것)**:
1. 같은 task pair (예: T-008 cache.go ↔ T-009 cache_test.go) 에서는 항상 test 부터 작성. `go test ./internal/tools/web/common/...` 가
   compile fail 또는 test fail 로 RED 임을 확인 후 production 코드로 GREEN.
2. T-013 / T-016 같은 큰 task 는 sub-AC 단위로 RED-GREEN 분할 (예: AC-WEB-005 redirect cap 만 먼저 RED-GREEN → AC-WEB-006 size cap → AC-WEB-009 blocklist → AC-WEB-010 method allowlist).
3. 각 GREEN 단계 직후 `go vet ./...` + `golangci-lint run ./internal/tools/web/...` 실행. 0 warning 유지.
4. T-018 ~ T-020 integration 테스트는 individual unit test 통과 후 마지막에 묶음 RED → GREEN.

---

## 5. Risk Surface (M1-specific)

순서: 위험도 High → Low.

### High

1. **AC-WEB-018 audit 4-line 정확도 + 동시성 ordering** —
   - 위험: 4 도구를 순차 호출하더라도 DualWriter 의 `mu sync.Mutex` (writer.go:66) 가 단일 라인 단위 atomic 만 보장.
     timestamp 단조 증가 (acceptance.md AC-WEB-018) 는 `time.Now()` 호출 timing 에 의존.
   - 완화: 테스트에서 `time.Sleep(1ms)` 를 호출 사이에 강제 (mock time injection 권장, `Deps.Clock`).
   - 완화 2: writer.go 의 sync 가 100 writes 마다만 fsync 한다는 점 — 4 line 테스트 종료 직전 explicit `Close()` 또는
     `Sync()` 호출하여 flush 보장 (writer.go:104 `_ = w.file.Sync()`).

2. **robots.txt fetcher 의 자가 robots.txt 회피** —
   - 위험: robots.txt 자체를 fetch 할 때 다시 robots.txt 검사하면 무한 재귀.
   - 완화: `common/robots.go` 의 `Fetch(host)` 함수는 fetch path 가 `/robots.txt` 인 경우 robots check 자체를 skip.
     (PR description 에 명시 + 테스트 케이스로 self-fetch 시나리오 검증.)

3. **CheckRedirect 콜백 의미 (net/http 표준 동작)** —
   - 위험: `http.Client.CheckRedirect` 가 nil 반환 시 follow, error 반환 시 abort 하지만 마지막 응답을 무시하지 않음
     (response 는 caller 에 반환, body 는 닫혀 있을 수 있음).
   - 완화: redirect 6회 시점에 `errors.New("too_many_redirects")` 반환 → http.Get 이 `*url.Error` wrap 하여 반환.
     도구 코드는 `errors.Is(err, ErrTooManyRedirects)` 검사로 분기 (sentinel error 노출 필요).
   - 검증: T-014 테스트에서 mock chain 6 + caller 가 sentinel 잡는 것 확인.

### Medium

4. **bbolt single-file lock vs concurrent test 병렬성** —
   - 위험: bbolt 는 file lock 기반. `go test -parallel N` 시 같은 cache.db 를 여러 goroutine 이 열면 lock contention.
   - 완화: 각 test 는 `t.TempDir()` 로 격리된 cache dir 사용 (acceptance.md AC-WEB-007 이미 명시).
   - 추가: cache layer 가 `Open(dir string)` 으로 path 를 받도록 — global singleton 금지.

5. **Subject ID 결정 메커니즘 미확정 (Assumption #6)** —
   - 위험: M1 코드에서 SubjectID 를 hardcode 하면 후속 SPEC 에서 agent/skill 단위로 분리할 때 광범위 수정 필요.
   - 완화: `Deps.SubjectIDProvider func(ctx context.Context) string` 를 옵셔널 필드로 두고, default 는 `"agent:goose"` 반환.
     ctx value 로 override 가능한 helper 도 함께 (`ctx.WithSubject(ctx, "agent:planner")` 패턴).

6. **AC-WEB-007 cache TTL 테스트는 시간 조작에 의존** —
   - 위험: 실제 system clock 으로 24h 대기 불가능. acceptance.md 는 "캐시 entry 의 mtime 을 25h 전으로 강제 변경" 명시.
   - 완화: cache layer 가 `Deps.Clock func() time.Time` 사용 (injected clock). 테스트는 `Clock` 을 advance 하는 mock 사용.
   - 대안: `expires_at` 를 entry 안에 저장 (gob 또는 JSON binary). plan.md §6.1 이 이미 이 방식 명시.

7. **Brave / Tavily / Exa rate-limit header 포맷 미확인** —
   - 위험: ratelimit/parser_*.go 는 Anthropic / OpenAI / OpenRouter 만. Brave 등 제 3자 search provider 는 헤더 구조 다름.
   - 완화: T-015 에서 Brave 응답 샘플로 `parser_brave.go` 작성. M1 시작 시점 1회 실 API 호출하여 헤더 확인 후 parser 코드 작성.
   - tavily / exa 는 M1 default fallback 미사용 → M1 에서는 brave parser 만. tavily/exa parser 는 M2/3 시점 별도.

### Low

8. **blocklist 파일 부재 시 default seed loading order** —
   - 위험: 첫 실행에서 `~/.goose/security/url_blocklist.txt` 가 없으면 zero block 으로 시작.
   - 완화: `common/safety.go` 의 `LoadBlocklist(path)` 가 file not exist 시 empty list 반환. 이후 사용자가
     수동 등록 (FS-ACCESS-001 의 write_paths 가 해당 path 를 허용 — M1 task T-022 의 seed 작업).
   - 명시: 경고 로그 (zap.Info) 출력만, 실패 처리 안 함.

9. **jsonschema/v5 vs v6 schema validator collision** —
   - 위험: 없음 (v5 미사용 확인됨).
   - 명시: plan.md §7 의 v5 표기는 typo. M1 PR 에서 plan.md 도 함께 수정.

10. **integration test (build tag `integration`) 의 BRAVE_SEARCH_API_KEY missing skip** —
    - 위험: PR CI 에서 secret 미주입 시 integration 통과로 false-pass 위험.
    - 완화: integration test 는 `if os.Getenv("BRAVE_SEARCH_API_KEY") == "" { t.Skip(...) }` 로 skip 시 명확한 reason.
      별도 nightly job 에서 secret 주입하여 실행. CI matrix 에서 unit-only / integration-only 분리.

---

## 6. Integration Points (concrete API contracts)

### 6.1 Tool 인터페이스 — 정확한 시그니처
출처: `internal/tools/tool.go:13-22`.

```go
type Tool interface {
    Name() string
    Schema() json.RawMessage
    Scope() Scope
    Call(ctx context.Context, input json.RawMessage) (ToolResult, error)
}

type ToolResult struct {
    Content  []byte
    IsError  bool
    Metadata map[string]any
}
```

본 SPEC 의 도구는 다음과 같이 매핑:
- `Name()` → `"web_search"` 또는 `"http_fetch"`.
- `Schema()` → plan.md §2.3 / §2.4 의 JSON literal.
- `Scope()` → `tools.ScopeShared`.
- `Call()` → 표준 응답 wrapper JSON 을 `Content` 에 marshal. `IsError` = `!resp.OK`. `Metadata` = nil (외부에는 응답 JSON 의 metadata 필드만 노출).

### 6.2 Schema validation
- Executor 단계에서 자동 (`executor.go:94-97`). 본 SPEC 도구 `Call()` 내부에서 schema 재검증 안 함.
- AC-WEB-002 검증은 **schema_test.go** 에서 `tool.Schema()` 를 v6 validator 로 메타-스키마 검증 + `additionalProperties: false` 필드 체크.

### 6.3 Permission Manager
출처: `internal/permission/grant.go:33-48` + `manager.go:166`.

```go
req := permission.PermissionRequest{
    SubjectID:    "agent:goose",     // ctx 에서 추출 (자세히는 §5 위험 #5)
    SubjectType:  permission.SubjectAgent,
    Capability:   permission.CapNet,
    Scope:        host,               // "api.search.brave.com" 등
    RequestedAt:  now,
}
dec, err := mgr.Check(ctx, req)
if err != nil || !dec.Allow {
    return errResponse("permission_denied", err.Error())
}
```

**중요**: `Manager.Check` 는 호출 전 `Manager.Register(subjectID, manifest)` 가 선행되어야 한다 (manager.go:174-177
`ErrSubjectNotReady`). M1 bootstrap 시점 또는 도구 첫 호출 직전에 `Manager.Register("agent:goose", Manifest{NetHosts: <provider list>})`
필요. **이는 plan.md 가 묵시적으로 가정한 부분 — strategy 가 명시화한다.**

### 6.4 Audit DualWriter
출처: `internal/audit/dual.go:75` + `event.go:112`.

```go
ev := audit.NewAuditEvent(
    time.Now(),
    audit.EventTypeToolWebInvoke,    // T-023 에서 추가될 신규 상수
    audit.SeverityInfo,
    "web tool invoked",               // sanitizeMessage 처리됨
    map[string]string{
        "tool":        "web_search",
        "host":        "api.search.brave.com",
        "method":      "GET",
        "status_code": "200",
        "cache_hit":   "false",
        "duration_ms": "234",
        "outcome":     "ok",
    },
)
if err := dw.Write(ev); err != nil { /* log only, do not fail tool */ }
```

### 6.5 RateLimit Tracker
출처: `internal/llm/ratelimit/tracker.go:86`.

```go
// 호출 전 검사 (AC-WEB-008)
state := tracker.State("brave")
if state.RequestsMin.UsagePct() >= 100 {
    return errResponse("ratelimit_exhausted", "...", retryAfter: state.RequestsMin.RemainingSecondsNow(now))
}

// 호출 후 갱신
respHeaders := map[string]string{}
for k, vs := range resp.Header {
    if len(vs) > 0 { respHeaders[k] = vs[0] }
}
_ = tracker.Parse("brave", respHeaders, time.Now())
```

**전제**: tracker 는 bootstrap 시점에 `tracker.RegisterParser(BraveParser{})` 로 brave parser 가 등록되어 있어야 함.
이 등록은 web 패키지 init() 또는 main() 에서 1회 (T-015 산출물 사용).

---

## 7. Sprint Contract Seed (refined by evaluator-active in Phase 2.0)

### 7.1 Done Criteria (M1, 8 testable items)

1. AC-WEB-001: `tools.NewRegistry(WithBuiltins(), web.WithWeb()).ListNames()` 가 길이 8 (M1: built-in 6 + web M1 2 = `["http_fetch", "web_search"]`) 또는 향후 14 (M2-M4 완료 후) 반환. **M1 한정 시 8 이름만 검증**.
2. AC-WEB-002: M1 의 2개 web 도구 (`web_search`, `http_fetch`) 의 `Schema()` 가 v6 메타-스키마 valid 이며 `additionalProperties: false` 보유.
3. AC-WEB-003: `web_search` 첫 호출 시 mock Confirmer 의 `Ask` 가 정확히 1회 호출. 이후 동일 입력은 cache hit 으로 외부 fetch + Confirmer 모두 0회 (lookup hit).
4. AC-WEB-004: `http_fetch` 가 `Disallow: /private` 매칭 host 호출 시 `robots_disallow` error + 데이터 fetch 0회.
5. AC-WEB-005: `http_fetch` redirect chain 6단계 시 `too_many_redirects` error. `max_redirects=11` 입력은 schema_validation_failed.
6. AC-WEB-006: `http_fetch` 가 12MB body host 호출 시 `response_too_large` error. 10MB+1 byte 시점 abort.
7. AC-WEB-007: cache hit 응답이 외부 fetch 0회. clock advance 25h 후 동일 input 재호출 시 fetch 1회.
8. AC-WEB-008: Tracker 의 brave `RequestsMin` UsagePct 100% 시 `web_search` 호출이 `ratelimit_exhausted` + retry_after_seconds 필드 보유.
9. AC-WEB-009: blocklist 매칭 host 는 PERMISSION 단계 진입 전 차단 (mock Confirmer.Ask 호출 0회 검증).
10. AC-WEB-010: `http_fetch` `method=POST` 는 schema_validation_failed (Executor 단계).
11. AC-WEB-012: `web_search` + `http_fetch` 의 성공/실패 응답 4종이 모두 `{ok, data|error, metadata}` JSON unmarshal 성공.
12. AC-WEB-017: `web_search` provider 미지정 + config 부재 시 brave endpoint 호출. `provider="brave"` 명시 호출 동일.
13. AC-WEB-018: M1 도구 2종 + 첫 호출 동의 → 2 호출, audit.log 정확히 2 line, 모든 키 존재 + outcome="ok".

(M1 범위 외 AC: AC-WEB-011/013/014/015/016 는 M2~M4 에서 GREEN.)

### 7.2 Edge Cases (반드시 cover)

- blocklist glob 매칭: `*.evil.com` 패턴이 `sub.evil.com` 차단.
- robots.txt cache miss 후 hit: 두 번째 호출은 fetch 0회 (LRU 캐시).
- ratelimit 응답 `429 Too Many Requests`: tracker 가 `Retry-After` header 반영 + Parse 호출 후 다음 호출이 exhausted.
- redirect chain 정확히 5회: success (boundary).
- `http_fetch` `max_redirects=0`: 첫 redirect 즉시 too_many_redirects.
- cache TTL 정확히 24h 시점: hit (boundary, expires_at > now).
- 빈 query string + valid `web_search` 입력: schema validation 실패 (minLength: 1).
- robots.txt 자체 fetch 가 robots check 재귀하지 않음.

### 7.3 Hard Thresholds

- Coverage ≥ 85% on `internal/tools/web/...` (per file: response 90%+, safety 90%+, robots 85%+, cache 85%+, http 85%+, search 85%+).
- LSP errors 0 on web package (Phase 2.7 drift guard 와 정렬).
- `golangci-lint run ./internal/tools/web/...` zero warnings.
- `go vet ./internal/tools/web/...` zero issues.
- `go test -race ./internal/tools/web/...` GREEN (multi-goroutine cache + tracker 검증).
- godoc 영문 (CLAUDE.local.md §2.5).

---

## 8. Effort Priority

- **Priority High (M1 foundational)** — without these, all subsequent code blocked:
  - T-001/002 response.go (response shape — every tool depends).
  - T-004/005 safety.go (blocklist + redirect cap + size cap — http_fetch 의존).
  - T-006/007 robots.go (http_fetch / web_search 모두 의존).
  - T-008/009 cache.go (TTL behavior — AC-WEB-007 필수).
  - T-010/011 deps.go + register.go (DI + 등록 패턴).
  - T-013/014 http.go + http_test.go (AC-WEB-005/006/009/010/012 cover).
- **Priority Medium (M1 feature)** —
  - T-015 brave parser (ratelimit 통합 prerequisite).
  - T-016/017 search.go + search_test.go (Brave default + provider 추상화).
  - T-018/019/020 integration tests (PERMISSION/AUDIT/RATELIMIT 통합 검증).
  - T-021 schema_test.go (AC-WEB-002 meta-test).
- **Priority Low (post-PR polish)** —
  - golden response fixtures (`testdata/golden/web_search_*.json`, `testdata/golden/http_fetch_*.json`).
  - e2e bootstrap (PERMISSION + RATELIMIT + AUDIT + FS-ACCESS 4 시스템 통합 시나리오; PR 단위가 아니라 별도 commit).
  - T-022 fsaccess seed 갱신 (M1 마지막 PR 또는 별도 chore PR).
  - T-023 audit/event.go EventType 추가 (PR 1번에 포함, 8 LOC).

---

## 9. Open Questions (require user clarification before code begins)

1. **Subject ID 할당 메커니즘**: M1 에서는 단일 `"agent:goose"` 로 시작해도 OK 인가, 아니면 호출자 (skill / agent) 마다 분리해야 하는가?
   - 영향: `permission.Manager.Register` 호출 위치 / DI 인터페이스 / ctx propagation 방식.
   - 임시 선택 (사용자 미응답 시): `"agent:goose"` 단일 + `Deps.SubjectIDProvider` 옵셔널 helper.

2. **Permission Manager 인스턴스 lifecycle**: M1 의 `Deps.PermMgr` 는 어디서 생성되어 주입되는가?
   - bootstrap (cmd/goose/main.go 또는 internal/runtime/...) 시점에 1회 생성하여 web 패키지 init() 에 주입?
   - 아니면 web 패키지 자체 lazy init? **bootstrap 주입이 testability 측면에서 권장**.
   - 현재 코드베이스 grep 결과 `permission.Manager` 인스턴스 생성 위치 미확인 — manager-spec 또는 expert-backend 가
     bootstrap 진입점 read 후 결정 필요.

3. **Brave parser registration 위치**: `tracker.RegisterParser(BraveParser{})` 는 web 패키지 init() 에서? bootstrap 에서?
   - 권장: web 패키지 init() (자기 contained). 하지만 tracker 인스턴스가 web 보다 먼저 존재해야 하므로
     `Deps.RateTracker` 주입 후 RegisterParser 호출 — 즉 web 패키지 register.go 의 `WithWeb(WithDeps(deps))` 옵션 시점.

4. **`web_search` config (`~/.goose/config/web.yaml`) 로드 시점**: AC-WEB-017 의 default_search_provider 처리.
   - M1 에서 config loader 미구현 시 hardcode `"brave"` 로 시작 가능한가? 또는 M1 PR 에 web.yaml 로더도 포함?
   - 권장: M1 에 minimal `LoadWebConfig()` 포함 (yaml + path resolution). 80~100 LOC 추가.

5. **bbolt vs SQLite 최종 선택**: plan.md §6.1 은 bbolt default. SQLite 의 CGO 부담을 회피 — **bbolt 채택**으로 본 strategy 가 진행.
   - 사용자 override 의사 있으면 명시.

→ 사용자가 1~5 에 응답하지 않으면 manager-tdd 는 위 임시 선택을 기본값으로 RED-GREEN 진입.

---

## 10. Cognitive Bias Check (final)

- **Anchoring**: plan.md 의 task breakdown 에 anchor 되었는가? — 일부 (T-1.1 ~ T-1.10 매핑 유지). 그러나 23개로 세분화하여
  TDD pair 단위 명확화. plan.md 의 task ID (T1.1 등) 와 본 strategy 의 ID (T-001 등) 분리하여 ambiguity 차단.
- **Confirmation bias**: bbolt / Brave default 등은 plan.md 결론을 그대로 채택 — research.md §6.2 / §1.2 에 의해 trade-off 가
  명시되어 있고 본 strategy 가 별도 alternatives 도출 책임 없음 (M1 scope 내 bias 영향 적음).
- **Sunk cost**: 없음 (신규 패키지).
- **Overconfidence**: §9 Open Questions 의 5개 항목은 코드 작성 직전 손볼 가치 있음. assumption #4/#6 도 동일.
  잘못 추정 시 광범위 수정 필요한 영역으로 식별됨. → 우려 표면화 완료.

**Why preferred option might fail**:
- `Manager.Check` 호출 chain 이 예상보다 복잡 (Register 누락 → ErrSubjectNotReady 광범위 발생)
- bbolt 의 single-file lock 이 `go test -race` 와 상호작용하여 flaky test 유발
- AC-WEB-018 의 audit timestamp 단조 증가가 system clock 정밀도 (ns 단위) 부족으로 깨질 수 있음 — `time.Sleep(1 * time.Microsecond)` 강제

---

Version: 0.1.0
Last Updated: 2026-05-06
Author: manager-strategy (Opus 4.7 / ultrathink / thorough harness)
Scope: M1 (Search & HTTP + common infra) ONLY. M2~M4 deferred.
