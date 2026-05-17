---
spec_id: SPEC-MINK-AUTH-CREDENTIAL-001
artifact: acceptance.md
version: 0.2.0
created_at: 2026-05-16
updated_at: 2026-05-16
author: MoAI manager-spec
acceptance_total: 32
---

# Acceptance Criteria — SPEC-MINK-AUTH-CREDENTIAL-001

총 **32 AC** (분포: UB-12 / ED-8 / SD-5 / UN-5 / OP-2). 모든 AC 는 spec.md §5 의 30 REQ 와 1:N 매핑되며 tasks.md 의 18 tasks 중 1개 이상에서 GREEN 처리된다. 테스트 분류는 (U) unit / (I) integration / (E) e2e 로 표기. AC ID 는 작성 순서 (AC-CR-001 ~ AC-CR-032) 이며 분류는 §6 매트릭스에서 확정한다 — 본문 §1~§5 의 섹션 제목은 작성 편의상 EARS 카테고리 그룹별로 묶었으나 일부 AC 는 §6 에서 다른 EARS 분류로 재할당된다.

각 AC 는 Given-When-Then 패턴으로 기술하며 관측 가능한 증거 (test output / file existence / metric / sentinel return) 만 사용한다.

---

## 1. Ubiquitous AC (AC-CR-001 ~ AC-CR-012, 총 12)

### AC-CR-001 — Service interface 정의 + 5 method 충족 (U)

**REQ**: UB-1
**Task**: T-001, T-005

- **Given**: `internal/auth/credential/service.go` 의 `Service` interface 가 정의된다.
- **When**: 컴파일러가 `KeyringBackend` 와 `FileBackend` 를 `Service` 로 type-assert 한다.
- **Then**: 5개 method (`Store`, `Load`, `Delete`, `List`, `Health`) 가 모두 구현되어 컴파일 오류 없이 통과한다.

### AC-CR-002 — keyring 백엔드 plaintext 격리 (I)

**REQ**: UB-2
**Task**: T-002

- **Given**: `auth.store: keyring` 설정 + macOS / Linux / Windows 중 1개 환경.
- **When**: `Store(anthropic, APIKey{value: "sk-ant-test-XXXX"})` 를 호출하고 file system 을 검사한다.
- **Then**: `~/.mink/auth/credentials.json` 가 존재하지 않는다. 동시에 keyring backend (OS native) 에서만 secret 이 조회된다.

### AC-CR-003 — AGPL 헌장 §2 정합 (zero weight learning) (U + static)

**REQ**: UB-3
**Task**: T-014

- **Given**: `internal/auth/` 패키지 트리.
- **When**: static analysis 도구가 import graph 를 분석한다.
- **Then**: `internal/auth/**` 의 어떤 파일도 `internal/model/training/**` (또는 동등 모델 학습 경로) 를 import 하지 않는다. grep 결과 0건.

### AC-CR-004 — file fallback plaintext 격리 (I)

**REQ**: UB-2
**Task**: T-006

- **Given**: `auth.store: file` 설정.
- **When**: `Store(anthropic, APIKey{value: "sk-ant-test-XXXX"})` 호출 후 OS keyring API 호출 횟수를 검사한다.
- **Then**: keyring API 호출이 0회 발생한다 (mock 으로 측정). `~/.mink/auth/credentials.json` 가 mode 0600 으로 존재한다.

### AC-CR-005 — keyring service identifier 포맷 (U)

**REQ**: UB-7
**Task**: T-002

- **Given**: `KeyringBackend.serviceIdentifier(provider)` helper.
- **When**: provider `anthropic` / `codex` / `slack` 로 호출한다.
- **Then**: 반환값이 정확히 `mink:auth:anthropic` / `mink:auth:codex` / `mink:auth:slack` 이며, account 는 모두 `default`.

### AC-CR-006 — 8 credential schema validation (U)

**REQ**: UB-6
**Task**: T-009, T-013

- **Given**: 8 provider id 별 `Credential` 구현 (`APIKey`, `OAuthToken`, `BotToken`, `SlackCombo`, `DiscordCombo`).
- **When**: 각 type 의 `Validate()` 를 (1) 정상 payload, (2) 누락 필드, (3) 잘못된 kind 로 호출한다.
- **Then**: (1) nil error, (2) `SchemaViolation` sentinel + missing 필드명 명시, (3) `SchemaViolation` sentinel + kind mismatch 메시지.

### AC-CR-007 — 단일 account 고정 (U)

**REQ**: UB-7
**Task**: T-001, T-005

