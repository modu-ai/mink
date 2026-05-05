# SPEC-GOOSE-LLM-ROUTING-V2-001 — Acceptance Criteria (상세)

> **목적**: spec.md §10 의 11 AC 를 fuller Given/When/Then 형식으로 재구성. 구체 입력/출력, mock provider 동작, fallback chain trace 형식, coverage gate, DoD 명시.

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-05-05 | 초안 — 11 AC 상세화 + RoutingReason 형식 검증 + coverage gate + DoD | manager-spec |

---

## 0. 일반 규약

### 0.1 Test fixture 위치

- `internal/llm/router/v2/testdata/policy_*.yaml` — 5 fixture (prefer_local, prefer_cheap, prefer_quality, always_specific, with_excluded)
- `internal/llm/router/v2/testdata/capability_matrix_expected.json` — spec.md §6.1 표의 JSON 직렬화 (regression 보호)

### 0.2 Mock 추상화

- **MockRouter** (v1): 단순히 `RoutingRequest` 의 첫 메시지 길이로 primary/cheap 결정 (table-driven 입력)
- **MockRateLimitView**: `BucketUsage(provider) (rpm, tpm, rph, tph float64)` 를 hardcoded 반환
- **MockErrorClassifier**: 주어진 error 를 14 `FailoverReason` 중 지정 enum 으로 분류
- **MockProvider** (httptest): integration_test 에서 status code + 응답 body 제어

### 0.3 Coverage gate

- P1 (policy.go, loader.go): ≥ 90%
- P2 (capability.go, ratelimit_filter.go): ≥ 90%
- P3 (fallback.go, router.go, pricing.go, trace.go): ≥ 90%
- P4 (integration_test 추가): 패키지 전체 ≥ 92%

### 0.4 DoD 공통 항목

각 AC 충족 시점에서:
- `go test -race ./internal/llm/router/v2/...` pass
- `gofmt -l ./internal/llm/router/v2/...` 빈 출력
- `golangci-lint run ./internal/llm/router/v2/...` 0 issue
- progress.md 에 AC 진척 기록

---

## AC 상세

### AC-RV2-001 — 정책 파일 부재 시 v1 byte-identical (REQ-RV2-001, -002)

**Phase**: P1

**Given**:
- `~/.goose/routing-policy.yaml` 가 존재하지 않음
- v1 MockRouter 가 message="hello" 입력에 `Route{Provider: "anthropic", Model: "claude-sonnet-4.6", RoutingReason: "v1:simple"}` 반환하도록 설정

**When**:
- `policy, err := v2.LoadPolicy("/nonexistent/path")` 호출
- `r := v2.New(mockV1, policy, matrix, mockRLView)` 생성
- `route, _ := r.Route(ctx, RoutingRequest{Messages: []Msg{{Content: "hello"}}})` 호출

**Then**:
- `err == nil` (silent default)
- `policy.Mode == PreferQuality`
- `policy.FallbackChain == nil`
- `policy.RequiredCapabilities == nil`
- `policy.ExcludedProviders == nil`
- `route.Provider == "anthropic"` (v1 결정 그대로)
- `route.Model == "claude-sonnet-4.6"`
- `route.RoutingReason == "v1:simple"` (v2 prefix 미추가, BC 보장)

---

### AC-RV2-002 — AlwaysSpecific 모드가 v1 결정 override (REQ-RV2-001, -008)

**Phase**: P1, P3

**Given**:
- `routing-policy.yaml`:
  ```yaml
  mode: always_specific
  fallback_chain:
    - provider: groq
      model: llama-3.3-70b
    - provider: openai
      model: gpt-4o
  ```
- v1 MockRouter 가 `Route{Provider: "anthropic", Model: "claude-sonnet-4.6"}` 반환
- MockRLView 모든 bucket 0.0 (rate limit 여유)

**When**:
- `r.Route(ctx, req)` 호출

**Then**:
- `route.Provider == "groq"` (chain[0] 강제, v1 결정 무시)
- `route.Model == "llama-3.3-70b"`
- `route.RoutingReason` 가 `"v2:policy_always_specific_groq"` 로 시작
- v1 결정 (anthropic) 은 어디에도 사용되지 않음

---

### AC-RV2-003 — PreferCheap 정책의 비용 오름차순 (REQ-RV2-007)

**Phase**: P3

**Given**:
- `routing-policy.yaml`:
  ```yaml
  mode: prefer_cheap
  ```
