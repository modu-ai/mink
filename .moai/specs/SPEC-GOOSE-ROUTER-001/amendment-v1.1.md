---
id: SPEC-GOOSE-ROUTER-001-amendment-v1.1
version: 1.1.0
base_spec_version: 1.0.0
status: proposed
created_at: 2026-04-29
updated_at: 2026-04-29
author: manager-spec
priority: P1
issue_number: null
phase: 2
labels: [routing, llm, infrastructure, delegation, amendment]
---

# Amendment v1.1 — 3-Layer Routing + Delegation Rules

**Base SPEC**: SPEC-GOOSE-ROUTER-001 v1.0.0 (completed, 2026-04-27)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 1.1.0 | 2026-04-29 | 3-layer routing + delegation rules + fallback chains + override prefixes 확장 | manager-spec |

---

## 1. 개요 (Overview)

본 Amendment는 완료된 SPEC-GOOSE-ROUTER-001 v1.0.0(2-layer: primary/cheap 정적 heuristic 라우팅)을 **3-layer 라우팅 + 동적 delegation rules**로 확장한다. v1.0.0의 기존 REQ/AC는 그대로 유지되며, 본 문서의 REQ는 추가/확장만을 규정한다.

확장 핵심:

- **3 routing layers**: Layer 1 (Local/Ollama), Layer 2 (Cloud API), Layer 3 (CLI Delegation)
- **4 routing strategies**: local-only, cloud-only, hybrid, delegation
- **Pattern-based delegation rules**: regex 매칭으로 특정 provider에게 위임
- **Explicit override prefixes**: `!claude`, `!gemini`, `!codex` 등 사용자 직접 지정
- **Fallback chains**: primary → secondary → tertiary 순차 대체
- **Routing decision logging**: 사용자 콘텐츠 노출 없이 결정 메타데이터 기록

---

## 2. 배경 (Background)

### 2.1 왜 3-Layer 확장이 필요한가

- v1.0.0은 primary/cheap 2-layer 정적 heuristic만 지원. AI.GOOSE가 다양한 실행 환경(로컬 Ollama, 클라우드 API, 외부 CLI 도구)을 활용함에 따라 더 유연한 라우팅이 필요.
- product.md §3.1 "로컬 우선" 정책: 로컬 모델로 처리 가능하면 로컬, 불가하면 클라우드로 auto-escalation.
- security audit, architecture review 등 특정 태스크는 Claude CLI에게 위임하는 것이 품질/비용 면에서 유리. Pattern-based delegation으로 이를 자동화.

### 2.2 v1.0.0과의 호환성

- v1.0.0의 `SimpleClassifier`, `ProviderRegistry`, `Route`, `RoutingConfig` 타입은 그대로 유지.
- v1.1.0은 `RoutingConfig`에 새 필드를 추가(확장)하고, `Router.Route()` 로직에 layer selection + delegation 단계를 추가.
- 기존 v1.0.0 설정으로 구성된 환경은 `mode: local-only`(기본값)로 동작하여 backward compatible.

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE (본 Amendment 추가 분)

1. `RoutingConfig`에 `Mode` 필드 추가: `local-only`, `cloud-only`, `hybrid`, `delegation`.
2. `RoutingConfig`에 `Local`, `Cloud`, `Delegation` 하위 설정 구조 추가.
3. Delegation rules: `pattern` (regex) → `target` (provider/CLI) 매핑.
4. Explicit override prefixes: `!claude`, `!gemini`, `!codex` 등 메시지 접두사 감지.
5. Fallback chains: `primary → secondary → tertiary` provider 순차 대체.
6. Hybrid mode auto-escalation: local confidence < threshold → cloud escalation.
7. Routing decision logging (user content 제외, 결정 메타데이터만).

### 3.2 OUT OF SCOPE (변경 없음, v1.0.0 OUT OF SCOPE 동일 + 아래 추가)

- **학습 기반 라우팅**: INSIGHTS-001 (Phase 4).
- **Rate limit 기반 동적 라우팅**: RATELIMIT-001 연동은 후속.
- **CLI delegation의 실제 프로세스 실행**: 본 SPEC은 routing 결정까지만 담당. CLI spawn은 QUERY-001이나 agent runtime이 담당.
- **Cost tracking**: 가격 기반 라우팅 결정은 후속 SPEC.

---

## 4. EARS 요구사항 (Requirements — Amendment 전용)

### 4.1 Ubiquitous (시스템 상시 불변)

**REQ-RT-017 [Ubiquitous]** — The router **shall** support 3 routing layers: Layer 1 (Local/Ollama), Layer 2 (Cloud API via Anthropic/OpenAI/Google/etc.), Layer 3 (CLI Delegation to external tools).