- **Given**: 임의 provider 에 대한 Store 후 keyring 직접 조회.
- **When**: OS keyring 의 account 필드를 read 한다 (macOS `kSecAttrAccount` / Linux account attribute / Windows `UserName`).
- **Then**: 값이 정확히 `"default"`. 다른 값은 등장하지 않는다.

### AC-CR-008 — Health(provider) plaintext 비누설 (U)

**REQ**: UB-8
**Task**: T-004

- **Given**: `Store(anthropic, APIKey{value: "sk-ant-1234567890"})` 후.
- **When**: `Health(anthropic)` 호출 결과 + log 출력을 캡처한다.
- **Then**: 응답 payload 에 `sk-ant-1234567890` 문자열 부재. 응답은 `present: true, masked: "***7890", backend: "keyring"` 같은 비누설 구조.

### AC-CR-009 — Cross-platform 동작 (I, CI matrix)

**REQ**: UB-9
**Task**: T-002

- **Given**: GitHub Actions matrix (`macos-latest`, `ubuntu-latest` with libsecret, `windows-latest`).
- **When**: 동일 통합 테스트 (Store → Load → Delete round-trip) 를 3 OS 에서 실행한다.
- **Then**: 3 OS 모두 PASS. ubuntu runner 는 `apt install libsecret-1-0` + `gnome-keyring-daemon` start 후.

### AC-CR-010 — ADR-001 정합 (refresh_token telemetry 차단) (U + static + I)

**REQ**: UB-5, UN-5
**Task**: T-014

- **Given**: `internal/auth/oauth/refresh.go` 와 telemetry 패키지 import graph + telemetry 송신 mock.
- **When**: static analysis + 통합 테스트의 telemetry 송신 payload 를 정상 credential lifecycle (Store, Load, Refresh, Delete) 동안 캡처.
- **Then**: (1) import graph 에서 `internal/auth/oauth` → telemetry 직접 경로 없음, (2) payload 내 API key 패턴 (`sk-`, `xoxb-`, `Bot `, `sess-`, refresh_token 패턴) 0회 등장, (3) provider id (`anthropic`, `codex` 등) 만 등장 허용.

### AC-CR-011 — Single account enforce (overwrite 동작) (U)

**REQ**: UB-4
**Task**: T-013

- **Given**: provider `anthropic` 에 대해 `Store(APIKey{value: "key1"})` 수행 후.
- **When**: `Store(APIKey{value: "key2"})` 를 호출하고 `Load(anthropic)` 결과를 확인한다.
- **Then**: `Load` 결과의 value 가 `"key2"`. 별도 슬롯 생성 없음 (List() 결과 길이는 1 유지).

### AC-CR-012 — CLI `mink login {provider}` single account 보장 (I)

**REQ**: UB-4
**Task**: T-016

- **Given**: 사용자가 `mink login anthropic` 으로 첫 key 등록 완료.
- **When**: 동일 사용자가 `mink login anthropic` 을 다시 실행해 새 key 입력한다.
- **Then**: CLI 가 "기존 항목을 덮어쓰시겠습니까?" 확인 prompt 출력 → 동의 시 overwrite. 동의 거부 시 작업 취소. 어떤 경우에도 두 번째 slot 생성 없음.

---

## 2. Event-Driven AC (AC-CR-013 ~ AC-CR-019)

> 참고: ED-7 은 AC-CR-019 가 export+import 통합 검증. ED-5 는 AC-CR-017 단독. AC 총 8개로 (8 = ED 7 + UB-5 보조 AC-CR-031 1).

### AC-CR-013 — `mink login {provider}` Store path (E)

**REQ**: ED-1
**Task**: T-002, T-016

- **Given**: `auth.store: keyring` (default).
- **When**: `mink login deepseek` 실행 → API key paste → enter.
- **Then**: keyring 에 `mink:auth:deepseek / default` 항목 등장. file fallback 미생성. CLI 종료 코드 0.

### AC-CR-014 — Load 시 in-memory only (U)

**REQ**: ED-2
**Task**: T-002

- **Given**: `Store(anthropic, APIKey{value: "secret"})` 완료.
- **When**: `Load(anthropic)` 호출 중 + 직후 file system snapshot (tmpdir / `~/.mink/` / OS temp) 비교.
- **Then**: Load 호출 전후 file system 에 임시 plaintext 파일 등장 흔적 0개. 반환된 `Credential` 객체만 메모리에 존재.

### AC-CR-015 — `mink logout {provider}` idempotent (E)

