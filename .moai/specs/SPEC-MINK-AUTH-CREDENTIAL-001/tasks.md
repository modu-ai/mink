---
spec_id: SPEC-MINK-AUTH-CREDENTIAL-001
artifact: tasks.md
version: 0.2.0
created_at: 2026-05-16
updated_at: 2026-05-16
author: MoAI manager-spec
---

# Tasks — SPEC-MINK-AUTH-CREDENTIAL-001

총 18 tasks (M1: 5 / M2: 4 / M3: 5 / M4: 4). 각 task 는 Go 패키지 경로, GREEN 기준 AC, 의존 task, 산출 파일을 명시한다. priority label 만 사용 — 시간 추정 없음 (agent-common-protocol §Time Estimation).

---

## 0. 패키지 ↔ Task 매핑 (overview)

| 패키지 | tasks |
|-------|-------|
| `internal/auth/credential/` | T-001, T-005, T-013 |
| `internal/auth/keyring/` | T-002, T-003, T-004 |
| `internal/auth/file/` | T-006, T-009 |
| `internal/auth/dispatch/` | T-007, T-008 |
| `internal/auth/oauth/` | T-010, T-011, T-012 |
| `internal/llmrouter/` (consumer wiring) | T-014 |
| `internal/cli/` (Cobra commands) | T-015, T-016 |
| `internal/userdata/migrate/` | T-015 |
| `.moai/docs/`, `.moai/state/` | T-017, T-018 |

---

## 1. M1 — keyring abstraction (High priority)

### T-001 — Service interface + sentinels + masking helper

**패키지**: `internal/auth/credential/`

**작업**:
- `service.go`: `Service` interface 정의 (Store / Load / Delete / List / Health)
- `errors.go`: `KeyringUnavailable`, `NotFound`, `SchemaViolation`, `ReAuthRequired` sentinel 에러
- `mask.go`: `MaskedString(s string) string` (≥5 chars: `***LAST4`, <5 chars: `***`)
- 라이선스 헌장 (`.moai/project/dependency-licenses.md` 또는 동등 파일) 에 `zalando/go-keyring` v0.2.x / `godbus/godbus/v5` / `x/oauth2` 항목 추가

**의존**: 없음 (M1 진입 task)

**GREEN AC**: AC-CR-001 (UB-1 interface), AC-CR-007 (UB-7 identifier format — 부분), AC-CR-024 (UN-1 masking)

### T-002 — KeyringBackend (go-keyring wrapper)

**패키지**: `internal/auth/keyring/`

**작업**:
- `backend.go`: `KeyringBackend` struct, Service interface 구현
- `mink:auth:{provider}` service identifier 매핑 + account `default` 고정
- macOS / Linux / Windows 통합 테스트 매트릭스 (CI runner)
- timeout context (default 5s) 적용 — R8 완화

**의존**: T-001

**GREEN AC**: AC-CR-002 (UB-2 keyring write path), AC-CR-005 (UB-7 service identifier), AC-CR-009 (UB-9 cross-platform), AC-CR-013 (ED-1 store path), AC-CR-014 (ED-2 load in-memory)

### T-003 — KeyringUnavailable probe + auto-detect

**패키지**: `internal/auth/keyring/`

**작업**:
- `probe.go`: D-Bus / libsecret / Wincred 가용성 detection
  - Linux: `DBUS_SESSION_BUS_ADDRESS` env + Secret Service object probe
  - macOS: `SecKeychainGetStatus` 결과 / Keychain 잠금 상태
  - Windows: Wincred API ping
- KeyringUnavailable sentinel 반환 path 정의

**의존**: T-002

**GREEN AC**: AC-CR-020 (SD-1 keyring unavailable sentinel), AC-CR-006 (UB-6 backend detection — 부분, M2 에서 보강)

### T-004 — Health(provider) API + mink doctor auth-keyring subcommand

**패키지**: `internal/auth/keyring/`, `internal/cli/`

**작업**:
- `Health(provider)` 구현: presence + masked LAST4 + 백엔드 식별자 반환 (token plaintext 누설 금지)
- CLI: `mink doctor auth-keyring` — 8 credential 상태 + 활성 백엔드 표시

**의존**: T-002, T-003

**GREEN AC**: AC-CR-008 (UB-8 health API), AC-CR-031 (`mink doctor auth-keyring` 출력 검증, UB-8 + UB-9 결합), AC-CR-015 (ED-3 logout idempotent — 부분, T-005 에서 GREEN)

### T-005 — Delete(provider) + List() 구현 + M1 통합 테스트

**패키지**: `internal/auth/credential/`, `internal/auth/keyring/`

