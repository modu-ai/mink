---
id: SPEC-GOOSE-CREDPOOL-001
version: 0.3.0
status: implemented
created_at: 2026-04-21
updated_at: 2026-04-27
author: manager-spec
priority: P0
issue_number: null
phase: 1
size: 대(L)
lifecycle: spec-anchored
labels: [credential, llm, oauth, phase-1, zero-knowledge]
---

# SPEC-GOOSE-CREDPOOL-001 — Credential Pool (OAuth/API 통합, 4 Strategy, Rotation)

> **v0.2 Amendment (2026-04-24, v0.3.0 확정)**: SPEC-GOOSE-ARCH-REDESIGN-v0.2 Tier 4 통합 요구.
> 본 SPEC이 관리하는 것은 **credential reference (keyring_id)** 이며, **raw secret value는 절대 agent 메모리에 로드하지 않는다**.
> 실제 secret 조회 및 transport-layer injection은 SPEC-GOOSE-CREDENTIAL-PROXY-001 의 `goose-proxy` 프로세스가 담당.
> CredentialPool은 (a) 어떤 credential을 다음에 쓸 것인가 (selection strategy), (b) 언제 OAuth refresh가 필요한가 (expiry 추적), (c) refresh 요청을 proxy에 위임하는 흐름을 관리한다.
> OS keyring 백엔드: `github.com/zalando/go-keyring` (macOS Keychain / libsecret / Windows Credential Vault) — 본 SPEC 범위 외, SPEC-GOOSE-CREDENTIAL-PROXY-001 담당.
> 구현 시 `.moai/design/goose-runtime-architecture-v0.2.md` §5 Tier 4 참조.
>
> **파급 규칙 (v0.3.0 명문화)**:
> 1. **REQ-004 (persistence)**: "credential 전체를 persist" → "**metadata만 persist** (status, cooldown, usage count, expires_at). access_token / refresh_token / api_key는 persist 대상이 아니며 풀 메모리에도 존재하지 않는다."
> 2. **REQ-014 (redaction)**: `PooledCredential.String()` redaction 의무는 vacuously 충족 — struct에 secret field 자체가 없기 때문(`pooled.go`의 `KeyringID`만 존재). redaction은 "struct-level 무-secret 불변(invariant)"으로 재정의된다.
> 3. **§3.1 rule 8 / §6.4 JSON 스키마**: `access_token` / `refresh_token` / `api_key` 필드는 영속 JSON에서도 **제외**. v0.1 스키마의 raw secret 필드는 삭제된 것으로 간주.
> 4. **`Refresher` 인터페이스**: 본 SPEC은 interface의 **호출 트리거 규칙 (when to refresh)** 만 규정한다. 실제 HTTP refresh 및 token rotation은 SPEC-GOOSE-CREDENTIAL-PROXY-001 + SPEC-GOOSE-ADAPTER-001 (Anthropic PKCE) 이 담당한다. 구현 시그니처는 `Refresh(ctx, cred) error`(Zero-Knowledge: pool에 새 token이 돌아오지 않음)로 수정한다.

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-21 | 초안 작성 (hermes-llm.md §1-3 + ROADMAP v2.0 Phase 1 기반) | manager-spec |
| 0.3.0 | 2026-04-25 | mass audit iteration 1 결함 수정: (a) 프론트매터 `labels` 채움, (b) v0.2 Zero-Knowledge 파급 — REQ-004(persist)를 metadata-only로 재정의, REQ-014 redaction을 struct-level 무secret 제약으로 변환, §3.1 rule 8 / §6.4 JSON 스키마를 metadata-only로 교정, (c) Strategy 명명을 구현 코드 실체(`PriorityStrategy`/`RoundRobinStrategy`/`WeightedStrategy`/`LRUStrategy`)에 정합화, (d) REQ-011 LRU를 `UsageCount` 기반으로 완화, (e) D13 ExpiresAt 필터 누락은 Phase C1(commit 79d92ff)에서 수정됨을 AC-011로 고정, (f) Skeleton 미착수(외부 소스/Refresher 배선/영속 Storage)를 Open Items로 이관. | manager-spec |

---

## 1. 개요 (Overview)

AI.GOOSE Phase 1의 **Multi-LLM Infrastructure 진입점**을 정의한다. Hermes Agent의 `credential_pool.py`(~1.3K LoC) 원형을 Go로 포팅하여, OAuth 토큰과 API key 자격을 하나의 풀에 통합하고, 선택 전략(4종)·자동 갱신·고갈 쿨다운·소프트 임대·외부 디스크 동기화를 제공하는 `internal/llm/credential` 패키지를 구현한다.

본 SPEC이 Plan·Run을 통과한 시점에서 `CredentialPool`은:

- `Select(ctx, strategy)`를 호출하면 즉시 사용 가능한 `PooledCredential` 하나를 반환(또는 풀 전체 소진 시 `ErrExhausted`)하고,
- OAuth 엔트리가 `expires_at` 임박 시 자동 `Refresh()`를 수행(PKCE single-use refresh token rotation 포함)하고,
- HTTP 429/402 등 고갈 응답을 받은 호출자가 `MarkExhaustedAndRotate(id, statusCode)`를 호출하면 해당 엔트리에 1시간 쿨다운을 설정하고 다음 가용 엔트리로 전환하며,
- 동시성 제어를 위한 `AcquireLease(id) / ReleaseLease(id)`(soft lease)를 노출하고,
- `~/.claude/.credentials.json`, `~/.codex/auth.json`, `~/.hermes/auth.json` 파일을 감지하면 해당 엔트리를 풀에 자동 주입하며 변경 시 반영한다.

본 SPEC은 위 동작의 **인터페이스 계약과 관찰 가능 행동**을 규정한다. Provider별 HTTP 호출은 ADAPTER-001, Rate limit 헤더 해석은 RATELIMIT-001, Smart routing은 ROUTER-001로 위임한다.

---

## 2. 배경 (Background)

### 2.1 왜 지금 필요한가

- ROADMAP v2.0 Phase 1 row 06은 CREDPOOL-001을 `CONFIG-001` 이후의 **Multi-LLM 인프라 최초 SPEC**으로 명시한다. 후속 SPEC(ROUTER-001, RATELIMIT-001, ADAPTER-001)이 모두 "풀에서 자격을 1건 취득한다"는 계약 위에 세워져 있다.
- `.moai/project/research/hermes-llm.md` §1-3은 Hermes `credential_pool.py`의 구조(4 strategy + availability filter + refresh + exhaustion cooldown + soft lease + persistent JSON state)를 Go 포팅 매핑(§9)과 함께 제시한다.
- v4.0 포지셔닝 원칙("모든 LLM을 API 또는 OAuth로 연결")의 **유일한 자격 단일 진입점**을 제공한다. Phase 2~6의 모든 LLM 의존 모듈이 본 풀만 바라보게 만드는 것이 본 SPEC의 책임이다.

### 2.2 상속 자산 (패턴 계승)

- **Hermes Agent Python** (`./hermes-agent-main/agent/credential_pool.py` 1.3K LoC): `CredentialPool`, `PooledCredential`, `SelectionStrategy`, `_refresh_entry`, `_available_entries`, `acquire_lease`. Python asyncio → Go `context.Context`+mutex 재해석.
- **Anthropic Claude Code credential storage**: `~/.claude/.credentials.json` 스키마 (OAuth PKCE 2.1 + single-use refresh token).
- **OpenAI Codex storage**: `~/.codex/auth.json` (OAuth with refresh rotation).
- **Nous Portal**: `~/.hermes/auth.json` (Agent Key TTL).
- **MoAI-ADK-Go**: 본 레포에 미러 없음. 패턴 참고 전무.

