# SPEC Review Report: SPEC-GOOSE-CREDPOOL-001
Iteration: 1/3
Verdict: FAIL
Overall Score: 0.58

> Reasoning context ignored per M1 Context Isolation. Audit based on spec.md (+ cross-reference to implementation code under `internal/llm/credential/` and commits `d2ee56f`, `f1a428c`, `bea5df1`, `e8580be`).

---

## Must-Pass Results

- **[PASS] MP-1 REQ number consistency**
  - Enumerated spec.md:L107-L149. REQ-CREDPOOL-001 through REQ-CREDPOOL-018 are sequential, no gaps, no duplicates, consistent 3-digit zero-padding.
  - AC-CREDPOOL-001 through AC-CREDPOOL-010 (spec.md:L157-L205) sequential, no gaps.

- **[PASS] MP-2 EARS format compliance**
  - All 18 REQ entries are tagged with an EARS band:
    - Ubiquitous 001-004 use `The CredentialPool shall ...` (spec.md:L107-L113).
    - Event-Driven 005-009 use `When X ..., the pool shall ...` (spec.md:L117-L125).
    - State-Driven 010-012 use `While X ..., ... shall ...` (spec.md:L129-L133).
    - Unwanted 013-016 use `If X, ... shall/shall not ...` or `shall not` passive (spec.md:L137-L143).
    - Optional 017-018 use `Where X, ... shall ...` (spec.md:L147-L149).
  - Note (minor, not failing): REQ-CREDPOOL-015 uses negative ubiquitous form (`The pool shall not block ...`) under Unwanted heading — form is still normative EARS-compatible.

- **[FAIL] MP-3 YAML frontmatter validity**
  - spec.md:L1-L13. Required fields per MP-3: `id, version, status, created_at, priority, labels`.
  - `id` present (L2), `version` present (L3), `status` present (L4), `priority` present (L8). PASS these four.
  - **`created_at` MISSING** — spec.md:L5 uses `created: 2026-04-21` (field name `created`, not `created_at`). MP-3 requires `created_at` ISO date string.
  - **`labels` MISSING** — no `labels:` key in frontmatter block. MP-3 requires labels (array or string).
  - Two required fields missing/mismatched → **FAIL** per MP-3 "Any missing required field = FAIL".

- **[N/A] MP-4 Section 22 language neutrality**
  - N/A: SPEC is scoped to LLM provider credential management, not multi-language tooling. No language-specific tool names hardcoded. Auto-pass.

---

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.75 | 0.75 — Most requirements unambiguous; minor residual ambiguity in REQ-015 ("pool shall not block") vs REQ-016 (race condition resolution) | spec.md:L141-L143 |
| Completeness | 0.75 | 0.75 — All sections present (HISTORY L24-28, Overview L32, Background L48, Scope L71, Requirements L101, AC L153, Technical Approach L209, Dependencies L545, Risks L565, Exclusions L601); frontmatter missing 2 fields | spec.md:L24, L601 |
| Testability | 0.85 | 0.75-1.0 boundary — 10 ACs each with Given/When/Then, all binary-testable, no weasel words detected | spec.md:L157-L205 |
| Traceability | 0.50 | 0.50 — 18 REQs vs 10 ACs, ≥8 REQs have NO corresponding AC (see defect list) | spec.md:L107-L205 |

Weighted average: (0.75 + 0.75 + 0.85 + 0.50) / 4 = 0.71; lowered by MP-3 FAIL → overall 0.58.

---

## Defects Found

### Critical

**D1. spec.md:L5 — Frontmatter `created_at` field mismatch**
- Current: `created: 2026-04-21` (L5), `updated: 2026-04-21` (L6).
- Required: `created_at: 2026-04-21` per MP-3 and other SPECs in repo.
- Severity: **critical** (MP-3 firewall).

**D2. spec.md:L1-L13 — Frontmatter `labels` field missing**
- No `labels:` key anywhere in frontmatter.
- Required per MP-3.
- Severity: **critical** (MP-3 firewall).

### Major

