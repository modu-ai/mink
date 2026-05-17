# Acceptance — SPEC-MINK-ONBOARDING-001-PHASE5-AMEND-001

총 28 AC (M1 10 + M2 3 + M3 5 + M4 10). REQ↔AC traceable (1:N).

## §1 M1 (10 AC)
- **OB5-001 [REQ-ONB5-001/P0]**: 5 provider 카드 순서 (Anthropic/DeepSeek/OpenAI/Codex/GLM), Verify: unit
- **OB5-002 [REQ-ONB5-002/P0]**: 인증 mode + 상태 indicator 각 카드, Verify: e2e
- **OB5-003 [REQ-ONB5-003/P0]**: AUTH-CREDENTIAL 위임 — Web layer 평문 0, Verify: unit
- **OB5-004 [REQ-ONB5-004/P0]**: LLM-ROUTING test endpoint 호출 — Web 직접 API 0, Verify: integration
- **OB5-008 [REQ-ONB5-008/P0]**: 4 provider key paste 새 탭 — provider URL 정확, Verify: integration
- **OB5-009 [REQ-ONB5-009/P0]**: paste form 검증 + 저장 — regex + test API + AUTH Store, Verify: integration
- **OB5-016 [REQ-ONB5-016/P0]**: validation 성공 시 Next 활성, Verify: e2e
- **OB5-017 [REQ-ONB5-017/P0]**: 0 validation 시 Next 비활성, Verify: e2e
- **OB5-020 [REQ-ONB5-020/P0]**: credential 로그 mask — `sk-****`, Verify: unit
- **OB5-021 [REQ-ONB5-021/P0]**: localStorage/sessionStorage/cookies 에 credential 0, Verify: integration

## §2 M2 (3 AC)
- **OB5-010 [REQ-ONB5-010/P0]**: OAuth PKCE — code_verifier random + code_challenge SHA-256 + state token, Verify: unit
- **OB5-011 [REQ-ONB5-011/P0]**: OAuth callback 검증 + token exchange + refresh Store, Verify: integration
- **OB5-018 [REQ-ONB5-018/P1]**: Codex 카드 spinner 60s timeout, Verify: e2e

## §3 M3 (5 AC)
- **OB5-006 [REQ-ONB5-006/P1]**: 3 채널 카드 (Telegram/Slack/Discord), Verify: unit
- **OB5-013 [REQ-ONB5-013/P1]**: Telegram bot token paste + `/start` 발견 ping, Verify: integration
- **OB5-014 [REQ-ONB5-014/P1]**: Slack OAuth v2 — workspace 선택 redirect, Verify: integration
- **OB5-015 [REQ-ONB5-015/P1]**: Discord bot invite + Ed25519 입력 + 권한 scope, Verify: integration
- **OB5-023 [REQ-ONB5-023/P1]**: Step 3 optional — 채널 연결 없어도 installation 완료, Verify: e2e

## §4 M4 (10 AC)
- **OB5-005 [REQ-ONB5-005/P1]**: AGPL 헤더 신규 .go/.tsx grep 0 missing, Verify: CI
- **OB5-007 [REQ-ONB5-007/P1]**: CSRF + Origin allowlist 모든 요청, Verify: integration
- **OB5-012 [REQ-ONB5-012/P0]**: drag-and-drop priority 저장 — LLM-ROUTING config, Verify: e2e
- **OB5-019 [REQ-ONB5-019/P2]**: dev mode mocked credential 허용 — `MINK_DEV=1`, Verify: integration
- **OB5-022 [REQ-ONB5-022/P1]**: 동시 provider validation race 0 — per-provider mutex, Verify: integration
- **OB5-024 [REQ-ONB5-024/P2,OPT]**: custom OpenAI-compat endpoint 추가 카드, Verify: integration
- **OB5-025 [REQ-ONB5-025/P2,OPT]**: brand theme 적용, Verify: e2e visual
- **OB5-026 [REQ-ONB5-026/P2,OPT]**: MINK_ONBOARDING_LANG env 적용, Verify: integration
- **OB5-027 [REQ-ONB5-005 보강/P0]**: go vet + golangci-lint + tsc --noEmit + eslint CI clean, Verify: CI
- **OB5-028 [REQ-ONB5-005 보강/P1]**: gofmt/prettier 신규 파일 clean, Verify: CI

## §5 DoD
28/28 GREEN, coverage ≥ 85% (internal/server/install/, web/install/src/Step2/Step3), Playwright E2E PASS (5 provider mock + OAuth callback mock + 3 channel), plan-auditor pass.