**REQ-RT-018 [Ubiquitous]** — The router **shall** support the following routing strategies via `RoutingConfig.Mode`: `local-only`, `cloud-only`, `hybrid`, `delegation`.

### 4.2 Event-Driven (이벤트 기반)

**REQ-RT-019 [Event-Driven]** — **When** `Mode` is `hybrid`, the router **shall** attempt Layer 1 (local) first; **when** the local model confidence is below the configured threshold (default: 0.7), the router **shall** auto-escalate to Layer 2 (cloud). Confidence **shall** be computed by the existing `SimpleClassifier` from v1.0.0 as `classifier.Score(msg).Confidence` (a float in [0.0, 1.0] derived from message length, keyword complexity, and tool-call presence heuristics). Future SPEC INSIGHTS-001 may replace this heuristic with a learned model.

**REQ-RT-020 [Event-Driven]** — **When** delegation rules are enabled and a user message matches a delegation rule pattern (regex), the router **shall** route to the rule's target provider/CLI. **When** multiple rules match, the router **shall** select the first matching rule in the `rules` array order (declaration-order priority); later rules are not evaluated (short-circuit).

**REQ-RT-021 [Event-Driven]** — **When** a user message starts with an explicit override prefix (`!claude`, `!gemini`, `!codex`, etc.), the router **shall** strip the prefix and route to the specified provider, bypassing all heuristic classification. Override prefixes **shall** only be recognized at the start of the message (position 0); the same string appearing elsewhere in the message **shall not** trigger an override.

**REQ-RT-022 [Event-Driven]** — **When** no delegation rule matches and `Mode` is `hybrid`, the router **shall** use the local model as the default route.

### 4.3 State-Driven (상태 기반)

**REQ-RT-023 [State-Driven]** — **While** the primary provider is unavailable (network error, rate limit exhausted), the router **shall** fall back to the secondary provider; **while** the secondary is also unavailable, the router **shall** fall back to the tertiary provider defined in the fallback chain.

### 4.4 Unwanted Behavior (방지)

**REQ-RT-024 [Unwanted]** — Routing decision logs **shall not** include user message content, system prompts, or any PII; logs **shall** contain only: routing reason, provider name, model name, layer number, latency_ms, and signature.

**REQ-RT-025 [Unwanted]** — The router **shall not** perform actual network calls during delegation routing decisions; CLI process spawning is the caller's responsibility.

---

## 5. 수용 기준 (Acceptance Criteria — Amendment 전용)

**AC-RT-015 — Hybrid mode local-first routing**
- **Given** `Mode: hybrid`, local model `ai-goose/gemma4-e4b-rl-v1`, cloud default `anthropic/claude-sonnet`, threshold `0.7`
- **When** `Route(ctx, req)` is called with a simple message
- **Then** route is Layer 1 (local), `route.Layer == 1`, `route.Provider == "ollama"`

**AC-RT-016 — Hybrid mode auto-escalation**
- **Given** `Mode: hybrid`, local confidence threshold `0.7`
- **When** `Route(ctx, req)` is called with a complex message (e.g., "debug this architecture") and local model confidence evaluates below 0.7
- **Then** route is Layer 2 (cloud), `route.Layer == 2`, `route.Provider` is the configured cloud default

**AC-RT-017 — Delegation rule pattern match**
- **Given** `delegation.rules` contains `{pattern: "security audit|architecture review", target: "claude-cli"}` and a message `"perform a security audit on this codebase"`
- **When** `Route(ctx, req)` is called
- **Then** `route.Layer == 3`, `route.Provider == "claude-cli"`, `route.RoutingReason == "delegation_rule_match"`

**AC-RT-018 — Delegation rule no match → fallback**
- **Given** `delegation.rules` contains patterns but none match the message, `delegation.fallback: local`, `Mode: hybrid`
- **When** `Route(ctx, req)` is called with `"what is the weather today"`
- **Then** route uses the delegation fallback (local), `route.Layer == 1`

**AC-RT-019 — Explicit override prefix**
- **Given** override prefixes `!claude → anthropic`, `!gemini → google`, `!codex → codex-cli` are configured
- **When** user message is `"!claude analyze this design document"`
- **Then** prefix `!claude` is stripped, message becomes `"analyze this design document"`, `route.Provider == "anthropic"`, `route.RoutingReason == "explicit_override"`

**AC-RT-020 — Fallback chain primary → secondary**
- **Given** fallback chain `[anthropic, google, ollama]` and `anthropic` is marked as unavailable
- **When** `Route(ctx, req)` is called
- **Then** `route.Provider == "google"` (secondary), `route.RoutingReason` includes "fallback"