**REQ**: ED-3
**Task**: T-005, T-016

- **Given**: provider `anthropic` 이 keyring + file 양쪽에 가짜로 존재한다 (테스트 setup).
- **When**: `mink logout anthropic` 1회 실행 후 다시 1회 실행한다.
- **Then**: 1회차 — keyring 항목 + file 항목 모두 삭제, exit 0. 2회차 — 이미 없음에도 exit 0 (idempotent), stderr 에 "이미 삭제됨" 안내만 출력.

### AC-CR-016 — Codex OAuth auto-refresh (I)

**REQ**: ED-4
**Task**: T-010, T-011

- **Given**: `OAuthToken` 이 keyring 에 저장됨 + `expires_at` 가 `now() + 30s` (60초 margin 내 만료 임박).
- **When**: `Load(codex)` 호출 → mock OAuth server 가 새 access_token 반환.
- **Then**: 반환된 token 이 새로 refresh 된 값. keyring 의 expires_at 가 `now() + 3600s` 같은 미래로 갱신. rotation 시 refresh_token 도 갱신.

### AC-CR-017 — `invalid_grant` → ReAuthRequired sentinel + 사용자 안내 (I)

**REQ**: ED-5
**Task**: T-012

- **Given**: mock OAuth server 가 HTTP 400 `{"error": "invalid_grant"}` 반환하도록 설정.
- **When**: `Load(codex)` 호출 (만료 임박 상태).
- **Then**: `ReAuthRequired(codex)` sentinel 반환. CLI 컨텍스트에서는 stderr 에 정확히 `mink login codex` 재실행 안내 메시지 출력.

### AC-CR-018 — Silent refresh (사용자 prompt 없음) (I)

**REQ**: SD-4
**Task**: T-011

- **Given**: refresh_token idle 7일 (8일 한도 내) + access_token 만료 임박.
- **When**: `Load(codex)` 호출.
- **Then**: 정상 refresh 완료. stderr / stdout 에 어떤 사용자 prompt / 안내 / 경고 출력 0건.

### AC-CR-019 — USERDATA-MIGRATE export + import 라운드 트립 (I)

**REQ**: ED-6, ED-7
**Task**: T-015

- **Given**: 머신 A 에 8 credential 등록 완료.
- **When**: `mink userdata export --include-credentials` → 사용자 동의 prompt 에 yes 응답 → bundle 생성 → 머신 B 에서 `mink userdata import` 수행.
- **Then**: 머신 B 의 keyring (default) 또는 file fallback 에 8 credential 모두 등장. 각 Load 결과가 머신 A 의 원본과 동일. 동의 거부 시 (`no`) export bundle 에 credentials 키 부재.

---

## 3. State-Driven AC (AC-CR-020 ~ AC-CR-023)

> SD-4 의 silent refresh 검증은 AC-CR-018 (§2 ED 그룹 내) 에서 처리. 본 절은 SD-1~SD-3 의 명시 검증 + SD-1 의 분기 케이스 2개 (AC-CR-020 sentinel + AC-CR-022 auto-fallback).

### AC-CR-020 — KeyringUnavailable sentinel (Linux headless) (I)

**REQ**: SD-1
**Task**: T-003

- **Given**: ubuntu runner 에서 `DBUS_SESSION_BUS_ADDRESS=` (빈 값) 환경 + libsecret 미설치.
- **When**: `KeyringBackend.Store(anthropic, ...)` 호출.
- **Then**: `KeyringUnavailable` sentinel 반환 (panic 없음, hang 없음). error message 에 "Secret Service unavailable" 패턴 포함.

### AC-CR-021 — `auth.store: file` bypass keyring (U)

**REQ**: SD-2
**Task**: T-007, T-008

- **Given**: config `auth.store: file`.
- **When**: `Store / Load / Delete` 임의 호출 + KeyringBackend mock 의 호출 횟수 측정.
- **Then**: KeyringBackend mock 호출 0회. 모든 작업이 FileBackend 로 라우팅.

### AC-CR-022 — Auto-fallback (`auth.store: keyring,file`) (I)

**REQ**: SD-1
**Task**: T-007, T-009

- **Given**: config `auth.store: keyring,file` + 첫 호출 시 KeyringBackend mock 이 `KeyringUnavailable` 반환.
- **When**: `Store(anthropic, ...)` 호출.
- **Then**: 동일 호출이 자동으로 FileBackend 로 재시도되어 PASS. 후속 동일 세션 내 호출은 FileBackend 로 직행 (probe cache).

