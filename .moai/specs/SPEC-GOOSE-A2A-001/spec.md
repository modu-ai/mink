---
id: SPEC-GOOSE-A2A-001
version: 0.1.0
status: planned
created_at: 2026-04-21
updated_at: 2026-05-17
author: manager-spec
priority: P2
issue_number: null
phase: 7
size: 대(L)
lifecycle: spec-anchored
labels: [a2a, agent-protocol, hermes-acp, ide-integration, phase-7]
target_milestone: v0.2.0
mvp_status: deferred
deferred_reason: "0.1.0 MVP 범위 외 — v0.2.0 이월 (2026-05-17 사용자 확정). 외부 에이전트 통신·IDE 통합은 MVP 후순위."
---

# SPEC-GOOSE-A2A-001 — Agent Communication Protocol (Google A2A v0.3 + Hermes ACP 어댑터 + IDE 통합)

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (hermes-learning.md §9 + ecosystem.md §2.3 + MCP-001 인터페이스 계승) | manager-spec |

---

## 1. 개요 (Overview)

MINK 에이전트가 **외부 에이전트(다른 MINK, Claude, Gemini, 사용자 정의 A2A 에이전트)** 와 통신하고 태스크를 위임하며, **IDE(VS Code / Zed / JetBrains)** 와 프로토콜 수준에서 연동되도록 한다. 본 SPEC 은 다음 다섯 요소를 하나의 일관된 계약으로 제공한다:

1. **Agent Card 디스커버리** — YAML 기반 Agent Card v1 (Google A2A v0.3 호환), `/.well-known/agent-card` 엔드포인트.
2. **A2A Message Envelope** — sender / receiver / task / payload / attachments. 전송: HTTPS(JSON) 또는 gRPC.
3. **OAuth 2.0 PKCE 인증** — 외부 에이전트 간 상호 인증, short-lived bearer token.
4. **Task Delegation + Escrow** — 결제 기반 외부 에이전트 호출 시 tokens 에스크로 후 조건부 해제.
5. **ACP Adapter (IDE 통합)** — Hermes ACP 패턴을 기반으로 VS Code / Zed / JetBrains 의 IDE 에이전트 프로토콜을 MINK 의 MCP 인터페이스로 브리지.

본 SPEC 은 `internal/a2a/` 패키지의 **프로토콜 계약·메시지 스키마·인증 플로우** 를 규정한다. reputation 시스템은 "기본 수준" 만(aggregate score + recency), A2A 마켓플레이스 전체 로직은 Phase 7 별도 SPEC (BAZAAR-001) 에 위임한다.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- `.moai/project/ecosystem.md` §2.3 (Agent Card 예시, A2A v0.3) 와 §9.2 (Agent Card 디스커버리 sequence) 는 외부 에이전트 생태계의 프로토콜 층을 요구.
- `.moai/project/research/hermes-learning.md` §9 (ACP Adapter) 는 Hermes 의 `acp_adapter/{server,events,tools,session,auth,permissions}.py` 구조와 Google A2A v0.3 의 차이점을 문서화. 두 프로토콜을 동시에 지원하는 어댑터가 필요.
- SPEC-GOOSE-MCP-001 (Client/Server, stdio/WS/SSE, OAuth) 과 SPEC-GOOSE-SUBAGENT-001 (fork/worktree/bg isolation) 이 Phase 2 에서 이미 확정. A2A 는 그 **외부 확장**: MCP 는 single-tenant tool, A2A 는 multi-tenant agent.
- Phase 7 의 "Ecosystem" 중 본 로드맵이 유지하는 유일한 SPEC (나머지 BAZAAR/GATEWAY/PRIVACY 는 `ROADMAP-ECOSYSTEM.md`).

### 2.2 상속 자산 (패턴만 계승)

