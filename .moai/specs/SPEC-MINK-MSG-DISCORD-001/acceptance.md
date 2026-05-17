# Acceptance — SPEC-MINK-MSG-DISCORD-001

총 24 AC (M1 4 + M2 5 + M3 3 + M4 12). REQ ↔ AC traceable (1:N). Verify: unit / integration / e2e / manual.

## §1 M1 (4 AC)

### AC-DCD-001 [REQ-DCD-001 / P0] — Ed25519 서명 검증
- **Given**: Discord 발급 Ed25519 public key 등록
- **When**: 위조된 서명 헤더 + 정상 body 의 inbound POST
- **Then**: HTTP 401 응답, body 처리 안 됨
- **Verify**: unit (crypto/ed25519 mock)

### AC-DCD-002 [REQ-DCD-002 / P0] — 3초 SLA
- **Given**: 정상 inbound Interaction
- **When**: handler entry → first response
- **Then**: 응답 latency p95 ≤ 2.8초 (3초 SLA 안전 마진)
- **Verify**: integration (load test, 100 req)

### AC-DCD-007 [REQ-DCD-007 / P0] — PING/PONG
- **Given**: type 1 (PING) interaction
- **When**: handler 처리
- **Then**: type 1 (PONG) 응답, 3초 이내
- **Verify**: integration

### AC-DCD-011 [REQ-DCD-011 / P1] — 서명 실패 시 401
- **Given**: 서명 verify 실패
- **When**: handler 응답
- **Then**: HTTP 401 (not 500), discord retry 안 트리거
- **Verify**: unit

## §2 M2 (5 AC)

### AC-DCD-003 [REQ-DCD-003 / P0] — LLM 위임
- **Given**: APPLICATION_COMMAND interaction
- **When**: handler 처리
- **Then**: LLM-ROUTING-V2-AMEND-001 Router.Route 호출, 직접 provider API 호출 0
- **Verify**: integration (mock router)

### AC-DCD-006 [REQ-DCD-006 / P1] — BRIDGE-001 canonical schema
- **Given**: Discord-specific payload
- **When**: normalize 호출
- **Then**: BRIDGE-001 canonical schema (user_id, channel_id, guild_id, text, timestamp, thread_id) 생성
- **Verify**: unit

### AC-DCD-008 [REQ-DCD-008 / P0] — APPLICATION_COMMAND + deferred
- **Given**: APPLICATION_COMMAND interaction
- **When**: handler 처리
- **Then**: type 5 (DEFERRED_CHANNEL_MESSAGE_WITH_SOURCE) 응답 3초 이내, follow-up 비동기 post
- **Verify**: integration

### AC-DCD-010 [REQ-DCD-010 / P1] — slash command 등록
- **Given**: bot 신규 install
- **When**: 등록 API 호출
- **Then**: applications.{app_id}.commands 에 정의된 모든 slash command 등록
- **Verify**: integration (mock Discord REST)

### AC-DCD-013 [REQ-DCD-013 / P0] — webhook follow-up
- **Given**: deferred response 후 LLM 응답 완료
- **When**: webhook endpoint 호출
- **Then**: final 메시지 채널에 표시
- **Verify**: integration

## §3 M3 (3 AC)

### AC-DCD-009 [REQ-DCD-009 / P0] — MESSAGE_COMPONENT parsing
- **Given**: type 3 interaction with custom_id
- **When**: handler parsing
- **Then**: custom_id 추출 + 해당 service action 호출
- **Verify**: unit

### AC-DCD-014 [REQ-DCD-014 / P1] — DM 채널
- **Given**: DM 채널 interaction
- **When**: 응답 처리
- **Then**: slash command 또는 @mention 없이 응답
- **Verify**: integration

### AC-DCD-022 [REQ-DCD-022 / P2, OPT] — MINK_DISCORD_ALLOWED_GUILDS
- **Given**: `MINK_DISCORD_ALLOWED_GUILDS=guild_a,guild_b` 설정
- **When**: 다른 guild interaction 도착
- **Then**: 무시 + 로그
- **Verify**: integration

## §4 M4 (12 AC)

### AC-DCD-004 [REQ-DCD-004 / P0] — AUTH-CREDENTIAL 위임
- **Given**: Ed25519 public key + bot token 등록
- **When**: handler 시작
- **Then**: AuthCredentialService.Load 호출, 평문 config 0
- **Verify**: unit

### AC-DCD-005 [REQ-DCD-005 / P1] — AGPL 헤더
- **Given**: 본 SPEC 신규 .go
- **When**: `grep -L "AGPL-3.0-only"`
- **Then**: 0 missing
- **Verify**: CI

### AC-DCD-012 [REQ-DCD-012 / P2] — Rate limit Retry-After
- **Given**: Discord 429 응답
- **When**: handler 처리
- **Then**: Retry-After 헤더 값만큼 대기 후 1회 재시도
- **Verify**: integration

### AC-DCD-015 [REQ-DCD-015 / P1] — server 채널 @mention/slash
- **Given**: server channel
- **When**: bot @mention 없는 일반 메시지
- **Then**: 응답 안 함
- **Verify**: integration

### AC-DCD-016 [REQ-DCD-016 / P1] — Bot invite link
- **Given**: invite link 생성 요청
- **When**: 생성
- **Then**: scopes=bot+applications.commands, 권한 비트 명시
- **Verify**: unit

### AC-DCD-017 [REQ-DCD-017 / P0] — PII 마스킹
- **Given**: inbound payload 에 이메일/전화 포함
- **When**: MEMORY-QMD-001 색인 (opt-in)
- **Then**: PII redact pipeline 통과 후 색인
- **Verify**: integration

### AC-DCD-018 [REQ-DCD-018 / P1] — unsolicited message 0
- **Given**: 사용자 trigger 없음
- **When**: 1시간 모니터
- **Then**: bot 자발적 메시지 0
- **Verify**: e2e

### AC-DCD-019 [REQ-DCD-019 / P1] — webhook follow-up retry max 3
- **Given**: webhook 실패 (5xx)
- **When**: retry
- **Then**: 최대 3회, exponential backoff
- **Verify**: integration

### AC-DCD-020 [REQ-DCD-020 / P2, OPT] — Gateway opt-in
- **Given**: `MINK_DISCORD_GATEWAY=1`
- **When**: bot 시작
- **Then**: WebSocket connection 활성
- **Verify**: integration (Gateway mock)

### AC-DCD-021 [REQ-DCD-021 / P2, OPT] — Ephemeral message
- **Given**: `/mink-private` slash command
- **When**: 응답
- **Then**: ephemeral flag (64) 설정
- **Verify**: integration

### AC-DCD-023 [REQ-DCD-002 보강 / P0] — 3초 SLA Gateway 옵션 검증
- **Given**: HTTP-only 모드 (default)
- **When**: 100 req 부하 테스트
- **Then**: SLA 위반 0건
- **Verify**: bench

### AC-DCD-024 [REQ-DCD-005 보강 / P1] — gofmt+lint 신규 파일
- **Given**: 본 SPEC 신규 .go
- **When**: gofmt -l + golangci-lint
- **Then**: clean
- **Verify**: CI

## §5 Definition of Done

- 24/24 AC GREEN
- coverage ≥ 85% (internal/channel/discord/)
- AGPL 헤더 누락 0
- 3초 SLA 위반 0 (bench)
- plan-auditor pass
