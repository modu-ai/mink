---
spec_id: SPEC-MINK-AUTH-CREDENTIAL-001
artifact: plan.md
version: 0.2.0
created_at: 2026-05-16
updated_at: 2026-05-16
author: MoAI manager-spec
---

# Plan — SPEC-MINK-AUTH-CREDENTIAL-001

OS keyring 통합 credential 저장소 (default + 평문 fallback) 구현 계획. 4 마일스톤 / 18 tasks. 각 마일스톤은 priority label (High / Medium / Low) 로 정렬되며 시간 추정은 사용하지 않는다 (agent-common-protocol §Time Estimation).

---

## 1. 목표 (Goals)

spec.md §5 의 30 EARS REQ 와 acceptance.md 의 32 AC 를 모두 만족하는 통합 credential 저장소를 Go 패키지로 구현한다. 8 credential (5 LLM + 3 채널) 의 lifecycle (Store / Load / Delete / Refresh) 을 OS keyring default + 평문 fallback 모드에서 동등하게 지원한다. LLM-ROUTING-V2 와 USERDATA-MIGRATE-001 의 consumer wiring 을 본 SPEC 의 implementation 범위에 포함한다.

## 2. 기술 접근 (Technical Approach)

### 2.1 Go 패키지 분할

```
internal/auth/
├── credential/        # Service interface, schema, sentinels, errors
│   ├── service.go     # Service interface 정의
│   ├── schema.go      # 8 credential schema (api_key | oauth | bot_token | slack_combo | discord_combo)
│   ├── errors.go      # KeyringUnavailable / NotFound / ReAuthRequired / SchemaViolation sentinels
│   └── mask.go        # ***LAST4 로깅 마스킹 helper
├── keyring/           # go-keyring wrapper
│   ├── backend.go     # KeyringBackend struct, Service identifier 매핑
│   ├── probe.go       # availability detection (D-Bus / libsecret / OS)
│   └── platform_*.go  # OS 별 미세 차이 흡수 (필요 시)
├── file/              # 평문 fallback backend
│   ├── backend.go     # FileBackend struct, atomic write
│   ├── schema_json.go # JSON serialize/deserialize
│   └── perms.go       # mode 0600 / NTFS ACL 검증
├── oauth/             # Codex OAuth client
│   ├── codex.go       # PKCE flow + refresh client (golang.org/x/oauth2 wrapping)
│   └── refresh.go     # auto-refresh + 8일 idle detection
└── dispatch/          # backend selection + auto-fallback
    └── dispatcher.go  # config-driven routing, SD-1 트리거
```

### 2.2 핵심 인터페이스

```go
// internal/auth/credential/service.go (forward-declaration, plan only — 실제 시그니처는 M1 RED 단계에서 결정)
type Service interface {
    Store(provider string, cred Credential) error
    Load(provider string) (Credential, error)
    Delete(provider string) error
    List() ([]string, error)
    Health(provider string) (HealthStatus, error)
}

type Credential interface {
    Kind() Kind          // api_key | oauth | bot_token | slack_combo | discord_combo
    MaskedString() string // for logging
    Validate() error      // schema check (UB-6)
}
```

### 2.3 외부 의존

| 라이브러리 | 용도 | 라이선스 |
|-----------|------|---------|
| `github.com/zalando/go-keyring` v0.2.x | keyring 백엔드 단일 의존 | MIT |
| `golang.org/x/oauth2` | Codex PKCE + refresh client | BSD-3 |
| `github.com/godbus/godbus/v5` (transitive, Linux only) | Secret Service D-Bus | BSD-2 |

라이선스 헌장 (`.moai/project/`) 의존성 표 갱신 필요 (M1 task T-001 부수 산출).

### 2.4 CLI 설계

| 명령 | 동작 | 관련 REQ |
|------|------|---------|
| `mink login {provider}` | 대화형 입력 + Store (5 LLM + 3 채널) | ED-1 |
| `mink login codex` | OAuth 2.1 + PKCE 브라우저 flow + Store | ED-1, ED-4 |
| `mink logout {provider}` | Delete (keyring + file 둘 다 sweep) | ED-3 |
| `mink config set auth.store {keyring|file}` | dispatcher 모드 변경 | SD-1, SD-2 |
| `mink config set auth.store keyring,file` | auto-fallback 모드 옵트인 | SD-1 |
| `mink doctor auth-keyring` | 백엔드 가용성 + 8 credential 상태 표시 | UB-8, UB-9 |