- v1 MockRouter 가 `Route{Provider: "anthropic", Model: "claude-opus-4-7"}` 반환
- 모든 provider capability 동일 (FunctionCalling) 가정
- MockRLView 모든 bucket 0.0
- ProviderRegistry 의 AdapterReady=true 인 15 provider 중 input 가격 오름차순 상위 3 = ollama (0.00) → groq:llama-3.3-70b (0.00 free) → mistral:nemo (0.02)

**When**:
- `r.Route(ctx, req)` 호출

**Then**:
- `route.Provider == "ollama"` (가격 0.00 최저, 정렬 안정성으로 ollama 우선)
- `route.RoutingReason == "v2:policy_prefer_cheap_ollama"` 형식
- 만약 ollama 가 capability/ratelimit 필터로 제거되면 다음 후보 `groq` 선택

---

### AC-RV2-004 — required_capabilities=[vision] 필터 (REQ-RV2-010)

**Phase**: P2

**Given**:
- `routing-policy.yaml`:
  ```yaml
  mode: prefer_quality
  required_capabilities:
    - vision
  ```
- spec.md §6.1 매트릭스: vision=true 인 provider = anthropic, openai, google, xai, ollama (llava), openrouter (model dependent), together (some), fireworks (some), qwen (qwen3-vl)
- vision=false 인 provider = deepseek, groq, cerebras, mistral, kimi, zai_glm

**When**:
- `r.Route(ctx, req)` 호출

**Then**:
- `route.Provider` 는 vision=true 인 provider 중 하나
- vision=false 인 provider (deepseek/groq/cerebras/mistral/kimi/zai_glm) 가 후보에서 제거된 사실이 progress.md trace 에 기록
- `route.RoutingReason` 가 `"v2:capability_vision_required_9_candidates"` 형식 (9 = vision 지원 provider 수)

---

### AC-RV2-005 — Rate limit 80% 임계 회피 (REQ-RV2-009)

**Phase**: P2

**Given**:
- `routing-policy.yaml`:
  ```yaml
  mode: prefer_quality
  rate_limit_threshold: 0.80
  ```
- MockRLView:
  - `anthropic`: rpm=0.85, tpm=0.10, rph=0.50, tph=0.30 (RPM 임계 초과)
  - `openai`: rpm=0.20, tpm=0.10, rph=0.30, tph=0.20 (모두 여유)
- v1 MockRouter 가 anthropic 반환

**When**:
- `r.Route(ctx, req)` 호출

**Then**:
- `route.Provider != "anthropic"` (RPM 0.85 ≥ 0.80 임계로 제외)
- `route.Provider == "openai"` (다음 후보)
- `route.RoutingReason` 가 `"v2:rate_limit_avoid_anthropic_rpm_0.85"` 형식 포함

---

### AC-RV2-006 — FailoverReason.RateLimit 시 chain 다음 후보 (REQ-RV2-005)

**Phase**: P3

**Given**:
- `routing-policy.yaml`:
  ```yaml
  mode: always_specific
  fallback_chain:
    - provider: anthropic
      model: claude-sonnet-4.6
    - provider: openai
      model: gpt-4o
    - provider: google
      model: gemini-2.0-flash
  ```
- MockProvider for `anthropic`: 첫 호출 시 429 반환 → `MockErrorClassifier` 가 `FailoverReason.RateLimit` 분류
- MockProvider for `openai`: 200 OK 반환

**When**:
- `r.Route(ctx, req)` 호출 후 caller 가 anthropic 호출 → 429 → `FallbackExecutor.Execute(chain, fn)` 진행

**Then**:
- 1차 anthropic 호출 실패 (429)
- 2차 openai 호출 성공 (200)
- 최종 사용 provider = `openai`
- progress.md trace:
  ```
  v2:fallback_chain_step_1_rate_limit (anthropic, claude-sonnet-4.6, 120ms)
  v2:fallback_chain_step_2_success (openai, gpt-4o, 230ms)
  ```

---

### AC-RV2-007 — FailoverReason.ContentFilter 시 chain 즉시 중단 (REQ-RV2-013)

**Phase**: P3

**Given**:
- `routing-policy.yaml`:
  ```yaml
  mode: always_specific
  fallback_chain:
    - provider: anthropic
      model: claude-sonnet-4.6
    - provider: openai
      model: gpt-4o
  ```
- MockProvider for `anthropic`: ContentFilter 에러 반환 → `MockErrorClassifier` 가 `FailoverReason.ContentFilter` 분류
- MockProvider for `openai`: 호출되어선 안 됨

**When**:
- `FallbackExecutor.Execute(chain, fn)` 진행