**작업**:
- KeyringBackend 의 `Delete` 구현 (없는 항목 idempotent)
- `List()` 구현 — 등록된 provider id 목록 반환 (값은 반환 금지, UN-1)
- M1 통합 테스트: Anthropic API key 단일 provider 로 Store → Load → List → Delete round-trip
- macOS Keychain unlock prompt headless 시나리오 (SSH session 환경 mock) → KeyringUnavailable 반환 검증

**의존**: T-001, T-002, T-003

**GREEN AC**: AC-CR-003 (UB-3 AGPL 정합 — 부분, T-014 에서 완전), AC-CR-015 (ED-3 logout idempotent), AC-CR-007 (UB-7 identifier format), AC-CR-001 (UB-1 interface 전체)

---

## 2. M2 — 평문 fallback + auto-detect + CLI config (High priority)

### T-006 — FileBackend + atomic write + mode 0600 검증

**패키지**: `internal/auth/file/`

**작업**:
- `backend.go`: `FileBackend` struct, Service interface 구현
- atomic write: `os.WriteFile(temp, data, 0600)` → `os.Rename(temp, final)`
- `perms.go`: POSIX mode 검증 (`os.Stat` mode bits) + Windows ACL 검증
- JSON schema (research.md §4.2) 직렬화 / 역직렬화
- 부모 디렉터리 자동 생성 (`~/.mink/auth/`, mode 0700)

**의존**: T-001

**GREEN AC**: AC-CR-004 (UB-2 file fallback path), AC-CR-026 (UN-2 git-tracked 차단), AC-CR-027 (UN-6 mode 0600 검증)

### T-007 — Dispatcher (config-driven backend selection)

**패키지**: `internal/auth/dispatch/`

**작업**:
- `dispatcher.go`: config 읽기 (`auth.store: keyring | file | keyring,file`)
- backend 선택 로직 + auto-fallback (SD-1):
  - `keyring`: keyring only, 실패 시 error 반환
  - `file`: file only, keyring 호출 없음
  - `keyring,file`: keyring 시도 → KeyringUnavailable 시 file
- KeyringBackend / FileBackend 모두 Service interface 충족 → 동일 method 호출

**의존**: T-002, T-006

**GREEN AC**: AC-CR-021 (SD-2 auth.store: file bypass), AC-CR-022 (SD-1 auto-fallback realized)

### T-008 — CLI: `mink config set auth.store {keyring|file|keyring,file}`

**패키지**: `internal/cli/`

**작업**:
- Cobra subcommand 등록
- config 값 검증 (3 가지만 허용 + OP placeholder `hsm`, `op-cli` 는 `NotImplemented` 안내 후 reject)
- 변경 시 dispatcher reload

**의존**: T-007

**GREEN AC**: AC-CR-021 (SD-2 — CLI 경로), AC-CR-029 (OP-1 placeholder), AC-CR-030 (OP-2 placeholder)

### T-009 — M2 통합 테스트 + 클라우드 폴더 감지 경고

**패키지**: `internal/auth/file/`, `internal/auth/dispatch/`

**작업**:
- 통합 테스트: file 단독 / dispatcher routing / auto-fallback 전환 round-trip
- 클라우드 동기화 폴더 경로 패턴 감지 (`iCloud Drive` / `OneDrive` / `Dropbox` / `Google Drive` 하위)
- 감지 시 stderr 에 경고 출력 (write 자체는 진행, UN-2 위반 아님 — 사용자 인지 강화 목적)

**의존**: T-006, T-007

**GREEN AC**: AC-CR-006 (UB-6 schema validation 완전), AC-CR-022 (SD-1 fallback 시나리오)

---

## 3. M3 — 8 credential schema + Codex OAuth + LLM-ROUTING-V2 wiring (High priority)

### T-010 — Codex OAuth PKCE flow (`mink login codex`)

**패키지**: `internal/auth/oauth/`

**작업**:
- `codex.go`: `golang.org/x/oauth2` 기반 PKCE (S256) flow
- 127.0.0.1:random_port 로컬 HTTP listener (callback 수신)
- 사용자 브라우저 launch (`mink login codex` CLI 가 launch URL 출력 + 자동 open 시도)
- authorization_code → access_token + refresh_token 교환
- Store 호출로 `OAuthToken` credential 저장

**의존**: T-007 (dispatcher 필요)

**GREEN AC**: AC-CR-016 (ED-4 refresh auto-trigger — initial flow), AC-CR-008 (UB-8 — Codex health 부분)

### T-011 — Auto-refresh + 60초 safety margin

**패키지**: `internal/auth/oauth/`

**작업**:
- `refresh.go`: `Load(codex)` 호출 시 `expires_at < now() + 60s` 검사
- 만료 임박 → POST `https://auth.openai.com/oauth/token` (grant_type=refresh_token)
- refresh_token rotation 응답 처리 (새 refresh_token 발급 시 갱신)
- Store 로 갱신된 토큰 persist

**의존**: T-010