## 3. 마일스톤 (Milestones)

| ID | 제목 | priority | 핵심 REQ |
|----|------|----------|---------|
| M1 | keyring abstraction (go-keyring wrapper + Service interface) | High | UB-1, UB-7, UB-8, UB-9, ED-1, ED-2, ED-3, SD-1, UN-1 |
| M2 | 평문 fallback + auto-detect + CLI config | High | UB-2, UB-6, SD-1, SD-2, UN-2, UN-6 |
| M3 | 8 credential schema + Codex OAuth refresh + LLM-ROUTING-V2 wiring | High | UB-3, UB-5, UB-6, ED-4, ED-5, SD-3, SD-4, UN-3, UN-4, UN-5 |
| M4 | USERDATA-MIGRATE 통합 + CLI 완성 + OP placeholder | Medium | UB-4, ED-6, ED-7, OP-1, OP-2, OP-3, OP-4 |

### 3.1 M1 — keyring abstraction

**목표**: `internal/auth/credential` Service interface 정의 + `internal/auth/keyring` go-keyring wrapper 구현. 단일 provider (Anthropic API key) 로 RED → GREEN → REFACTOR cycle 검증.

**범위**:
- `Service` interface (Store / Load / Delete / List / Health) 정의
- `KeyringBackend` 구현 — `mink:auth:{provider}` service identifier
- `KeyringUnavailable` sentinel + auto-detect probe (D-Bus / libsecret / macOS / Wincred)
- 로깅 마스킹 (UN-1) helper
- 통합 테스트: macOS + Linux + Windows CI runner (CROSSPLAT 호환)

**완료 정의 (DoD)** (audit B2 fix — M2/M3 영역 AC 제외, M1 메인 책임만 enumerate):
- AC-CR-001, AC-CR-002, AC-CR-005, AC-CR-007 (부분, T-001), AC-CR-008, AC-CR-009, AC-CR-013, AC-CR-014, AC-CR-015 (부분, T-005), AC-CR-020, AC-CR-024 GREEN
- AC-CR-003 (헌장 정합 static 검증) = M3 (T-014) 으로 이월
- AC-CR-004 (file backend) = M2 (T-006) 으로 이월
- LSP 0 error
- 단일 provider 로 macOS Keychain Store/Load/Delete round-trip 검증
- Linux libsecret 미설치 환경에서 `KeyringUnavailable` sentinel 정확히 반환

### 3.2 M2 — 평문 fallback + auto-detect + CLI config

**목표**: `internal/auth/file` 평문 fallback backend + `internal/auth/dispatch` config-driven router 구현. M1 의 Service 인터페이스 위에 두 backend 가 plug-in.

**범위**:
- `FileBackend` 구현 — atomic write, mode 0600 검증
- JSON schema (research.md §4.2) serialize/deserialize
- `Dispatcher` 구현 — config 읽기 (`auth.store`), backend 선택, auto-fallback (SD-1)
- CLI: `mink config set auth.store {keyring|file|keyring,file}` 구현
- 통합 테스트: file backend 단독 / dispatcher routing / fallback 전환

**완료 정의 (DoD)**:
- AC-CR-006, AC-CR-007 (UB), AC-CR-021, AC-CR-022 (SD), AC-CR-026, AC-CR-027 (UN) GREEN
- POSIX mode 검증 (`os.Stat`) + Windows ACL 검증 (icacls 호출 또는 windows API)
- 클라우드 동기화 폴더 검출 안내 (`~/iCloud Drive/` / `~/OneDrive/` / `~/Dropbox/` 하위 경로 감지 시 경고)

### 3.3 M3 — 8 credential schema + Codex OAuth + LLM-ROUTING-V2 wiring

**목표**: 8 credential 의 schema 검증 + Codex OAuth refresh 자동화 + LLM-ROUTING-V2 의 graceful degradation 계약 wiring.

**범위**:
- `credential.Credential` interface 의 구체 타입 5종 (`APIKey`, `OAuthToken`, `BotToken`, `SlackCombo`, `DiscordCombo`) 정의
- `Validate()` 메서드 schema check (UB-6)
- `internal/auth/oauth/codex.go` — PKCE flow (브라우저 launch + 127.0.0.1:random_port callback listener) + refresh client
- `internal/auth/oauth/refresh.go` — `expires_at` 추적, 60초 safety margin, `invalid_grant` detection → `ReAuthRequired` sentinel
- LLM-ROUTING-V2 wiring: `Load(provider)` 호출 시 `NotFound` → 라우팅 skip (SD-3)
- 통합 테스트: OAuth refresh mock server (`httptest.Server`) 로 8일 idle 시나리오

