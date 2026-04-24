---
spec: SPEC-GOOSE-ADAPTER-002
updated: 2026-04-24
---

# SPEC-GOOSE-ADAPTER-002 Atomic Tasks

## M1 — Simple Factory 3종

| ID | Task | Status | File |
|----|------|--------|------|
| T-001 | groq/client.go 팩토리 구현 | DONE | `internal/llm/provider/groq/client.go` |
| T-002 | groq/client_test.go RED-GREEN | DONE | `internal/llm/provider/groq/client_test.go` |
| T-003 | cerebras/client.go 팩토리 구현 | DONE | `internal/llm/provider/cerebras/client.go` |
| T-004 | cerebras/client_test.go RED-GREEN | DONE | `internal/llm/provider/cerebras/client_test.go` |
| T-005 | mistral/client.go 팩토리 구현 | DONE | `internal/llm/provider/mistral/client.go` |
| T-006 | mistral/client_test.go RED-GREEN | DONE | `internal/llm/provider/mistral/client_test.go` |
| T-007 | M1 go build/vet/test -race PASS | DONE | — |
| T-008 | M1 commit | DONE | `f11b524` |

## M2 — ExtraHeaders 활용 3종

| ID | Task | Status | File |
|----|------|--------|------|
| T-009 | openrouter/client.go ExtraHeaders 주입 구현 | DONE | `internal/llm/provider/openrouter/client.go` |
| T-010 | openrouter/client_test.go 3 tests (BaseURL/헤더주입/빈옵션) | DONE | `internal/llm/provider/openrouter/client_test.go` |
| T-011 | together/client.go 팩토리 구현 | DONE | `internal/llm/provider/together/client.go` |
| T-012 | together/client_test.go RED-GREEN | DONE | `internal/llm/provider/together/client_test.go` |
| T-013 | fireworks/client.go 팩토리 구현 | DONE | `internal/llm/provider/fireworks/client.go` |
| T-014 | fireworks/client_test.go RED-GREEN | DONE | `internal/llm/provider/fireworks/client_test.go` |
| T-015 | M2 go build/vet/test -race PASS | DONE | — |
| T-016 | M2 commit | DONE | `bc5658f` |

## M3 — Region 선택 2종

| ID | Task | Status | File |
|----|------|--------|------|
| T-017 | qwen/client.go Region 로직 + ErrInvalidRegion | DONE | `internal/llm/provider/qwen/client.go` |
| T-018 | qwen/client_test.go 4 tests (default/cn/envvar/invalid) | DONE | `internal/llm/provider/qwen/client_test.go` |
| T-019 | kimi/client.go Region 로직 + ErrInvalidRegion | DONE | `internal/llm/provider/kimi/client.go` |
| T-020 | kimi/client_test.go 4 tests | DONE | `internal/llm/provider/kimi/client_test.go` |
| T-021 | M3 go build/vet/test -race PASS | DONE | — |
| T-022 | M3 commit | DONE | `3ccea04` |

## M4 — GLM thinking mode

| ID | Task | Status | File |
|----|------|--------|------|
| T-023 | glm/thinking.go BuildThinkingField + ThinkingCapableModels | DONE | `internal/llm/provider/glm/thinking.go` |
| T-024 | glm/thinking_test.go 5 tests | DONE | `internal/llm/provider/glm/thinking_test.go` |
| T-025 | glm/adapter.go *openai.OpenAIAdapter embedding + Stream/Complete override | DONE | `internal/llm/provider/glm/adapter.go` |
| T-026 | glm/adapter_test.go 5 tests (BaseURL/thinking inject/degradation/preserve/complete) | DONE | `internal/llm/provider/glm/adapter_test.go` |
| T-027 | M4 go build/vet/test -race PASS | DONE | — |
| T-028 | M4 commit | DONE | `7202de4` |

## M5 — DefaultRegistry 업데이트

| ID | Task | Status | File |
|----|------|--------|------|
| T-029 | router/registry.go GLM endpoint Z.ai 이전 | DONE | `internal/llm/router/registry.go` |
| T-030 | router/registry.go GLM DisplayName "Z.ai GLM", 5 suggested models | DONE | same |
| T-031 | router/registry.go groq/mistral/openrouter/qwen/kimi AdapterReady=true | DONE | same |
| T-032 | router/registry.go together/fireworks/cerebras 신규 등록 | DONE | same |
| T-033 | router/registry_test.go 6→15 AdapterReady + 신규 테스트 | DONE | `internal/llm/router/registry_test.go` |
| T-034 | factory/registry_builder.go RegisterAllProviders 신규 | DONE | `internal/llm/factory/registry_builder.go` |
| T-035 | factory/registry_builder_test.go 3 tests | DONE | `internal/llm/factory/registry_builder_test.go` |
| T-036 | factory/registry_defaults.go SPEC-002 9개 케이스 추가 | DONE | `internal/llm/factory/registry_defaults.go` |
| T-037 | factory/registry_defaults_test.go SPEC-002 provider 테스트 추가 | DONE | same |
| T-038 | M5 go build/vet/test -race PASS | DONE | — |
| T-039 | M5 commit | DONE | `4d39a6c` |

## M6 — 문서

| ID | Task | Status | File |
|----|------|--------|------|
| T-040 | progress.md 생성 | DONE | `.moai/specs/SPEC-GOOSE-ADAPTER-002/progress.md` |
| T-041 | tasks.md 생성 | DONE | `.moai/specs/SPEC-GOOSE-ADAPTER-002/tasks.md` |
| T-042 | M6 commit | DONE | — |

## Summary

| Category | Count |
|----------|-------|
| 총 Task | 42 |
| DONE | 42 |
| 신규 Go 파일 (prod) | 10 |
| 신규 Go 파일 (test) | 10 |
| 수정 파일 | 4 |
| 신규 provider 어댑터 | 9 |
| 전체 provider (AdapterReady) | 15 |