**GREEN AC**: AC-CR-016 (ED-4 refresh auto-trigger 완전), AC-CR-018 (SD-4 silent refresh)

### T-012 — invalid_grant detection + ReAuthRequired sentinel + 8일 idle 시나리오

**패키지**: `internal/auth/oauth/`, `internal/cli/`

**작업**:
- refresh 응답 HTTP 400 `invalid_grant` 또는 HTTP 401 → `ReAuthRequired(codex)` sentinel 반환
- 다른 에러 (네트워크 / 5xx) 와 구분 → retry 가능 분류
- CLI: ReAuthRequired sentinel 캐치 시 `mink login codex` 재실행 안내 출력
- 통합 테스트: `httptest.Server` 4 시나리오 (정상 / rotation / invalid_grant / 5xx)

**의존**: T-011

**GREEN AC**: AC-CR-017 (ED-5 re-auth prompt), AC-CR-025 (UN-4 expired token 차단)

### T-013 — 8 credential schema (5 LLM + 3 채널) + Validate

**패키지**: `internal/auth/credential/`

**작업**:
- `schema.go`: 5 종 Credential 구현 — `APIKey`, `OAuthToken`, `BotToken`, `SlackCombo`, `DiscordCombo`
- 각 type 의 `Validate()` 메서드 (research.md §4.2 스키마 준수)
- 8 provider id 등록표: `anthropic`, `deepseek`, `openai_gpt`, `codex`, `zai_glm`, `telegram_bot`, `slack`, `discord`
- 잘못된 provider id → `SchemaViolation`
- 잘못된 kind / 필드 → `SchemaViolation`

**의존**: T-001, T-006 (file backend 의 JSON 직렬화와 정합)

**GREEN AC**: AC-CR-006 (UB-6 schema validation 완전 — 8 종), AC-CR-011 (UB-4 single account), AC-CR-028 (UN-3 multi-account 차단 = overwrite)

### T-014 — LLM-ROUTING-V2 wiring + 헌장 §2 / ADR-001 정합 회귀

**패키지**: `internal/llmrouter/` (LLM-ROUTING-V2 의 기존 패키지 확장)

**작업**:
- 5 LLM provider 의 `Load(provider)` 호출 통합
- `NotFound` 반환 시 라우팅에서 해당 provider skip (SD-3 graceful degradation)
- 회귀 테스트: LLM-ROUTING-V2 의 기존 통합 테스트 모두 PASS
- AGPL §2 헌장 정합 검증: credential 코드 경로가 모델 학습 흐름과 격리됨을 grep / static analysis 로 확인 (no import from `internal/model/training/` 등)
- ADR-001 정합 검증: refresh_token 이 telemetry / metrics 경로로 송신 안 됨 grep (UN-5)

**의존**: T-013

**GREEN AC**: AC-CR-003 (UB-3 AGPL 헌장), AC-CR-010 (UB-5 + UN-5 결합 — zero-weight retention + telemetry plaintext 차단), AC-CR-023 (SD-3 graceful degradation)

---

## 4. M4 — USERDATA-MIGRATE 통합 + CLI 완성 + OP placeholder (Medium priority)

### T-015 — USERDATA-MIGRATE export/import 통합

**패키지**: `internal/userdata/migrate/`

**작업**:
- export schema 확장: `credentials` 키 추가 (USERDATA-MIGRATE-001 spec.md 의 schema 확장 필요 시 amendment SPEC 발행)
- export 시 사용자 동의 prompt (yes/no per export, default no)
- import 시 keyring 우선 저장 + dispatcher 분기
- 통합 테스트: round-trip (export → 새 머신 import) 시나리오 (mock keyring 사용)
- USERDATA-MIGRATE-001 회귀 테스트 PASS 검증

**의존**: T-013, T-014

**GREEN AC**: AC-CR-019 (ED-6, ED-7 export/import)

### T-016 — CLI 완성 (`mink login` 8종, `mink logout`)

**패키지**: `internal/cli/`

**작업**:
- `mink login anthropic|deepseek|openai_gpt|zai_glm` — API key 대화형 입력 + Store
- `mink login codex` — T-010 의 OAuth flow 진입
- `mink login telegram_bot` — bot token 입력
- `mink login slack` — signing_secret + bot_token 2단계 입력
- `mink login discord` — public_key + bot_token 2단계 입력
- `mink logout {provider}` — Delete (keyring + file 둘 다 sweep)
- 입력 echo off (terminal raw mode) — stdout 노출 방지 (UN-1 보강)

**의존**: T-013, T-015

**GREEN AC**: AC-CR-012 (UB-4 — CLI 경로), AC-CR-015 (ED-3 logout 완전), AC-CR-032 (OP-3 manual sync via export)

### T-017 — 사용자 문서 `.moai/docs/auth-credential.md`

**패키지**: `.moai/docs/`