**AC-RT-021 — Fallback chain exhausted**
- **Given** fallback chain `[anthropic, google]` and all providers are unavailable
- **When** `Route(ctx, req)` is called
- **Then** `ErrFallbackExhausted` is returned with the list of attempted providers

**AC-RT-022 — Routing decision logging without PII**
- **Given** a `RoutingDecisionHook` that logs to structured output
- **When** any routing decision is made
- **Then** the log entry contains `provider`, `model`, `layer`, `reason`, `signature`, `latency_ms` but does NOT contain any portion of the user's message content

**AC-RT-023 — local-only mode ignores cloud**
- **Given** `Mode: local-only`, cloud providers are configured
- **When** `Route(ctx, req)` is called with a complex message
- **Then** route stays Layer 1 (local), regardless of complexity

**AC-RT-024 — cloud-only mode ignores local**
- **Given** `Mode: cloud-only`, local Ollama is configured
- **When** `Route(ctx, req)` is called with a simple message
- **Then** route goes to Layer 2 (cloud), local is not considered

**AC-RT-025 — Backward compatibility with v1.0.0 config**
- **Given** a v1.0.0 `RoutingConfig` without `Mode`, `Local`, `Cloud`, `Delegation` fields
- **When** the router is initialized
- **Then** it operates in `local-only` mode using the existing `Primary`/`CheapRoute` fields, producing identical results to v1.0.0

**AC-RT-026 — No network calls during routing** (verifies REQ-RT-025)
- **Given** a `Route()` call with any message and any `Mode`
- **When** the routing decision is computed
- **Then** no HTTP requests, subprocess spawns, or network I/O occur during the `Route()` call itself; only in-memory classification and pattern matching are performed

---

## 6. 기술적 접근 (Technical Approach — Amendment 전용)

### 6.1 Config 확장

```go
// RoutingConfig 확장 필드 (v1.1.0)

type RoutingMode string

const (
    RoutingModeLocalOnly  RoutingMode = "local-only"
    RoutingModeCloudOnly  RoutingMode = "cloud-only"
    RoutingModeHybrid     RoutingMode = "hybrid"
    RoutingModeDelegation RoutingMode = "delegation"
)

type LocalConfig struct {
    Provider string // e.g., "ollama"
    Model    string // e.g., "ai-goose/gemma4-e4b-rl-v1"
}

type CloudConfig struct {
    Default  string // e.g., "anthropic"
    Fallback string // e.g., "google"
}

type DelegationConfig struct {
    Enabled bool
    Rules   []DelegationRule
    Fallback string // "local" | "cloud" | "error"
}

type DelegationRule struct {
    Pattern string // regex pattern
    Target  string // provider or CLI identifier
}

type OverridePrefix struct {
    Prefix   string // e.g., "!claude"
    Provider string // e.g., "anthropic"
}

type FallbackChain struct {
    Providers []string // ordered: ["anthropic", "google", "ollama"]
}

// RoutingConfig에 추가되는 필드
type RoutingConfig struct {
    // ... v1.0.0 필드 유지 ...
    Mode      RoutingMode     // default: "local-only"
    Local     LocalConfig     // Layer 1
    Cloud     CloudConfig     // Layer 2
    Delegation DelegationConfig // Layer 3
    Overrides []OverridePrefix
    Fallback  FallbackChain
    ConfidenceThreshold float64 // hybrid escalation threshold, default 0.7
}
```

### 6.2 Route 확장

```go
type Route struct {
    // ... v1.0.0 필드 유지 ...
    Layer          int    // 1=local, 2=cloud, 3=delegation
    DelegationRule string // matched rule pattern (if applicable), empty otherwise
    OverrideUsed   string // prefix used (if applicable), empty otherwise
    FallbackFrom   string // original provider before fallback, empty if no fallback
}
```

### 6.3 확장 알고리즘 의사코드

```
Route_v1_1(ctx, req):
  // Step 0: Check explicit overrides
  msg = lastUserMessage(req.Messages)
  for override in cfg.Overrides:
    if strings.HasPrefix(msg, override.Prefix + " "):
      msg = strings.TrimPrefix(msg, override.Prefix + " ")
      return buildRoute(override.Provider, "explicit_override", Layer=2)

  // Step 1: Check delegation rules (if enabled, first-match wins)
  if cfg.Delegation.Enabled:
    for rule in cfg.Delegation.Rules:
      if regexMatch(rule.Pattern, msg):
        return buildRoute(rule.Target, "delegation_rule_match", Layer=3)
    // no rule matched → fall through to mode-based routing

  // Step 2: Mode-based routing
  switch cfg.Mode:
    case "local-only":
      return routeLocal(cfg.Local, classifier, msg)
    case "cloud-only":
      return routeCloud(cfg.Cloud, cfg.Fallback)
    case "hybrid":
      local_route = routeLocal(cfg.Local, classifier, msg)
      if classifier.confidence < cfg.ConfidenceThreshold:
        return routeCloud(cfg.Cloud, cfg.Fallback)
      return local_route
    case "delegation":
      // delegation rules already checked in Step 1
      // fallback to delegation.Fallback target
      return routeByFallback(cfg.Delegation.Fallback)
```

