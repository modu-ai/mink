## SPEC-GOOSE-ADAPTER-001 Progress

- Started: 2026-04-24
- Worktree: /Users/goos/.moai/worktrees/goose/SPEC-GOOSE-ADAPTER-001
- Branch: feature/SPEC-GOOSE-ADAPTER-001
- Base commit: 103803b (main)
- Harness level: thorough
- Mode: TDD
- Effort: xhigh
- Planned files: 29 tasks across M0-M5
- LSP baseline: to be captured at Phase 1.7

### Phase Log

- Phase 0 complete: worktree created, branch `feature/SPEC-GOOSE-ADAPTER-001` checked out
- Phase 0.9 complete: language=Go (go.mod: `github.com/modu-ai/goose`), skill=moai-lang-go
- Phase 0.95 complete: Full Pipeline mode (file_count>10, complexity high, P0)
- Phase 1 complete: manager-strategy plan approved by user (전체 승인 · M0~M5 단일 run)
  - READY_FOR_APPROVAL: yes
  - Scope expansion approved: 5 skeleton 패키지 + CREDPOOL 확장 + SecretStore interface
- Phase 1.5 complete: 29 atomic tasks decomposed in tasks.md
- Phase 1.6 complete: AC-ADAPTER-001~012 scope defined
- Phase 1.7 complete: file scaffolding (M0+M1, 20 production files)
- Phase 1.8 complete: LSP baseline captured (go build 0 errors, go vet 0 warnings)
- Phase 2A complete: M0 + M1 TDD implementation (manager-tdd, Round 1)
  - Dependencies added: github.com/stretchr/testify v1.9.0, go.uber.org/goleak v1.3.0
  - M0 tasks: T-001 (message/types.go), T-002 (tool/definition.go), T-003 (query/types.go),
    T-004 (ratelimit/tracker.go), T-005 (cache/planner.go), T-006 (provider/secret.go),
    T-007 (credential/pool.go AcquireLease+MarkExhaustedAndRotate + lease.go Lease type)
  - M1 tasks: T-010 (provider/provider.go + errors.go), T-011 (provider/registry.go),
    T-012 (provider/llm_call.go), T-013 (anthropic/models.go), T-014 (anthropic/thinking.go),
    T-015 (anthropic/tools.go), T-016 (anthropic/content.go), T-017 (anthropic/stream.go),
    T-018 (anthropic/cache_apply.go), T-019 (anthropic/oauth.go), T-020 (anthropic/token_sync.go),
    T-021 (anthropic/adapter.go)
  - testhelper package: internal/llm/provider/testhelper/helpers.go

### M0+M1 Checkpoint (2026-04-24)

- LSP: `go build ./...` 0 errors, `go vet ./...` 0 warnings
- Test results:
  - internal/message: PASS (100% coverage — types only)
  - internal/tool: PASS (100% coverage — types only)
  - internal/query: PASS (100% coverage — types only)
  - internal/llm/cache: PASS (100% coverage)
  - internal/llm/ratelimit: PASS (100% coverage)
  - internal/llm/credential: PASS (87.5% coverage)
  - internal/llm/provider: PASS (89.2% coverage)
  - internal/llm/provider/anthropic: PASS (76.2% coverage)
  - internal/llm/router: PASS (97.2% coverage — pre-existing)
- Race detector: PASS all packages
- goleak: PASS all packages
- AC coverage: AC-001 ✓, AC-002 ✓, AC-003 ✓, AC-008 ✓, AC-010 ✓, AC-012 ✓
- Production LOC: ~1,900 (20 files)
- Test LOC: ~2,250 (18 test files)
- Total: ~4,150 LOC

### Round 2 readiness
- M2 (OpenAI/xAI/DeepSeek) ready: Provider interface + registry + testhelper in place
- M3 (Google Gemini) ready: same
- M4 (Ollama) ready: same
- M5 (Fallback + wiring) ready: after M2~M4

### Phase 2B complete: M2 + M3 + M4 + M5 TDD implementation (manager-tdd, Round 2)

- Dependencies added: google.golang.org/genai v1.54.0
- M2 tasks: T-030 (openai/adapter.go), T-031 (openai/stream.go), T-032 (openai/tools.go),
  T-033 (xai/grok.go), T-034 (deepseek/client.go)
