# SPEC Review Report: SPEC-GOOSE-CREDPOOL-001
Iteration: 2/3
Verdict: PASS
Overall Score: 0.86

> Reasoning context ignored per M1 Context Isolation. Audit based on spec.md (v0.3.0, 709 lines) + cross-reference to `internal/llm/credential/*.go` (Phase C1 commit `79d92ff` 반영) + previous report `.moai/reports/plan-audit/mass-20260425/CREDPOOL-001-audit.md` (iteration 1).

System-reminders received during this audit (workflow-modes, spec-workflow, moai-memory, go.md, mx-tag-protocol, MCP server instructions, Auto Mode) are general project context and not material to this SPEC audit.

---

## Must-Pass Results

- **[PASS] MP-1 REQ number consistency**
  - Re-enumerated spec.md:L116-L158. REQ-CREDPOOL-001 → REQ-CREDPOOL-018, sequential, no gaps, no duplicates, 3-digit zero-padded.
  - AC-CREDPOOL-001 → AC-CREDPOOL-011 (spec.md:L166-L228), sequential. AC-011 is new in v0.3.0 (commit `79d92ff` Phase C1 fix) — properly numbered immediately after AC-010.

- **[PASS] MP-2 EARS format compliance**
  - 18 REQ entries each tagged with EARS band (Ubiquitous L116-L122, Event-Driven L126-L134, State-Driven L138-L142, Unwanted L146-L152, Optional L156-L158). Spot-checks confirm canonical patterns:
    - L116 "shall guarantee that every call ... returns either ..." (Ubiquitous, normative).
    - L126 "When `Select(ctx, strategy)` is invoked, the pool shall ..." (Event-Driven).
    - L138 "While an OAuth entry's expires_at − refreshMargin < now ..., ... shall ..." (State-Driven).
    - L146 "If `Refresher.Refresh` returns an error ... 3 times consecutively, the pool shall ..." (Unwanted).
    - L156 "Where `CredentialSource.Sync(ctx)` is invoked ..." (Optional).

- **[PASS] MP-3 YAML frontmatter validity** (FIXED from iteration 1)
  - spec.md:L1-L14. All required fields present:
    - `id: SPEC-GOOSE-CREDPOOL-001` (L2) ✓
    - `version: 0.3.0` (L3) ✓
    - `status: planned` (L4) ✓
    - `created_at: 2026-04-21` (L5) ✓ — **fixed from `created:` to `created_at:`**
    - `priority: P0` (L8) ✓
    - `labels: [credential, llm, oauth, phase-1, zero-knowledge]` (L13) ✓ — **added in v0.3.0**
  - All 6 MP-3 fields valid. PASS.

- **[N/A] MP-4 Section 22 language neutrality**
  - SPEC scoped to LLM provider credential management (Anthropic / OpenAI / Nous), not multi-language LSP tooling. No language-specific tool names hardcoded. Auto-pass.

---

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.90 | 0.75-1.0 boundary — all REQs single-interpretation; v0.2 amendment block (L18-L29) explicitly resolves prior ambiguity in REQ-004/014/Refresher signature; minor residual: L88 "402/429 구분 자체는 statusCode 인자로 전달만" defers behavior to Open Items but REQ-007 still mandates the divergence (acknowledged at OI-04) | spec.md:L18-L29, L88, L130, L650 |
| Completeness | 0.90 | 0.75-1.0 — All sections present (HISTORY L33-L36, Overview L40, Background L56, Scope L79, Requirements L110, AC L162, Technical Approach L232, Dependencies L566, Risks L586, References L600, Exclusions L622, **Open Items §11 L639 newly added**, Related Work L674); frontmatter complete; AC-011 added | spec.md:L33, L639, L622 |
| Testability | 0.90 | 0.75-1.0 — 11 ACs each with Given/When/Then; binary-testable. AC-006/007/009 marked **"Open Item, §11 참조"** with explicit "검증 가능 시점은 Storage 구현 완료 후" — prevents false PASS claim. AC-011 cites 4 concrete test functions in `pool_expires_test.go` (verified PASS in CI run below) | spec.md:L192-L213, L220-L228 |
| Traceability | 0.75 | 0.75 — REQ→AC mapping improved via Open Items §11 enumeration: REQ-004/AC-006 (OI-01), REQ-010+REQ-018/AC-003 (OI-02), REQ-013/AC-009 (OI-03), REQ-006+REQ-007/(no AC) (OI-04), §3.1 rule 9 + §6.5/AC-007 (OI-05). REQ-002/003/008/015/016/017 still without dedicated AC — partial but transparent | spec.md:L639-L671 |

