# SPEC-GENIE-A2A-001 research.md — A2A Protocol 구현 자료

> spec.md 의 설계 결정을 Go 코드로 옮길 때 참고. 외부 표준, ACP Flavor 차이, 암호 선택, TDD 전략, 리스크를 정리.

---

## 1. 외부 표준·의존

| 표준/의존 | 버전 | 용도 | 근거 |
|----|----|----|----|
| Google A2A Specification | v0.3 | Agent Card 필드·discovery 패턴 | ecosystem §2.3 |
| OAuth 2.0 PKCE (RFC 7636) | — | 인증 | DD-A2A-03 |
| DPoP (RFC 9449) | — | Token PoP | DD-A2A-03 |
| Ed25519 (RFC 8032) | — | Card 서명 | DD-A2A-01 |
| JSON (RFC 8259) / CBOR (RFC 8949) | — | Envelope | DD-A2A-02 |
| Hermes ACP 계층 | — | IDE bridge | hermes-learning §9 |
| `fxamacker/cbor` | v2.7.x | CBOR serde | go ecosystem |
| `go-yaml/yaml` | v3.0.1 | Agent Card YAML | 표준 |
| `xeipuuv/gojsonschema` | v1.2.0 | JSON schema 검증 | 표준 |
| `crypto/ed25519` (stdlib) | — | 서명 검증 | stdlib |
| `github.com/lestrrat-go/jwx/v2` | v2.0.x | DPoP JWS | PoP 토큰 |
| `google.golang.org/grpc` | v1.66 | grpc transport (옵션) | — |

JSON schema 파일은 `internal/a2a/schemas/agent_card_v1.json` 에 저장, `//go:embed` 로 바이너리 포함.

---

## 2. 패키지 디렉터리 계획

```
internal/a2a/
├── doc.go
├── types.go              // AgentCard, A2AMessage, Task, Budget
├── errors.go
├── interface.go          // A2AServer, A2AClient, Escrow, Reputation, ACPAdapter
├── card/
│   ├── loader.go         // LoadAgentCard (YAML → struct)
│   ├── schema.go         // JSON schema 검증
│   ├── signer.go         // Ed25519 sign/verify
│   └── loader_test.go
├── envelope/
│   ├── json.go
│   ├── cbor.go
│   └── envelope_test.go
├── server/
│   ├── server.go         // POST /a2a/v1/task 핸들러
│   ├── auth.go           // OAuth 2.0 PKCE + DPoP 검증
│   └── server_test.go
├── client/
│   ├── client.go         // DelegateTask
│   ├── oauth_pkce.go
│   └── client_test.go
├── escrow/
│   ├── local.go          // NewLocalEscrow (파일 ledger)
│   ├── ledger.go         // 원자 쓰기
│   └── local_test.go
├── reputation/
│   ├── ema.go            // NewEMAReputation
│   └── ema_test.go
├── acp/
│   ├── adapter.go        // Flavor 기반 ACPAdapter
│   ├── flavor_vscode.go
│   ├── flavor_zed.go
│   ├── flavor_jetbrains.go
│   └── adapter_test.go
└── discovery/
    └── wellknown.go      // /.well-known/agent-card HTTP 서버
```

---

## 3. Agent Card 서명·검증

### 3.1 Canonicalization

YAML → 정규화 단계:
1. `yaml.Unmarshal` → Go struct
2. 필드 정렬 (alphabetical, `signature` 제외)
3. JSON encode (`encoding/json`, `SetIndent("","")`) → canonical bytes
4. Ed25519 sign/verify

```go
func canonicalBytes(card AgentCard) ([]byte, error) {
    card.Signature = Signature{}       // 서명 제외
    return json.Marshal(card)           // Go JSON marshaler → 필드 순서 고정
}
```

Go 의 `json.Marshal` 이 struct field 순서를 고정하므로 추가 sort 불필요.

### 3.2 서명 생성/검증