### 2.3 범위 경계

- **IN**: `PooledCredential` 구조체 + 상태 머신(OK/EXHAUSTED/PENDING), `CredentialPool` 선택·갱신·회전·임대 API, 4 `SelectionStrategy`, OAuth 자동 갱신 훅(실제 HTTP는 ADAPTER-001), 외부 디스크 파일 동기화(`CredentialSource` interface), 영속 JSON 상태 저장소, 동시성 안전성, 고갈 쿨다운 정책.
- **OUT**: 실제 OAuth HTTP 요청(토큰 엔드포인트 POST — ADAPTER-001/Anthropic/OpenAI 구현), PKCE 로그인 플로우 UI(CLI-001), 429 헤더 파싱(RATELIMIT-001), 모델 선택(ROUTER-001), 프롬프트 캐시 마커(PROMPT-CACHE-001), 사용량 가격 계산(후속 메트릭 SPEC).

---

## 3. 스코프 (Scope)

### 3.1 IN SCOPE (본 SPEC이 구현하는 것)

1. `internal/llm/credential/` 패키지: `PooledCredential` (metadata + KeyringID만), `CredentialPool`, `Lease`, `SelectionStrategy` (interface), `Refresher` (interface), `CredentialSource` (interface).
2. 선택 전략 4종 (**구현 코드 실체 기준**) — `PriorityStrategy`(Priority 값 최고 우선, 동일 Priority는 RR 타이브레이커), `RoundRobinStrategy`(카운터 기반 순환), `WeightedStrategy`(Weight 비례 가중치 무작위), `LRUStrategy`(UsageCount 최저 우선).
   - 비고: v0.1.0 SPEC의 `FillFirst` / `Random` / `LeastUsed` 명명은 **구현에 맞춰 각각 `Priority` / `Weighted` / `LRU`로 교체되었다**(v0.3.0). 의미 차이: (a) `FillFirst` → `Priority`: "첫 슬롯 채움"이 아닌 "최고 Priority 값 우선"으로 명확화. (b) `Random` → `Weighted`: 균등 난수가 아닌 Weight-비례 가중치 난수. (c) `LeastUsed` → `LRU`: `active_leases` 다기준이 아닌 `UsageCount` 단일 기준.
3. 가용성 필터링 (Phase C1 commit `79d92ff` 반영): `!leased` AND `status == CredOK` (쿨다운 만료 시 자동 복구) AND (`ExpiresAt.IsZero()` OR `ExpiresAt > now`). **`ExpiresAt.IsZero()`는 API key의 "영구 유효" 표식**이고, 비-zero이고 `now` 이전이면 만료된 OAuth로 간주하여 선택 후보에서 제외한다(REQ-CREDPOOL-001 (b)).
4. OAuth 자동 갱신 훅 (호출 트리거 규칙만 정의): `Refresher` 인터페이스를 통해 외부 refresh 구현을 주입받고, `expires_at - refreshMargin < now`일 때 Select 경로에서 호출한다. 실 refresh HTTP 호출과 token 교체는 본 SPEC 범위 외(§3.2 참조).
5. 고갈 처리: HTTP 429 → 호출자가 `retryAfter` 기간을 전달하면 해당 기간 쿨다운, 그 외 기본값 사용. HTTP 402 → 영구 고갈은 운영자 개입 대기. (본 SPEC에서 402/429 구분 자체는 `statusCode` 인자로 전달만 하고, 현 MVP 구현은 로깅용; 영구 고갈 분기는 REQ-013 참조.)
6. 회전: `MarkExhaustedAndRotate(ctx, id, statusCode, retryAfter)` 호출 시 (a) 해당 엔트리 상태를 `CredExhausted` 전환, (b) `exhaustedUntil = now + retryAfter`, (c) 리스 해제, (d) 다음 가용 엔트리 선택 후 반환.
7. 소프트 임대(`AcquireLease/ReleaseLease`): 엔트리를 명시적으로 점유 상태로 만든다. `AcquireLease`는 `*Lease`를 반환하고 `Lease.Release()`로 반환한다. 복수 임대가 아닌 단일 임대 시맨틱(구현 `lease.go`: 이미 점유 시 재획득 불가). `LRUStrategy`는 `UsageCount` 기반이며 `active_leases` 카운터는 참조하지 않는다(REQ-011 참조).
8. 영속 상태 (**metadata-only, Zero-Knowledge 준수**): `~/.goose/credentials/<provider>.json` 파일에 **metadata만** 저장한다 — `id`, `provider`, `keyring_id` (참조만), `status`, `last_error_at_ms`, `last_error_reset_at_ms`, `exhausted_until_ms`, `usage_count`, `expires_at_ms`, `priority`, `weight`. **`access_token` / `refresh_token` / `api_key` 등 raw secret 필드는 포함하지 않는다**. 실제 secret은 SPEC-GOOSE-CREDENTIAL-PROXY-001의 OS keyring에 저장되고 `keyring_id`로만 참조된다.
9. 외부 소스 동기화 (**본 SPEC은 interface 정의까지만 IN SCOPE**): `CredentialSource` 인터페이스를 정의하고 `Load(ctx) []*PooledCredential` 계약을 명시한다. 3개 provider별 구현(`AnthropicClaudeSource`, `OpenAICodexSource`, `NousSource`)은 skeleton으로 이관됨 — §11 Open Items 참조. 현 MVP는 `DummySource`(테스트용)만 구현되어 있다.
10. 관측성: `Size() (total, available int)` 노출. 상세 `PoolStats{RefreshesTotal, RotationsTotal, ...}`는 §11 Open Items.
11. 에러 타입 (구현 코드 실체): `ErrExhausted`(풀 전체 소진), `ErrNotFound`(id 미존재), `ErrAlreadyReleased`(중복 Release). SPEC-v0.1의 `ErrRefreshFailed` / `ErrInvalidEntry` / `ErrLeaseUnavailable`는 구현에 없으며, 필요 시 SPEC-GOOSE-CREDENTIAL-PROXY-001에서 재정의.
12. `CONFIG-001`의 `LLMConfig.Providers[*].Credentials[]`를 읽어 초기 풀 구성(§11 Open Items — CONFIG 배선은 후속 작업).

### 3.2 OUT OF SCOPE (명시적 제외)

- **OAuth HTTP refresh 실 구현**: `Refresher.Refresh(ctx, entry)` 인터페이스만 정의. Anthropic PKCE·OpenAI token rotation 구현은 ADAPTER-001.
- **PKCE 로그인 UI/브라우저 리다이렉트**: CLI-001 + ADAPTER-001 조합.
- **Rate limit 헤더 파싱**: 호출자가 RATELIMIT-001 결과(statusCode + resetSeconds)를 `MarkExhaustedAndRotate`에 전달. 본 풀은 헤더를 직접 읽지 않음.
- **모델 라우팅/비용 기반 선택**: ROUTER-001 결정을 풀은 소비만 한다(선택 후보 필터링은 provider 단위까지).
- **GitHub/Google Workspace OAuth**: 본 Phase는 LLM provider 3종(Anthropic, OpenAI Codex, Nous)만. 다른 provider는 후속 SPEC.
- **키체인/1Password/Vault 연동**: 본 SPEC은 평문 JSON 파일만. 보안 저장소 통합은 후속.
- **크로스 프로세스 락**: 단일 `goosed` 프로세스 가정. 다중 프로세스 락(flock)은 후속.
- **Hot reload/fsnotify**: `CONFIG-001`의 방침과 동일하게 load-once + 명시적 Refresh.

---

## 4. EARS 요구사항 (Requirements)