**작업**:
- 보안 trade-off 설명 (keyring vs file fallback)
- 8 credential 등록 방법 (`mink login {provider}` per provider)
- 클라우드 동기화 폴더 위험 안내 (`~/.mink/` 위치 확인 가이드)
- USERDATA-MIGRATE 와의 통합 사용법 (cross-device 시나리오)
- `mink doctor auth-keyring` 출력 해석법

**의존**: T-001 ~ T-016 의 인터페이스 확정 (문서화 대상)

**GREEN AC**: AC-CR-032 (OP-3 manual sync 문서화 — 부분), 사용자 문서 검증 (plan-auditor pass 의 일부)

### T-018 — progress.md sync + plan-auditor pass

**패키지**: `.moai/specs/SPEC-MINK-AUTH-CREDENTIAL-001/`

**작업**:
- 4 마일스톤 완료 후 progress.md 의 overall + 각 milestone 행 갱신 (implemented 표기)
- plan-auditor agent invocation (사용자 직접 실행 또는 후속 PR 에서 처리)
- spec.md frontmatter `status` planned → implemented (M4 종결 시)
- HISTORY 에 v0.3.0 entry 추가

**의존**: T-001 ~ T-017 모두 GREEN

**GREEN AC**: 없음 (메타 task — 산출물 sync 만)

---

## 5. Task 의존 그래프 요약

```
T-001 (interface) ─┬─ T-002 ─┬─ T-003 ─┬─ T-004
                   │         │         └─ T-005
                   │         │
                   │         └────────── T-007 ─ T-008
                   │
                   └─ T-006 ─┴────────── T-007 ─ T-009

T-001 ─ T-013 ─ T-014 ─ T-015 ─ T-016 ─ T-017 ─ T-018
        │                              ↑
        └─ T-010 ─ T-011 ─ T-012 ───────┘
```

선형 실행 시 M1 (T-001 → T-005) → M2 (T-006 → T-009) → M3 (T-010 → T-014) → M4 (T-015 → T-018).

병렬 가능 구간:
- M1 내부: T-002 와 T-006 (M2 진입) 일부 병렬 (T-001 완료 후)
- M3 내부: T-010~T-012 (OAuth 흐름) 과 T-013 (schema) 병렬

## 6. tasks ↔ AC ↔ REQ traceability 요약

| task | 핵심 GREEN AC | 핵심 REQ |
|------|--------------|---------|
| T-001 | AC-CR-001, AC-CR-007 (부분), AC-CR-024 | UB-1, UN-1 |
| T-002 | AC-CR-002, AC-CR-005, AC-CR-009, AC-CR-013, AC-CR-014 | UB-2 (keyring), UB-7, UB-9, ED-1, ED-2 |
| T-003 | AC-CR-006 (부분, schema validation), AC-CR-020 | UB-6 (부분), SD-1 (sentinel) |
| T-004 | AC-CR-008, AC-CR-015 (부분), AC-CR-031 | UB-8, UB-9, ED-3 (부분) |
| T-005 | AC-CR-001, AC-CR-003 (부분), AC-CR-007, AC-CR-015 | UB-1, UB-3 (부분), UB-7, ED-3 |
| T-006 | AC-CR-004, AC-CR-026, AC-CR-027 | UB-2 (file), UN-2, UN-6 |
| T-007 | AC-CR-021, AC-CR-022 | SD-1, SD-2 |
| T-008 | AC-CR-021, AC-CR-029, AC-CR-030 | SD-2, OP-1, OP-2 |
| T-009 | AC-CR-006, AC-CR-022 | UB-6, SD-1 |
| T-010 | AC-CR-008 (Codex health 부분), AC-CR-016 (initial) | UB-8 (부분), ED-4 |
| T-011 | AC-CR-016, AC-CR-018 | ED-4, SD-4 |
| T-012 | AC-CR-017, AC-CR-025 | ED-5, UN-4 |
| T-013 | AC-CR-006, AC-CR-011, AC-CR-028 | UB-6, UB-4, UN-3 |
| T-014 | AC-CR-003, AC-CR-010, AC-CR-023 | UB-3, UB-5, UN-5 (AC-CR-010 결합), SD-3 |
| T-015 | AC-CR-019 | ED-6, ED-7 |
| T-016 | AC-CR-012, AC-CR-015, AC-CR-032 | UB-4, ED-3, OP-3 |
| T-017 | AC-CR-032 (문서) | OP-3 |
| T-018 | (meta) | (meta) |

각 AC 가 최소 1개 task 에서 GREEN 되며, 각 REQ 가 최소 1개 task 에서 다뤄짐을 확인. acceptance.md §섹션과 cross-validate.

---

Version: 0.2.0
Last Updated: 2026-05-16
Total Tasks: 18 (M1: 5, M2: 4, M3: 5, M4: 4)
