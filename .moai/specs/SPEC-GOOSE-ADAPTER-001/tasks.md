# Task Decomposition — SPEC-GOOSE-ADAPTER-001

- SPEC: SPEC-GOOSE-ADAPTER-001 (6 Provider 어댑터)
- Harness: thorough
- Mode: TDD (RED-GREEN-REFACTOR)
- Session effort: xhigh (Opus 4.7 Adaptive Thinking)
- Total tasks: 29 (planned) · Phase 2.X/Y/Z 추가 증분으로 확장됨
- Milestones: M0~M5
- Drift guard baseline: 29 planned tasks
- Author (Phase 1): manager-strategy
- Approved by user: 2026-04-24
- v1.0.0 (2026-04-25): spec.md audit fix 반영 — AC-013~017 신설에 따라 §"Acceptance Criteria → Task Mapping" 표 확장

## Milestones Overview

| Milestone | Scope | Task Count | Prod LoC (est.) | Key ACs |
|-----------|-------|------------|-----------------|---------|
| M0 | Dependency skeleton + CREDPOOL 확장 | 7 | ~560 | compile-only |
| M1 | Provider core + Anthropic 완전 구현 | 12 | ~1,430 | AC-001/002/003/008/010/012 |
| M2 | OpenAI + xAI + DeepSeek | 5 | ~570 | AC-004/005/011 |
| M3 | Google Gemini | 1 | ~240 | AC-006 |
| M4 | Ollama | 1 | ~180 | AC-007 |
| M5 | Fallback + capability gate + wiring | 4 | ~100 | AC-009 |

## Atomic Task Table

### M0 — Dependency Skeleton + CREDPOOL 확장

| Task ID | Description | REQ | Deps | Planned Files | Status |
|---------|-------------|-----|------|---------------|--------|
| T-001[C] | `message.{Message,StreamEvent,ContentBlock}` 선제 정의 | REQ-ADAPTER-001,006 | — | internal/message/types.go | completed |
| T-002[C] | `tool.Definition` 선제 정의 | REQ-ADAPTER-011 | — | internal/tool/definition.go | completed |
| T-003[C] | `query.{LLMCallReq,LLMCallFunc}` 선제 정의 | REQ-ADAPTER-006 | T-001, T-002 | internal/query/types.go | completed |
| T-004[C] | `ratelimit.Tracker` noop stub (Parse 시그니처) | REQ-ADAPTER-004 | — | internal/llm/ratelimit/tracker.go | completed |
| T-005[C] | `cache.BreakpointPlanner` empty-plan stub | REQ-ADAPTER-015 | T-001[C] | internal/llm/cache/planner.go | completed |
| T-006[C] | `SecretStore` interface + FileSecretStore MVP | REQ-ADAPTER-005 | — | internal/llm/provider/secret.go | completed |
| T-007[C] | CREDPOOL 확장: `MarkExhaustedAndRotate`, `AcquireLease` | AC-ADAPTER-008 (REQ-005) | — | internal/llm/credential/pool.go, lease.go, pool_test.go | completed |

### M1 — Provider Core + Anthropic

| Task ID | Description | REQ | Deps | Planned Files | Status |
|---------|-------------|-----|------|---------------|--------|
| T-010[C] | Provider interface + CompletionRequest/Response + Capabilities + Errors | REQ-ADAPTER-001,002 | T-001..T-005 | internal/llm/provider/{provider.go,errors.go} | completed |
| T-011[C] | ProviderRegistry (instance) + Register/Get + ErrProviderNotFound | REQ-ADAPTER-002 | T-010[C] | internal/llm/provider/registry.go | completed |
| T-012[C] | `NewLLMCall` (QUERY-001 수신자) | REQ-ADAPTER-006 | T-011[C] | internal/llm/provider/llm_call.go | completed |
| T-013[C] | Anthropic `models.go` alias + normalize | REQ-ADAPTER-018 | T-010[C] | internal/llm/provider/anthropic/models.go | completed |
| T-014[C] | Anthropic `thinking.go` adaptive vs budget 분기 | REQ-ADAPTER-010 | T-013[C] | internal/llm/provider/anthropic/thinking.go | completed |
| T-015[C] | Anthropic `tools.go` schema 변환 + tool_choice | REQ-ADAPTER-011 | T-002, T-013 | internal/llm/provider/anthropic/tools.go | completed |
| T-016[C] | Anthropic `content.go` message/image/tool_result 변환 | REQ-ADAPTER-011,017 | T-001, T-015 | internal/llm/provider/anthropic/content.go | completed |
| T-017[C] | Anthropic `stream.go` SSE → StreamEvent (thinking/tool_use/text) | REQ-ADAPTER-009 | T-001[C] | internal/llm/provider/anthropic/stream.go | completed |
| T-018[C] | Anthropic `cache_apply.go` cache marker 적용 | REQ-ADAPTER-015 | T-005, T-016 | internal/llm/provider/anthropic/cache_apply.go | completed |
| T-019[C] | Anthropic `oauth.go` PKCE refresh + single-use rotation | REQ-ADAPTER-007 | T-006[C] | internal/llm/provider/anthropic/oauth.go | completed |
| T-020[C] | Anthropic `token_sync.go` atomic write `~/.claude/.credentials.json` | REQ-ADAPTER-016 | T-019[C] | internal/llm/provider/anthropic/token_sync.go | completed |
| T-021[C] | Anthropic `adapter.go` Stream/Complete + 429 retry-once 통합 | REQ-ADAPTER-003,005,006,008,013 | T-013..T-020 | internal/llm/provider/anthropic/adapter.go | completed |