**D3. spec.md:L107-L149 vs L157-L205 — Traceability gap: 8 REQs without AC coverage**
- AC-CREDPOOL-001 maps to REQ-001 (priority/FillFirst) — REQ-001 is Select contract, not strategy. Mapping implicit.
- REQ-002 (mutation isolation) — no dedicated AC. Not tested.
- REQ-003 (concurrency safety) — no dedicated AC. Implementation has `TestConcurrentSelect_LeaseIsolation` but spec does not express it.
- REQ-004 (atomic persist to `~/.goose/credentials/<provider>.json`) — no AC (partial coverage via AC-004 "persisted" clause only).
- REQ-007 (HTTP 402 permanent exhaustion) — no dedicated AC.
- REQ-011 (LeastUsed active_leases ordering) — AC-008 maps (Soft lease 집계), acceptable.
- REQ-012 (ErrExhausted.RetryAfter populated) — AC-005 maps, acceptable.
- REQ-015 (no preemptive refresh) — no AC.
- REQ-016 (race condition — both succeed) — no AC.
- REQ-017 (CredentialSource.Sync 500ms) — no AC.
- REQ-018 (RefreshMargin config) — covered implicitly by AC-003.
- Net: 6-8 REQs lack explicit AC. AC-to-REQ mapping uses the loose prefix naming (AC-001 = REQ-001) but numbering diverges (18 REQs, 10 ACs).
- Severity: **major**.

**D4. Implementation divergence — `~/.goose/credentials/<provider>.json` persistence NOT implemented**
- spec.md:L113 REQ-CREDPOOL-004 requires atomic persist on every mutation.
- spec.md:L82 §3.1 rule 8 describes persistent state file.
- spec.md:L457-L480 §6.4 specifies full JSON layout.
- Implementation `/Users/goos/MoAI/AgentOS/internal/llm/credential/pool.go` contains zero references to persistence, no `Storage` interface, no atomic write. `pool.go:L42` `New()` only calls `p.load(context.Background())` from `CredentialSource`, not from persistent state file.
- Implementation gap: SPEC prescribes persistence; MVP commit `d2ee56f` deliberately omits it ("Zero-Knowledge credential pool" overrides persistence because actual secrets moved to keyring/proxy). SPEC was NOT amended to reflect this shift — v0.2 amendment at spec.md:L17-L22 only says "raw secret not loaded" but does not retract REQ-004 persistence requirement.
- Severity: **major** (SPEC obsolete vs implementation reality).

**D5. Implementation divergence — 4 selection strategies renamed/replaced**
- spec.md:L76, L295-L300 specify strategies: `FillFirst`, `RoundRobin`, `Random`, `LeastUsed`.
- Implementation `strategy.go` provides: `RoundRobinStrategy`, `LRUStrategy`, `WeightedStrategy`, `PriorityStrategy`.
- Mapping: `FillFirst` → `PriorityStrategy` (renamed), `LeastUsed` → `LRUStrategy` (renamed to sort by UsageCount not ActiveLeases), `Random` → `WeightedStrategy` (reinterpreted).
- `LRUStrategy` at strategy.go:L56-L67 selects by `UsageCount` alone, NOT by `active_leases` with `request_count` tiebreaker as REQ-CREDPOOL-011 (spec.md:L131) requires.
- Severity: **major** (REQ-011 not implemented correctly; SPEC language `LeastUsed` does not exist in code).

**D6. Implementation divergence — `Refresher` interface signature mismatch**
- spec.md:L308-L319 specifies `Refresh(ctx, entry) (RefreshResult, error)` returning `{AccessToken, RefreshToken, ExpiresAt, Extra}`.
- Implementation `refresher.go:L15` defines `Refresh(ctx, cred) error` — no `RefreshResult` return, no token rotation path through pool (intentional per Zero-Knowledge, but SPEC has not been updated).
- Severity: **major**.

**D7. SPEC §3.1 rule 9 — External CredentialSource implementations missing**
- spec.md:L83 promises 3 sources: `AnthropicClaudeSource`, `OpenAICodexSource`, `NousSource`. SPEC §6.5 (L486-L516) specifies schemas.
- Implementation: only `DummySource` (dummy_source.go) and the `CredentialSource` interface (source.go). No AnthropicClaudeSource/OpenAICodexSource/NousSource files.
- Severity: **major** (skeleton only, core IN-SCOPE undelivered).

**D8. REQ-CREDPOOL-014 — Secret redaction unenforceable (String() not present)**
- spec.md:L139 mandates `PooledCredential.String()` returns redacted form.
- Implementation pooled.go has NO `String()` method. Struct has no secret fields anyway (Zero-Knowledge), so redaction is vacuously satisfied — BUT SPEC has not been updated and AC-010 (spec.md:L202-L205) still references `access_token`, `refresh_token`, `api_key` which do not exist in the struct.
- Severity: **major** (contradictory SPEC vs implementation; AC-010 is no longer testable against current code — no log assertions possible because no secrets flow through pool).