- **Google A2A v0.3 specification**: `/.well-known/agent-card`, `modalities`, `capabilities`, `pricing`, `sla`, `a2a_version` 필드.
- **Hermes ACP (Agent Communication Protocol)**: `acp_adapter/` 계층. OpenAI 형식 tool schema 노출, session 관리, OAuth/JWT.
- **OAuth 2.0 PKCE (RFC 7636)**: Public client 보안 흐름, state/nonce/code_verifier.
- **fxamacker/cbor v2** 옵션: 바이너리 직렬화(선택), payload size 최적화.
- **VS Code Agent API** / **Zed AI** / **JetBrains AI Assistant**: IDE 측 프로토콜 차이는 어댑터 계층에서 흡수.

### 2.3 범위 경계

- **IN**: Agent Card v1 스키마/로더/서명 검증, A2A Message Envelope (JSON 기본 + CBOR 옵션), `/.well-known/agent-card` HTTP 엔드포인트, OAuth 2.0 PKCE 클라이언트·리소스 서버, Task Delegation API, Escrow (tokens 기반, PRIVACY/BAZAAR 독립), Reputation (aggregate score, 최소), ACP Adapter (IDE bridge), MCP tool schema → A2A skill 변환기.
- **OUT**: 실제 결제 정산(→ BAZAAR-001/token-economy), 분산 원장/블록체인 escrow, Agent Card 마켓플레이스 검색 UI, 상업 콘텐츠 정책 심사(→ ecosystem §8.3), 사용자 권한 모델의 UI(PERMISSION-001 같은 별도 SPEC), 다국어 Agent Card 번역 자동화, Federated trust chain.

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

1. `internal/a2a/` 패키지 생성.
2. `AgentCard` 구조체 + YAML 로더 (`LoadAgentCard(path string) (AgentCard, error)`).
3. `/.well-known/agent-card` HTTP 엔드포인트 (MINK 가 자신을 외부에 노출할 때).
4. `AgentCardClient` — 외부 엔드포인트에서 Agent Card 조회 + Ed25519 서명 검증.
5. `A2AMessage` envelope 구조체 + JSON/CBOR serde.
6. `A2AServer` — 타 에이전트의 요청 수신, authN (OAuth 2.0 PKCE), authZ (scope), 라우팅 (task → 내부 subagent).
7. `A2AClient` — 외부 에이전트에 delegation 요청 송신, token 취득, 결과 수신.
8. OAuth 2.0 PKCE 구현: code_verifier/code_challenge, state/nonce, token endpoint, 짧은 bearer (기본 15분).
9. `TaskDelegation` API — `DelegateTask(target AgentCard, task Task, budget Budget) (TaskHandle, error)`.
10. `Escrow` 기본 구현 — local-only tokens ledger, `Hold(amount) → Release(taskID, success) | Refund(taskID, reason)`. BAZAAR-001 이 후에 이를 대체 가능한 인터페이스.
11. `Reputation` 기본 — `Record(agentID, outcome, latency)`, `Score(agentID) float64` (EMA 기반 0-1).
12. `ACPAdapter` — IDE 측 JSON-RPC ↔ MINK MCP-001 tool 변환. VS Code / Zed / JetBrains 벤더별 differ 를 작은 `Flavor` enum 으로 흡수.
13. MCP tool schema → A2A skill 변환기 (`ExposeSkillFromMCP(tool MCPTool) A2ASkill`).
14. Agent Card 에 선언된 `modalities` (text / pdf / docx / image) 는 metadata 수준만 저장, 실제 처리는 본 SPEC 범위 밖.
15. Rate limiting: 에이전트당 초당 요청 수, RATELIMIT-001 재사용.

### 3.2 OUT OF SCOPE

- **결제 정산 / fiat 변환**: BAZAAR-001 + token-economy.
- **블록체인 / x402 / USDC**: 해당 별도 SPEC.
- **Agent Card 마켓플레이스 UI**: ecosystem §5 (Bazaar).
- **콘텐츠 심사 / 악성 감지**: PRIVACY / SECURITY 별도 SPEC.
- **다국어 번역 자동화**: Agent Card 의 `description_ko`, `description_en` 등 여러 locale 필드는 수동 제공만.
- **Multi-hop delegation graph analysis**: A 에게 위임된 task 가 B 에게 재위임될 때 순환 탐지 등은 차기 SPEC.
- **Identity federation**: DID/VC, self-sovereign identity 는 범위 외.