> 각 REQ는 TDD RED 단계에서 바로 실패 테스트로 변환 가능한 수준의 구체성을 가진다.

### 4.1 Ubiquitous (시스템 상시 불변)

**REQ-CREDPOOL-001 [Ubiquitous]** — The `CredentialPool` **shall** guarantee that every call to `Select()` returns either (a) a `*PooledCredential` whose `Status == CredOK` AND `!leased` AND (`ExpiresAt.IsZero()` OR `ExpiresAt > now`) — where `ExpiresAt.IsZero()` signifies a permanently-valid credential (API key), and a non-zero `ExpiresAt` that has elapsed is treated as expired OAuth and excluded — or (b) `ErrExhausted` if no such entry exists; **shall not** return an entry in `CredExhausted` state or a leased entry. (v0.3.0: ExpiresAt 필터 규칙을 명문화. 구현은 Phase C1 commit `79d92ff`에서 `pool.go` `available()` 및 `availableCount()` 양쪽에 반영되었으며, `pool_expires_test.go`의 4개 테스트가 이를 검증한다 — AC-CREDPOOL-011 참조.)

**REQ-CREDPOOL-002 [Ubiquitous]** — All mutations to `PooledCredential.status`, `last_error_reset_at`, and `request_count` **shall** go through `CredentialPool` public methods; external callers **shall not** mutate credential fields directly.

**REQ-CREDPOOL-003 [Ubiquitous]** — The `CredentialPool` **shall** be safe for concurrent `Select`/`MarkExhaustedAndRotate`/`AcquireLease` calls from multiple goroutines; internal synchronization uses `sync.Mutex` scoped to the pool instance.

**REQ-CREDPOOL-004 [Ubiquitous]** — Every mutation **shall** be persistable to `~/.goose/credentials/<provider>.json` as **metadata only** (status / cooldown / usage count / expires_at / keyring_id reference — NO raw secrets). When a `Storage` implementation is wired, mutations **shall** atomic-write (temp file + rename) before the public method returns. (v0.3.0 파급: Zero-Knowledge 원칙상 `access_token` / `refresh_token` / `api_key`는 persist 대상에서 제외된다. 실제 Storage 배선은 §11 Open Items.)

### 4.2 Event-Driven (이벤트 기반)

**REQ-CREDPOOL-005 [Event-Driven]** — **When** `Select(ctx, strategy)` is invoked, the pool **shall** (a) snapshot its entries, (b) compute the available subset per §3.1 rule 3, (c) trigger `Refresher.Refresh()` on entries whose `expires_at - refreshMargin < now`, (d) apply the requested `SelectionStrategy` to the available subset, and (e) return the chosen entry within 100ms (excluding refresh RTT).

**REQ-CREDPOOL-006 [Event-Driven]** — **When** `MarkExhaustedAndRotate(id, statusCode, retryAfterSeconds)` is invoked with `statusCode == 429`, the pool **shall** (a) set that entry's `status = EXHAUSTED`, (b) set `last_error_reset_at = now + max(retryAfterSeconds, defaultCooldown)` where `defaultCooldown = 1h`, (c) persist, and (d) select the next available entry per the pool's default strategy or return `ErrExhausted`.

**REQ-CREDPOOL-007 [Event-Driven]** — **When** `MarkExhaustedAndRotate` is invoked with `statusCode == 402`, the pool **shall** set the entry to a permanent exhausted state (`last_error_reset_at = far-future sentinel`) and log at ERROR level; subsequent `Refresh()` on this entry is a no-op until operator intervention.

**REQ-CREDPOOL-008 [Event-Driven]** — **When** an external credential source file (e.g., `~/.claude/.credentials.json`) is modified and `Pool.RefreshSources(ctx)` is called, the pool **shall** reconcile its entries with the file: new entries are appended, deleted entries are removed, mutated entries update their tokens without resetting `request_count`.

**REQ-CREDPOOL-009 [Event-Driven]** — **When** `AcquireLease(id)` is called, the pool **shall** increment `entry.active_leases` and return a `*Lease` whose `Release()` decrements the counter; if `id` is not found, returns `ErrInvalidEntry`.

### 4.3 State-Driven (상태 기반)

**REQ-CREDPOOL-010 [State-Driven]** — **While** an OAuth entry's `expires_at - refreshMargin < now AND expires_at > now`, the next `Select()` that would return this entry **shall** first invoke `Refresher.Refresh(ctx, entry)`; upon refresh success, update `access_token`, `refresh_token`(if rotated), and `expires_at`; upon refresh failure, mark the entry as `EXHAUSTED` with a 5-minute cooldown and try the next entry.

**REQ-CREDPOOL-011 [State-Driven]** — **While** the pool's strategy is `LRUStrategy` (v0.1 `LeastUsed`의 v0.3.0 재명명), the `Select()` ordering **shall** prefer entries with the **lowest `UsageCount`** first. 동일 `UsageCount`일 경우 첫 매칭 엔트리를 반환한다. (v0.3.0 완화: 구현 `strategy.go:L56-L67`이 `UsageCount` 단일 기준을 사용한다. `active_leases` + `request_count` 다기준 ordering은 struct에 `ActiveLeases` 필드가 없어 불가능하며, 도입 시 §11 Open Items로 추적.)

**REQ-CREDPOOL-012 [State-Driven]** — **While** all entries are in `EXHAUSTED` state with future `last_error_reset_at`, `Select()` **shall** return `ErrExhausted` with `RetryAfter` populated to the earliest reset time.

### 4.4 Unwanted Behavior (방지)

**REQ-CREDPOOL-013 [Unwanted]** — **If** `Refresher.Refresh` returns an error for the same entry **3 times consecutively**, the pool **shall** mark the entry as permanently exhausted (`last_error_code = -1`) and emit a WARN log; **shall not** retry this entry in subsequent `Select()` calls until operator intervention (e.g., `Pool.Reset(id)`).

**REQ-CREDPOOL-014 [Unwanted]** — The pool **shall not** hold raw secret values (`access_token`, `refresh_token`, `api_key`) as fields of `PooledCredential`; this invariant **shall** be enforced by a reflection-based test (e.g., `TestPooledCredential_NoSecretFields` at `pool_test.go:L328-L347`). 이로써 log 경로에서의 secret 유출은 vacuously 불가능해지고, provider-facing HTTP 호출 경로의 redaction은 SPEC-GOOSE-CREDENTIAL-PROXY-001이 담당한다. (v0.3.0 재정의: v0.1의 "String() redaction" 의무를 "struct-level 무-secret 불변"으로 격상.)

**REQ-CREDPOOL-015 [Unwanted]** — The pool **shall not** block on refresh for entries other than the one currently being considered; specifically, `Select()` **shall not** preemptively refresh all entries in the background.

**REQ-CREDPOOL-016 [Unwanted]** — **If** two concurrent `Select()` calls race and both observe the same entry as the best candidate, both **shall** succeed (returning the same entry pointer); hard-lease enforcement is out of scope — callers rely on provider rate limit feedback via `MarkExhaustedAndRotate`.

### 4.5 Optional (선택적)

**REQ-CREDPOOL-017 [Optional]** — **Where** `CredentialSource.Sync(ctx)` is invoked with a non-nil `context.Context`, the source **shall** respect `ctx.Done()` and abort partial file reads within 500ms.

**REQ-CREDPOOL-018 [Optional]** — **Where** `PoolOptions.RefreshMargin` is configured, OAuth entries **shall** be refreshed when `expires_at - RefreshMargin < now`; default is 5 minutes.

---

## 5. 수용 기준 (Acceptance Criteria)

> 각 AC는 Given-When-Then. `*_test.go`로 변환 가능한 수준.