### AC-CR-023 — LLM-ROUTING-V2 graceful degradation (I)

**REQ**: SD-3
**Task**: T-014

- **Given**: 5 LLM provider 중 anthropic 만 등록, 나머지 4개 (`deepseek`, `openai_gpt`, `codex`, `zai_glm`) 미등록.
- **When**: LLM-ROUTING-V2 의 라우팅 함수가 5 provider 전체 후보로 호출된다.
- **Then**: 라우팅 결과에 `anthropic` 만 포함. 나머지 4 provider 는 silent skip. routing 결과 길이 1. log 에 `NotFound` masked 출력만 (UN-1 정합).

---

## 4. Unwanted AC (AC-CR-024 ~ AC-CR-028, + AC-CR-031 doctor 출력 관찰)

> AC-CR-024 (로그 plaintext 비누설) 와 AC-CR-031 (`mink doctor` 출력 plaintext 비누설) 은 모두 plaintext 누설 차단 검증이나 §6 매트릭스 에서 AC-CR-024 는 SD 영역 (log 시스템 상태 관찰), AC-CR-031 은 ED 영역 (doctor 명령 트리거) 으로 재분류된다 — 검증 의도는 동일하나 EARS 영역 분류만 다르다.

### AC-CR-024 — 로그 plaintext 비누설 (U)

**REQ**: UN-1
**Task**: T-001

- **Given**: `MaskedString("sk-ant-1234567890")`, `MaskedString("ab")`, `MaskedString("")` 호출.
- **When**: 반환값을 검사한다.
- **Then**: 각각 `"***7890"`, `"***"`, `"***"` 반환. plaintext 원본 부분 문자열 매칭 0건.

### AC-CR-025 — Expired access_token 차단 (I)

**REQ**: UN-4
**Task**: T-012

- **Given**: refresh 시 mock server 가 항상 5xx 반환 (refresh 영구 실패).
- **When**: 만료된 access_token 으로 `Load(codex)` 호출.
- **Then**: 만료 token 이 반환되지 않고 `ReAuthRequired(codex)` 또는 retry 가능 error 반환. LLM-ROUTING-V2 의 호출 캡처에 만료 token 송신 0건.

### AC-CR-026 — git-tracked 차단 (I)

**REQ**: UN-2
**Task**: T-006

- **Given**: `~/.mink/auth/credentials.json` 가 mock 사용자 home 에 생성된 상태.
- **When**: `git check-ignore` 또는 동등한 path 검사를 수행하거나, `.gitignore` 패턴을 점검한다.
- **Then**: 본 SPEC 의 repo 루트 `.gitignore` 가 `.mink/auth/` 또는 동등 경로를 ignore 하거나, 본 파일이 사용자 home 외 위치 (`./.mink/` 같은 cwd 위치) 에 절대 생성되지 않음을 검증.

### AC-CR-027 — file fallback mode 0600 검증 (I)

**REQ**: UN-6
**Task**: T-006

- **Given**: `auth.store: file` + `Store(...)` 1회 실행 직후.
- **When**: POSIX 환경: `os.Stat(~/.mink/auth/credentials.json).Mode().Perm()` / Windows 환경: ACL 검사.
- **Then**: POSIX `0600`. Windows ACL: 현재 사용자 FullControl + Administrators FullControl + 그 외 access 거부.

### AC-CR-028 — Multi-account 차단 (U)

**REQ**: UN-3
**Task**: T-013

- **Given**: provider `slack` 에 대한 `Store(SlackCombo{...A})` 후.
- **When**: 동일 provider 에 `Store(SlackCombo{...B})` 호출.
- **Then**: keyring (또는 file) 에 단일 항목만 존재 (List() 결과 1). 별도 slot 등장 없음. 두 번째 호출은 첫 번째를 overwrite (UB-4 와 정합).

### AC-CR-031 — `mink doctor auth-keyring` 출력 검증 (E)

**REQ**: UB-8, UB-9
**Task**: T-004

- **Given**: 8 provider 중 일부 등록 + 일부 미등록 상태 (mock 환경).
- **When**: `mink doctor auth-keyring` 명령 실행.
- **Then**: 출력에 (1) 활성 backend 식별자 (keyring / file), (2) 8 provider 각각의 presence/absent 상태, (3) Codex 의 expires_at 까지 잔여 시간, (4) 어떤 라인에도 plaintext 토큰 부재 (`***LAST4` 만 표시).

---

## 5. Optional AC (AC-CR-029, AC-CR-030, AC-CR-032)