**완료 정의 (DoD)**:
- AC-CR-008 (UB), AC-CR-016~AC-CR-018 (ED), AC-CR-023 (SD), AC-CR-025, AC-CR-028 (UN) GREEN
- 8 credential schema 모두 Validate / 직렬화 / 역직렬화 round-trip 검증
- Codex refresh mock server 시나리오 4종 (정상 / rotation / invalid_grant / 네트워크 5xx) PASS
- LLM-ROUTING-V2 의 graceful degradation 회귀 테스트 PASS

### 3.4 M4 — USERDATA-MIGRATE 통합 + CLI 완성 + OP placeholder

**목표**: USERDATA-MIGRATE-001 export/import 에 credentials 항목 추가 + CLI 명령 완성 + OP 후보 placeholder 등록.

**범위**:
- USERDATA-MIGRATE-001 export schema 확장 (credentials 키 추가, 사용자 동의 prompt 통합)
- import 시 keyring 우선 저장, fallback 자동 분기
- CLI: `mink login {provider}` 8 종 + `mink logout {provider}` + `mink doctor auth-keyring`
- OP placeholder: `auth.store: hsm | op-cli` 식별자 reserve (config 검증만, 실행 시 `NotImplemented` 반환)
- 문서: `.moai/docs/auth-credential.md` 사용자 가이드 + 보안 trade-off 설명

**완료 정의 (DoD)**:
- AC-CR-010, AC-CR-011, AC-CR-012 (UB), AC-CR-019 (ED), AC-CR-029, AC-CR-030 (OP) GREEN
- USERDATA-MIGRATE-001 회귀 PASS (credentials 항목 export/import 라운드 트립)
- 사용자 문서 `.moai/docs/auth-credential.md` 작성 (보안 trade-off, fallback 옵트인 가이드)

## 4. REQ ↔ Milestone 매핑 매트릭스

각 REQ 가 어느 마일스톤에서 GREEN 되는지 명시. 일부 REQ 는 다중 마일스톤에 걸쳐 확장 (예: SD-1 은 M1 에서 sentinel 정의, M2 에서 auto-fallback 실현).

| REQ | M1 | M2 | M3 | M4 |
|-----|:--:|:--:|:--:|:--:|
| UB-1 | ✓ | | | |
| UB-2 | | ✓ | | |
| UB-3 | | | ✓ | |
| UB-4 | | | | ✓ |
| UB-5 | | | ✓ | |
| UB-6 | | ✓ | ✓ | |
| UB-7 | ✓ | | | |
| UB-8 | ✓ | | | |
| UB-9 | ✓ | | | |
| ED-1 | ✓ | | | |
| ED-2 | ✓ | | | |
| ED-3 | ✓ | | | |
| ED-4 | | | ✓ | |
| ED-5 | | | ✓ | |
| ED-6 | | | | ✓ |
| ED-7 | | | | ✓ |
| SD-1 | ✓ | ✓ | | |
| SD-2 | | ✓ | | |
| SD-3 | | | ✓ | |
| SD-4 | | | ✓ | |
| UN-1 | ✓ | | | |
| UN-2 | | ✓ | | |
| UN-3 | | | ✓ | |
| UN-4 | | | ✓ | |
| UN-5 | | | ✓ | |
| UN-6 | | ✓ | | |
| OP-1 | | | | ✓ |
| OP-2 | | | | ✓ |
| OP-3 | | | | ✓ |
| OP-4 | | | | ✓ |

## 5. 위험 + 완화 (Risks + Mitigations)

