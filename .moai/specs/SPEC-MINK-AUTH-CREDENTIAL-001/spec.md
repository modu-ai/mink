---
id: SPEC-MINK-AUTH-CREDENTIAL-001
version: 0.1.0
status: draft
created_at: 2026-05-16
updated_at: 2026-05-16
author: MoAI orchestrator
priority: high
issue_number: null
phase: 3
size: 중(M)
lifecycle: spec-first
labels: [auth, credential, keyring, oauth, security, sprint-3]
related:
  - ADR-002 (AGPL-3.0 전환)
  - SPEC-MINK-LLM-ROUTING-V2-AMEND-001
  - SPEC-MINK-ONBOARDING-001
  - SPEC-MINK-USERDATA-MIGRATE-001
  - SPEC-MINK-CROSSPLAT-001
---

# SPEC-MINK-AUTH-CREDENTIAL-001 — OS Keyring 통합 Credential 저장소 (default + 평문 fallback)

> **STUB / DRAFT (2026-05-16)**: 본 SPEC 은 사용자 결정 진입을 표시하는 *plan stub* 이다. 본격 EARS 요구사항·plan-auditor pass 는 후속 PR (manager-spec spawn) 에서 처리한다.

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-05-16 | plan stub — 5라운드 사용자 결정 totals 진입 표시 | MoAI orchestrator |

---

## 1. 개요 (Overview)

MINK 가 외부 GOAT LLM 5종(SPEC-MINK-LLM-ROUTING-V2-AMEND-001) 과 3 채널(Telegram/Slack/Discord) 의 인증 자격증명을 안전하게 저장·갱신·revoke 하는 통합 credential 저장소 정의.

### 1.1 저장 정책 (2026-05-16 사용자 확정)

**Default = OS keyring + 평문 fallback** (Round 3 AskUserQuestion 답변):

- macOS: Keychain (Security.framework)
- Linux: Secret Service (libsecret / GNOME Keyring / KWallet)
- Windows: Credential Manager (Wincred API)
- Fallback: `~/.mink/auth/credentials.json` (mode 0600) — keyring 미지원 환경 (headless Linux Docker 등)

Hermes/OpenClaw/Codex 와의 차별점: 세 프로젝트 모두 keyring 을 *옵션* 으로만 지원. MINK 는 **default 가 keyring** → AGPL-3.0 헌장 + 사용자 권리 보장 의 일관성.

### 1.2 보관 대상 (8 credential)

| 종류 | 항목 | 보관 형태 |
|---|---|---|
| LLM (4 key paste) | Anthropic / DeepSeek / OpenAI GPT / z.ai GLM | API key string |
| LLM (1 OAuth) | Codex (ChatGPT) | access_token + refresh_token + expires_at (auto-refresh 8일 idle 한도) |
| Channel (3) | Telegram bot token / Slack signing secret + bot token / Discord interactions Ed25519 public key + bot token | 채널별 schema |

## 2. 배경 (Background)

### 2.1 평문 vs Keyring 비교 근거

(상세 분석은 conversation 2026-05-16 Round 3, MEMORY 의 paste-ready prompt 참조)

| 위험 | 평문 mode 600 | Keyring |
|---|---|---|
| 동일 사용자 프로세스 노출 | 노출 | 격리 |
| 잠금 노트북 cold-boot | 직독 가능 | 평문화 불가 |
| iCloud/OneDrive 백업 | 동기화 가능 | OS 자동 제외 |
| OAuth refresh long-lived | 위험 큼 | 위험 ↓ |

### 2.2 라이브러리 후보

- `github.com/zalando/go-keyring` (MIT, AGPL 호환) — 우선 검토
- platform-native (cgo 또는 syscall) — Windows Wincred / macOS Security.framework 직접 호출 fallback

## 3. 스코프 (Scope)

### 3.1 IN SCOPE (본격 plan EARS 화 예정)

- 8 credential 통합 저장 (5 LLM + 3 채널)
- keyring default + 평문 fallback 자동 분기
- `mink config set auth.store {keyring|file}` 명시적 사용자 override
- Codex OAuth refresh token 자동 갱신 + 8일 idle 재로그인 트리거
- USERDATA-MIGRATE-001 export 통합 (credentials 항목 추가)
- CROSSPLAT-001 §5.1 가드 호환 (Windows native / Linux libsecret 미설치 감지)
- credential revoke / rotate API

### 3.2 OUT OF SCOPE

- Hardware Security Module (HSM) / Secure Enclave 직접 통합 (별도 SPEC, post-launch)
- Cross-device credential sync (사용자 직접 export/import)
- 다중 계정 per provider (사용자당 1 provider 1 credential, multi-account 는 별도 SPEC)
- 만료 알림 (별도 후속 enhancement)

## 4. 의존 SPEC

- LLM-ROUTING-V2-AMEND-001: 5 provider credential consumer
- ONBOARDING-001 Phase 5: Web Step 2 에서 본 저장소 호출
- USERDATA-MIGRATE-001: export/import 항목 추가
- CROSSPLAT-001 §5.1: OS 가드
- MEMORY-QMD-001: bypass (ollama localhost 평문 충분)
- ADR-002: AGPL-3.0 헌장

## 5. 본격 plan 이월 (후속 PR)

- research.md: go-keyring 성능·플랫폼 매트릭스, OAuth PKCE/device code, 8일 idle 재인증 동작 검증
- plan.md: 4 마일스톤 (M1 keyring abstraction / M2 fallback / M3 OAuth refresh / M4 export 통합)
- tasks.md, acceptance.md, progress.md
- plan-auditor pass

## 6. References

- 사용자 결정 (2026-05-16 Round 3): Keyring default + 평문 fallback (권장)
- ADR-002 (AGPL-3.0 전환)
- go-keyring: https://github.com/zalando/go-keyring
- macOS Security framework: https://developer.apple.com/documentation/security/keychain_services
- Linux Secret Service spec: https://specifications.freedesktop.org/secret-service-spec/
- Windows Wincred: https://learn.microsoft.com/en-us/windows/win32/api/wincred/
- Codex CLI Auth: https://developers.openai.com/codex/auth