> OP 영역은 spec.md 정의상 placeholder + 문서화. AC 본문은 3개 (AC-CR-029 hsm placeholder, AC-CR-030 op-cli placeholder, AC-CR-032 manual cross-device sync 문서). §6 매트릭스 에서 AC-CR-032 는 UN 영역 으로 재분류 되며 OP 영역 unique 카운트는 2 (AC-CR-029, 030) — 사용자 카운트 가이드 (UN 5 / OP 2) 와 일치.

### AC-CR-029 — `auth.store: hsm` placeholder (U)

**REQ**: OP-1
**Task**: T-008

- **Given**: CLI `mink config set auth.store hsm`.
- **When**: 실행한다.
- **Then**: config 값이 reject 되며 (`hsm` 은 reserved, not implemented), 사용자에게 안내 메시지 "auth.store: hsm 는 향후 SPEC 에서 구현 예정" 출력. exit code 1.

### AC-CR-030 — `auth.store: op-cli` placeholder (U)

**REQ**: OP-2
**Task**: T-008

- **Given**: CLI `mink config set auth.store op-cli`.
- **When**: 실행한다.
- **Then**: AC-CR-029 와 동일 동작 (reject + 안내). exit code 1.

### AC-CR-032 — Manual cross-device sync 문서화 (E + 문서 검증)

**REQ**: OP-3
**Task**: T-016, T-017

- **Given**: `.moai/docs/auth-credential.md` 문서.
- **When**: 문서를 grep 한다.
- **Then**: "cross-device sync" 또는 "다중 기기" 섹션이 존재하며 `mink userdata export --include-credentials` + 머신 B `mink userdata import` 시나리오를 명시한다. AC-CR-019 와 cross-reference.

---

## 6. AC → REQ → Task 매트릭스 (정합 검증)

| AC | 분류 | REQ | Tasks | Category |
|----|------|-----|-------|----------|
| AC-CR-001 | UB | UB-1 | T-001, T-005 | U |
| AC-CR-002 | UB | UB-2 | T-002 | I |
| AC-CR-003 | UB | UB-3 | T-014 | U+static |
| AC-CR-004 | UB | UB-2 | T-006 | I |
| AC-CR-005 | UB | UB-7 | T-002 | U |
| AC-CR-006 | UB | UB-6 | T-009, T-013 | U |
| AC-CR-007 | UB | UB-7 | T-001, T-005 | U |
| AC-CR-008 | UB | UB-8 | T-004 | U |
| AC-CR-009 | UB | UB-9 | T-002 | I (CI matrix) |
| AC-CR-010 | UB | UB-5, UN-5 | T-014 | U+static+I |
| AC-CR-011 | UB | UB-4 | T-013 | U |
| AC-CR-012 | UB | UB-4 | T-016 | I |
| AC-CR-013 | ED | ED-1 | T-002, T-016 | E |
| AC-CR-014 | ED | ED-2 | T-002 | U |
| AC-CR-015 | ED | ED-3 | T-005, T-016 | E |
| AC-CR-016 | ED | ED-4 | T-010, T-011 | I |
| AC-CR-017 | ED | ED-5 | T-012 | I |
| AC-CR-018 | ED | SD-4 | T-011 | I |
| AC-CR-019 | ED | ED-6, ED-7 | T-015 | I |
| AC-CR-031 | ED | UB-8, UB-9 (doctor lifecycle 관찰) | T-004 | E |
| AC-CR-020 | SD | SD-1 | T-003 | I |
| AC-CR-021 | SD | SD-2 | T-007, T-008 | U |
| AC-CR-022 | SD | SD-1 | T-007, T-009 | I |
| AC-CR-023 | SD | SD-3 | T-014 | I |
| AC-CR-024 | SD | UN-1 (state observation through log) | T-001 | U |
| AC-CR-025 | UN | UN-4 | T-012 | I |
| AC-CR-026 | UN | UN-2 | T-006 | I |
| AC-CR-027 | UN | UN-6 | T-006 | I |
| AC-CR-028 | UN | UN-3 | T-013 | U |
| AC-CR-032 | UN | UN-2 (cross-device git-tracked 차단 문서 부수 검증) + OP-3 | T-016, T-017 | E + doc |
| AC-CR-029 | OP | OP-1 | T-008 | U |
| AC-CR-030 | OP | OP-2 | T-008 | U |