```go
func SignCard(card AgentCard, priv ed25519.PrivateKey) ([]byte, error) {
    b, err := canonicalBytes(card)
    if err != nil { return nil, err }
    return ed25519.Sign(priv, b), nil
}

func VerifyCard(card AgentCard, pub ed25519.PublicKey) error {
    sig, err := base64.StdEncoding.DecodeString(card.Signature.ValueB64)
    if err != nil { return fmt.Errorf("decode signature: %w", err) }
    b, err := canonicalBytes(card)
    if err != nil { return err }
    if !ed25519.Verify(pub, b, sig) {
        return ErrCardSignatureInvalid
    }
    return nil
}
```

---

## 4. OAuth 2.0 PKCE + DPoP

### 4.1 PKCE 흐름

```
Client                     Auth Server                 Resource Server
  │                              │                            │
  ├─ code_verifier (random 64B)  │                            │
  ├─ code_challenge = SHA256(cv) │                            │
  ├─ GET /authorize?code_challenge=... ────────►             │
  │                              │                            │
  │  ◄── redirect with code ─────┤                            │
  │                              │                            │
  ├─ POST /token {code, code_verifier, dpop_jkt} ─►          │
  │  ◄── access_token (TTL 900s), refresh_token ──┤          │
  │                              │                            │
  ├─ POST /a2a/v1/task                                        │
  │   Authorization: DPoP <token>                             │
  │   DPoP: <jwt with htm, htu, iat, nonce>  ───────────────►│
  │                                                           │
  │  ◄── result ──────────────────────────────────────────────┤
```

### 4.2 DPoP JWT 구조

```json
{
  "alg": "EdDSA",
  "typ": "dpop+jwt",
  "jwk": { ... public_key ... }
}
{
  "jti": "uuid-v7",
  "htm": "POST",
  "htu": "https://example.com/a2a/v1/task",
  "iat": 1713715000,
  "nonce": "server-issued-nonce"
}
```

서버 측 검증:
- `htm` == 실제 HTTP 메서드
- `htu` == 정확한 URL (scheme, host, path 일치)
- `iat` 는 현재시간 ±60s
- `jti` 는 5분 이내 중복 금지 (replay)
- `jwk.thumbprint` == bearer token 의 `cnf.jkt` (PoP binding)

---

## 5. Envelope (JSON vs CBOR)

### 5.1 Content-Type 매핑

| Transport | Content-Type | 직렬화 |
|----|----|----|
| json (기본) | `application/json; charset=utf-8` | `encoding/json` |
| cbor | `application/cbor` | `fxamacker/cbor` |

### 5.2 CBOR 장점

- Binary attachments(PDF, image) 를 base64 없이 bytes field 로 직접.
- 20-30% 크기 축소.
- 스키마 호환(tag 지정 시).

### 5.3 엔벨로프 ID

`message_id` 는 UUID v7 (시간정렬) — replay 탐지에 유리.

---

## 6. Escrow 로컬 원장

### 6.1 스키마 (`escrow.json`)

```json
{
  "schema_version": 1,
  "user_balances": {
    "user-abc123": 12500
  },
  "holds": [
    {
      "task_id": "task-xyz",
      "user_id": "user-abc123",
      "amount": 2000,
      "held_at": "2026-04-21T10:05:00Z",
      "status": "held"  // held | released | refunded
    }
  ],
  "history": [
    { "task_id": "task-old", "amount": 500, "action": "released", "at": "..." }
  ]
}
```

### 6.2 원자 연산

- 파일 뮤텍스 (`flock` 또는 서비스 내 `sync.Mutex`).
- `Hold` → `Release/Refund` 트랜잭션은 단일 goroutine 에서 처리 (queue 기반).
- AES-256-GCM 암호화 at rest (device key).

### 6.3 잔고 불변식

```
Σ(user_balances) + Σ(held.amount) == 초기 충전 총액 - 사용된 released 총액
```

periodic check 로 위반 시 `ErrLedgerCorrupted` + alert.

---

## 7. ACP Flavor 차이

### 7.1 VS Code (2026 Q2 API)

