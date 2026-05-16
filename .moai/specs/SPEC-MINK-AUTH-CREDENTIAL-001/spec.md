---
id: SPEC-MINK-AUTH-CREDENTIAL-001
version: 0.2.0
status: planned
created_at: 2026-05-16
updated_at: 2026-05-16
author: MoAI manager-spec
priority: high
issue_number: null
phase: 3
size: 대(L)
lifecycle: spec-anchored
labels: [auth, credential, keyring, oauth, security, sprint-3]
related:
  - ADR-001 (사용자 데이터 zero-weight retention)
  - ADR-002 (AGPL-3.0 전환)
  - SPEC-MINK-LLM-ROUTING-V2-AMEND-001
  - SPEC-MINK-ONBOARDING-001
  - SPEC-MINK-USERDATA-MIGRATE-001
  - SPEC-MINK-CROSSPLAT-001
trust_metrics:
  requirements_total: 30
  acceptance_total: 32
  milestones: 4
  tasks: 18
---

# SPEC-MINK-AUTH-CREDENTIAL-001 — OS Keyring 통합 Credential 저장소 (default + 평문 fallback)

> **v0.2.0 (2026-05-16) — STATUS: planned**. v0.1.0 stub (PR #231) 의 5라운드 사용자 결정 totals 를 본격 EARS 30 REQ / 32 AC / 4 마일스톤 / 18 tasks 로 구체화. research.md 의 7개 조사 항목을 normative input 으로 인용.

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-05-16 | plan stub — 5라운드 사용자 결정 totals 진입 표시 | MoAI orchestrator |
| 0.2.0 | 2026-05-16 | 본격 plan EARS 30 REQ / 32 AC / 4 마일스톤 / 18 tasks. size 중 → 대, status draft → planned, lifecycle spec-first → spec-anchored | manager-spec |

---

## 1. 개요 (Overview)

MINK 가 외부 LLM 5종(Anthropic Claude / DeepSeek / OpenAI GPT / Codex OAuth / z.ai GLM) 과 3 채널(Telegram bot / Slack signing+bot / Discord Ed25519+bot) 의 인증 자격증명을 안전하게 저장·갱신·revoke 하는 통합 credential 저장소를 정의한다.

### 1.1 저장 정책 (2026-05-16 사용자 확정)

**Default = OS keyring + 평문 fallback**:

| 환경 | 저장 백엔드 |
|------|------------|
| macOS | Keychain (Security.framework `SecItem*` API) |
| Linux (D-Bus + libsecret 가용) | Secret Service (GNOME Keyring / KWallet / KeePassXC) |
| Windows | Credential Manager (Wincred + DPAPI) |
| Fallback (headless Linux / Docker / WSL2 / libsecret 미설치) | `~/.mink/auth/credentials.json` mode 0600 |

Hermes / Codex CLI 와의 차별점: 두 프로젝트 모두 평문이 default. MINK 는 **default 가 keyring** → AGPL-3.0 헌장 + 사용자 권리 보장의 일관성. 상세 비교는 research.md §5.1.

### 1.2 보관 대상 (8 credential)

| 종류 | 항목 | 보관 형태 |
|---|---|---|
| LLM API key (4) | Anthropic / DeepSeek / OpenAI GPT / z.ai GLM | `kind: api_key`, `value` 문자열 |
| LLM OAuth (1) | Codex (ChatGPT) | `kind: oauth`, `access_token` + `refresh_token` + `expires_at` + `scope` (auto-refresh, 8일 idle 한도) |
| Channel (3) | Telegram bot / Slack signing+bot / Discord Ed25519+bot | 채널별 schema (research.md §4.2) |

### 1.3 라이브러리

- `github.com/zalando/go-keyring` v0.2.x (MIT, AGPL-3.0 호환) — keyring 백엔드 단일 의존
- `golang.org/x/oauth2` (BSD-3) — Codex OAuth 2.1 + PKCE 클라이언트
- file fallback 은 자체 구현 (`internal/auth/file/`, atomic rename, JSON schema)

상세는 research.md §1, §3.

## 2. 배경 (Background)

상세는 research.md §5 (위협 모델), §6 (헌장 정합) 참조. 요약:

- 평문 fallback (mode 0600) 대비 OS keyring 의 보안 우위 4항 (동일 사용자 프로세스 격리 / cold-boot / 클라우드 백업 제외 / OAuth refresh long-lived)
- Hermes / Codex CLI 가 평문 default 인 반면 MINK 는 keyring default — 차별점
- AGPL-3.0 헌장 §2 (가중치 학습 0) + ADR-001 (사용자 데이터 격리) 정합

## 3. 스코프 (Scope)

### 3.1 IN SCOPE

- 8 credential 통합 저장 (5 LLM + 3 채널)
- keyring default + 평문 fallback 자동 분기 + 명시적 사용자 override (`mink config set auth.store {keyring|file}`)
- Codex OAuth refresh token 자동 갱신 (60초 safety margin) + 8일 idle 만료 시 `mink login codex` 재인증 안내
- USERDATA-MIGRATE-001 export/import 통합 (credentials 항목 추가, export 시 사용자 동의 prompt)
- CROSSPLAT-001 §5.1 가드 호환 (install 단계 libsecret 감지, `mink doctor auth-keyring` subcommand)
- credential revoke / rotate API (`mink logout {provider}`, Store overwrite)
- LLM-ROUTING-V2 consumer wiring (graceful degradation: missing provider 자동 skip)

### 3.2 OUT OF SCOPE

- Hardware Security Module (HSM) / Secure Enclave 직접 통합 (별도 SPEC, post-launch)
- Cross-device credential sync (USERDATA-MIGRATE 의 사용자 직접 export/import 로 충분)
- 다중 계정 per provider (사용자 × provider × 1 credential, multi-account 는 별도 SPEC)
- 만료 알림 cron / notification (별도 후속 enhancement)
- age (CLI) passphrase-encrypted fallback (OP 후보, 별도 SPEC)
- Touch ID / Apple Watch biometric unlock (OP-4 후보, 별도 SPEC)

## 4. 의존 SPEC

| SPEC | 관계 |
|------|------|
| SPEC-MINK-LLM-ROUTING-V2-AMEND-001 | 5 LLM provider credential consumer (graceful degradation 계약) |
| SPEC-MINK-ONBOARDING-001 Phase 5 | Web Step 2 + CLI wizard 가 본 저장소 호출 |
| SPEC-MINK-USERDATA-MIGRATE-001 | export/import 항목에 credentials 추가 (동의 prompt 후) |
| SPEC-MINK-CROSSPLAT-001 §5.1 | OS detection + libsecret probe + `mink doctor` |
| SPEC-MINK-MEMORY-QMD-001 | bypass (ollama localhost 평문 충분, 본 SPEC 적용 외) |
| ADR-001 | 사용자 데이터 zero-weight retention 헌장 |
| ADR-002 | AGPL-3.0 전환 헌장 |

## 5. EARS Requirements (30 REQ)

각 REQ 는 acceptance.md 의 1개 이상 AC 로 검증된다. 카테고리별 분포: Ubiquitous 9 / Event-Driven 7 / State-Driven 4 / Unwanted 6 / Optional 4 = 30.

### 5.1 Ubiquitous (UB-1 ~ UB-9)

- **UB-1**: The credential service shall expose a `Service` interface with `Store(provider, credential)`, `Load(provider) credential`, `Delete(provider)`, `List() []provider`, `Health() status` methods.
- **UB-2**: The credential service shall persist credential plaintext only in (a) the active OS keyring or (b) the file fallback at `~/.mink/auth/credentials.json` with POSIX mode `0600` / Windows ACL user-only; the service shall never write credential plaintext to stdout, stderr, log files, telemetry, or any other channel.
- **UB-3**: The credential service shall comply with the AGPL-3.0 헌장 §2 (zero model weight learning) — credential storage code paths shall not feed into model training or weight update flows.
- **UB-4**: The credential service shall enforce a single account per provider (user × provider × 1 credential); subsequent `Store(provider, ...)` calls shall overwrite the existing entry.
- **UB-5**: The credential service shall comply with ADR-001 (zero-weight retention) — Codex OAuth refresh tokens are user credentials and shall remain on the user's machine only; the service shall not transmit refresh tokens to MINK-controlled servers.
- **UB-6**: The credential service shall validate every `Store` call against the 8-credential schema (`kind: api_key | oauth | bot_token | slack_combo | discord_combo`) defined in research.md §4.2; invalid payloads shall be rejected with a `SchemaViolation` sentinel.
- **UB-7**: The credential service shall use `mink:auth:{provider}` as the keyring service identifier (macOS `kSecAttrService` / Linux Secret Service attribute / Windows `TargetName`) and `default` as the account name across all OS backends.
- **UB-8**: The credential service shall expose a `Health(provider)` API that probes credential presence and validity without leaking the plaintext credential value into the returned error or status payload.
- **UB-9**: The credential service shall support macOS, Linux, and Windows in a CROSSPLAT-001 §5.1 compliant manner, with platform detection driving backend selection automatically.

### 5.2 Event-Driven (ED-1 ~ ED-7)

- **ED-1**: When the user runs `mink login {provider}`, the system shall Store the credential via the keyring backend by default (or the file backend if `auth.store: file` is configured or auto-fallback was triggered).
- **ED-2**: When LLM-ROUTING-V2 (or another consumer) calls `Load(provider)`, the system shall return the decrypted credential value in-memory only; the credential shall not be persisted to disk in any temporary form during the call.
- **ED-3**: When the user runs `mink logout {provider}`, the system shall Delete the credential from both the keyring backend and the file fallback (idempotent — absence in either is not an error).
- **ED-4**: When a stored Codex OAuth `access_token` has `expires_at < now() + 60s` and a `Load(codex)` is invoked, the system shall automatically refresh the access_token via the OAuth refresh_token, persist the new tokens (handling refresh_token rotation if present), and return the fresh access_token to the caller.
- **ED-5**: When the Codex OAuth refresh call returns HTTP 400 `invalid_grant` or HTTP 401 (refresh_token revoked or 8-day idle expiry), the system shall surface a `ReAuthRequired(codex)` sentinel to the caller and emit a user-facing prompt instructing `mink login codex` re-execution.
- **ED-6**: When USERDATA-MIGRATE-001 export is triggered, the system shall include credential entries in the export bundle only after explicit user consent via interactive prompt (yes/no per export); keyring-backed entries shall be read via the active backend.
- **ED-7**: When USERDATA-MIGRATE-001 import is triggered, the system shall import credential entries into the keyring backend by default (or the file fallback per the destination machine's `auth.store` configuration).

### 5.3 State-Driven (SD-1 ~ SD-4)

- **SD-1**: While the keyring backend is unavailable (macOS Keychain locked without UI, Linux D-Bus / libsecret unavailable, Windows Wincred error), the system shall return a `KeyringUnavailable` sentinel on the first failed call and, if the user has opted in via `auth.store: keyring,file` (auto-fallback mode), shall transparently route subsequent operations to the file backend until the keyring recovers.
- **SD-2**: While `auth.store: file` is explicitly configured, the system shall bypass the keyring backend entirely for all Store / Load / Delete operations; no keyring API call shall be made.
- **SD-3**: While a provider credential is missing (`Load(provider)` returns `NotFound`), LLM-ROUTING-V2 shall skip that provider in its routing decisions (graceful degradation contract); the credential service shall not synthesize, mock, or substitute a credential value.
- **SD-4**: While the Codex `refresh_token` is valid (idle duration < 8 days), the auto-refresh in ED-4 shall complete without surfacing any user-facing prompt; the user shall experience uninterrupted Codex access.

### 5.4 Unwanted (UN-1 ~ UN-6)

- **UN-1**: The system shall not log credential plaintext in any log file, telemetry payload, or debug output; all credential references in logs shall be masked as `***LAST4` (last 4 characters of the token) or `***` for short tokens.
- **UN-2**: The system shall not write credential values to git-tracked files; `~/.mink/auth/credentials.json` and its parent directory shall remain outside any repository tracked by `git`.
- **UN-3**: The system shall not support multi-account per provider; attempts to register a second account for an existing provider shall overwrite the prior entry (UB-4) rather than create a new slot.
- **UN-4**: The system shall not use an expired Codex `access_token` for LLM-ROUTING-V2 calls; if refresh fails (ED-5), the call shall fail with `ReAuthRequired` rather than transmit the expired token.
- **UN-5**: The system shall not transmit credential values (API keys, access_tokens, refresh_tokens) to MINK telemetry endpoints, MINK metrics endpoints, or any MINK-controlled server; telemetry payloads shall reference providers by identifier only (e.g., `codex`, not `codex:rt-...`).
- **UN-6**: The system shall not write the file fallback with POSIX permissions broader than `0600` or with NTFS ACLs permitting non-owner users; mode validation shall occur on every Store via `os.Stat` (POSIX) or `icacls` equivalent (Windows).

### 5.5 Optional (OP-1 ~ OP-4)

- **OP-1**: Where Hardware Security Module (HSM) or Secure Enclave hardware is available, the system may integrate via a separate post-launch SPEC; the present SPEC reserves the `auth.store: hsm` configuration key for forward compatibility but does not implement it.
- **OP-2**: Where the 1Password CLI (`op` command) is installed and the user opts in via `auth.store: op-cli`, the system may delegate Store / Load / Delete to 1Password vault operations; this remains a post-launch SPEC and is not implemented in v0.2.0.
- **OP-3**: Where cross-device credential synchronization is required, the system shall rely on the USERDATA-MIGRATE-001 export / import flow (ED-6, ED-7); no automatic cloud sync is implemented.
- **OP-4**: Where biometric unlock (macOS Touch ID, Windows Hello) is available, the system may surface `kSecAttrAccessControl` / equivalent flags via a future opt-in flag (`auth.biometric: true`); the present SPEC reserves the configuration key but does not implement the runtime path.

## 6. 본격 plan 산출물 (이 PR)

| 파일 | 목적 |
|------|------|
| research.md | 라이브러리·플랫폼·OAuth·헌장 정합 조사 (7 절) |
| spec.md (본 파일) | EARS 30 REQ + 8 credential 스키마 + 의존 SPEC |
| plan.md | 4 마일스톤 (M1 keyring abstraction / M2 fallback / M3 OAuth refresh + 8 schema / M4 USERDATA + CLI), 위험·완화 |
| tasks.md | 18 tasks (M1 5 / M2 4 / M3 5 / M4 4) + Go 패키지 매핑 |
| acceptance.md | 32 AC (UB 12 / ED 8 / SD 5 / UN 5 / OP 2), REQ↔AC traceable |
| progress.md | overall 진행률 + 마일스톤별 행, plan-auditor pass 대상 |

## 7. References

- 사용자 결정 (2026-05-16 Round 3): Keyring default + 평문 fallback (권장 채택)
- ADR-001 (사용자 데이터 zero-weight retention), ADR-002 (AGPL-3.0 전환)
- research.md (본 SPEC 부속 조사)
- `github.com/zalando/go-keyring` (MIT) — https://github.com/zalando/go-keyring
- macOS Security framework — https://developer.apple.com/documentation/security/keychain_services
- Linux Secret Service spec — https://specifications.freedesktop.org/secret-service-spec/
- Windows Wincred API — https://learn.microsoft.com/en-us/windows/win32/api/wincred/
- OpenAI Codex CLI Auth — https://developers.openai.com/codex/auth

---

## 8. Surface Assumptions (Agent Core Behavior §1)

본격 plan 진입 시점에 surface 한 가정. 모두 사용자가 2026-05-16 conversation 에서 확정한 결정에 기반하나, 후속 PR (run phase) 가 실제 구현 시 재검증해야 한다.

1. **zalando/go-keyring v0.2.x 의 Linux Secret Service 구현이 headless 환경에서 graceful failure** 한다고 가정 (panic 없이 error 반환). 후속 검증 필요: M1 통합 테스트에서 `DBUS_SESSION_BUS_ADDRESS=` 환경에서 확인.
2. **OpenAI Codex OAuth refresh_token TTL 정책 = 30일 절대 + 8일 idle** 로 가정. 정책이 2026-05 이후 변동 시 ED-5 sentinel trigger 조건 (HTTP 400 invalid_grant) 만 유지하면 무관 — UI 메시지만 갱신.
3. **Windows Wincred `CredentialBlobSize` 2560 bytes 제한**이 8 credential 최대 크기 (Codex OAuth ~2KB) 를 수용한다고 가정 (research.md §2.4). OAuth scope 가 향후 크게 확장되면 chunking 필요 — M3 monitor only.
4. **`zalando/go-keyring` 의 MIT 라이선스 + transitive `godbus/godbus/v5` BSD-2 + `golang.org/x/oauth2` BSD-3 가 AGPL-3.0 호환** 한다고 가정. 라이선스 헌장 (.moai/project/) 의존성 표 자동 검증과 정합.
5. **사용자가 `~/.mink/auth/credentials.json` 평문 fallback 의 보안 trade-off 를 이해**하고 옵트인한다고 가정 — onboarding Phase 5 / `mink doctor` 출력이 명시적 안내를 포함해야 한다. (의도된 사용자 경험 요건, plan.md §위험 5에서 다룸)
6. **USERDATA-MIGRATE-001 의 export schema 가 credentials 항목을 신규 키로 수용 가능**하다고 가정. 후속 PR 에서 USERDATA-MIGRATE schema 확장이 필요할 경우 별도 amendment SPEC 검토.

→ Plan-auditor pass 시 본 박스의 각 항목을 검증 가능한 AC 또는 후속 task 로 매핑되었는지 확인한다.

---

Version: 0.2.0
Last Updated: 2026-05-16
SDD lifecycle: spec-anchored
REQ coverage: 30 (UB-9 / ED-7 / SD-4 / UN-6 / OP-4)
AC coverage: 32 (UB-12 / ED-8 / SD-5 / UN-5 / OP-2)
Milestones: 4
Tasks: 18