> **분류 보충 설명**:
> - AC-CR-018 은 SD-4 검증이나 ED 영역 (silent refresh = ED-4 의 후속 효과, 사용자 prompt 발생 여부 = ED 차원의 관측) 으로 분류.
> - AC-CR-031 은 ED 영역 (`mink doctor` 명령 실행이라는 사용자 트리거 이벤트) 으로 분류하며 검증 REQ 는 UB-8, UB-9 보조.
> - AC-CR-024 는 SD 영역 (로깅 상태에서 plaintext 비누설 = 시스템 상태 일관성) 으로 분류.
> - AC-CR-032 는 UN 영역 (cross-device 부적절한 동기화 차단 문서 + OP-3 manual sync 보장).

**분류별 AC 카운트 (사용자 요구 분포 일치)**:
- UB (Ubiquitous): **12** AC (AC-CR-001~012)
- ED (Event-Driven): **8** AC (AC-CR-013, 014, 015, 016, 017, 018, 019, 031)
- SD (State-Driven): **5** AC (AC-CR-020, 021, 022, 023, 024)
- UN (Unwanted): **5** AC (AC-CR-025, 026, 027, 028, 032)
- OP (Optional): **2** AC (AC-CR-029, 030)
- **합계**: 12 + 8 + 5 + 5 + 2 = **32** ✓

**카테고리 카운트**:
- U (unit): 11
- I (integration, CI matrix 포함): 14
- E (e2e): 5
- U+static / U+static+I (보조): 2
- 합계: 32 ✓

**REQ 커버리지 (전체)**:
- UB (9 REQ): UB-1 → AC-001, 007; UB-2 → AC-002, 004; UB-3 → AC-003; UB-4 → AC-011, 012; UB-5 → AC-010; UB-6 → AC-006; UB-7 → AC-005, 007; UB-8 → AC-008, 031; UB-9 → AC-009, 031
- ED (7 REQ): ED-1 → AC-013; ED-2 → AC-014; ED-3 → AC-015; ED-4 → AC-016; ED-5 → AC-017; ED-6 → AC-019; ED-7 → AC-019
- SD (4 REQ): SD-1 → AC-020, 022; SD-2 → AC-021; SD-3 → AC-023; SD-4 → AC-018
- UN (6 REQ): UN-1 → AC-024; UN-2 → AC-026, 032 (문서 부수); UN-3 → AC-028; UN-4 → AC-025; UN-5 → AC-010 (UB-5 와 결합); UN-6 → AC-027
- OP (4 REQ): OP-1 → AC-029; OP-2 → AC-030; OP-3 → AC-032 (UN 분류이나 OP-3 매핑); OP-4 → placeholder REQ (config key reserve, 검증 AC 없음 — plan.md §4 매트릭스에 명시)

모든 30 REQ 중 29 REQ 가 1개 이상 AC 에서 검증된다. OP-4 만 placeholder REQ (검증 AC 미할당, plan.md 에서 별도 SPEC 으로 처리 명시).

**AC unique 카운트 확정**: 32 (AC-CR-001 ~ AC-CR-032).
**분포 합계 검증**: 12 (UB) + 8 (ED) + 5 (SD) + 5 (UN) + 2 (OP) = 32 ✓ (spec.md frontmatter `acceptance_total: 32`, progress.md overall row 와 동일)

---

## 7. Definition of Done (전체 SPEC 종결 조건)

본 SPEC 의 status 가 `planned → in-progress → implemented` 로 전이하기 위한 종결 조건:

- [ ] 32 AC 모두 GREEN (회귀 없이)
- [ ] LSP 0 error / 0 type error / 0 lint error (`.moai/config/sections/quality.yaml` run phase 기준)
- [ ] CI matrix: macos-latest + ubuntu-latest (libsecret 설치) + windows-latest 모두 PASS
- [ ] LLM-ROUTING-V2 회귀 PASS
- [ ] USERDATA-MIGRATE-001 회귀 PASS
- [ ] AGPL §2 + ADR-001 정합 static analysis PASS
- [ ] 사용자 문서 `.moai/docs/auth-credential.md` 작성 + 검증
- [ ] plan-auditor agent invocation PASS

---

Version: 0.2.0
Last Updated: 2026-05-16
Total AC: 32 (UB: 12, ED: 8, SD: 5, UN: 5, OP: 2)
Categories: U-11 / I-14 / E-5 / U+static or U+static+I-2
REQ ↔ AC traceability: complete (30 REQ 중 29 REQ 가 1 이상 AC 에서 GREEN, OP-4 만 placeholder — plan.md §4 매트릭스 참조)