---

## 4. 목표 (Goals)

- Agent Card 조회 p95 레이턴시 300ms 이하 (외부 HTTPS, TLS 핸드셰이크 포함).
- A2A task delegation round-trip 은 payload 4KB 기준 p95 1.2s 이하 (localhost 간은 50ms 이하).
- OAuth 2.0 PKCE 토큰 교환 실패율 < 0.1% (정상 네트워크).
- ACP adapter 가 VS Code / Zed / JetBrains 모두에서 동일 tool 호출 시퀀스를 관찰 (동등성 테스트).
- Escrow 불변식: Hold 총합 == 미해제 잔고 (드리프트 0원).
- Reputation 은 1,000 record/second 를 drop 없이 흡수.
- 외부 Agent Card 의 Ed25519 서명이 검증 실패하면 task delegation 은 절대 수행되지 않음.

## 5. 비목표 (Non-Goals)

- 천만 에이전트 스케일의 discovery registry.
- 실시간 escrow 대시보드 UI.
- IDE 확장 패키지 자체 배포(본 SPEC 은 프로토콜 브리지만).
- 외부 에이전트의 SLA 강제 (SLA 는 메타데이터, 위반 시 reputation 반영만).

---

## 6. 요구사항 (EARS Requirements)

### REQ-A2A-001 [Ubiquitous]
The A2A service shall expose `A2AServer` and `A2AClient` Go interfaces; the server shall accept inbound requests at `POST /a2a/v1/task` and the client shall submit requests to the same path at the target agent.

### REQ-A2A-002 [Ubiquitous]
Every outbound Agent Card lookup shall verify an Ed25519 signature attached in the `signature` field of the card; verification failure shall return `ErrCardSignatureInvalid`.

### REQ-A2A-003 [Event-Driven]
When `LoadAgentCard(path)` is called, the service shall validate the YAML against the Agent Card v1 JSON schema (`schemas/agent_card_v1.json`); schema violation shall return `ErrCardSchemaInvalid` with JSON-path pointers.

### REQ-A2A-004 [Event-Driven]
When an external request arrives at `POST /a2a/v1/task`, the server shall authenticate the caller via OAuth 2.0 PKCE bearer token; tokens older than `auth.token_ttl` (default 900s) shall be rejected with HTTP 401.

### REQ-A2A-005 [Event-Driven]
When `A2AClient.DelegateTask(target, task, budget)` is invoked, the client shall:
  (a) fetch `target.endpoint.url`'s Agent Card and verify its signature,
  (b) perform OAuth 2.0 PKCE if no valid token is cached,
  (c) place `budget.amount` into escrow via `Escrow.Hold`,
  (d) submit the task envelope,
  (e) return a `TaskHandle` for subsequent status polling.

### REQ-A2A-006 [Event-Driven]
When a delegated task returns a result, the client shall call `Escrow.Release(taskID, success=true)` on success; on reported failure or SLA breach, it shall call `Escrow.Refund(taskID, reason)`.

### REQ-A2A-007 [State-Driven]
While an escrow hold is active, the tokens shall not be usable for any other task by any other caller; double-spend attempts shall return `ErrEscrowConflict`.

### REQ-A2A-008 [Ubiquitous]
The `A2AMessage` envelope shall contain fields: `version`, `message_id` (UUID v7), `sender_agent_id`, `receiver_agent_id`, `task_id`, `task_type`, `payload`, `attachments`, `signature`, `timestamp`. Any missing mandatory field shall return `ErrEnvelopeInvalid`.

### REQ-A2A-009 [State-Driven]
While the configuration value `a2a.transport` is `json` (default), envelopes shall be serialized as JSON (RFC 8259); while `cbor`, they shall be serialized as CBOR (RFC 8949). The content-type header shall match.