- JSON-RPC 2.0 over stdio 또는 WebSocket.
- tool schema: `{ name, description, inputSchema (JSON Schema) }`
- result: `{ content: [{type: "text", text: "..."}, ...] }`

### 7.2 Zed (0.150+)

- JSON-RPC 2.0 over WebSocket.
- tool schema: `{ name, summary, parameters }`
- result: `{ output: "..."}` 단일 문자열 기본, rich 콘텐츠는 `output_parts: []`

### 7.3 JetBrains (AI Assistant 2026)

- JSON-RPC 2.0 over stdio.
- tool schema: `{ id, displayName, paramsSchema }`
- result: `{ status: "ok"|"err", payload: {...} }`

### 7.4 Flavor 어댑터 시그니처

```go
type Flavor string

type flavorImpl interface {
    TranslateToolCatalog(tools []MCPTool) any            // flavor 고유 JSON
    ParseIncomingCall(raw json.RawMessage) (MCPInvocation, error)
    FormatResult(res MCPResult) any
    FormatError(err error) any
}

// factory
func newFlavorImpl(f Flavor) (flavorImpl, error) { ... }
```

테스트는 각 Flavor 별 golden file (`testdata/acp/vscode/*.json` 등) 에서 입력 → 출력 매칭.

---

## 8. Reputation EMA

```
α = 0.1
score_new = (1-α) * score_old + α * outcome
```

`outcome` 계산:
```
outcome = 0.6 * normalized_success
       + 0.3 * normalized_latency_score   (빠를수록 1)
       + 0.1 * normalized_cost_efficiency (예산 대비)
```

영속화: `reputation.json` 5 초 주기 flush (원자 rename). in-memory ringbuffer 1024 entries.

---

## 9. TDD 테스트 전략

### 9.1 RED 단계

- `card_schema_test.go`: 필드 누락 각 케이스별 ErrCardSchemaInvalid.
- `signer_test.go`: 1 byte flip → Verify 실패.
- `envelope_json_cbor_test.go`: roundtrip 보존.
- `pkce_test.go`: code_verifier entropy 256-bit 이상.
- `dpop_test.go`: htu mismatch, replay, iat drift.
- `escrow_local_test.go`: 100 concurrent Hold 의 잔고 불변식.

### 9.2 GREEN 단계

1. types/errors 정의.
2. card loader + signer.
3. envelope serde.
4. server auth 미들웨어 (PKCE + DPoP).
5. client DelegateTask (target card fetch → escrow → submit).
6. ACP adapter vscode flavor 먼저 → Zed → JetBrains.
7. reputation EMA.

### 9.3 REFACTOR

- server/client 공통 HTTP middleware 를 `internal/a2a/http/` 로 추출.
- flavor 구현체의 공통 JSON-RPC 프레이밍.

### 9.4 통합 (-tags=integration)

- 두 개의 genied 인스턴스를 port 9001/9002 로 spawn, A → B delegation E2E.
- ACP: mock IDE 클라이언트(golden file 기반)로 3 flavor 순차 검증.

### 9.5 Fuzz

- `FuzzEnvelopeParseJSON`
- `FuzzEnvelopeParseCBOR`
- `FuzzAgentCardYAML`

panic 없이 모든 입력이 에러로만 귀결될 것.

### 9.6 커버리지

- 전체 85%+.
- `card/`, `envelope/`, `server/auth.go` 는 95%+.

---

## 10. 성능 목표

| 작업 | p50 | p95 | 비고 |
|----|----|----|----|
| Agent Card fetch (외부) | 150 ms | 300 ms | TLS 재사용 |
| OAuth PKCE 발급 | 80 ms | 200 ms | 로컬 auth server |
| Envelope JSON encode/decode | 15 µs | 40 µs | 4 KB payload |
| Envelope CBOR encode/decode | 10 µs | 30 µs | 4 KB payload |
| DPoP 검증 | 100 µs | 300 µs | signature verify 포함 |
| Escrow Hold | 0.5 ms | 2 ms | in-memory + async flush |
| Reputation Score | 0.1 µs | 0.5 µs | in-memory |