| # | 위험 | 영향 | 완화 |
|---|------|------|------|
| R1 | go-keyring 의 Linux Secret Service 백엔드가 일부 환경 (KeePassXC 비표준 / KWallet5) 에서 호환성 회귀 | M1 GREEN 지연 | 통합 테스트에서 GNOME Keyring / KWallet / KeePassXC 3종 매트릭스 실행, 비표준 환경은 SD-1 → file fallback 으로 graceful |
| R2 | OpenAI Codex OAuth 정책이 2026-05 이후 변경 (TTL / refresh_token rotation) | ED-4, ED-5, SD-4 회귀 | `invalid_grant` sentinel detection 만 핵심 신호로 의존, 시간 기반 가정 (8일) 은 UI 메시지에만 사용 |
| R3 | Windows Wincred `CredentialBlobSize` 2560 bytes 한계가 OAuth scope 확장 시 초과 | M3 회귀 | M3 monitor only, 초과 감지 시 chunking SPEC 별도 발행 (현재 8 credential 모두 한계 내) |
| R4 | 평문 fallback 사용자가 클라우드 동기화 폴더 (iCloud Drive / OneDrive / Dropbox) 에 `~/.mink/` 를 두는 실수 | UN-2 위반 (간접) | `mink doctor` + onboarding Phase 5 가 부모 경로의 동기화 폴더 패턴 감지 시 경고 출력 |
| R5 | 사용자가 fallback 평문의 보안 trade-off 이해 부족 → 부적절한 옵트인 | 보안 침해 위험 | onboarding Phase 5 + `mink doctor` + 사용자 문서 `.moai/docs/auth-credential.md` 가 명시적 trade-off 안내 (R1+R4 와 결합) |
| R6 | zalando/go-keyring 의 transitive godbus 의존이 향후 라이선스 변경 (이론상 BSD-2 → 다른 라이선스) | AGPL 정합 위반 | M1 task T-001 에서 라이선스 헌장 표 갱신, 후속 dependency bump 시 라이선스 헌장 점검 절차 (M4 문서) |
| R7 | USERDATA-MIGRATE-001 schema 확장이 기존 export 와 backward incompatibility | M4 회귀 | USERDATA-MIGRATE schema 의 `version` 키 활용, credentials 항목 누락 시 graceful skip 처리 |
| R8 | macOS Keychain 의 자동 unlock prompt 가 headless SSH 세션에서 hang | M1 통합 테스트 flakiness | go-keyring 호출에 context timeout 적용 (default 5s), timeout 시 `KeyringUnavailable` 반환 |

## 6. 회귀 검증 전략

- M1 ~ M4 각각의 DoD AC 가 정확히 GREEN (regress count = 0)
- 매 마일스톤 종료 시 LSP 0 error / 0 type error / 0 lint error (`.moai/config/sections/quality.yaml` run phase 기준)
- 통합 테스트 매트릭스: macOS-latest / ubuntu-latest (libsecret 설치) / windows-latest CI runner
- LLM-ROUTING-V2 회귀 테스트가 본 SPEC 의 wiring 변경으로 깨지지 않는지 확인 (M3 DoD)
- USERDATA-MIGRATE-001 회귀 테스트가 본 SPEC 의 schema 확장으로 깨지지 않는지 확인 (M4 DoD)

## 7. 외부 SPEC 연계 마일스톤

| 마일스톤 | 외부 SPEC | 연계 작업 |
|----------|-----------|-----------|
| M1 | CROSSPLAT-001 §5.1 | `mink doctor auth-keyring` subcommand 가 §5.1 의 OS detection 결과 사용 |
| M3 | LLM-ROUTING-V2-AMEND-001 | provider Load(NotFound) → 라우팅 skip 계약 wiring (SD-3) |
| M4 | USERDATA-MIGRATE-001 | export schema 확장 (credentials 키), 사용자 동의 prompt 통합 |
| M4 | ONBOARDING-001 Phase 5 | Web Step 2 / CLI wizard 가 본 저장소 호출 (M4 task T-016 부수) |

## 8. plan-auditor 입력 체크리스트

- [ ] research.md ↔ spec.md ↔ plan.md ↔ acceptance.md ↔ progress.md 모두 동일 카운트 (30 REQ / 32 AC / 4 마일스톤 / 18 tasks)
- [ ] 모든 REQ 가 §4 매트릭스에서 1 이상의 마일스톤에 매핑
- [ ] 모든 AC 가 acceptance.md §섹션에서 REQ 와 1:N 매핑 (forward + back-reference)
- [ ] §5 위험 8 항목 모두 완화책 명시
- [ ] §8 Surface Assumptions (spec.md §8) 의 6 가정이 후속 검증 task 또는 AC 로 매핑
- [ ] OUT OF SCOPE 항목 (spec.md §3.2) 이 plan.md 어디에도 구현 작업으로 등장하지 않음
- [ ] AGPL-3.0 헌장 §2 + ADR-001 정합 검증 task 포함 (M3 task T-014)

---

Version: 0.2.0
Last Updated: 2026-05-16
Milestones: 4
Tasks: 18 (M1: 5, M2: 4, M3: 5, M4: 4)
REQ coverage: 30
AC coverage: 32