### REQ-A2A-010 [Event-Driven]
When `ACPAdapter.Start(flavor)` is called with flavor in `{vscode, zed, jetbrains}`, the adapter shall expose the MCP tool catalog as ACP skills and bridge each tool invocation into a corresponding MCP call at the local MINK daemon.

### REQ-A2A-011 [Event-Driven]
When an ACP tool invocation completes, the adapter shall send the result back to the IDE in the flavor-specific JSON-RPC schema; the adapter shall not leak internal MINK session identifiers into the IDE response.

### REQ-A2A-012 [Event-Driven]
When `Reputation.Record(agentID, outcome, latency)` is called, the service shall update the EMA-based score within `O(1)` and persist the update atomically (`reputation.json` with rename).

### REQ-A2A-013 [Unwanted]
The A2A service shall not execute any task whose Agent Card contains `capabilities` not explicitly listed in the caller's declared scope; attempts shall return `ErrCapabilityDenied`.

### REQ-A2A-014 [Unwanted]
The service shall not forward a task to a subagent if the Agent Card's `languages` list excludes the user's configured `conversation_language` (language.yaml) AND the card does not declare `languages: ["*"]`.

### REQ-A2A-015 [Optional]
Where the Agent Card declares `modalities: ["pdf", "docx"]`, the client may attach binary files as `A2AMessage.attachments`; absent modality declaration, attachments of that type shall be rejected.

### REQ-A2A-016 [Optional]
Where the configuration value `a2a.allow_multihop = true`, the server may accept tasks that will be re-delegated to a third agent; absent flag defaults to `false` and multi-hop shall return `ErrMultihopDisabled`.

### REQ-A2A-017 [Complex]
While a delegated task is in flight, when the client receives HTTP 5xx or network timeout, it shall retry up to `client.retry.max` (default 3) with exponential backoff (base=500ms, factor=2, jitter=0.2); after exhaustion it shall call `Escrow.Refund` and return `ErrDelegationFailed`.

### REQ-A2A-018 [Event-Driven]
When `ExposeSkillFromMCP(tool)` is called, the service shall emit an `A2ASkill` whose `input_schema` == `tool.input_schema`, `output_schema` == `tool.output_schema`, and `handler` routes to the local MCP client.

### REQ-A2A-019 [Ubiquitous]
All bearer tokens issued by the service shall be PoP-bound (proof-of-possession) via `DPoP` header (RFC 9449); tokens without valid DPoP shall be rejected.

### REQ-A2A-020 [Ubiquitous]
All persistent artifacts (tokens cache, escrow ledger, reputation store) shall be encrypted at rest using the `$MINK_HOME/.secret` device-local key (AES-256-GCM).

---

## 7. 설계 결정 (Design Decisions)

### DD-A2A-01 — Agent Card v1 YAML

YAML 이 사람이 편집/검토하기 쉽고 주석 지원. JSON schema 는 대조 검증용으로만 별도 보관. Ed25519 서명은 card body(YAML canonicalization 후) 에 대해 수행하고 `signature:` 필드에 base64 로 저장.

### DD-A2A-02 — JSON 기본, CBOR 옵션

사람이 읽을 수 있는 JSON 이 기본. 대용량 attachment (PDF 수십 MB) 가 있는 경우 CBOR 로 전환 시 직렬화 오버헤드 20~30% 감소. 두 transport 인터페이스를 동일 Go 코드 경로로 통합.

### DD-A2A-03 — OAuth 2.0 PKCE + DPoP

Public client 에서의 authorization code 탈취 방지(PKCE) + 탈취된 bearer token 의 재사용 방지(DPoP). Access token 15분 / refresh 24시간. MCP-001 의 OAuth 와 인터페이스 공유.

### DD-A2A-04 — Escrow 는 로컬 시작