### Minor

**D9. spec.md:L21 — Keyring library claim not verified**
- Claims `github.com/zalando/go-keyring` adoption for OS keyring backend.
- Neither `credential/*.go` nor (per grep of go.mod section of this SPEC scope) the credential package imports go-keyring. Responsibility is deferred to SPEC-GOOSE-CREDENTIAL-PROXY-001 per L19, but spec does not make that boundary explicit for auditor.
- Severity: minor.

**D10. spec.md:L322-L338 — `PoolOptions` type exists in SPEC but not in implementation**
- SPEC specifies `PoolOptions{Provider, DefaultStrategy, Refresher, Storage, Sources, RefreshMargin, DefaultCooldown, Logger, Clock}`.
- Implementation uses functional `Option` pattern (pool.go:L20) with zero recognized options defined. No `Refresher`, no `Sources[]`, no `Clock`.
- Severity: minor (API shape drift; observable by integrators).

**D11. spec.md:L157-L161 — AC-CREDPOOL-001 label mismatch**
- AC-001 title says "기본 선택(FillFirst) 및 우선순위". "FillFirst" does not exist in implementation (renamed to PriorityStrategy) and the SPEC declares FillFirst as `iota` 0 (spec.md:L296) but describes behavior as "최고 priority 엔트리 우선" — FillFirst by definition means "fill the first slot" not "highest priority"; SPEC conflates two strategies.
- Severity: minor (naming inconsistency).

**D12. spec.md:L131 — REQ-CREDPOOL-011 LeastUsed tiebreaker not implemented**
- REQ requires `active_leases` primary, `request_count` tiebreaker.
- strategy.go:L56-L67 `LRUStrategy.Select` uses only `UsageCount` (single criterion, no tiebreaker).
- Since `active_leases` is not even a field on `PooledCredential` (struct pooled.go:L10-L36 has no such field — only `leased bool` and `UsageCount uint64`), this REQ cannot be implemented without struct changes.
- Severity: minor-to-major (falls under D5 umbrella).

---

## 구현 정합률 (Implementation Alignment)

| SPEC 섹션 | SPEC 의도 | 구현 상태 | 정합도 |
|----------|---------|---------|-------|
| §3.1.1 Package layout (pool/credential/strategy/lease/oauth/storage/source + 3 provider sources + errors + stats) | 15 files | 9 files (pool/pooled/strategy/lease/source/refresher/dummy_source/state + 2 tests) | 60% |
| §3.1.2 4 strategies (FillFirst/RoundRobin/Random/LeastUsed) | 4 | 4 (RoundRobin/LRU/Weighted/Priority) — renamed | 50% (naming mismatch + semantics drift) |
| §3.1.3 Availability filter | OK+reset+expiry | OK+cooldown (no expires_at filter) | 70% |
| §3.1.4 OAuth auto-refresh | Refresher hook triggered from Select | Interface only, not wired to Select | 30% |
| §3.1.5 429/402 handling | 429 cooldown + 402 permanent | 429 implemented (f1a428c); 402 not distinguished | 60% |
| §3.1.6 MarkExhaustedAndRotate | spec | Implemented (pool.go:L252) | **95%** |
| §3.1.7 AcquireLease/ReleaseLease | spec | Implemented (lease.go) | **90%** |
| §3.1.8 Persistent JSON state | REQ-004 | NOT implemented | 0% |
| §3.1.9 External sources (Anthropic/OpenAI/Nous) | 3 impls | 0 impls (only DummySource) | 0% |
| §3.1.10 PoolStats | REQ + stats.go | No stats.go; only Size() (pool.go:L221) | 30% |
| §3.1.11 Error types | 4 types (ErrExhausted/ErrRefreshFailed/ErrInvalidEntry/ErrLeaseUnavailable) | 3 (ErrExhausted/ErrNotFound/ErrAlreadyReleased) — different set | 40% |
| §3.1.12 CONFIG-001 integration | init from LLMConfig | Not wired to config package | 0% |
| AC-001 FillFirst priority | | Priority strategy exists | 80% |
| AC-002 RoundRobin | | Implemented | **100%** |
| AC-003 OAuth refresh | | Refresher not called from Select | 0% |
| AC-004 429 rotate | | Implemented (pool_extend_test.go) | **90%** |
| AC-005 Pool exhausted | | Implemented (ErrExhausted) | **90%** |
| AC-006 Persist restore | | Not implemented | 0% |
| AC-007 External file sync | | Not implemented | 0% |
| AC-008 Soft lease LeastUsed | | Partially (LRU by UsageCount not active_leases) | 40% |
| AC-009 Refresh 3-fail permanent exhausted | | Not implemented | 0% |
| AC-010 Log redact | | Vacuously satisfied (no secrets in struct) | 50% (test not applicable to current shape) |