### M2 — OpenAI compat + xAI + DeepSeek

| Task ID | Description | REQ | Deps | Planned Files | Status |
|---------|-------------|-----|------|---------------|--------|
| T-030 | OpenAI `adapter.go` generic chat + base_url swappable + retry-once | REQ-ADAPTER-003,005,012,013 | T-010,T-011 | internal/llm/provider/openai/adapter.go | completed |
| T-031 | OpenAI `stream.go` SSE + tool_calls aggregation | REQ-ADAPTER-009 analogous | T-001[C] | internal/llm/provider/openai/stream.go | completed |
| T-032 | OpenAI `tools.go` passthrough + tool_call_id | REQ-ADAPTER-011 | T-002[C] | internal/llm/provider/openai/tools.go | completed |
| T-033 | xAI `grok.go` factory (openai + BaseURL 오버라이드) | REQ-ADAPTER-012 | T-030 | internal/llm/provider/xai/grok.go | completed |
| T-034 | DeepSeek `client.go` factory + Vision=false capability | REQ-ADAPTER-012,017 | T-030 | internal/llm/provider/deepseek/client.go | completed |

### M3 — Google Gemini

| Task ID | Description | REQ | Deps | Planned Files | Status |
|---------|-------------|-----|------|---------------|--------|
| T-040 | Google `gemini.go` genai SDK streaming + tool + vision (stream.go 병합) | REQ-ADAPTER-001,003,013,019 | T-010[C] | internal/llm/provider/google/gemini.go | completed |

### M4 — Ollama

| Task ID | Description | REQ | Deps | Planned Files | Status |
|---------|-------------|-----|------|---------------|--------|
| T-050 | Ollama `local.go` /api/chat JSON-L streaming + no-auth (stream.go 병합) | REQ-ADAPTER-001,003 | T-010[C] | internal/llm/provider/ollama/local.go | completed |

### M5 — Fallback + Capability Gate + Wiring

| Task ID | Description | REQ | Deps | Planned Files | Status |
|---------|-------------|-----|------|---------------|--------|
| T-060 | 공유 fallback model chain helper (provider-agnostic) | REQ-ADAPTER-008 | T-021,T-030,T-040,T-050 | internal/llm/provider/fallback.go | completed |
| T-061 | Capability pre-check (vision block with Vision=false) in NewLLMCall | REQ-ADAPTER-017 | T-012[C] | internal/llm/provider/llm_call.go (확장) | completed |
| T-062 | goroutine leak verification (`goleak.VerifyTestMain`) 전 adapter | REQ-ADAPTER-003 | all | all _test.go | completed |
| T-063 | Integration wiring: DefaultRegistry 메타 + Provider 인스턴스 연결 | REQ-ADAPTER-002,006 | all | internal/llm/factory/registry_defaults.go | completed |

## Acceptance Criteria → Task Mapping

### 기존 AC (M0-M5)

| AC | 주 REQ | Task(s) | Verification File |
|----|--------|---------|-------------------|
| AC-ADAPTER-001 Anthropic streaming | REQ-001/004/006 | T-017, T-021 | anthropic/stream_test.go, anthropic/adapter_test.go |
| AC-ADAPTER-002 Anthropic tool_use | REQ-011 | T-015, T-017, T-021 | anthropic/tools_test.go, anthropic/adapter_test.go |
| AC-ADAPTER-003 OAuth auto-refresh | REQ-007/016 | T-006, T-019, T-020, T-021 | anthropic/oauth_test.go |
| AC-ADAPTER-004 OpenAI streaming | REQ-001/004 | T-030, T-031 | openai/adapter_test.go |
| AC-ADAPTER-005 xAI base_url | REQ-012 | T-033 | xai/grok_test.go |
| AC-ADAPTER-006 Google Gemini | REQ-001/003 | T-040 | google/gemini_test.go |
| AC-ADAPTER-007 Ollama localhost | REQ-001/003 | T-050 | ollama/local_test.go |
| AC-ADAPTER-008 429 rotation (+ Phase 2.X `pool.Release(next)` fix) | REQ-005 | T-007, T-021 | anthropic/adapter_test.go (rotation case) |
| AC-ADAPTER-009 Fallback model chain (production wired) | REQ-008 | T-060, T-063 | fallback_test.go, llm_call_test.go (NewLLMCall full-stack) |
| AC-ADAPTER-010 Context cancellation | REQ-003 | T-017, T-021, T-031, T-040, T-050 | *_test.go (ctx timeout) |
| AC-ADAPTER-011 Capability unsupported | REQ-017 | T-034, T-061 | llm_call_test.go |
| AC-ADAPTER-012 Thinking mode adaptive + E2E streaming (Phase 2.X) | REQ-009/010 | T-014, T-021 + `TestAnthropic_ThinkingMode_EndToEnd` | anthropic/thinking_test.go, anthropic/adapter_test.go |