Phase 7 첫 릴리즈는 local-only escrow (사용자 자신의 token 원장). BAZAAR-001 가 추후 중앙 집중 escrow 서비스로 교체 가능하도록 `Escrow` 인터페이스 분리. 블록체인/crypto 는 범위 외 (DD-ECOSYSTEM 에서 후속).

### DD-A2A-05 — Reputation 기본값

EMA 기반 단순 점수: `score_new = 0.9*score_old + 0.1*outcome`. `outcome ∈ [0,1]` 로 사전에 정규화. 1000 rps 요구사항은 in-memory ringbuffer + 5s 주기 flush 로 해결.

### DD-A2A-06 — ACP Adapter 의 Flavor enum

VS Code / Zed / JetBrains 의 프로토콜 차이(필드명 case, event 이름, 오류 포맷)는 공통 Go 인터페이스 뒤의 `Flavor` enum 으로 숨김. 프로토콜 버전 변동은 `FlavorVersion` 추가 필드로 관리.

### DD-A2A-07 — Multi-hop 기본 OFF

순환 방지·경계 모호·책임 추적 복잡. 기본 비활성, 사용자 명시적 opt-in. 현재 SPEC 은 ACC/TRACE 헤더만 설계하고 본격 graph enforcement 는 차기.

### DD-A2A-08 — At-rest 암호화 필수

Agent Card 가 사용자의 identity-linked 크리덴셜을 보관 가능. 키는 장치별(`$MINK_HOME/.secret` 64byte 랜덤). 메인 DB(MEMORY-001) 와 같은 원칙. Key 유실 == 재가입(고지).

---

## 8. 데이터 모델 (Data Model)

### 8.1 Agent Card v1 스키마 (YAML 예시)

```yaml
schema_version: 1
id: "urn:goose:agent:my-personal-goose"
name: "My Personal MINK"
version: "1.0.0"
author:
  name: "Goos"
  verified: true
  kyc_level: 0
description:
  en: "My personalized agent"
  ko: "내 개인화 에이전트"
capabilities:
  - text_chat
  - code_review
  - schedule_management
modalities: ["text", "image"]
languages: ["en", "ko"]
pricing:
  model: "per_task"
  base_cost: 0
  currency: "GEN"
sla:
  max_response_time: "5 minutes"
  availability: "99.0%"
endpoint:
  type: "https"
  url: "https://example.com/a2a/v1"
  auth: "oauth2_pkce"
a2a_version: "0.3"
signature:
  alg: "Ed25519"
  public_key_b64: "..."
  value_b64: "..."
```

### 8.2 Go 타입

