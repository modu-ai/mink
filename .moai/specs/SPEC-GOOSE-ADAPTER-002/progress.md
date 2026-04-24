---
spec: SPEC-GOOSE-ADAPTER-002
status: Completed
updated: 2026-04-24
---

# SPEC-GOOSE-ADAPTER-002 Run Phase Progress

## Phase 0: Environment Verification

- [x] Worktree: `/Users/goos/.moai/worktrees/goose/SPEC-GOOSE-ADAPTER-002`
- [x] Branch: `feature/SPEC-GOOSE-ADAPTER-002` (main @ `bea5df1` 기반)
- [x] LSP: `go build ./...` 0 errors
- [x] SPEC-001 자산 가용: `openai.OpenAIAdapter`, `ExtraHeaders`, `ExtraRequestFields`, credential/ratelimit 연계
- [x] 참조 구현: `xai/grok.go`, `deepseek/client.go` 패턴 확인

## Phase 1: Strategy

- TDD 방법론: RED-GREEN-REFACTOR 5 milestone
- SPEC-001의 `openai.New` + `OpenAIOptions` 팩토리 패턴 직접 재사용
- `registry_builder.go` 위치: `internal/llm/factory/` (import cycle 방지)

## Phase 2: Milestone Execution

### M1 — Groq / Cerebras / Mistral

- [x] RED: 테스트 파일 3종 작성
- [x] GREEN: 팩토리 구현 3종 (각 ~55 LOC)
- [x] PASS: `go test -race ./internal/llm/...` PASS
- [x] COMMIT: `f11b524` feat(llm/provider): Groq / Cerebras / Mistral 어댑터 팩토리 추가

### M2 — OpenRouter / Together / Fireworks

- [x] RED: 테스트 파일 3종 (OpenRouter 3 tests, Together/Fireworks 각 1)
- [x] GREEN: 팩토리 구현 3종 (OpenRouter ExtraHeaders 주입 포함)
- [x] PASS: 전체 PASS
- [x] COMMIT: `bc5658f` feat(llm/provider): OpenRouter / Together / Fireworks 어댑터 팩토리 추가

### M3 — Qwen (DashScope) / Kimi (Moonshot)

- [x] RED: 테스트 파일 2종 (각 4 tests: default/region/envvar/invalid)
- [x] GREEN: Region 선택 로직 + ErrInvalidRegion 구현
- [x] PASS: 전체 PASS
- [x] COMMIT: `3ccea04` feat(llm/provider): Qwen / Kimi 지역 선택 어댑터

### M4 — GLM (Z.ai) with thinking mode

- [x] RED: `adapter_test.go` (4 tests) + `thinking_test.go` (5 tests)
- [x] GREEN: `adapter.go` (embedding + Stream/Complete override) + `thinking.go` (BuildThinkingField)
- [x] PASS: 9 tests PASS, glm 83.8% coverage
- [x] COMMIT: `7202de4` feat(llm/provider/glm): Z.ai GLM 어댑터

### M5 — DefaultRegistry 업데이트

- [x] RED: `registry_test.go` 업데이트 (6→15 AdapterReady), 신규 검증 테스트
- [x] GREEN: `registry.go` GLM endpoint 이전 + 9 신규 provider + 3 AdapterReady 전환
- [x] GREEN: `factory/registry_builder.go` 신규 (RegisterAllProviders, import cycle 방지)
- [x] GREEN: `factory/registry_defaults.go` SPEC-002 9개 케이스 추가
- [x] PASS: 전체 PASS, router 97.2%, factory 77.0%
- [x] COMMIT: `4d39a6c` feat(llm): DefaultRegistry 15 provider adapter-ready 완성

## Phase 3: Final State

| Metric | Value |
|--------|-------|
| 신규 파일 | 20개 (prod 10 + test 10) |
| 신규 provider | 9종 |
| 전체 provider (AdapterReady) | 15종 |
| 테스트 수 (SPEC-002 신규) | 30+ |
| go build 에러 | 0 |
| go vet 경고 | 0 |
| go test -race 결과 | 21 패키지 전부 PASS |
| GLM 커버리지 | 83.8% |
| factory 커버리지 | 77.0% |
| router 커버리지 | 97.2% |

## AC Coverage

| AC ID | 설명 | 상태 |
|-------|------|------|
| AC-ADP2-001 | GLM 기본 streaming (thinking off) | GREEN |
| AC-ADP2-002 | GLM thinking mode on (GLM-4.6) | GREEN |
| AC-ADP2-003 | GLM thinking graceful degradation | GREEN |
| AC-ADP2-004 | Groq streaming | GREEN |
| AC-ADP2-005 | OpenRouter ranking 헤더 주입 | GREEN |
| AC-ADP2-006 | Together streaming | GREEN |
| AC-ADP2-007 | Fireworks streaming | GREEN |
| AC-ADP2-008 | Cerebras streaming | GREEN |
| AC-ADP2-009 | Mistral streaming | GREEN |
| AC-ADP2-010 | Qwen 기본 intl region | GREEN |
| AC-ADP2-011 | Qwen cn 환경변수 | GREEN |
| AC-ADP2-012 | Qwen 잘못된 region 거부 | GREEN |
| AC-ADP2-013 | Kimi 장문 context (구현 범위 외 — REQ Optional) | N/A |
| AC-ADP2-014 | Kimi cn region | GREEN |
| AC-ADP2-015 | Vision 미지원 거부 (SPEC-001 ErrCapabilityUnsupported 재사용) | N/A (인프라) |
| AC-ADP2-016 | DefaultRegistry 15 provider AdapterReady | GREEN |
| AC-ADP2-017 | RegisterAllProviders 무에러 | GREEN |
| AC-ADP2-018 | Provider 이름 중복 거부 | GREEN |

## 다음 단계

- `/moai sync SPEC-GOOSE-ADAPTER-002` — PR 생성 및 문서 동기화