### v1.0 신설 AC (plan-audit D1 gap 해소)

| AC | 주 REQ | Task(s) / Phase | Verification File |
|----|--------|-----------------|-------------------|
| AC-ADAPTER-013 Heartbeat timeout watchdog | REQ-013 | Phase 2.Y: `constants.go` 신규 + 4 adapter `HeartbeatTimeout` Options + reader goroutine watchdog | anthropic/adapter_test.go (`TestAnthropic_HeartbeatTimeout_EmitsError`), openai/adapter_test.go (`TestOpenAI_HeartbeatTimeout_EmitsError`), ollama/local_test.go (`TestOllama_HeartbeatTimeout_EmitsError`), google/gemini_test.go (`TestGoogle_HeartbeatTimeout_EmitsError`) |
| AC-ADAPTER-014 PII log 금지 (indirect) | REQ-014 | 코드 리뷰 + `zaptest` observed-logs 검증 (T-021/T-030/T-040/T-050 전반) | 각 adapter `_test.go`에 log allowlist assertion — evaluator Security 축에서 grep 검증 |
| AC-ADAPTER-015 Disk write 제한 (indirect) | REQ-016 | T-006 (`FileSecretStore.CredentialFile` path traversal 방어) + Phase 2.X `oauth.go` pathSafe 정리 | provider/secret_test.go (path traversal 거부), anthropic/oauth_test.go (atomic write 검증) |
| AC-ADAPTER-016 JSON mode (Deferred) | REQ-019 [Optional] | **DEFERRED to SPEC-GOOSE-ADAPTER-003** — 본 SPEC v1.0 합격 기준 외 | N/A (후속 SPEC에서 실구현) |
| AC-ADAPTER-017 UserID forwarding (Deferred) | REQ-020 [Optional] | **DEFERRED to SPEC-GOOSE-ADAPTER-003** — 본 SPEC v1.0 합격 기준 외 | N/A (후속 SPEC에서 실구현) |

### REQ → AC 역매핑 체크리스트 (v1.0 완전성)

| REQ | 매핑된 AC | 상태 |
|-----|----------|------|
| REQ-ADAPTER-001 | AC-001/004/006/007 | 직접 검증 |
| REQ-ADAPTER-002 | registry unit test (T-011) | 직접 검증 |
| REQ-ADAPTER-003 | AC-006/007/010 | 직접 검증 |
| REQ-ADAPTER-004 | AC-001/004 | 직접 검증 (tracker.Parse 호출) |
| REQ-ADAPTER-005 | AC-008 | 직접 검증 |
| REQ-ADAPTER-006 | AC-001 | 직접 검증 (NewLLMCall full-stack) |
| REQ-ADAPTER-007 | AC-003 | 직접 검증 |
| REQ-ADAPTER-008 | AC-009 | 직접 검증 (production wiring 포함) |
| REQ-ADAPTER-009 | AC-012 E2E | 직접 검증 (Phase 2.X) |
| REQ-ADAPTER-010 | AC-012 | 직접 검증 |
| REQ-ADAPTER-011 | AC-002 | 직접 검증 |
| REQ-ADAPTER-012 | AC-005 | 직접 검증 |
| REQ-ADAPTER-013 | AC-013 (신설) | 직접 검증 (Phase 2.Y) |
| REQ-ADAPTER-014 | AC-014 (신설, indirect) | indirect — log allowlist |
| REQ-ADAPTER-015 | AC-001/002 간접 (plan 소비 검증) | 간접 |
| REQ-ADAPTER-016 | AC-015 (신설, indirect) | indirect — filesystem scan |
| REQ-ADAPTER-017 | AC-011 | 직접 검증 |
| REQ-ADAPTER-018 | T-013 unit test | 직접 검증 |
| REQ-ADAPTER-019 | AC-016 (Deferred) | **SPEC-003으로 연기** |
| REQ-ADAPTER-020 | AC-017 (Deferred) | **SPEC-003으로 연기** |

## Dependencies & Risks (Reference)

- CREDPOOL-001 확장 (T-007)은 본 SPEC 선행 구현. CREDPOOL-001 IMPLEMENTATION-ORDER.md cross-reference 필요.
- QUERY-001/message/tool/ratelimit/cache skeleton은 본 SPEC에서 선제 정의. 후속 SPEC은 확장만 허용(기존 필드 의미 변경 금지).
- Single-session token budget 관리: M1 Anthropic 완료 시점 LSP/테스트 green checkpoint.
- Google genai SDK 테스트 훅 난이도 대비 `geminiClient` internal interface 추상화.
- T-063: import cycle 방지를 위해 internal/llm/factory 패키지에 배치 (provider → openai/xai/deepseek/ollama 역방향 의존성 차단).