```go
// internal/a2a/types.go

type AgentCard struct {
    SchemaVersion int             `yaml:"schema_version"`
    ID            string          `yaml:"id"`             // urn:goose:agent:...
    Name          string          `yaml:"name"`
    Version       string          `yaml:"version"`
    Author        AuthorInfo      `yaml:"author"`
    Description   map[string]string `yaml:"description"`  // locale → text
    Capabilities  []string        `yaml:"capabilities"`
    Modalities    []string        `yaml:"modalities"`
    Languages     []string        `yaml:"languages"`
    Pricing       Pricing         `yaml:"pricing"`
    SLA           SLA             `yaml:"sla"`
    Endpoint      Endpoint        `yaml:"endpoint"`
    A2AVersion    string          `yaml:"a2a_version"`    // "0.3"
    Signature     Signature       `yaml:"signature"`
}

type AuthorInfo struct {
    Name       string
    Verified   bool
    KYCLevel   int
    Reputation float64    // 외부 조회 시 reputation 서비스에서 채움
}

type Pricing struct {
    Model    string    // "per_task" | "per_minute" | "free"
    BaseCost int64     // GEN tokens
    Currency string    // "GEN"
}

type SLA struct {
    MaxResponseTime string
    Availability    string
}

type Endpoint struct {
    Type string   // "https" | "grpc"
    URL  string
    Auth string   // "oauth2_pkce" | "api_key"
}

type Signature struct {
    Alg          string    // "Ed25519"
    PublicKeyB64 string
    ValueB64     string
}

// A2AMessage envelope
type A2AMessage struct {
    Version         string              `json:"version"`           // "1"
    MessageID       string              `json:"message_id"`        // UUID v7
    SenderAgentID   string              `json:"sender_agent_id"`
    ReceiverAgentID string              `json:"receiver_agent_id"`
    TaskID          string              `json:"task_id"`
    TaskType        string              `json:"task_type"`         // capability name
    Payload         map[string]any      `json:"payload"`
    Attachments     []Attachment        `json:"attachments,omitempty"`
    Signature       *EnvelopeSignature  `json:"signature,omitempty"`
    Timestamp       time.Time           `json:"timestamp"`
}

type Attachment struct {
    Name     string  `json:"name"`
    MimeType string  `json:"mime_type"`
    SizeBytes int64  `json:"size_bytes"`
    ContentB64 string `json:"content_b64"`       // 또는 별도 URL
    Modality string  `json:"modality"`           // "pdf" | "docx" | "image"
}

// TaskDelegation
type Task struct {
    Type     string
    Payload  map[string]any
    Deadline time.Time
    Scope    []string        // 허용 capability 범위
}

type Budget struct {
    Amount   int64    // GEN tokens
    Currency string
}

type TaskHandle interface {
    ID() string
    Status(ctx context.Context) (TaskStatus, error)
    Wait(ctx context.Context) (TaskResult, error)
    Cancel(ctx context.Context) error
}

type TaskStatus string

const (
    TaskStatusQueued     TaskStatus = "queued"
    TaskStatusInProgress TaskStatus = "in_progress"
    TaskStatusCompleted  TaskStatus = "completed"
    TaskStatusFailed     TaskStatus = "failed"
    TaskStatusCanceled   TaskStatus = "canceled"
)

type TaskResult struct {
    Success    bool
    Payload    map[string]any
    ErrorCode  string
    Latency    time.Duration
}

// Escrow
type Escrow interface {
    Hold(ctx context.Context, userID, taskID string, amount int64) (HoldReceipt, error)
    Release(ctx context.Context, taskID string) error
    Refund(ctx context.Context, taskID, reason string) error
    Balance(ctx context.Context, userID string) (int64, error)
}

type HoldReceipt struct {
    TaskID    string
    Amount    int64
    HeldAt    time.Time
    Signature string
}

// Reputation
type Reputation interface {
    Record(ctx context.Context, agentID string, outcome float64, latency time.Duration) error
    Score(ctx context.Context, agentID string) (float64, error)
}

// ACP Adapter
type ACPAdapter interface {
    Start(ctx context.Context, flavor Flavor, cfg ACPConfig) error
    Stop(ctx context.Context) error
}

type Flavor string

const (
    FlavorVSCode     Flavor = "vscode"
    FlavorZed        Flavor = "zed"
    FlavorJetBrains  Flavor = "jetbrains"
)
```

---

## 9. API/인터페이스 (Public Surface)

공개 심볼:

- `AgentCard` / `A2AMessage` / `Task` / `TaskHandle` / `TaskResult` / `Budget` / `HoldReceipt`
- `A2AServer` / `A2AClient` 인터페이스 + `NewA2AServer(cfg)` / `NewA2AClient(cfg)`
- `LoadAgentCard(path)` / `VerifyAgentCard(card, publicKey)`
- `Escrow` / `Reputation` 인터페이스 + `NewLocalEscrow(path)` / `NewEMAReputation(path)`
- `ACPAdapter` + `NewACPAdapter(flavor, cfg)`
- `ExposeSkillFromMCP(tool)`

에러 심볼: `ErrCardSchemaInvalid`, `ErrCardSignatureInvalid`, `ErrEnvelopeInvalid`, `ErrCapabilityDenied`, `ErrLanguageMismatch`, `ErrEscrowConflict`, `ErrMultihopDisabled`, `ErrDelegationFailed`, `ErrTokenExpired`, `ErrDPoPInvalid`.

---