**AC-CREDPOOL-001 — 기본 선택 (PriorityStrategy, v0.3.0 재명명)**
- **Given** 3개 엔트리 `[{id:"a", Priority:10}, {id:"b", Priority:5}, {id:"c", Priority:1}]`, 모두 `CredOK`
- **When** `New(src, NewPriorityStrategy())` 후 `Select(ctx)` 호출
- **Then** 반환된 엔트리의 `ID == "a"` (최고 Priority), `UsageCount == 1`, `leased == true`. (v0.3.0: v0.1의 `FillFirst`는 "첫 슬롯 채움"으로 오해 소지가 있어 `Priority`로 재명명되었고, 구현 `strategy.go:L125-L166` `PriorityStrategy`가 이 의미론을 실현한다.)

**AC-CREDPOOL-002 — RoundRobinStrategy**
- **Given** 3개 엔트리 `[a, b, c]` 모두 `CredOK`, 전략 `NewRoundRobinStrategy()`
- **When** 4회 `Select` + 매 호출 후 `Release`
- **Then** 반환 순서는 `[a, b, c, a]` (wrap-around). 내부 `counter atomic.Uint64`가 단조 증가한다(`strategy.go:L25-L44`).

**AC-CREDPOOL-003 — OAuth refresh trigger (호출 규칙만, v0.3.0 Zero-Knowledge 조정)**
- **Given** 엔트리 `e1`의 `ExpiresAt = now + 2분`, `RefreshMargin = 5분`. 스텁 `Refresher.Refresh(ctx, cred) error`가 호출 횟수를 기록하고 `nil`을 반환
- **When** `Select(ctx)` 호출
- **Then** `Refresher.Refresh`가 정확히 1회 호출됨 (`ExpiresAt - RefreshMargin < now` 트리거). **실제 token rotation은 본 SPEC 범위 외** — proxy가 keyring의 값을 갱신하고 풀은 `Reload(ctx)` 또는 외부 신호로 `ExpiresAt`만 재-load한다 (§11 Open Items). v0.1의 "새 access_token이 entry에 주입" 시나리오는 Zero-Knowledge 원칙상 불가능하므로 삭제.

**AC-CREDPOOL-004 — 429 고갈 후 회전 (구현 완료, `f1a428c`/`bea5df1`)**
- **Given** 2개 엔트리 `[a, b]` 모두 `CredOK`, `a`가 선택된 후 외부 호출이 HTTP 429(retryAfter=60s)를 반환
- **When** `MarkExhaustedAndRotate(ctx, "a", 429, 60s)` 호출
- **Then** (1) `a.Status == CredExhausted`, `a.exhaustedUntil ≈ now + 60s`, `a.leased == false`, (2) 반환값은 `b`이고 `b.Status == CredOK`, `b.leased == true`, `b.UsageCount++`. (3) 영속 persist 검증은 `Storage` 배선이 완료된 이후 — §11 Open Items.
- **검증 테스트**: `pool_extend_test.go` 의 `TestMarkExhaustedAndRotate_*`.

**AC-CREDPOOL-005 — 풀 전체 소진**
- **Given** 모든 엔트리가 `EXHAUSTED`, `last_error_reset_at = now + 30s`
- **When** `Select`
- **Then** 반환은 `nil, ErrExhausted`이고, `ErrExhausted.RetryAfter == 30s`

**AC-CREDPOOL-006 — 재기동 시 영속 상태 복원 (Open Item, §11 참조)**
- **Given** 엔트리 `a`가 `CredExhausted`, `exhaustedUntil = now + 5분`으로 metadata-only 영속 파일에 저장된 상태에서 풀을 파괴 후 새 풀 생성
- **When** 새 풀이 `load(ctx)`
- **Then** `a.Status == CredExhausted`, `exhaustedUntil` 복원, `Select()` 시 `a`는 available 필터에서 제외됨
- **상태**: 본 SPEC에서 계약만 정의. `Storage` interface 배선 및 atomic write 구현은 Open Item (§11). 검증 가능 시점은 Storage 구현 완료 후.

**AC-CREDPOOL-007 — 외부 파일 소스 동기화 (Open Item, §11 참조)**
- **Given** `AnthropicClaudeSource.Load(ctx)`가 `[claude-1]` 반환 후 두 번째 호출 시 `[claude-1, claude-2]` 반환하도록 stub
- **When** `Pool.Reload(ctx)` (현재 구현 API 명칭)
- **Then** `pool.Size().total == 2`, `claude-2`가 Select 후보에 포함됨. `claude-1`의 `UsageCount` / `leased` / `exhaustedUntil`은 보존됨 (`pool.go:L73-L84`의 기존 상태 병합 로직).
- **상태**: `Reload` 기존 상태 병합은 구현 완료. 그러나 3개 provider 구현(`AnthropicClaudeSource` / `OpenAICodexSource` / `NousSource`)은 아직 skeleton — §11 Open Items. 현 MVP는 `DummySource`로만 검증.

**AC-CREDPOOL-008 — LRUStrategy UsageCount 우선 (v0.3.0 완화)**
- **Given** 엔트리 `[a, b]` 모두 `CredOK`, 전략 `NewLRUStrategy()`. `a.UsageCount = 3`, `b.UsageCount = 0` (예: `a`를 3회 먼저 Select+Release)
- **When** 네 번째 `Select(ctx)`
- **Then** 반환은 `b` (lowest `UsageCount` 우선, `strategy.go:L56-L67`). (v0.3.0: v0.1은 `active_leases`를 1차 키로, `request_count`를 tiebreaker로 기대했으나 struct에 `ActiveLeases` 필드가 없으므로 `UsageCount` 단일 기준으로 완화됨.)

**AC-CREDPOOL-009 — Refresh 3회 연속 실패 → 영구 고갈 (Open Item, §11 참조)**
- **Given** 엔트리 `e1`의 `ExpiresAt`이 `RefreshMargin` 임박, `Refresher.Refresh(ctx, cred)`가 항상 에러 반환
- **When** 3회 `Select` 시도
- **Then** 3회 모두 내부 refresh 시도가 발생하고, 3회 이후 `e1`가 영구 고갈 상태로 전환되어 후속 `Select`에서 후보 제외 (운영자 개입 대기).
- **상태**: `refreshFailCount` 카운터와 `Refresher` 배선이 Select 경로에 연결되어야 검증 가능. 현 MVP는 `Refresher` interface만 존재하고 Select 경로에 연결되지 않음 — §11 Open Items.

**AC-CREDPOOL-010 — 구조체 수준 무-secret 불변 (v0.3.0 재정의)**
- **Given** `PooledCredential` struct (`pooled.go`) 정의
- **When** `TestPooledCredential_NoSecretFields`(`pool_test.go:L328-L347`)를 실행
- **Then** reflect로 모든 exported/unexported field를 순회하여 `access_token` / `refresh_token` / `api_key` / `secret` / `password` 유사 이름의 필드가 **단 하나도 존재하지 않음**을 검증. `KeyringID`만 secret reference로 허용.