---

## 11. 리스크 & 완화

| 리스크 | 영향 | 완화 |
|----|----|----|
| Agent Card 서명키 유실 | High | `genie a2a rotate-key` + 새 키 배포, 기존 card 는 grace 30일 |
| DPoP nonce reuse 공격 | High | server-side nonce store TTL 5min, jti 중복 거부 |
| Escrow ledger 손상 | High | 무결성 체크섬(`HMAC-SHA256`) 각 transaction 에 첨부 |
| Flavor 프로토콜 변동 | Medium | golden file 기반 회귀 + flavor version 필드 |
| MCP↔A2A schema drift | Medium | `ExposeSkillFromMCP` 단일 진입점에서 버전 동기화 검증 |
| 외부 에이전트 availability | Low | 재시도 backoff + circuit breaker (기본 5회 연속 실패 후 30s open) |
| CBOR parser 취약점 | Medium | fuzz 테스트 필수, `fxamacker/cbor` 업데이트 주기 고정 |
| IDE 측 token leak | High | DPoP PoP binding, 토큰은 키체인(OS-level) 저장 권장 |

---

## 12. 관측·운영

Prometheus metrics:
- `genie_a2a_task_total{direction=in|out, result=success|failure}`
- `genie_a2a_task_duration_seconds`
- `genie_a2a_auth_failures_total{reason}`
- `genie_a2a_escrow_holds_open`
- `genie_a2a_reputation_updates_total`

Structured logs:
- `a2a.card.loaded`, `a2a.card.verify.fail`
- `a2a.task.delegated`, `a2a.task.completed`, `a2a.task.refunded`
- `a2a.acp.flavor.started`, `a2a.acp.call`

---

## 13. CLI 스케치 (CLI-001 후속 PR)

```bash
$ genie a2a publish-card ./agent-card.yaml
[sign] Ed25519 signature ok
[serve] /.well-known/agent-card available at https://example.com

$ genie a2a call urn:genie:agent:legal-review --task code_review --budget 2000
[card] verified (kyc=2, reputation=4.8)
[oauth] token acquired (ttl 900s)
[escrow] hold 2000 GEN
[task] submitted → task_id=xyz
[status] in_progress (elapsed 1m24s)
[result] success → 3 findings
[escrow] released 2000 GEN
[reputation] recorded outcome=0.92

$ genie acp start --flavor vscode
[acp] VS Code flavor ready on stdio
[mcp] exposed 12 tools as skills
```

---

## 14. 추적성

- spec.md REQ-A2A-001..020 → 본 문서 §3 (서명), §4 (PKCE/DPoP), §5 (envelope), §6 (escrow), §7 (ACP), §8 (reputation).
- AC-A2A-001..012 → 본 문서 §9 (TDD 전략).
- ecosystem.md §2.3 (Agent Card) → spec.md §8.1 YAML 예시.
- hermes-learning.md §9 (ACP) → spec.md §7 DD-A2A-06 Flavor enum.
- MCP-001 tool schema → spec.md REQ-A2A-018 `ExposeSkillFromMCP`.

---

## 15. Phase 7 후속 연계

본 SPEC 의 완료 후 다음 SPEC 이 직접 consume 한다:

- **BAZAAR-001** — `Escrow` / `Reputation` 인터페이스를 중앙 집중 구현으로 교체.
- **GATEWAY-001** — Multi-platform (Telegram/Discord/Slack) 브릿지가 ACP flavor 세트에 추가됨.
- **PRIVACY-001** — Agent Card 서명키 KMS/HSM 관리, DPoP key rotation 정책.

Phase 7 의 ROADMAP-ECOSYSTEM.md 가 본 SPEC 의 인터페이스(Escrow/Reputation/ACPAdapter) 를 **계약 수준에서 재사용**하며, 본 SPEC 은 차기 SPEC 이 교체 가능하도록 **인터페이스만 노출** 하는 설계 원칙을 유지.