## 10. Exclusions (What NOT to Build)

- 결제 정산 로직 (→ BAZAAR-001, token-economy).
- 블록체인 / x402 / USDC escrow.
- Agent Card 등록/검색 UI (→ ecosystem §5 Bazaar).
- Federated identity (DID/VC).
- IDE 확장 패키지 자체 (.vsix, Zed extension 등 배포 파일).
- Multi-hop delegation 의 그래프 분석·순환 탐지 (차기 SPEC).
- 다국어 Agent Card 자동 번역.
- 콘텐츠 moderation / 악성 에이전트 탐지.
- Reputation 의 sybil 공격 방지 메커니즘 (단순 EMA 기반 외).
- Agent Card 에 서명하는 키의 분산 KMS 관리.

---

## 11. Acceptance Criteria

### AC-A2A-001 — Agent Card 로드·서명 검증
Given 유효한 Ed25519 키쌍으로 서명된 agent-card.yaml 이 있을 때, When `LoadAgentCard("agent-card.yaml")` → `VerifyAgentCard(card, publicKey)` 를 호출하면, Then 에러 없이 통과한다. 1 byte 라도 변조 시 `ErrCardSignatureInvalid` 가 반환된다.

### AC-A2A-002 — 스키마 검증
Given `a2a_version` 필드가 누락된 agent-card.yaml 에 대해 `LoadAgentCard` 를 호출할 때, Then `ErrCardSchemaInvalid` 가 반환되고 에러 메시지에 JSON-path `$.a2a_version` 이 포함된다.

### AC-A2A-003 — OAuth 2.0 PKCE 플로우
Given 두 개의 MINK 인스턴스(A, B)가 로컬에서 실행될 때, When `A` 가 `B` 에게 `DelegateTask` 를 호출하면, Then OAuth 2.0 PKCE 토큰이 발행되고 재사용되며, `auth.token_ttl` 만료 후에는 refresh 흐름이 자동 실행된다.

### AC-A2A-004 — DPoP 검증
Given 유효 bearer token 이 발급된 상태에서 DPoP 헤더 없이 task 요청이 도착할 때, Then 서버는 HTTP 401 을 반환하고 로그에 `ErrDPoPInvalid` 가 기록된다.

### AC-A2A-005 — Capability 기반 접근 제어
Given Agent Card 의 `capabilities: ["text_chat"]` 이 선언되었을 때, When 수신한 task 의 `task_type` 이 `"code_review"` 이면, Then `ErrCapabilityDenied` 가 반환되고 에스크로는 자동 환불된다.

### AC-A2A-006 — Language 일치
Given 사용자 conversation_language 가 `ja` 이고 Agent Card 의 `languages: ["en", "ko"]` 일 때, When delegation 을 시도하면, Then `ErrLanguageMismatch` 가 반환된다. 단 Agent Card 가 `languages: ["*"]` 을 선언하면 통과한다.

### AC-A2A-007 — Escrow 불변식
Given 100 concurrent `Hold(100 GEN)` 호출이 같은 user 로 수행될 때, Then 모든 `Hold` 가 원자적으로 처리되고 총 잔고 + 총 hold 합이 항상 초기값과 일치한다(드리프트 0).

### AC-A2A-008 — Escrow Release/Refund 일관성
When 임의의 task 에 대해 `Release` 와 `Refund` 가 순차 호출되면, Then 두 번째 호출은 `ErrEscrowAlreadySettled` 를 반환하고 원장은 첫 번째 호출 결과로 고정된다.

### AC-A2A-009 — Reputation 업데이트
Given 동일 agent 에 대해 10 record 를 순차 기록하고 각 outcome=0.5 로 설정할 때, When `Score(agent)` 를 호출하면, Then 0.5 ± 1e-6 을 반환한다 (EMA 수렴).