### 6.4 YAML 설정 예시

```yaml
llm:
  mode: hybrid
  local:
    provider: ollama
    model: ai-goose/gemma4-e4b-rl-v1
  cloud:
    default: anthropic
    fallback: google
  delegation:
    enabled: true
    rules:
      - pattern: "security audit|architecture review"
        target: claude-cli
      - pattern: "code generation|refactoring"
        target: codex-cli
      - pattern: "image analysis|document parsing"
        target: gemini-cli
    fallback: local
  overrides:
    - prefix: "!claude"
      provider: anthropic
    - prefix: "!gemini"
      provider: google
    - prefix: "!codex"
      provider: codex-cli
  fallback_chain:
    providers:
      - anthropic
      - google
      - ollama
  confidence_threshold: 0.7
```

### 6.5 TDD 진입

1. **RED**: `TestOverride_ClaudePrefix_RoutesToAnthropic` — AC-RT-019
2. **RED**: `TestHybrid_SimpleGreeting_LocalRoute` — AC-RT-015
3. **RED**: `TestHybrid_ComplexQuery_CloudEscalation` — AC-RT-016
4. **RED**: `TestDelegation_SecurityAudit_MatchesRule` — AC-RT-017
5. **RED**: `TestDelegation_NoMatch_FallsBackToLocal` — AC-RT-018
6. **RED**: `TestFallback_PrimaryDown_SecondaryUsed` — AC-RT-020
7. **RED**: `TestFallback_AllDown_ErrorReturned` — AC-RT-021
8. **RED**: `TestLocalOnly_IgnoresCloud` — AC-RT-023
9. **RED**: `TestCloudOnly_IgnoresLocal` — AC-RT-024
10. **RED**: `TestBackwardCompat_V1Config_WorksIdentically` — AC-RT-025
11. **GREEN**: 최소 구현
12. **REFACTOR**: layer selection 로직을 `LayerSelector` interface로 분리

---

## 7. 의존성 (Dependencies — Amendment 전용)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 기반 SPEC | SPEC-GOOSE-ROUTER-001 v1.0.0 | 본 Amendment의 base |
| 선행 SPEC | SPEC-GOOSE-TRAIN-001 | 학습된 로컬 모델(hybrid mode의 Layer 1) |
| 선행 SPEC | SPEC-GOOSE-ADAPTER-001 | Cloud provider HTTP 호출 |
| 후속 SPEC | SPEC-GOOSE-INSIGHTS-001 | Confidence score 학습 기반 최적화 |

---

## 8. 리스크 & 완화 (Risks & Mitigations — Amendment 전용)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | Confidence score가 실제 응답 품질과 불일치 | 높 | 중 | 초기에는 classifier heuristic 기반(복잡도 점수). 후속 INSIGHTS-001에서 모델 기반 confidence로 전환 |
| R2 | Delegation rule regex가 한국어 메시지에서 부정확 | 중 | 중 | 패턴은 한국어+영어 모두 지원. 사용자 정의 패턴 override 가능 |
| R3 | Fallback chain 순환 또는 무한 대기 | 낮 | 높 | 동일 provider로의 fallback 금지. 최대 3회 시도 제한 |
| R4 | Override prefix가 정상 대화 내용과 충돌 | 낮 | 중 | prefix는 `!` + 영문 알파벳만 허용. 공백 뒤 내용이 있어야 override로 인식 |

---

## Exclusions (What NOT to Build — Amendment 전용)

- 본 Amendment는 **CLI 프로세스 실제 실행**을 포함하지 않는다. Layer 3 delegation은 routing 결정까지만 담당.
- 본 Amendment는 **Rate limit 기반 동적 fallback**을 포함하지 않는다. RATELIMIT-001 연동은 후속.
- 본 Amendment는 **Cost-aware routing**(비용 기반 provider 선택)을 포함하지 않는다.
- 본 Amendment는 **Learning-based confidence scoring**을 포함하지 않는다. Heuristic 기반. INSIGHTS-001에서 확장.
- 본 Amendment는 **Multi-user routing policy**(사용자별 다른 routing 규칙)를 포함하지 않는다.

---

**End of Amendment v1.1 — SPEC-GOOSE-ROUTER-001**
