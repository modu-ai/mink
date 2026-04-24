# Plan — SPEC-GOOSE-ROUTER-001 구현 (M1 Phase 1)

## Context

**왜 이 변경이 필요한가**: GOOSE M1 Multi-LLM 마일스톤의 `Critical Path: CREDPOOL → ROUTER → ADAPTER`에서 CREDPOOL-001 MVP는 이미 완료(`d2ee56f` → PR #1 squash `12588b6`에 흡수). 다음 의존 선행 작업이 ROUTER이며, ROUTER가 끝나야 ADAPTER-001(M1 마지막)·PROMPT-CACHE-001·RATELIMIT-001이 "Router가 결정한 provider/model"이라는 전제를 가진다.

**범위 경계**: 본 작업은 **순수 라우팅 결정 로직만** 구현한다. 실제 LLM HTTP 호출은 ADAPTER-001, credential 취득은 CREDPOOL-001, rate limit 고려는 RATELIMIT-001로 명시 분리되어 있어 ROUTER는 네트워크/자격 접근 없는 stateless 결정 엔진이다. Hermes Agent Python `model_router.py`의 `choose_cheap_model_route` 알고리즘을 Go 포트한다.

**산출 목표**: `/moai run SPEC-GOOSE-ROUTER-001` 완료 시 `internal/llm/router/` 패키지에서 8개 Acceptance Criteria 모두 통과, 15+ provider metadata 사전 등록, 테스트 커버리지 85%+ 달성. 이후 ADAPTER-001 착수 가능.

---

## Pre-execution: Git State 복구

로컬 main이 origin/main과 divergence 상태(local 12 / origin 1). origin/main squash commit `12588b6`은 로컬 12 커밋 내용을 모두 흡수했음을 `git diff HEAD origin/main --stat` 및 squash commit body로 확인 완료.

```bash
# 1. 백업 태그 (복구 가능성 보장)
git tag backup/pre-reset-20260424 HEAD

# 2. origin/main과 동기화 (파괴적 연산, 백업 태그로 복구 가능)
git reset --hard origin/main

# 3. 잔존 작업 브랜치 정리
git branch -D sync/agency-absorb-20260424

# 4. ROUTER 전용 feature branch 분기
git checkout -b feature/SPEC-GOOSE-ROUTER-001
```

---

## Implementation Approach: TDD (brownfield enhancement)

`.moai/config/sections/quality.yaml` development_mode가 TDD(default). SPEC §6.6에 이미 RED → GREEN → REFACTOR 순서가 명시되어 있으므로 그대로 따른다. 기존 `internal/llm/credential/` 패턴을 일관되게 재사용한다.

### 준수할 기존 패턴 (재사용, 새로 만들지 말 것)

| 패턴 | 기존 위치 | ROUTER 적용 |
|-----|--------|-----------|
| Option pattern (Functional Options) | `internal/llm/credential/pool.go:20` | `RouterOption func(*Router)` |
| Stateless 설계 주석 | `internal/llm/credential/pool.go:22-24` | Router struct 상단 동일 주석 |
| Error variable 패턴 (`var ErrXxx = errors.New(...)`) | `internal/llm/credential/pool.go:10-17` | `errors.go`에 `ErrProviderNotRegistered`, `ErrCheapRouteUndefined` |
| Context-first 시그니처 | `internal/llm/credential/pool.go:144` `Select(ctx)` | `Route(ctx, req)` |
| Table-driven 테스트 + 수동 assertion (testify 미사용) | `internal/llm/credential/pool_test.go:19+` | 모든 `_test.go`에 적용 |
| `t.Parallel()` 명시 | `internal/llm/credential/*_test.go` | Router는 stateless라 전 테스트 parallel OK |
| zap logger 주입 | `internal/llm/credential/pool.go:42` | `New(cfg, registry, logger *zap.Logger)` |
| 에러 wrapping `fmt.Errorf("%s: %w", ctx, err)` | `internal/config/bootstrap_config.go:49,65` | 동일 |

---

## Critical Files — 생성 (모두 신규)

### Production code (6 파일)

1. **`internal/llm/router/router.go`**
   - `type Router struct { cfg, registry, cls, logger }` — stateless
   - `type RoutingRequest`, `type Route`, `type RouteDefinition`
   - `func New(cfg RoutingConfig, registry *ProviderRegistry, logger *zap.Logger) (*Router, error)` — primary provider registry 등록 여부 검증 (REQ-ROUTER-011)
   - `func (r *Router) Route(ctx context.Context, req RoutingRequest) (*Route, error)` — §6.5 의사코드 그대로

2. **`internal/llm/router/classifier.go`**
   - `type Classifier interface { Classify(msg string) ClassifierResult }`
   - `type ClassifierResult struct { IsSimple bool; Reasons []string }`
   - `type SimpleClassifier struct { MaxChars, MaxWords, MaxNewlines int; ComplexKeywords map[string]struct{} }`
   - 6 판정 기준: char_count, word_count (unicode space split, CJK 구두점 포함), newline_count, has_code_block (` ``` ` or `~~~` 또는 연속 2줄 이상 leading whitespace, REQ-ROUTER-013), has_url (`https?://\S+` regex, pre-compiled), has_complex_keyword (word-boundary + case-insensitive). 6개 **모두** true일 때만 IsSimple=true (conservative).

3. **`internal/llm/router/registry.go`**
   - `type ProviderMeta struct { Name, DisplayName, DefaultBaseURL, AuthType string; SupportsStream, SupportsTools, SupportsVision, SupportsEmbed, AdapterReady bool; SuggestedModels []string }`
   - `type ProviderRegistry struct { providers map[string]*ProviderMeta }`
   - `func NewRegistry() *ProviderRegistry`, `Register(meta) error`, `Get(name) (*ProviderMeta, bool)`, `List() []*ProviderMeta`
   - `func DefaultRegistry() *ProviderRegistry` — 15+ provider 사전 등록 (REQ-ROUTER-003). Phase 1 AdapterReady=true: **Anthropic, OpenAI, Google Gemini, xAI, DeepSeek, Ollama**. Metadata-only AdapterReady=false: **OpenRouter, Nous, Mistral, Groq, Qwen, Kimi, GLM, MiniMax** (총 14개, 요구는 15+이므로 1개 더 추가 검토 — Cohere 권장).

4. **`internal/llm/router/config.go`**
   - `type ForceMode string` + const `ForceModeAuto/Primary/Cheap`
   - `type RouteDefinition struct { Model, Provider, BaseURL, Mode, Command string; Args map[string]any }`
   - `type RoutingConfig struct { Primary; CheapRoute *RouteDefinition; ForceMode; MaxChars/Words/Newlines int; ComplexKeywords []string; CustomClassifier Classifier; RoutingDecisionHooks []RoutingDecisionHook }`
   - `type RoutingDecisionHook func(req RoutingRequest, route *Route)` (observational only, REQ-ROUTER-015)

5. **`internal/llm/router/signature.go`**
   - `func makeSignature(r *Route) string` — canonical `"model|provider|base_url|mode|command|args_hash"`
   - `argsHash` = `sha256(canonical_json(r.Args))[:12]` — `encoding/json` + 키 정렬
   - REQ-ROUTER-014: 시간·credential·user token 미포함 검증

6. **`internal/llm/router/errors.go`**
   - `var ErrCheapRouteUndefined = errors.New("router: cheap route undefined")`
   - `type ProviderNotRegisteredError struct { Name string }` + `func (e *ProviderNotRegisteredError) Error() string`

### Test code (3 파일)

7. **`internal/llm/router/classifier_test.go`**
   - RED #1~#5 (SPEC §6.6): Table-driven 6 기준 × boundary (경계값: 160/161 chars, 28/29 words, 2/3 newlines, 코드블록 유/무, URL 유/무, 키워드 word-boundary 케이스)
   - 한국어·CJK 테스트 케이스 포함 (R6 완화)
   - 총 48개 case 이상 목표 (6 기준 × 경계 상/하 + 복합 케이스)

8. **`internal/llm/router/router_test.go`**
   - RED #6: `TestRouter_CheapRouteNil_FallsBackToPrimary` (AC-ROUTER-006)
   - RED #7: `TestRouter_Signature_Reproducible` (AC-ROUTER-007)
   - RED #8: `TestRouter_UnregisteredProvider_ReturnsError` (AC-ROUTER-008)
   - `TestRouter_ForceMode_Primary`, `TestRouter_ForceMode_Cheap_NoCheapDefined_ReturnsError` (REQ-ROUTER-009)
   - `TestRouter_SimpleGreeting_CheapRoute`, `TestRouter_ComplexKeyword_PrimaryRoute` (AC-ROUTER-001/002 통합)
   - `TestRouter_DecisionHook_Called` (REQ-ROUTER-015)
   - `TestRouter_Stateless_Concurrent` (REQ-ROUTER-001 — goroutine 병렬 호출 동일 결과 검증)
   - `TestRouter_InputImmutable` (REQ-ROUTER-004 — 입력 RoutingRequest 미변경 확인)

9. **`internal/llm/router/registry_test.go`**
   - `TestRegistry_DefaultRegistry_HasFifteenProviders` (REQ-ROUTER-003)
   - `TestRegistry_Register_Duplicate_ReturnsError`
   - `TestRegistry_Get_Unregistered_ReturnsFalse`
   - Phase 1 adapter-ready 6종 및 metadata-only 나머지 분리 검증

---

## Wire-up (선택적, 본 SPEC 범위 외로 명시됨)

`cmd/goosed/main.go` 또는 `internal/core/runtime.go`에서 Router 초기화는 **본 SPEC 외부**. SPEC §3.2 OUT: "실제 호출은 ADAPTER-001". 따라서 `cmd/goosed/`는 이번 PR에서 건드리지 않는다. ADAPTER-001에서 `Runtime.Router *router.Router` 필드 추가 여부 결정.

**이 제약을 지킴으로써**: ROUTER는 순수 라이브러리로 격리 검증 가능, ADAPTER 착수 시 의존성 역류 없음.

---

## Reused Utilities (참조)

- `go.uber.org/zap` — 이미 `go.mod`에 있음 (pool.go에서 사용)
- `context`, `errors`, `fmt`, `regexp`, `strings`, `unicode`, `crypto/sha256`, `encoding/json` — 표준 라이브러리만
- `github.com/stretchr/testify` — SPEC §7 Dependencies에는 언급되나 기존 `internal/llm/credential/*_test.go`는 **testify 미사용**. 일관성 원칙으로 **testify도 도입하지 않음**. 표준 `testing.T` + 수동 assertion 유지.

---

## Verification — 종단 검증 절차

```bash
# 1. 패키지 단독 테스트 (race + coverage)
go test -race -coverprofile=coverage.out -covermode=atomic ./internal/llm/router/...
go tool cover -func=coverage.out | tail -5
# 기대: 총 coverage 85%+

# 2. 전체 repo 회귀
go test -race ./...
# 기대: 기존 CREDPOOL·CORE 테스트 영향 없음

# 3. Lint + format
go vet ./internal/llm/router/...
gofmt -l internal/llm/router/
golangci-lint run ./internal/llm/router/...
# 기대: 무출력

# 4. AC별 구체 검증
go test -run TestClassifier -v ./internal/llm/router/...  # AC-001~005
go test -run TestRouter_CheapRouteNil -v ./internal/llm/router/...  # AC-006
go test -run TestRouter_Signature -v ./internal/llm/router/...  # AC-007
go test -run TestRouter_UnregisteredProvider -v ./internal/llm/router/...  # AC-008

# 5. Concurrency 검증 (REQ-ROUTER-001)
go test -race -run TestRouter_Stateless_Concurrent -count=100 ./internal/llm/router/...
```

**PR 생성 조건**: 위 4단계 모두 pass + CREDPOOL 호환성 smoke test (`go build ./...` 무에러) + 커버리지 85% 이상.

---

## Out of Scope (본 작업에서 하지 않을 것)

- ADAPTER-001 구현 (실제 HTTP 호출)
- RATELIMIT-001 상태 참조하는 동적 라우팅
- PROMPT-CACHE-001 cache marker 로직
- `cmd/goosed/main.go` 또는 `runtime.go` wire-up (ADAPTER 단계에서)
- 학습 기반 라우팅 (Phase 4 INSIGHTS-001)
- Tool schema 변환 (ADAPTER-001 provider별)
- Multi-round classification (직전 turn 참조) — SPEC §10 Exclusions

---

## 완료 후 다음 단계

1. `go test -race ./...` + lint pass 확인
2. 커밋 메시지: `feat(router): SPEC-GOOSE-ROUTER-001 — Smart Model Routing + Provider Registry`
3. `gh pr create --base main --head feature/SPEC-GOOSE-ROUTER-001` — 본문에 AC 8건 체크리스트 + provider 15+ 등록 확인
4. PR 머지 후 ADAPTER-001(M1 마지막 critical path) 착수