**Then**:
- 1차 anthropic 실패
- **openai 호출 시도 0 회** (chain 즉시 중단)
- `Execute()` 가 ContentFilter 에러 그대로 반환
- progress.md trace:
  ```
  v2:fallback_chain_step_1_content_filter (anthropic, claude-sonnet-4.6, 95ms)
  v2:fallback_chain_aborted_content_filter
  ```
- `MockProvider for openai` 의 호출 카운터 == 0

---

### AC-RV2-008 — excluded_providers + chain 충돌 silent skip (REQ-RV2-012)

**Phase**: P3

**Given**:
- `routing-policy.yaml`:
  ```yaml
  mode: always_specific
  excluded_providers:
    - anthropic
  fallback_chain:
    - provider: anthropic
      model: claude-sonnet-4.6
    - provider: openai
      model: gpt-4o
  ```
- MockProvider for `anthropic`: 호출되어선 안 됨
- MockProvider for `openai`: 200 OK

**When**:
- `r.Route(ctx, req)` + `FallbackExecutor.Execute()` 진행

**Then**:
- LoadPolicy 가 명시적 에러 반환하지 **않음** (init 시점 충돌 검증 안 함)
- chain 실행 시 anthropic 항목은 silent skip
- 최종 사용 provider = `openai`
- progress.md trace:
  ```
  v2:fallback_chain_step_1_silent_skip_excluded (anthropic, claude-sonnet-4.6, 0ms)
  v2:fallback_chain_step_2_success (openai, gpt-4o, 220ms)
  ```
- `MockProvider for anthropic` 의 호출 카운터 == 0

---

### AC-RV2-009 — 모든 후보 0 개 시 v1 silent recovery (REQ-RV2-014)

**Phase**: P3

**Given**:
- `routing-policy.yaml`:
  ```yaml
  mode: prefer_quality
  required_capabilities:
    - prompt_caching
    - realtime
  ```
- spec.md §6.1 매트릭스: prompt_caching=true 인 provider = anthropic 단 1 개. realtime=true 인 provider = openai 단 1 개. 둘 다 충족하는 provider = **0 개**.
- v1 MockRouter 가 `Route{Provider: "anthropic", Model: "claude-sonnet-4.6", RoutingReason: "v1:simple"}` 반환

**When**:
- `r.Route(ctx, req)` 호출

**Then**:
- v2 의 capability filter 결과 후보 0 개
- v2 가 v1 결정으로 silent recovery
- `route.Provider == "anthropic"`, `route.Model == "claude-sonnet-4.6"`
- `route.RoutingReason == "v2:fallback_exhausted_recover_v1"` (사용자 작업 불가능 회피, 명시적 trace)
- progress.md 에 `"v2:fallback_exhausted_recover_v1"` 기록

---

### AC-RV2-010 — fallback chain trace append-only (REQ-RV2-011)

**Phase**: P3

**Given**:
- `routing-policy.yaml`:
  ```yaml
  mode: always_specific
  fallback_chain:
    - provider: anthropic
      model: claude-sonnet-4.6
    - provider: openai
      model: gpt-4o
    - provider: google
      model: gemini-2.0-flash
  ```
- 1차 anthropic → 429 (RateLimit), 2차 openai → 503 (Server5xx), 3차 google → 200 OK

**When**:
- `FallbackExecutor.Execute()` 완료

**Then**:
- progress.md 에 정확히 3 lines append:
  ```
  v2:fallback_chain_step_1_rate_limit (anthropic, claude-sonnet-4.6, 120ms)
  v2:fallback_chain_step_2_server_5xx (openai, gpt-4o, 145ms)
  v2:fallback_chain_step_3_success (google, gemini-2.0-flash, 180ms)
  ```
- 각 line 형식: `v2:fallback_chain_step_<n>_<reason> (<provider>, <model>, <duration>)`
- duration 은 milliseconds 정수, ±5ms 허용
- progress.md 의 다른 section 은 변경 없음 (append-only)

---

### AC-RV2-011 — OpenRouter chain 명시 시 단순 provider 취급 (Section 2.2)

**Phase**: P4 (integration)

**Given**:
- `routing-policy.yaml`:
  ```yaml
  mode: prefer_cheap
  fallback_chain:
    - provider: openrouter
      model: gpt-oss-120b
    - provider: groq
      model: llama-3.3-70b
  ```
- v1 MockRouter 가 임의 결정 반환 (`anthropic`)
- MockProvider for `openrouter`: 200 OK with `{"content": "..."}` 반환

**When**:
- `r.Route(ctx, req)` 호출