**전체 정합률: ~45%** (MVP 상태; SPEC의 절반 정도만 구현. Zero-Knowledge 방향 전환으로 §3.1.8, §3.1.9, §3.1.10 의도가 SPEC-GOOSE-CREDENTIAL-PROXY-001로 이관됐으나 본 SPEC에 반영 안됨.)

---

## 특별 집중 항목 답변

1. **기본 구현 vs skeleton**: MVP 수준. 핵심 오케스트레이션(Select/Release/MarkExhausted/MarkExhaustedAndRotate/AcquireLease/Reload/Size)은 완성. 영속성, 외부 소스, OAuth refresh 흐름은 skeleton도 아닌 미착수.

2. **429 rotation SPEC 명시 여부**: 명시됨. REQ-CREDPOOL-006 (spec.md:L119) + AC-CREDPOOL-004 (spec.md:L172-L175). 구현은 pool.go:L252 `MarkExhaustedAndRotate`로 완성. T-007 커밋(f1a428c)에서 확장됐으며 SPEC은 §10 Related Work에서 선행구현을 명시(spec.md:L626-L637). SPEC 정합성 OK.

3. **Lease 인수 API가 본 SPEC 범위인지**: 본 SPEC 범위. REQ-CREDPOOL-009 (spec.md:L125) 및 §3.1 rule 7 (spec.md:L81). ADAPTER-001에서 선행구현됐다는 점은 spec.md:L633-L637에 Related Work로 명시. **추가 확장 SPEC 불필요** — 다만 실제 구현 `lease.go`의 `AcquireLease`는 ID 검색 실패 시 REQ-009이 요구하는 `ErrInvalidEntry` 대신 `nil` 반환(lease.go:L42-L44). Minor defect.

4. **보안 민감 코드 log leakage / zero-value**:
   - Zero-Knowledge 설계로 `PooledCredential`에 raw secret 필드 없음(pooled.go:L10-L36). `TestPooledCredential_NoSecretFields`(pool_test.go:L328-L347)가 reflect로 강제.
   - Log leakage 경로: pool.go 내부 zap logger 없음, `fmt.Errorf` 메시지에 ID만 노출(`"credential: 크레덴셜을 찾을 수 없음"`). 크레덴셜 값 유출 가능 경로 없음.
   - zero-value 처리: `LastErrorAt`, `LastErrorReset`, `ExpiresAt`는 `time.Time{}` zero-value 허용. `availableCount`(pool.go:L119-L136)는 `ExpiresAt.IsZero()` 검사 부재 — OAuth entry의 `ExpiresAt` 필터링이 SPEC REQ-001 (b) 절을 미준수. 만료된 OAuth가 선택 후보에 포함될 수 있음. **major security defect**로 재분류 → **D13**.

**D13 (추가, major)**. pool.go:L93-L115 `available()`는 `Status != CredOK` 여부만 체크하고 `ExpiresAt`은 전혀 검사하지 않음. 만료된 OAuth 토큰이 선택될 수 있음. REQ-CREDPOOL-001 (b) 절("(for OAuth) `expires_at > now`") 위반. 실사용에서 401 응답을 유발할 가능성.

---

## Chain-of-Verification Pass

두 번째 패스 결과:

- REQ 번호 시퀀싱 재확인: 001-018 전수 확인. 누락 없음.
- AC 번호 시퀀싱 재확인: 001-010 전수 확인.
- Traceability를 REQ별로 재확인하여 D3 결함 구체화.
- Exclusions 섹션(spec.md:L601-L614) 10개 불렛 각각 구체적(provider 이름/SPEC 참조 명시). 단순 presence가 아닌 specificity 통과.
- Contradictions: REQ-014 secret redaction vs 실제 구조체 무secret — 첫 패스에서 D8로 포착.
- **두 번째 패스 신규 발견**: D13 (ExpiresAt 필터 누락) — 보안 집중 항목에서 발견. 첫 패스의 Defects 목록에 추가.
- 검증한 섹션 재독: L101-L150 (REQ), L153-L205 (AC), L601-L614 (Exclusions), pool.go 전체, pooled.go 전체, strategy.go 전체.