### AC-A2A-010 — ACP Adapter 동등성
Given MCP 카탈로그에 3개 tool 이 등록된 상태에서, When `ACPAdapter.Start(FlavorVSCode)` → 테스트 하네스가 tool 3개 순차 invoke 할 때, Then 각 invocation 이 MCP 클라이언트까지 도달하고 결과가 IDE JSON-RPC 포맷으로 정확히 매핑된다. 동일 시나리오가 Zed/JetBrains flavor 에서도 통과한다.

### AC-A2A-011 — 재시도 + 환불
Given target agent 가 3회 HTTP 500 을 반환할 때, When `DelegateTask` 가 호출되면, Then 최대 3회 retry 후 `ErrDelegationFailed` 가 반환되고 `Escrow.Refund` 가 정확히 1번 호출된다.

### AC-A2A-012 — Multi-hop 기본 차단
Given `a2a.allow_multihop = false` (default) 일 때, 수신 task 의 메타데이터에 `x-a2a-forwarded-by: another-agent` 헤더가 존재하면, Then 서버는 `ErrMultihopDisabled` 를 반환한다.

---

## 12. 테스트 전략 (요약, 상세는 research.md)

- 단위: `card_schema_test.go`(서명·JSON-schema), `envelope_test.go`(JSON/CBOR roundtrip), `pkce_test.go`(code_verifier 엔트로피), `reputation_ema_test.go`.
- 통합: 두 goosed 인스턴스 (로컬 + 동일 머신 다른 port) 간 E2E, `-tags=integration`.
- Chaos: target agent 다운, 네트워크 TLS fail, DPoP replay attack, escrow double-spend.
- Fuzz: `fuzz/envelope_parse_fuzz.go` — 잘못된 JSON/CBOR 입력이 panic 없이 에러로만 귀결.
- 커버리지: 85%+.

---

## 13. 의존성 & 영향 (Dependencies & Impact)

- **상위 의존**: MCP-001(Tool schema·OAuth), SUBAGENT-001(task 수신 후 내부 subagent 로 위임), RATELIMIT-001(요청률 제한), CONFIG-001(`a2a.*`, `acp.*` 설정).
- **하위 소비자**: Phase 7 BAZAAR-001 (Escrow/Reputation 인터페이스 교체), GATEWAY-001 (Multi-platform), PRIVACY-001 (Agent Card 서명키 관리 업그레이드).
- **CLI 영향**: `mink a2a publish-card <path>`, `mink a2a call <target> <task>`, `mink a2a reputation <agentID>`, `mink acp start --flavor vscode`.

---

## 14. 오픈 이슈 (Open Issues)

1. **Agent Card 서명 키 로테이션**: 기본 Ed25519 단일 키. 키 탈취 시 revocation list(CRL 유사) 를 어디에 둘지 미정. 후속 PRIVACY-001.
2. **CBOR vs JSON 선택 기본값**: 현재 JSON 고정. `attachment.size_bytes >= N` 일 때 자동 CBOR 스위치 정책 연구 필요.
3. **Token discovery**: OAuth 2.0 PKCE 에서 authorization server 발견을 `/.well-known/oauth-authorization-server` 로 가정. Agent Card 가 별도 `auth_server` 를 선언하는 경우 우선순위 합의 필요.
4. **ACP Flavor version**: VS Code 2026-Q2 API 와 Zed 0.150 이 스키마 차이가 미묘. 마이너 버전 분기 관리 방법.
5. **Reputation 초기값 bootstrap**: 처음 만난 에이전트는 score=?(0.5 neutral vs 0.0 pessimistic). 기본 0.5.
6. **Escrow 암호화 키 회전**: `$MINK_HOME/.secret` 회전 시 기존 ledger 재암호화 절차. 후속 CLI `mink a2a rotate-key`.
7. **Multi-hop 추적 헤더**: 본 SPEC 은 차단만 수행. 추적이 필요할 때 OpenTelemetry trace_id 를 envelope 에 싣는 형태 or 별도 필드?
8. **SLA 강제 메커니즘**: SLA 메타데이터는 reputation 에 반영되지만 실시간 enforcement 는 없음. 필요 시 별도 watchdog.