**Then**:
- `mode: prefer_cheap` 임에도 OpenRouter 가 가격 정렬 우대 받지 **않음** (chain[0] 으로 명시적 위치만 인정)
- v2 의 PreferCheap pricing 정렬 (Section 6.2 표) 에 OpenRouter 는 등장하지 않음 — pricing.go 가 OpenRouter 항목을 의도적 제외
- chain[0] = openrouter 가 명시되었으므로 `route.Provider == "openrouter"` 선택
- `route.RoutingReason == "v2:policy_prefer_cheap_chain_step_1_openrouter"` 형식 — `prefer_cheap_<provider>` 가 아닌 chain step 표기 (가격 정렬 제외 명시)
- 회귀 보호 test: `pricing.go` 의 정적 표에 `openrouter:*` key 부재 검증

---

## DoD (Definition of Done) — 본 SPEC 전체

전체 SPEC 완료 시점에서 다음 모두 충족:

### Functional

- [ ] 11 AC (AC-RV2-001 ~ AC-RV2-011) 모두 GREEN
- [ ] 14 EARS REQ (REQ-RV2-001 ~ REQ-RV2-014) 가 acceptance 와 1:N 매핑 완료
- [ ] 5 fixture (`testdata/policy_*.yaml`) 모두 통과
- [ ] OpenRouter 의도적 제외 정책이 spec.md §2.2 + §3.2 + §14 + AC-RV2-011 일관 검증

### Quality

- [ ] Coverage ≥ 90% per file (P1, P2, P3), ≥ 92% 통합 (P4)
- [ ] `go test -race ./internal/llm/router/v2/...` pass
- [ ] `gofmt -l ./internal/llm/router/v2/...` 빈 출력
- [ ] `golangci-lint run ./internal/llm/router/v2/...` 0 issue
- [ ] `BenchmarkRouterV2_Route` < 5ms p99 (Non-Functional spec.md §11)

### Documentation

- [ ] spec.md HISTORY 에 v0.1.0 entry
- [ ] plan.md HISTORY 에 v0.1.0 entry
- [ ] acceptance.md HISTORY 에 v0.1.0 entry
- [ ] research.md (간단) 작성
- [ ] spec-compact.md (~30% token 절감) 작성
- [ ] progress.md 에 4 milestone 진척 기록

### Plan Audit

- [ ] plan-auditor 1라운드 PASS → status: draft → audit-ready 자동 전환
- [ ] CONDITIONAL_GO/FAIL 시 사용자 보고 후 종료 (자동 전환 안 함)

### Trackability

- [ ] 모든 commit message: conventional type + 한국어 본문 + `SPEC: SPEC-GOOSE-LLM-ROUTING-V2-001` trailer
- [ ] PR title: 영문 conventional + 한국어 설명 (CLAUDE.local.md §2.3)
- [ ] PR labels: type/feature + priority/p1-high + area/router (또는 area/llm-provider)
- [ ] Branch: `feature/SPEC-GOOSE-LLM-ROUTING-V2-001`

### Security

- [ ] `~/.goose/routing-policy.yaml` 권한 0600 강제 (loader.go)
- [ ] fallback trace 에 사용자 message 본문 미기록 (provider/model/reason/duration 만)
- [ ] model name 임의 주입 방어 (ProviderRegistry.AdapterReady 검증)

### Backward Compatibility

- [ ] v1 Router 사용자 코드 변경 0
- [ ] v1 테스트 100% 통과
- [ ] 정책 파일 부재 시 byte-identical (AC-RV2-001)

---

## Acceptance 진척 기록 형식 (progress.md)

```markdown
## SPEC-GOOSE-LLM-ROUTING-V2-001 Progress

### Phase 1 (Policy schema + loader)
- [x] AC-RV2-001 (2026-05-XX) — 정책 파일 부재 시 v1 byte-identical
- [ ] ...

### Phase 2 (Capability matrix + ratelimit)
- [ ] AC-RV2-004 — required_capabilities=[vision] 필터
- [ ] AC-RV2-005 — Rate limit 80% 임계 회피

### Phase 3 (Fallback + Decorator)
- [ ] AC-RV2-002 — AlwaysSpecific override
- [ ] AC-RV2-003 — PreferCheap 비용 오름차순
- [ ] AC-RV2-006 — FailoverReason.RateLimit chain 다음 후보
- [ ] AC-RV2-007 — FailoverReason.ContentFilter chain 즉시 중단
- [ ] AC-RV2-008 — excluded_providers silent skip
- [ ] AC-RV2-009 — 후보 0 개 v1 silent recovery
- [ ] AC-RV2-010 — fallback chain trace append-only

### Phase 4 (Integration)
- [ ] AC-RV2-011 — OpenRouter 단순 provider 취급
```
