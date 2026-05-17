# Acceptance — SPEC-MINK-MSG-SLACK-001

총 26 AC (M1 3 + M2 7 + M3 3 + M4 13). REQ ↔ AC traceable (1:N).

## §1 M1 (3 AC)
- **SLK-001 [REQ-SLK-001/P0]**: HMAC verify — 위조 서명 시 HTTP 401, Verify: unit
- **SLK-002 [REQ-SLK-002/P0]**: 3초 SLA — handler entry → response ≤ 2.8s p95, Verify: bench
- **SLK-008 [REQ-SLK-008/P0]**: url_verification challenge — type=url_verification 시 challenge 응답 ≤ 3s, Verify: integration

## §2 M2 (7 AC)
- **SLK-003 [REQ-SLK-003/P0]**: LLM-ROUTING-V2-AMEND 위임 — Router.Route 호출 (mock 검증), Verify: integration
- **SLK-006 [REQ-SLK-006/P1]**: BRIDGE canonical normalize — user_id/channel_id/team_id/text/ts/thread_ts, Verify: unit
- **SLK-009 [REQ-SLK-009/P0]**: app_mention 트리거 — @bot mention 시 LLM 호출, Verify: integration
- **SLK-010 [REQ-SLK-010, REQ-SLK-016/P0]**: message.im DM 처리 — DM 시 mention 불필요 (audit D8 fix: REQ-SLK-016 state-driven DM 매핑 추가), Verify: integration
- **SLK-011 [REQ-SLK-011/P0]**: slash command — `/mink prompt` ack ≤ 3s + chat.postMessage, Verify: integration
- **SLK-015 [REQ-SLK-015/P0]**: placeholder → update — "MINK is thinking..." 후 chat.update final, Verify: integration
- **SLK-017 [REQ-SLK-017/P1]**: public channel mention 강제 — @mention 없는 일반 메시지 응답 0, Verify: integration

## §3 M3 (3 AC)
- **SLK-004 [REQ-SLK-004/P0]**: AUTH-CREDENTIAL 위임 — signing_secret + bot_token 평문 0, Verify: unit
- **SLK-012 [REQ-SLK-012/P1]**: OAuth installation — workspace_id 키로 Store, Verify: integration
- **SLK-016 [REQ-SLK-025/P1]**: OAuth state token — CSRF 검증, single-use, ≤5분 TTL (audit D1 fix: REQ-016→REQ-025 신규), Verify: unit

## §4 M4 (13 AC)
- **SLK-005 [REQ-SLK-005/P1]**: AGPL 헤더 — 신규 .go grep 0 missing, Verify: CI
- **SLK-007 [REQ-SLK-007/P2]**: MEMORY-QMD 옵션 색인 — sessions/ collection PII redact 통과, Verify: integration
- **SLK-013 [REQ-SLK-013/P1]**: Block Kit Buttons/Select 처리 — payload parse → service dispatch, Verify: unit
- **SLK-014 [REQ-SLK-014/P2]**: rate limit 429 Retry-After 준수 — bench, Verify: integration
- **SLK-018 [REQ-SLK-018/P2]**: thread_ts threading — thread 안 reply, Verify: integration
- **SLK-019 [REQ-SLK-019/P0]**: unsolicited message 0 — 1h monitor, Verify: e2e
- **SLK-020 [REQ-SLK-020/P0]**: PII 마스킹 — 이메일/전화 redact 통과 후 색인, Verify: integration
- **SLK-021 [REQ-SLK-021/P1]**: sync 호출 금지 — ack 후 async post 만, Verify: unit (race detect)
- **SLK-022 [REQ-SLK-022/P1]**: chat.postMessage retry max 3 + backoff, Verify: integration
- **SLK-023 [REQ-SLK-023/P2,OPT]**: workspace admin memory share — `/mink-memory search`, Verify: integration
- **SLK-024 [REQ-SLK-024/P2,OPT]**: MINK_SLACK_ALLOWED_CHANNELS env 필터 — 무관 채널 이벤트 ignore, Verify: integration
- **SLK-025 [REQ-SLK-024 보강/P0]**: go vet+golangci-lint CI gate clean, Verify: CI
- **SLK-026 [REQ-SLK-005 보강/P1]**: gofmt+lint 신규 파일 clean, Verify: CI

## §5 DoD
26/26 GREEN, coverage ≥ 85%, AGPL 0 missing, 3초 SLA 위반 0, plan-auditor pass.