**AC-CREDPOOL-011 — 만료 OAuth 토큰 필터링 (Phase C1 commit `79d92ff` 반영, v0.3.0 신설)**
- **Given** 풀에 3개 엔트리 `[{oauth-expired, ExpiresAt: now-1h, Status: CredOK}, {oauth-valid-1, ExpiresAt: now+1h, Status: CredOK}, {oauth-valid-2, ExpiresAt: now+2h, Status: CredOK}]`
- **When** `Select()` 10회 반복 호출 (매 호출 후 `Release`)
- **Then** (1) `oauth-expired`는 한 번도 선택되지 않음 — REQ-CREDPOOL-001 (b) 준수, (2) `oauth-valid-1` 및 `oauth-valid-2`는 적어도 한 번씩 선택됨, (3) `ExpiresAt.IsZero()`인 엔트리는 영구 유효로 취급되어 `Size()` 의 `available` 카운트에 포함됨 (`TestAvailable_ZeroExpiresAt_IncludedAsNonExpiring`), (4) 미래 만료 엔트리는 정상 선택 및 `available` 집계 (`TestAvailable_ExpiresAtInFuture_Included`), (5) 만료 엔트리는 `Size()` 의 `available` 에서 제외됨 (`TestAvailable_ExpiredTokenExcludedFromSize`).
- **검증 테스트 (현재 존재)**: `internal/llm/credential/pool_expires_test.go` 의 4개 테스트:
  1. `TestAvailable_FiltersExpiredOAuthTokens`
  2. `TestAvailable_ZeroExpiresAt_IncludedAsNonExpiring`
  3. `TestAvailable_ExpiresAtInFuture_Included`
  4. `TestAvailable_ExpiredTokenExcludedFromSize`

---

## 6. 기술적 접근 (Technical Approach)

### 6.1 제안 패키지 레이아웃

```
internal/llm/credential/
├── pool.go                  # CredentialPool + Select + MarkExhaustedAndRotate
├── pool_test.go             # Unit tests (20+ cases)
├── credential.go            # PooledCredential + Status + String(redacted)
├── strategy.go              # SelectionStrategy enum + 4 구현
├── lease.go                 # Lease + AcquireLease/ReleaseLease
├── oauth.go                 # Refresher interface + refresh margin policy
├── storage.go               # Storage interface + JSON file backend (atomic write)
├── source.go                # CredentialSource interface + 3 구현
├── source_anthropic.go      # ~/.claude/.credentials.json reader
├── source_openai.go         # ~/.codex/auth.json reader
├── source_nous.go           # ~/.hermes/auth.json reader
├── errors.go                # ErrExhausted + ErrRefreshFailed + ErrInvalidEntry
└── stats.go                 # PoolStats
```

### 6.2 핵심 타입 (Go 시그니처 제안)

```go
// internal/llm/credential/pooled.go (v0.3.0: Zero-Knowledge 실체 반영)

// CredStatus는 풀 엔트리의 현재 상태.
type CredStatus int

const (
    CredOK        CredStatus = iota // 즉시 사용 가능
    CredExhausted                   // 고갈 (exhaustedUntil까지 쿨다운)
    // 주: v0.1의 "Pending" 상태는 구현에서 생략되었다. Refresh는 Select 트리거로만 발생.
)

// PooledCredential은 단일 자격 엔트리의 reference + metadata.
// **raw secret은 포함하지 않는다** (REQ-CREDPOOL-014 struct-level 불변).
// 필드는 Pool의 공개 메서드를 통해서만 mutate됨 (REQ-CREDPOOL-002).
type PooledCredential struct {
    ID             string        // 고유 식별자
    Provider       string        // "anthropic" | "openai" | "nous" | ...
    KeyringID      string        // OS keyring 참조 키 (raw secret 아님)
    Status         CredStatus    // CredOK | CredExhausted
    ExpiresAt      time.Time     // OAuth 만료. API key는 zero value (영구 유효)
    LastErrorAt    time.Time     // 마지막 에러 발생 시각
    LastErrorReset time.Time     // 에러 상태 초기화 시각
    UsageCount     uint64        // 누적 선택 횟수 (LRU 기준)
    Priority       int           // PriorityStrategy용 우선순위 (값이 클수록 우선)
    Weight         int           // WeightedStrategy용 가중치

    exhaustedUntil time.Time     // (내부) MarkExhausted 쿨다운 만료
    leased         bool          // (내부) 현재 리스 중 여부
}

// v0.3.0: `RuntimeAPIKey()` / `String()` redaction 메서드는 존재하지 않는다.
// Zero-Knowledge 원칙상 secret이 struct에 없으므로 redaction이 vacuously 성립.
// 검증: `TestPooledCredential_NoSecretFields` (pool_test.go:L328-L347) — reflect로 강제.
```

```go
// internal/llm/credential/strategy.go (v0.3.0: interface 기반으로 변경됨)

// SelectionStrategy는 사용 가능한 크레덴셜 중 하나를 선택하는 전략 interface이다.
type SelectionStrategy interface {
    Select(available []*PooledCredential) (*PooledCredential, error)
    Name() string
}

// 4 구현 (strategy.go, v0.3.0 명칭):
//   - NewPriorityStrategy()    : 최고 Priority, 동일 Priority는 RR 타이브레이커   (v0.1 "FillFirst")
//   - NewRoundRobinStrategy() : atomic.Uint64 카운터 순환                      (v0.1 "RoundRobin")
//   - NewWeightedStrategy()   : Weight 비례 가중치 무작위; 모두 0이면 RR 폴백    (v0.1 "Random")
//   - NewLRUStrategy()        : 최저 UsageCount                             (v0.1 "LeastUsed")
```

```go
// internal/llm/credential/refresher.go (v0.3.0: Zero-Knowledge 시그니처)

// Refresher는 외부 refresh 구현 주입점.
// 풀에 raw token이 돌아오지 않고, proxy/adapter가 keyring을 직접 갱신한 뒤
// 호출자가 ExpiresAt을 재-load하는 모델.
type Refresher interface {
    // Refresh는 keyring_id가 가리키는 secret의 갱신을 외부에 위임한다.
    // 성공 시 nil, 실패 시 error. 새 AccessToken은 반환하지 않는다 (Zero-Knowledge).
    Refresh(ctx context.Context, cred *PooledCredential) error
}

// v0.3.0: v0.1의 `RefreshResult{AccessToken, RefreshToken, ExpiresAt, Extra}` 반환 타입은
// 폐기되었다. 풀은 새 token을 받지 않으며, `ExpiresAt` 갱신은 Source.Load()의 재-load
// 또는 별도 메타데이터 hook으로 이루어진다 (§11 Open Items).

// RefreshMargin: ExpiresAt - margin < now → refresh trigger. 기본값 5분 (REQ-CREDPOOL-018).
// 주: 현 MVP는 Refresher를 Select 경로에 연결하지 않음 (§11 Open Items).
```

