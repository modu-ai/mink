# DEPRECATED

> **본 SPEC은 ROADMAP v2.0 재설계(2026-04-21)로 폐기됨.**

## 폐기 이유

v1.0 SPEC-GOOSE-LLM-001은 Ollama 단일 어댑터를 전제했으나, v2.0에서 사용자 요구사항("모든 LLM을 API 또는 OAuth로 연결")에 따라 **15+ provider credential pool 기반 구조**로 전면 재설계.

## 대체 SPEC (v2.0)

본 SPEC은 3개 SPEC으로 분할되어 재작성 예정:

- **SPEC-GOOSE-CREDPOOL-001** (Phase 1) — Credential Pool (OAuth/API, 4 strategy, rotation)
- **SPEC-GOOSE-ROUTER-001** (Phase 1) — Smart Model Routing + Provider Registry
- **SPEC-GOOSE-ADAPTER-001** (Phase 1) — 6 Provider 어댑터 (Anthropic/OpenAI/Google/xAI/DeepSeek/Ollama)

추가 연계:
- **SPEC-GOOSE-RATELIMIT-001** (Phase 1) — Rate Limit Tracker
- **SPEC-GOOSE-PROMPT-CACHE-001** (Phase 1) — Prompt Caching

## 참조

- `.moai/specs/ROADMAP.md` (v2.0)
- `.moai/project/research/hermes-llm.md` (credential pool 원형 분석)

## 이전 내용 복구

본 디렉토리의 spec.md / research.md는 보존. git history 참조.