- M3 tasks: T-040 (google/gemini.go + google/gemini_real.go)
- M4 tasks: T-050 (ollama/local.go)
- M5 tasks: T-060 (provider/fallback.go), T-061 (llm_call.go vision pre-check),
  T-062 (goleak.VerifyTestMain 검증), T-063 (internal/llm/factory/registry_defaults.go)
- MemorySecretStore added to provider/secret.go (test helper)
- Note: T-063 배치 변경 — import cycle 방지를 위해 internal/llm/factory 패키지에 배치

### M2+M3+M4+M5 Checkpoint (2026-04-24)

- LSP: `go build ./...` 0 errors, `go vet ./...` 0 warnings
- Test results:
  - internal/llm/cache: PASS (100% coverage)
  - internal/llm/credential: PASS (87.5% coverage)
  - internal/llm/factory: PASS (77.4% coverage)
  - internal/llm/provider: PASS (81.1% coverage)
  - internal/llm/provider/anthropic: PASS (76.2% coverage)
  - internal/llm/provider/deepseek: PASS (100% coverage)
  - internal/llm/provider/google: PASS (44.7% coverage — gemini_real.go는 라이브 API only)
  - internal/llm/provider/ollama: PASS (76.0% coverage)
  - internal/llm/provider/openai: PASS (77.8% coverage)
  - internal/llm/provider/xai: PASS (100% coverage)
  - internal/llm/ratelimit: PASS (100% coverage)
  - internal/llm/router: PASS (97.2% coverage)
- Race detector: PASS all packages
- goleak: PASS all packages (google: opencensus goroutine 필터링)
- AC coverage: AC-004 ✓, AC-005 ✓, AC-006 ✓, AC-007 ✓, AC-009 ✓, AC-011 ✓
- Production LOC added (Round 2): ~1,200 (11 new files)
- Test LOC added (Round 2): ~1,100 (11 test files)
- Total Round 2: ~2,300 LOC
- Cumulative: ~6,450 LOC (Round 1 ~4,150 + Round 2 ~2,300)

### Full SPEC Completion Status

- AC-ADAPTER-001~012 전수 GREEN: YES
  - Round 1: AC-001/002/003/008/010/012 ✓
  - Round 2: AC-004/005/006/007/009/011 ✓
- Coverage: 전체 평균 ~80% (google 44.7% — SDK real client 제외, 허용 범위)
- go test -race: PASS (internal/core 제외 — 기존 pre-existing 실패)
- Progress: SPEC-GOOSE-ADAPTER-001 M0~M5 모두 완료

### Phase 2.X evaluator-fix checkpoint (2026-04-24)

- evaluator PASS(0.789) 후 critical fixes 3건 + gofmt 적용
- Fix 1: gofmt -w internal/ (13 파일 포맷 정규화)
- Fix 2: anthropic/adapter.go — 429 rotation 후 `pool.Release(next)` 추가 (lease 반환 누락 수정)
- Fix 3: anthropic/oauth.go — `pathSafe` dead code 삭제, `readRawCred`/`storeRotatedRefreshToken`이 `fss.CredentialFile()`을 통해 path traversal 방어 로직 재사용. `provider/secret.go`에 `CredentialFile` exported wrapper 추가.
- Fix 4 (bonus): anthropic/thinking.go — `AnthropicThinkingParam.Type` 필드에 `json:"type"` 태그 추가 (API payload 직렬화 버그 수정). `adapter_test.go`에 `TestAnthropic_ThinkingMode_EndToEnd` e2e 테스트 추가 (AC-012 payload + SSE thinking_delta 변환 검증).
- 검증: gofmt -l 빈 출력, go build 0 errors, go vet 0 warnings, go test -race 전 패키지 PASS, anthropic 커버리지 76.2% 유지

### Phase 2.Y follow-up checkpoint (2026-04-24)

evaluator-active "commit 후 권장" 2건 처리:

**Fix 1: REQ-ADAPTER-013 heartbeat timeout 실구현**
- `internal/llm/provider/constants.go` 신규: `DefaultStreamHeartbeatTimeout=60s`, `DefaultNonStreamDataTimeout=30s`
- anthropic/adapter.go: `streamTimeout` 중복 상수 제거, `HeartbeatTimeout` Options 필드 추가
- openai/adapter.go: `HeartbeatTimeout` Options 필드 추가
- ollama/local.go: `HeartbeatTimeout` Options 필드 추가
- google/gemini.go: `HeartbeatTimeout` Options 필드 추가
- 4개 streaming 함수(ParseAndConvert×2, parseJSONL, consumeStream)에 reader goroutine + reslide-timer watchdog 삽입
- testhelper: `NewSilentSSEServer`, `NewSilentJSONLServer` helper 추가
- 4개 heartbeat timeout 테스트 추가 (200ms 주입, 2초 내 완료 검증):
  - `TestAnthropic_HeartbeatTimeout_EmitsError`
  - `TestOpenAI_HeartbeatTimeout_EmitsError`
  - `TestOllama_HeartbeatTimeout_EmitsError`
  - `TestGoogle_HeartbeatTimeout_EmitsError`

**Fix 2: xAI / DeepSeek New() 에러 전파 시그니처**
- `xai/grok.go`: `func New(...) (*openai.OpenAIAdapter, error)` (bubble up openai.New 에러)
- `deepseek/client.go`: 동일 시그니처 변경
- `factory/registry_defaults.go`: xai.New / deepseek.New 호출부 에러 핸들링 추가
- `xai/grok_test.go`, `deepseek/client_test.go`: 호출부 `err` 체크 추가

**검증 결과:**
- gofmt -l: 빈 출력
- go build ./...: 0 errors
- go vet ./internal/llm/...: 0 warnings
- go test -race 전 패키지 ALL PASS (anthropic 13s, openai/ollama/google/xai/deepseek/factory 각 1~4s)
- coverage: anthropic 77.0%, openai 78.7%, ollama 77.8%, deepseek/xai 100%, google 51.7%
- goleak: PASS (reader goroutine 누수 없음)
- 최종 8번째 commit SHA: db51dd7

### Phase 2.Z SPEC-002 prerequisite extension (2026-04-24)

- `openai.OpenAIOptions.ExtraHeaders map[string]string` 필드 추가
  - provider-specific HTTP 헤더 주입 (OpenRouter HTTP-Referer/X-Title 등)
  - New()에서 shallow clone — 호출자 post-mutation 방어
- `provider.CompletionRequest.ExtraRequestFields map[string]any` 필드 추가
  - provider-specific request body top-level 필드 (GLM thinking 파라미터 등)
  - openai adapter doRequest: 표준 필드 직렬화 후 ExtraRequestFields merge (사용자 우선)
- ExtraHeaders: doRequest에서 Authorization/Content-Type 설정 후 주입 (사용자 override 허용)
- backward compatible: nil 시 기존 동작 그대로 (분기 없음)
- 신규 테스트 3건:
  - `TestOpenAI_ExtraHeaders_InjectedInRequest`
  - `TestOpenAI_ExtraRequestFields_MergedInBody`
  - `TestOpenAI_ExtraRequestFields_OverridesStandard`
- go build / vet / fmt / test -race ALL PASS
- openai coverage: 79.6%
- SPEC-GOOSE-ADAPTER-002 의존 gap(R1) 해소
- 10번째 commit SHA: cb9605f

### Phase 3 Sync (2026-04-24)

- manager-docs: 문서 동기화 시작
  - CHANGELOG.md 최초 생성 (Keep a Changelog 형식, Unreleased 섹션 작성)
  - .moai/project/tech.md: §9 LLM Provider 섹션 대폭 확장
    - 9.1 현재 지원 6 provider 테이블
    - 9.2 계획 중 9 provider (SPEC-GOOSE-ADAPTER-002)
    - 9.3~9.7 아키텍처, 각 어댑터 상세, 온디바이스 추론, 우선순위
  - .moai/specs/SPEC-GOOSE-CREDPOOL-001/spec.md: §10 Related Work 섹션 추가
    - SPEC-GOOSE-ADAPTER-001과의 관계 명시
    - MarkExhaustedAndRotate + AcquireLease API 선행 구현 문서화
    - SPEC-CREDPOOL-001 Run phase 다음 단계 예시
- 다음: manager-git Phase 3 push + PR 생성