```go
// internal/llm/credential/pool.go (v0.3.0: 구현 실체 반영)

// Option은 CredentialPool 생성 functional 옵션.
type Option func(*CredentialPool)

// CredentialPool은 크레덴셜 reference + metadata 오케스트레이터.
// raw secret은 보유하지 않는다 (Zero-Knowledge).
type CredentialPool struct {
    mu       sync.RWMutex
    source   CredentialSource
    strategy SelectionStrategy // interface (구현: Priority/RoundRobin/Weighted/LRU 중 1)
    creds    map[string]*PooledCredential  // ID → entry
    order    []string                      // 로드 순서 보존
}

// New는 주어진 Source와 Strategy로 풀을 생성. Source.Load()로 초기 엔트리 구성.
// @MX:ANCHOR (pool.go:L39-L56): 풀 생성 진입점.
func New(source CredentialSource, strategy SelectionStrategy, opts ...Option) (*CredentialPool, error)

// Select는 가용 엔트리 1건을 선택하고 리스를 획득.
// 가용성 필터: !leased && Status==CredOK (쿨다운 만료 자동 복구) && (ExpiresAt.IsZero() || ExpiresAt > now).
// @MX:ANCHOR (pool.go:L156-L185): 핫패스.
func (p *CredentialPool) Select(ctx context.Context) (*PooledCredential, error)

// Release는 리스를 반환.
func (p *CredentialPool) Release(cred *PooledCredential) error

// MarkExhausted는 쿨다운 기간 동안 exhausted 표시.
func (p *CredentialPool) MarkExhausted(cred *PooledCredential, cooldownDur time.Duration) error

// MarkError는 에러 발생 시각을 기록 (exhausted로 전환하지 않음).
func (p *CredentialPool) MarkError(cred *PooledCredential, _ error) error

// MarkExhaustedAndRotate은 HTTP 429/402 수신 후 회전.
// @MX:ANCHOR (pool.go:L268-L315): 429/402 회전 핵심.
func (p *CredentialPool) MarkExhaustedAndRotate(
    ctx context.Context, id string, statusCode int, retryAfter time.Duration,
) (*PooledCredential, error)

// AcquireLease / ReleaseLease (lease.go)은 명시적 점유/반환 API.
func (p *CredentialPool) AcquireLease(id string) (*Lease, error)

// Size는 total / available 스냅샷.
func (p *CredentialPool) Size() (total, available int)

// Reload는 Source.Load()를 재호출하여 풀을 재초기화 (기존 리스/에러 상태 보존).
func (p *CredentialPool) Reload(ctx context.Context) error

// 주: v0.1의 `PoolOptions` 구조체, `RefreshSources`, `Stats() PoolStats`, `Reset(id)`는
// v0.3.0에서 §11 Open Items로 이관됨. Functional Option + Source.Load 모델이 실체.
```

```go
// internal/llm/credential/source.go

// CredentialSource는 외부 파일/키체인에서 엔트리를 읽어오는 인터페이스.
type CredentialSource interface {
    Provider() string
    // Load는 현재 파일 상태를 반환. mtime 변경 시 새 엔트리 목록.
    Load(ctx context.Context) ([]*PooledCredential, error)
    // 파일 mtime을 반환하여 변경 감지.
    ModTime(ctx context.Context) (time.Time, error)
}
```

```go
// internal/llm/credential/errors.go

var (
    ErrExhausted       = errors.New("credential pool exhausted")
    ErrRefreshFailed   = errors.New("credential refresh failed")
    ErrInvalidEntry    = errors.New("invalid credential entry id")
    ErrLeaseUnavailable = errors.New("lease unavailable")
)

type ExhaustedError struct {
    Provider   string
    RetryAfter time.Duration // 가장 빠른 reset
}

func (e *ExhaustedError) Error() string
func (e *ExhaustedError) Is(target error) bool // match ErrExhausted
```

### 6.3 선택 알고리즘 의사코드 (v0.3.0)

현 MVP (Phase C1 `79d92ff` 반영):

```
Select(ctx):
  ctx.Done() 체크 → ctx.Err()
  Lock()
  defer Unlock()

  // 1. 가용성 필터 (REQ-CREDPOOL-001)
  available = entries.filter(e =>
    !e.leased AND
    e.Status == CredOK  // 쿨다운 만료 시 available() 내부에서 자동 복구
    AND (e.ExpiresAt.IsZero() OR e.ExpiresAt > now))  // Phase C1 추가

  if available.empty():
    return nil, ErrExhausted

  // 2. 전략 적용 (interface dispatch)
  chosen, err = strategy.Select(available)
  if err: return nil, ErrExhausted

  chosen.leased = true
  chosen.UsageCount++
  return chosen, nil
```

v0.3.0 대상 확장 흐름 (§11 Open Items — 미구현):

```
// Refresher 배선 후 Select 경로에 삽입 예정
Select_with_refresh(ctx):
  ...1. 가용성 필터...

  // 2. OAuth refresh trigger: expires_at - margin < now
  for e in available.filter(e.ExpiresAt != zero):
    if e.ExpiresAt - RefreshMargin < now AND refresher != nil:
      Unlock()
      err = refresher.Refresh(ctx, e)   // Zero-Knowledge: pool에 token 안 돌아옴
      Lock()
      if err:
        e.refreshFailCount++
        if e.refreshFailCount >= 3:
          e.Status = CredExhausted; 영구 고갈 표시  // REQ-013
        else:
          e.Status = CredExhausted; e.exhaustedUntil = now + 5분
        persistMetadata(e)
        continue
      e.refreshFailCount = 0
      // ExpiresAt은 외부(proxy/adapter)가 갱신 후 Source.Load()에서 재-load

  ...3. 재평가 / 4. 전략 적용 / return...
```

### 6.4 영속 레이아웃 (metadata-only, v0.3.0 Zero-Knowledge)

`~/.goose/credentials/<provider>.json` 예시 (**비밀값 없음**):

```json
{
  "provider": "anthropic",
  "version": 2,
  "entries": [
    {
      "id": "a1b2c3",
      "keyring_id": "anthropic-work-claude",
      "status": "ok",
      "last_error_at_ms": 0,
      "last_error_reset_at_ms": 0,
      "exhausted_until_ms": 0,
      "expires_at_ms": 1746003600000,
      "usage_count": 42,
      "priority": 10,
      "weight": 1
    }
  ]
}
```

**핵심 제약 (v0.3.0)**:
- `access_token` / `refresh_token` / `api_key` / `id_token` / `agent_key` 등 모든 raw secret 필드는 **본 파일에 절대 포함되지 않는다**.
- 실제 secret은 SPEC-GOOSE-CREDENTIAL-PROXY-001 담당의 OS keyring (macOS Keychain / libsecret / Windows Credential Vault)에 저장된다.
- 풀은 `keyring_id`를 통해 proxy에 "이 키로 요청해달라"는 reference 전달만 수행한다.
- `version: 2`는 v0.1의 schema와 **호환되지 않음** (v0.1의 raw-secret 포함 버전은 폐기).
- atomic write (tmp + rename) 패턴은 유지되며 파일 권한은 0600.

**본 §6.4의 구현**: 현 MVP에 미착수 — §11 Open Items.

### 6.5 외부 소스 스키마 (read-only reference, v0.3.0)

> **v0.3.0 메모**: 아래 스키마는 외부 CLI(Claude Code / Codex / Hermes)가 관리하는 파일의 스키마이며, **풀은 이를 읽어 `keyring_id`로 매핑만 하고 raw token을 메모리에 보존하지 않는다**. 스키마 자체는 외부 벤더 스펙에 종속. 파일 내 `accessToken` 값은 SPEC-GOOSE-CREDENTIAL-PROXY-001이 keyring으로 이전하거나 transport-layer에서만 사용한다.
>
> **구현 상태**: 3개 Source 구현은 모두 skeleton 이관 (§11 Open Items).

**AnthropicClaudeSource** — `~/.claude/.credentials.json`:
```json
{
  "claudeAiOauth": {
    "accessToken": "...",
    "refreshToken": "...",
    "expiresAt": 1746003600000
  }
}
```

**OpenAICodexSource** — `~/.codex/auth.json`:
```json
{
  "OPENAI_API_KEY": null,
  "tokens": {
    "access_token": "...",
    "refresh_token": "...",
    "id_token": "..."
  },
  "last_refresh": "2026-04-20T..."
}
```

**NousSource** — `~/.hermes/auth.json`:
```json
{
  "agent_key": "...",
  "expires_at": "2026-05-01T..."
}
```

### 6.6 TDD 진입 순서 (RED → GREEN → REFACTOR)