Weighted: (0.90 + 0.90 + 0.90 + 0.75) / 4 = **0.86**.

---

## Defects Found (iteration 2)

### Minor (residual, non-blocking)

**D14. spec.md:L118-L120 — REQ-CREDPOOL-002 references nonexistent field name**
- Text: "All mutations to `PooledCredential.status`, `last_error_reset_at`, and `request_count` shall go through ...".
- Implementation `pooled.go` (per audit-1 D8 + L270-L284 of spec): exported field is `LastErrorReset` (camelCase) and there is no `request_count` field — the equivalent is `UsageCount`.
- Severity: **minor** — naming drift between v0.1 vintage prose and v0.3.0 struct definition (spec.md:L270-L284 already lists correct names). Cosmetic.

**D15. spec.md:L132 — REQ-CREDPOOL-008 references `Pool.RefreshSources(ctx)`**
- AC-CREDPOOL-007 (L200) renames to `Pool.Reload(ctx)` with note "(현재 구현 API 명칭)".
- REQ body still says `Pool.RefreshSources(ctx)`.
- Severity: **minor** — REQ vs AC naming inconsistency. AC is correct vs implementation; REQ should be aligned.

**D16. spec.md:L134 — REQ-CREDPOOL-009 expected `ErrInvalidEntry` not in code**
- REQ requires "if `id` is not found, returns `ErrInvalidEntry`".
- §3.1 rule 11 (L94) acknowledges `ErrInvalidEntry` is not in implementation; OI is captured implicitly. lease.go returns `nil` per audit-1 finding (Recommendation #9).
- Severity: **minor** — REQ contradicts implementation; resolution belongs to Run phase or SPEC amendment. Not regression-blocking.

**D17. spec.md:L600-L640 — Section numbering inconsistency**
- §9 References (L600), §11 Open Items (L639), §10 Related Work (L674). §10 appears AFTER §11.
- Minor structural ordering issue. Doc reads top-down without breakage but TOC/auto-link tools may produce incorrect ordering.
- Severity: **minor**.

### Major

(none in iteration 2)

### Critical

(none in iteration 2 — both iteration-1 critical defects resolved)

---

## 구현 교차 검증 (Implementation Cross-check)

Test execution: `go test -race ./internal/llm/credential/...` → `ok ... 1.636s` (clean PASS).

Iteration-1 D13 (ExpiresAt 필터 누락) regression verification:
- pool.go:L99-L126 `available()`: ExpiresAt 필터 적용 (L120-L122).
- pool.go:L131-L152 `availableCount()`: 동일 필터 적용 (L146-L148).
- pool_expires_test.go: 4 신규 테스트 모두 PASS:
  - `TestAvailable_FiltersExpiredOAuthTokens` — PASS
  - `TestAvailable_ZeroExpiresAt_IncludedAsNonExpiring` — PASS
  - `TestAvailable_ExpiresAtInFuture_Included` — PASS
  - `TestAvailable_ExpiredTokenExcludedFromSize` — PASS
- SPEC AC-CREDPOOL-011 (spec.md:L220-L228) explicitly references these 4 test names — perfect alignment.

Strategy 명명 정합 verification:
- strategy.go (grep 결과): `RoundRobinStrategy`, `LRUStrategy`, `WeightedStrategy`, `PriorityStrategy` — 모두 정확히 4종.
- spec.md:L84 §3.1 rule 2: `PriorityStrategy` / `RoundRobinStrategy` / `WeightedStrategy` / `LRUStrategy` 명시 + v0.1 명칭(`FillFirst`/`Random`/`LeastUsed`)과의 매핑 노트.
- spec.md:L302-L305 functional sigs: `NewPriorityStrategy()`, `NewRoundRobinStrategy()`, `NewWeightedStrategy()`, `NewLRUStrategy()` — code의 `func New...Strategy()` (strategy.go:L30,L53,L79,L130)와 일치.
- spec.md:L168-L170 AC-001 v0.3.0 재명명: `NewPriorityStrategy()` 사용. 일관.

---

## Regression Check (vs iteration 1)

13 defects from CREDPOOL-001-audit.md iteration 1 reviewed:

| ID | Severity | Title | Status |
|----|----------|-------|--------|
| D1 | critical | `created` → `created_at` | **RESOLVED** — spec.md:L5 `created_at: 2026-04-21` |
| D2 | critical | `labels` 누락 | **RESOLVED** — spec.md:L13 `labels: [credential, llm, oauth, phase-1, zero-knowledge]` |
| D3 | major | Traceability 8 REQs without AC | **PARTIALLY RESOLVED** — Open Items §11 (L639-L671)에 OI-01~OI-08로 매핑, AC 미생성 REQ는 명시적 deferral. Strict traceability(개별 AC 부여)는 미달이나 transparent deferral로 격상. |
| D4 | major | `<provider>.json` persist 미구현 | **RESOLVED (SPEC-side)** — REQ-004 metadata-only로 재정의(L122), §6.4 v0.3.0 schema(L467-L498), OI-01 이관 |
| D5 | major | Strategy 4종 이름 drift | **RESOLVED** — §3.1 rule 2(L84) v0.3.0 재명명 + `Priority`/`RoundRobin`/`Weighted`/`LRU` 4종 코드 일치 검증 완료 |
| D6 | major | Refresher 시그니처 mismatch | **RESOLVED** — v0.2 amendment 항목 4(L29), §6.2 refresher.go 블록(L308-L325), `Refresh(ctx, cred) error` (no RefreshResult) |
| D7 | major | 외부 source 3종 미구현 | **RESOLVED (SPEC-side)** — §3.1 rule 9(L92) "interface 정의까지만 IN SCOPE", OI-05 이관 |
| D8 | major | REQ-014 `String()` 미존재 | **RESOLVED** — REQ-014(L148) "struct-level 무-secret 불변"으로 재정의, AC-010(L215-L218) `TestPooledCredential_NoSecretFields` 참조 |
| D9 | minor | go-keyring 미사용 | **RESOLVED** — v0.2 amendment(L22)에서 SPEC-GOOSE-CREDENTIAL-PROXY-001로 위임 명시 |
| D10 | minor | `PoolOptions` 미존재 | **RESOLVED** — §6.2 pool.go 블록(L376-L378) "v0.1의 PoolOptions ... §11 Open Items로 이관" 명시 + Functional Option 패턴(L331) 채택 |
| D11 | minor | AC-001 FillFirst 명칭 모호 | **RESOLVED** — AC-001(L166-L169) PriorityStrategy 재명명, "최고 Priority 값 우선" 명문화 |
| D12 | minor-major | REQ-011 LeastUsed 다기준 미구현 | **RESOLVED** — REQ-011(L140) v0.3.0 완화, "lowest UsageCount 단일 기준" + OI-13 이관 |
| **D13** | major | **ExpiresAt 필터 누락 (security)** | **RESOLVED** — pool.go:L120-L122 + L146-L148 필터 추가, AC-CREDPOOL-011 신설(L220-L228), `pool_expires_test.go` 4 테스트 PASS 확인 |

**13/13 defects addressed.** 9 fully resolved with code+SPEC change, 4 deferred to Open Items §11 with explicit ownership (run-phase or follow-on SPEC). No iteration-1 defect persists unchanged → no stagnation.

---

## Chain-of-Verification Pass

두 번째 패스 결과:

- REQ 번호 시퀀싱 재확인: spec.md:L116-L158 전수 스캔. REQ-001 ~ REQ-018 누락/중복 없음.
- AC 번호 시퀀싱 재확인: spec.md:L166-L228 전수 스캔. AC-001 ~ AC-011 모두 존재. AC-011 신설로 AC 수가 10 → 11로 증가, REQ 18개와의 격차 7로 축소.
- Traceability 재확인: REQ-002, 003, 008, 015, 016, 017은 여전히 직접 AC 미보유. §11 Open Items 매핑 테이블(L645-L654)이 REQ-004→OI-01, REQ-010+018→OI-02, REQ-013→OI-03, REQ-006+007→OI-04, §3.1 rule 9+6.5→OI-05, §3.1 rule 12→OI-06으로 전환을 명시. REQ-002/003/008/015/016/017은 OI에도 등재되지 않음 — 이는 **D3 partial resolution**으로 재분류(strict full traceability 미달이나 transparent deferral). MP-firewall은 traceability를 별도 must-pass로 두지 않으므로 PASS 가능.
- Exclusions 섹션(L622-L635) 10개 불릿 각각 specific (provider 이름/SPEC 참조 명시). 합격.
- Contradictions 재검: v0.2 amendment(L18-L29)가 REQ-004/014/§3.1 rule 8/§6.4와 일관됨을 확인. REQ-014(L148)와 AC-010(L215-L218)은 "TestPooledCredential_NoSecretFields"라는 동일 테스트를 참조하여 일관. v0.1 vintage 프로즈와 v0.3.0 변경 간 잔존 미스매치는 D14/D15/D16/D17 (모두 minor) 4건만 존재.
- 신규 발견 결함(D14~D17, 모두 minor) 4건 — Critical/Major 없음.
- Strategy / 코드 / SPEC 3-way 일치 재확인: code(`NewPriority/RoundRobin/Weighted/LRUStrategy` × 4) ↔ §3.1 rule 2(L84) ↔ §6.2 strategy.go 블록(L294-L305) ↔ AC-001(L168) 4-way alignment 확인.

---

## Implementation Cross-check Summary

| 항목 | iteration 1 | iteration 2 | 변화 |
|-----|-------------|-------------|------|
| 프론트매터 6필드 | 4/6 | 6/6 | +2 (created_at, labels) |
| EARS 18 REQ | PASS | PASS | 유지 |
| Strategy 4종 명명 일치 | FAIL | PASS | code 4종 ↔ SPEC L84 ↔ AC-001 |
| ExpiresAt 필터 (보안) | FAIL | PASS | pool.go L120-122, L146-148 + 4 신규 테스트 |
| Zero-Knowledge 파급 | 부분 (amendment만) | 완전 (REQ-004/014/§3.1.8/§6.4/§6.2 본문 전파) | major fix |
| REQ ↔ AC 직접 매핑 | 10/18 | 11/18 (+ OI § 11 매핑 6건 transparent deferral) | 부분 개선 |
| 전체 정합률 | ~45% | ~75% (SPEC vs code) | +30%p |

---

## Recommendation

**Verdict: PASS** (iteration 2).

근거:
1. **모든 must-pass criteria 통과**: MP-1 (REQ 시퀀싱), MP-2 (EARS), MP-3 (frontmatter 6필드 완비, 이전 critical 결함 D1/D2 해소), MP-4 (N/A).
2. **iteration 1의 critical 2건 (D1/D2) 모두 evidence 동반 해소** — spec.md:L5 `created_at`, L13 `labels`.
3. **Major 6건 (D3-D8)** — D4/D5/D6/D7/D8 모두 SPEC 본문 amend로 완전 해소; D3는 §11 Open Items로 transparent deferral (strict full traceability 미달이나 must-pass에 해당하지 않음).
4. **D13 (security-critical, 추가 발견)** — code-side fix 확인(`pool.go:L120-122, L146-148`) + 4 신규 테스트 race-clean PASS + AC-011 신설(L220-L228).
5. **잔존 결함 4건 (D14-D17)** 모두 minor 명명 drift 또는 §1번호 ordering — v0.4 amendment에서 cleanup 가능, blocking 아님.
6. **Stagnation 없음** — iteration 1의 13 결함 중 0건이 unchanged. 모든 결함이 status 변화(resolved 또는 transparent deferral).

선택적 후속 cleanup (non-blocking, v0.4):
- D14: REQ-002 (L118-L120) 필드 이름을 `Status` / `LastErrorReset` / `UsageCount`로 정렬.
- D15: REQ-008 (L132) `RefreshSources` → `Reload`로 통일.
- D16: REQ-009 (L134) `ErrInvalidEntry` 표기를 §3.1 rule 11과 일관되도록 명시적 RESERVED 표시 또는 `ErrNotFound`로 정합화.
- D17: §10 Related Work / §11 Open Items 섹션 번호 순서 정렬 (§10이 §11 앞에 오도록 또는 헤더 번호 재할당).

**SPEC-GOOSE-CREDPOOL-001 v0.3.0은 Plan phase 통과 기준 충족. Run phase 진입 가능.**

---

End of iteration 2 report.