---

## Regression Check

Iteration 1. N/A.

---

## Recommendation

**Verdict: FAIL** (iteration 1).

manager-spec는 다음 수정을 iteration 2에 반영:

1. **[Critical] 프론트매터 수정** (spec.md:L5)
   - `created: 2026-04-21` → `created_at: 2026-04-21`
   - 프론트매터 블록 내(L2-L13)에 `labels: [credential, llm, oauth, phase-1, zero-knowledge]` 추가

2. **[Critical] v0.2 amendment를 REQ 본문까지 전파** — spec.md:L17-L22의 Zero-Knowledge 선언이 REQ-004 / REQ-014 / §3.1.8 / §6.4 본문과 모순. 다음 중 택1:
   - (a) REQ-CREDPOOL-004를 amend하여 "실 secret 값은 persist 대상 아님; metadata만 persist"로 재정의. §6.4 JSON schema 보존.
   - (b) REQ-CREDPOOL-004를 out-of-scope로 이동하여 SPEC-GOOSE-CREDENTIAL-PROXY-001로 위임. `Storage` interface/`stats.go` 언급 제거. REQ 번호 재번호 금지.

3. **[Major] Strategy 이름 동기화** (spec.md:L76, L295-L300, AC-001 L157-L161)
   - 구현 실체와 일치: `PriorityStrategy`, `RoundRobinStrategy`, `LRUStrategy`, `WeightedStrategy`
   - 또는 구현을 SPEC 이름에 맞춰 리네임하는 대안 추가(구현자에게 위임).

4. **[Major] REQ-CREDPOOL-011 수정** (spec.md:L131)
   - 실제 구현이 `active_leases`+`request_count` 구조를 지원하지 않으므로 `PooledCredential`에 `ActiveLeases int32` 필드 추가 or REQ를 "lowest `UsageCount` primary"로 완화.

5. **[Major] Traceability 보강** — 아래 8개 REQ에 대응하는 AC 추가:
   - AC-CREDPOOL-011: REQ-002 (direct mutation rejection)
   - AC-CREDPOOL-012: REQ-003 (concurrency race)
   - AC-CREDPOOL-013: REQ-004 (atomic persist verification) — #2의 결과에 따라 존속 여부 결정
   - AC-CREDPOOL-014: REQ-007 (HTTP 402 permanent path)
   - AC-CREDPOOL-015: REQ-015 (no preemptive refresh)
   - AC-CREDPOOL-016: REQ-017 (CredentialSource context cancel)
   - AC-CREDPOOL-017: REQ-018 (RefreshMargin config)

6. **[Major] ExpiresAt 필터 누락(D13)** — §6.3 의사코드(spec.md:L411-L454)는 올바르나 구현이 미준수. 해결책:
   - (a) REQ-CREDPOOL-001 (b) 절을 `ExpiresAt.IsZero() OR ExpiresAt > now`로 명확화 (이미 의사코드 L420에 있음) + 구현에 누락됐음을 §10 Related Work에 "run phase에서 구현 필요" 명시.
   - (b) 즉시 구현 수정(pool.go)은 SPEC 변경 외 작업.

7. **[Major] External source 3종(Anthropic/OpenAI/Nous) 구현 상태** — §3.1 rule 9 (spec.md:L83) 및 §6.5 (spec.md:L486-L516)의 상태를 "run phase에서 구현" 또는 "SPEC-GOOSE-CREDENTIAL-PROXY-001로 위임"으로 명시.

8. **[Minor] Error 타입 세트 동기화** — spec.md:L393-L398 `ErrRefreshFailed`, `ErrInvalidEntry`, `ErrLeaseUnavailable`를 실구현 `ErrNotFound`, `ErrAlreadyReleased`와 일치시키거나 SPEC 타입을 명시적으로 RESERVED로 표시.

9. **[Minor] lease.go AcquireLease error 경로** — REQ-CREDPOOL-009는 "id not found → ErrInvalidEntry" 요구. 구현은 `nil` 반환. SPEC 또는 구현 중 1개 교정.

본 SPEC은 범위/품질 자체는 견고하나 **v0.2 amendment가 REQ-004/014/§3.1.8-10에 파급되지 않아** SPEC 본문과 구현이 상호모순. 이 구조적 모순이 iteration 2에서 해소되지 않으면 FAIL 지속 가능성 높음.