1. **RED #1**: `TestPool_Select_FillFirst_ReturnsHighestPriority` — AC-CREDPOOL-001.
2. **RED #2**: `TestPool_Select_RoundRobin_WrapsAround` — AC-CREDPOOL-002.
3. **RED #3**: `TestPool_OAuthRefreshAutoOnExpiring` — AC-CREDPOOL-003.
4. **RED #4**: `TestPool_MarkExhaustedAndRotate_OnHTTP429` — AC-CREDPOOL-004.
5. **RED #5**: `TestPool_AllExhausted_ReturnsErrExhausted` — AC-CREDPOOL-005.
6. **RED #6**: `TestPool_PersistsStateAcrossRecreation` — AC-CREDPOOL-006.
7. **RED #7**: `TestSources_AnthropicClaudeFileSync` — AC-CREDPOOL-007.
8. **RED #8**: `TestPool_LeaseCounterAffectsLeastUsed` — AC-CREDPOOL-008.
9. **RED #9**: `TestPool_Refresh3FailuresMarksPermanentExhausted` — AC-CREDPOOL-009.
10. **RED #10**: `TestPool_LogsRedactSecrets` — AC-CREDPOOL-010.
11. **GREEN**: 최소 구현.
12. **REFACTOR**: strategy를 별도 파일로 분리, Storage atomic write 추상화 정리.

### 6.7 TRUST 5 매핑

| 차원 | 본 SPEC의 달성 방법 |
|-----|-----------------|
| **T**ested | 85%+ 커버리지, race detector 통과, 4 strategy × 3 상태 × 3 refresh 결과 = 36 cross-product의 주요 16 조합 테스트 |
| **R**eadable | 패키지 단일 책임, `Status`/`AuthType` enum, 공개 메서드 10 이하 |
| **U**nified | `go fmt` + `golangci-lint`, 모든 mutation은 Pool 메서드 경유, atomic write 패턴 통일 |
| **S**ecured | 비밀값 redaction 의무(REQ-CREDPOOL-014), 영속 파일은 비밀 미포함, 0600 권한 강제 |
| **T**rackable | 모든 상태 전환에 구조화 로그(`{entry_id, from_status, to_status, reason}`), `PoolStats`로 관측성 |

---

## 7. 의존성 (Dependencies)

| 타입 | 대상 | 설명 |
|-----|------|------|
| 선행 SPEC | SPEC-GOOSE-CORE-001 | zap logger, `GOOSE_HOME` 의미 상속 |
| 선행 SPEC | SPEC-GOOSE-CONFIG-001 | `LLMConfig.Providers[*]`로부터 초기 엔트리 구성 |
| 후속 SPEC | SPEC-GOOSE-ROUTER-001 | 풀에서 provider 단위 자격 선택 |
| 후속 SPEC | SPEC-GOOSE-RATELIMIT-001 | 429 status + Retry-After 헤더를 `MarkExhaustedAndRotate`로 전달 |
| 후속 SPEC | SPEC-GOOSE-ADAPTER-001 | `Refresher` 인터페이스의 provider별 구현 주입 |
| 외부 | Go 1.22+ | generics, `time.Time` 등 |
| 외부 | `go.uber.org/zap` v1.27+ | 구조화 로깅 |
| 외부 | stdlib `encoding/json` | 영속 JSON |
| 외부 | `github.com/stretchr/testify` v1.9+ | 테스트 |

**라이브러리 결정 (본 SPEC에서 채택만, 구현은 후속)**:
- 현 SPEC은 외부 OAuth 라이브러리 **미사용**(Refresher interface가 ADAPTER-001로 위임).
- ADAPTER-001이 `anthropic-sdk-go`(공식), `go-openai`(OpenAI-compat, xAI/DeepSeek/Groq 재사용) 등을 도입 예정.

---

## 8. 리스크 & 완화 (Risks & Mitigations)

| # | 리스크 | 가능성 | 영향 | 완화 |
|---|------|------|-----|------|
| R1 | 동시 `Select`/`MarkExhausted` race | 고 | 고 | `sync.Mutex` 스코프를 공개 메서드 전체로 확장, `go test -race` 필수 |
| R2 | Refresh 중 Unlock 구간에서 엔트리 mutate된 상태 불일치 | 중 | 중 | Refresh 호출 전 엔트리 ID 기준으로 post-refresh lookup, 발견되지 않으면 결과 drop |
| R3 | `~/.claude/.credentials.json` 스키마가 Anthropic 측에서 변경 | 중 | 중 | 스키마 버전 필드 검사, 미지 버전은 WARN 로그 + skip(기존 풀 유지) |
| R4 | 영속 JSON이 disk full에서 실패 | 낮 | 중 | atomic write 실패 시 풀 상태는 메모리에 남고, 다음 mutation에서 재시도 |
| R5 | OAuth token rotation에서 이전 refresh_token이 이미 무효화된 상태 | 고 | 고 | 3회 연속 실패 시 permanent exhausted (REQ-CREDPOOL-013), 운영자 `Pool.Reset` 또는 재로그인 요구 |
| R6 | 다중 `goosed` 인스턴스가 동일 영속 파일에 쓰면 충돌 | 중 | 중 | Phase 1 범위 외(단일 인스턴스 가정). 후속 SPEC에서 flock 또는 SQLite로 이관 |
| R7 | provider별 Retry-After 헤더 부재 시 기본 1시간이 과도 | 중 | 낮 | `PoolOptions.DefaultCooldown` 조정 가능, 호출자(RATELIMIT-001)가 더 정확한 값 제공 가능 |

---

## 9. 참고 (References)

### 9.1 프로젝트 문서 (본 SPEC 근거)

- `.moai/project/research/hermes-llm.md` §1 Credential Pool 아키텍처, §2 Provider 매트릭스, §3 Python 인터페이스, §9 Go 포팅 매핑, §10 SPEC 도출, §12 고리스크
- `.moai/specs/ROADMAP.md` §4 Phase 1 row 06, §13 핵심 설계 원칙 3
- `.moai/specs/SPEC-GOOSE-CONFIG-001/spec.md` §6.2 `LLMConfig.Providers` 스키마
- `.moai/project/tech.md` §3.2 Go LLM 스택

### 9.2 외부 참조

- **Hermes Agent Python**: `./hermes-agent-main/agent/credential_pool.py` — 원형 참고
- **OAuth 2.1 PKCE RFC**: https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1
- **Anthropic OAuth (Claude Code)**: https://docs.anthropic.com/claude-code
- **Go sync package**: https://pkg.go.dev/sync

### 9.3 부속 문서

- `./research.md` — Hermes `credential_pool.py` 구조 상세 분석 + 테스트 전략 + Python→Go 의미 보존 이슈

---

## Exclusions (What NOT to Build)

> **필수 섹션**: SPEC 범위 누수 방지.

- 본 SPEC은 **OAuth HTTP refresh의 실 구현을 포함하지 않는다**. `Refresher` interface만. ADAPTER-001이 각 provider별 HTTP 엔드포인트 호출을 구현.
- 본 SPEC은 **PKCE 로그인 브라우저 플로우를 구현하지 않는다**. CLI-001 + ADAPTER-001.
- 본 SPEC은 **Rate limit 헤더(x-ratelimit-*) 파싱을 포함하지 않는다**. 호출자가 RATELIMIT-001에서 파싱한 결과를 `MarkExhaustedAndRotate(retryAfter)`로 전달.
- 본 SPEC은 **모델 선택/비용 최적화를 포함하지 않는다**. ROUTER-001.
- 본 SPEC은 **프롬프트 캐시 마커 주입을 포함하지 않는다**. PROMPT-CACHE-001.
- 본 SPEC은 **사용량 가격 계산(토큰 × $/Mtoken)을 포함하지 않는다**. 후속 메트릭 SPEC.
- 본 SPEC은 **크로스 프로세스 flock을 구현하지 않는다**. 단일 `goosed` 프로세스 가정.
- 본 SPEC은 **키체인/1Password/Vault 통합을 포함하지 않는다**. 평문 JSON 파일 + env vars만.
- 본 SPEC은 **GitHub/Google Workspace OAuth를 지원하지 않는다**. LLM provider 3종만(Anthropic, OpenAI Codex, Nous).
- 본 SPEC은 **hot reload / fsnotify 감시를 포함하지 않는다**. 명시적 `Pool.RefreshSources(ctx)` 호출만.

---

## 11. Open Items (v0.3.0 신설, Run phase / 후속 SPEC 이관)

> mass audit iteration 1에서 식별된 SPEC vs 구현 gap 중, 본 SPEC의 Plan 단계에 포함하지 않고 Run phase 또는 후속 SPEC으로 이관되는 항목.

### 11.1 본 SPEC Run phase로 이관 (구현 미착수)

| ID | 항목 | 관련 REQ/AC | 상태 |
|----|-----|-----------|------|
| OI-01 | `Storage` interface + atomic JSON write backend (`~/.goose/credentials/<provider>.json`, metadata-only, 0600) | REQ-004, AC-006 | skeleton 미착수 |
| OI-02 | `Refresher` interface를 Select 경로에 배선 (ExpiresAt − RefreshMargin < now 트리거) | REQ-010, REQ-018, AC-003 | interface만 존재 |
| OI-03 | `refreshFailCount` 카운터 + 3회 연속 실패 시 영구 고갈 전환 | REQ-013, AC-009 | 구현 부재 |
| OI-04 | 402 vs 429 status code 분기 로직 (402 → 영구 고갈, 429 → 쿨다운) | REQ-006, REQ-007 | statusCode 인자는 수신하나 분기 없음 |
| OI-05 | 3개 provider `CredentialSource` 구현: `AnthropicClaudeSource`(`~/.claude/.credentials.json`) / `OpenAICodexSource`(`~/.codex/auth.json`) / `NousSource`(`~/.hermes/auth.json`) | §3.1 rule 9, §6.5, AC-007 | `DummySource`만 구현 |
| OI-06 | `CONFIG-001` 의 `LLMConfig.Providers[*].Credentials[]` 초기 풀 구성 배선 | §3.1 rule 12 | 미배선 |
| OI-07 | `PoolStats{Total, Available, Exhausted, RefreshesTotal, RotationsTotal}` 관측성 확장 (현재는 `Size()` 만 존재) | §3.1 rule 10 | 축소 구현 |
| OI-08 | `Pool.Reset(id)` 운영자 수동 영구 고갈 해제 API | §3.1 rule 참고 | 미구현 |

### 11.2 후속 SPEC으로 위임

| ID | 항목 | 이관 대상 |
|----|-----|---------|
| OI-09 | OS keyring 백엔드 통합 (`github.com/zalando/go-keyring`) | SPEC-GOOSE-CREDENTIAL-PROXY-001 |
| OI-10 | 실제 OAuth refresh HTTP 구현 (Anthropic PKCE / OpenAI Codex / Nous) | SPEC-GOOSE-CREDENTIAL-PROXY-001 + SPEC-GOOSE-ADAPTER-001 |
| OI-11 | Rate limit 헤더 파싱 (`x-ratelimit-*`, `Retry-After`) | SPEC-GOOSE-RATELIMIT-001 |
| OI-12 | 다중 goosed 프로세스용 flock / cross-process 잠금 | Phase 2+ |
| OI-13 | `LRUStrategy`의 `active_leases` 다기준 지원 (현재 `UsageCount` 단일 기준) | 별도 SPEC 혹은 본 SPEC v0.4.0 amendment |
| OI-14 | `StatusPending` 상태 도입 여부 (현재 구현은 `CredOK` / `CredExhausted` 2-상태) | Refresher 배선 시 검토 |

### 11.3 매핑 요약

- **SPEC 본문에 남아있는 미래형 기술**은 반드시 §11에 해당 OI-ID로 참조한다.
- iteration 2 감사 시 "SPEC에 있으나 구현에 없음"이 발견되면 §11에 해당 항목이 누락된 것이거나 Run phase 스코프가 아직 식별되지 않은 것이다.

---

## 10. Related Work

### SPEC-GOOSE-ADAPTER-001 (2026-04-24 완료)

**상태**: 11 commits, Phase 2.Z 완료 (Phase 3 sync 진행 중)

**관계**: SPEC-GOOSE-ADAPTER-001은 §3.1 rule 6/7 에서 본 SPEC이 약속한 아래 두 API를 **선행 구현**하였다:

1. **`MarkExhaustedAndRotate(ctx context.Context, id string, statusCode int, retryAfter string) error`**
   - HTTP 429 (rate limit) 또는 402 (payment required) 응답 수신 시 호출
   - 해당 credential을 명시적으로 고갈 표시하고, 다음 사용 대기 시간 설정
   - `internal/llm/credential/pool.go` — 원자적 상태 전환 구현
   - 본 SPEC 구현 시 해당 API의 의미 (§3.1 rule 6 exhausted status, cooldown duration 추적) 유지 보장

2. **`AcquireLease(id string) *Lease` / `Lease.Release()`**
   - 명시적 소프트 임대 생명주기 (`internal/llm/credential/lease.go`)
   - 호출자가 credential 사용 중임을 표시 → lease counter 증가
   - Lease.Release() 호출 시 풀로 반환 → lease counter 감소
   - 본 SPEC의 §5.2 least-used strategy에서 lease counter를 기반으로 최소 사용 credential 선택
   - 본 SPEC 구현 시 해당 API와의 계약 준수

**SPEC-001 기여**:
- `internal/llm/provider/anthropic/` — Anthropic OAuth token refresh (SPEC-CREDPOOL-001과 협동)
- `internal/llm/provider/openai/` — OpenAI API key 관리 (SPEC-CREDPOOL-001과 협동)
- `internal/llm/credential/` — MarkExhaustedAndRotate + AcquireLease 기본 인터페이스
- Test coverage: provider 평균 81.1% (credential 87.5% 포함)

**다음 단계 (SPEC-CREDPOOL-001 Run phase)**:
- §5 Pool selection strategy (fill-first, round-robin, least-used, weighted-random) 완전 구현
- §6 State machine (Available → Using (leased) → Exhausted-Cooldown → Refreshing → Available)
- §7 OAuth auto-refresh (Anthropic PKCE refresh_token rotation)
- §8 Persistent state (영속 JSON 읽기/쓰기)

---

## Implementation Notes (sync 정합화 2026-04-27)

- **Status Transition**: planned → implemented
- **Package**: `internal/llm/credential/` (31 파일, 11 `_test.go`)
- **Key Files**: `pool.go`, `pooled.go`(`KeyringID` 단일 reference 필드, secret 미포함), `lease.go`, `storage.go`, `state.go`, `stats.go`
- **Refresher**: `refresher.go` — `Refresh(ctx, cred) error` 시그니처 (Zero-Knowledge: pool에 새 token 미반환). 실제 HTTP refresh는 SPEC-GOOSE-CREDENTIAL-PROXY-001 에 위임됨
- **Sources**: `factory.go`, `source.go` + 4 구현 (`anthropic_source.go`, `openai_source.go`, `nous_source.go`, `dummy_source.go`)
- **Verified REQs (spot-check)**: REQ-CP-004 metadata-only persist, `Refresher` interface 시그니처 일치, 4-strategy 선택, v0.3.0 Amendment(Tier 4 위임) 코드 일관
- **Lifecycle**: spec-anchored Level 2 — 후속 동작 변경 시 SPEC 본문 갱신 필수

---

**End of SPEC-GOOSE-CREDPOOL-001**
