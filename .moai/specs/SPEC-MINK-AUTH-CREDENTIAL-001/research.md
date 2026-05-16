---
spec_id: SPEC-MINK-AUTH-CREDENTIAL-001
artifact: research.md
version: 0.2.0
created_at: 2026-05-16
updated_at: 2026-05-16
author: MoAI manager-spec
---

# Research — SPEC-MINK-AUTH-CREDENTIAL-001

OS keyring 통합 credential 저장소 (default + 평문 fallback) 의 본격 plan 진입 전 기술 조사. 본 문서는 라이브러리 선정, 플랫폼 매트릭스, OAuth 갱신 흐름, AGPL-3.0 라이선스 호환성, 위협 모델 비교를 다룬다. spec.md 의 REQ EARS 화는 본 문서 결론을 normative reference 로 인용한다.

---

## 1. 라이브러리 선정 — zalando/go-keyring v0.2.x

### 1.1 후보 비교

| 라이브러리 | 라이선스 | AGPL 호환 | macOS | Linux | Windows | file fallback | 비고 |
|-----------|---------|-----------|-------|-------|---------|---------------|------|
| `github.com/zalando/go-keyring` | MIT | ✓ | ✓ (Security.framework) | ✓ (Secret Service / libsecret) | ✓ (Wincred) | ✗ (별도 구현) | 단일 API, 안정적 |
| `github.com/99designs/keyring` | MIT | ✓ | ✓ | ✓ | ✓ | ✓ (file/pass/encryptedfile 내장) | 추상화 두꺼움, 의존성 증가 |
| `github.com/keybase/go-keychain` | MIT | ✓ | ✓ | ✗ | ✗ | ✗ | macOS 전용 fallback |
| platform-native (cgo/syscall) | n/a | ✓ | 가능 | 가능 (D-Bus 직접) | 가능 (Wincred syscall) | 별도 구현 | 유지보수 부담 큼 |

### 1.2 결정 — `zalando/go-keyring`

**채택 사유**:
- 단일 API surface (`Set`, `Get`, `Delete`, `DeleteAll`) → MINK Service interface 와 1:1 매핑 용이
- 의존성 최소화 (Linux 만 godbus 추가 의존)
- AGPL-3.0 헌장 §2 정합 (MIT, 가중치 학습 영향 없음 — credential 저장 코드 자체에 모델 가중치 사용 없음)
- 99designs/keyring 대비 1/3 코드 크기, 1/2 의존 패키지

**file fallback 은 자체 구현**: `internal/auth/file/` 에 mode 0600 + atomic rename + JSON schema validation 으로 독립 모듈로 분리. 99designs/keyring 의 file backend 는 MINK 의 schema 통합 요구 (5 LLM + 3 채널 구조) 와 정합 어렵다.

### 1.3 라이선스 정합 (ADR-002 + 라이선스 헌장)

| 의존 라이브러리 | 라이선스 | 사용 부분 |
|-----------------|---------|----------|
| `github.com/zalando/go-keyring` v0.2.x | MIT | keyring backend |
| `golang.org/x/oauth2` | BSD-3-Clause | OAuth 2.1 refresh client (Codex) |
| `github.com/godbus/godbus/v5` (transitive, Linux only) | BSD-2-Clause | Secret Service D-Bus client |

모두 AGPL-3.0 호환 (AGPL-compatible permissive licenses). ADR-001 의 가중치 학습 0 헌장과 무관 (credential 저장 코드는 모델 학습 데이터 흐름과 격리).

---

## 2. 플랫폼 매트릭스 — Native API 동작

### 2.1 macOS — Keychain Services (Security.framework)

go-keyring 의 macOS 백엔드는 `SecItem*` modern API 호출 (`SecItemAdd`, `SecItemCopyMatching`, `SecItemDelete`). 평면 `security` CLI 우회.

| 항목 | 값 |
|------|-----|
| `kSecClass` | `kSecClassGenericPassword` |
| `kSecAttrService` | `mink:auth:{provider}` (예: `mink:auth:anthropic`) |
| `kSecAttrAccount` | `default` (single account 정책, UN-3) |
| `kSecValueData` | UTF-8 bytes of secret JSON |
| `kSecAttrAccessGroup` | (미지정 — sandbox 미적용) |
| `kSecAttrAccessControl` | `kSecAccessControlBiometryCurrentSet` (선택, OP-4) |

**잠금 동작**: 사용자 macOS 로그인 시 default keychain 자동 unlock. 화면 잠금만으로는 keychain 잠기지 않음 (sleep 종료 후 일정 시간 idle 시 잠금 정책 사용자 설정 가능). 잠긴 keychain 호출 시 `errSecInteractionNotAllowed` (-25308) → 자동 unlock prompt 표시 (UI 환경) 또는 SD-1 trigger (headless / SSH).

**Touch ID / Apple Watch 인증** (OP-4 후보, 본 SPEC 도입 보류): `kSecAttrAccessControl` 에 `kSecAccessControlBiometryCurrentSet` 설정 시 매 Load 시 생체 인증. 사용자 경험 마찰 큼 → 본 SPEC 은 standard 접근으로 통일, 후속 SPEC 에서 옵션화.

### 2.2 Linux — Secret Service (freedesktop.org spec)

go-keyring 은 godbus 를 통해 `org.freedesktop.secrets` D-Bus 인터페이스 호출. 백엔드는 GNOME Keyring (`gnome-keyring-daemon`) / KWallet (KDE) / KeePassXC 가능.

| 항목 | 값 |
|------|-----|
| Collection | `/org/freedesktop/secrets/aliases/default` (보통 `login` collection) |
| Item attributes | `service: mink:auth:{provider}`, `account: default` |
| Encryption transport | D-Bus session bus (Dh-ietf1024-sha256-aes128-cbc-pkcs7 또는 plain — collection unlocked 동안) |
| 의존 패키지 | `libsecret-1-0` (Debian/Ubuntu), `libsecret` (Arch), `gnome-keyring` 또는 KWallet 데몬 |

**감지 실패 시나리오**:
1. `DBUS_SESSION_BUS_ADDRESS` 미설정 → headless / SSH session → go-keyring 에러 `secret service unavailable`
2. libsecret 미설치 → D-Bus 객체 없음 → `org.freedesktop.DBus.Error.ServiceUnknown`
3. WSL2 (D-Bus 없는 환경) → SSH 와 동일
4. Docker container 기본 (no session D-Bus) → 동일

이 경우 SD-1 trigger → `mink config get auth.store` 확인 후 file fallback 자동 사용 (사용자 동의 옵트인 시) 또는 명시적 `mink config set auth.store file` 안내.

**Collection 잠금**: GNOME Keyring 기본 default collection 은 로그인 시 자동 unlock. KWallet 는 사용자 설정. unlock prompt 가 D-Bus 통해 발송되나, headless 환경에서는 즉시 실패 → SD-1.

### 2.3 Windows — Credential Manager (Wincred)

go-keyring 은 syscall (cgo 없이 `golang.org/x/sys/windows` 의존 없이 ntdll 호출) 로 Wincred API 직접 호출.

| 항목 | 값 |
|------|-----|
| API | `CredWriteW`, `CredReadW`, `CredDeleteW` |
| `CRED_TYPE` | `CRED_TYPE_GENERIC` |
| `TargetName` | `mink:auth:{provider}` (UTF-16) |
| `UserName` | `default` |
| `CredentialBlob` | UTF-8 bytes |
| 보호 | DPAPI (Data Protection API) — 사용자별 master key 자동 격리 |
| Roaming | `CRED_PERSIST_LOCAL_MACHINE` 사용, 도메인 동기화 아님 |

Windows 는 keyring 항상 사용 가능 (별도 데몬 불필요, OS 가 보장). WSL1/WSL2 Linux 측은 Linux 매트릭스 적용. Windows native MINK binary 에서만 Wincred 적용.

### 2.4 size 제약

- macOS Keychain: secret 데이터 사실상 제한 없음 (수 MB 가능, 권장 < 1MB)
- Linux Secret Service: libsecret 명시적 제한 없으나 D-Bus message size 권장 < 64KB
- Windows Wincred: CredentialBlobSize <= 2560 bytes (메모 제한)

**MINK 의 single credential 최대 크기 추정**:
- API key: 100~200 bytes
- Codex OAuth (access_token + refresh_token + expires_at + scope): ~2KB
- Slack signing+bot (2 토큰 + signing secret): ~500 bytes
- Discord (Ed25519 public key 32 bytes hex + bot token): ~200 bytes

모두 Windows 2560 bytes 제한 내. 향후 OAuth scope 확장 시 Windows 제한 근접 가능 → chunking 미구현 (M3 OAuth refresh 에서 monitor).

---

## 3. OAuth 2.1 Refresh Flow — Codex (ChatGPT) 기준

### 3.1 Codex CLI OAuth 동작

`mink login codex` 는 ChatGPT (OpenAI) OAuth 2.1 + PKCE flow 실행:

```
1. mink → 사용자 브라우저 launch: https://auth.openai.com/authorize?...&code_challenge_method=S256
2. 사용자 ChatGPT 로그인 + consent
3. OpenAI → mink local HTTP listener (127.0.0.1:random_port) callback with authorization_code
4. mink → POST https://auth.openai.com/oauth/token
   body: grant_type=authorization_code, code, code_verifier (PKCE), redirect_uri
5. 응답: access_token (TTL 60분), refresh_token (TTL 8일 idle), expires_in
6. mink → credential storage: {access_token, refresh_token, expires_at, scope}
```

### 3.2 Refresh Flow

```
1. LLM-ROUTING-V2 가 Codex provider 호출 요청
2. mink → credential Load(codex)
3. 검사: expires_at < now() + 60s (60초 safety margin)
4. expired → POST https://auth.openai.com/oauth/token
   body: grant_type=refresh_token, refresh_token
5. 응답: 새 access_token (+ 일부 경우 새 refresh_token — rotation)
6. mink → credential Store(codex) 갱신
7. LLM-ROUTING-V2 에 access_token 반환
```

### 3.3 8일 idle 만료

OpenAI 정책 (2026-04 기준): refresh_token 은 30일 절대 만료 + **8일 idle 만료** (refresh_token 으로 갱신 호출이 8일간 없으면 무효). MINK 가 8일간 미실행 시 다음 실행에서 refresh 실패 → `invalid_grant` 응답 → ED-5 trigger → 사용자에게 `mink login codex` 재실행 안내.

**감지**: HTTP 400 + `{"error": "invalid_grant"}` 또는 HTTP 401. 다른 에러 (네트워크 / OpenAI 서버 5xx) 와 구분 후 재인증 prompt 발송.

### 3.4 refresh_token rotation

OpenAI 는 refresh 시 새 refresh_token 발급 (rotation 정책). 응답에서 `refresh_token` 필드 존재하면 갱신 저장. 누락 시 기존 refresh_token 유지.

### 3.5 다른 5 provider 의 refresh

| Provider | 인증 방식 | refresh 필요? |
|----------|----------|--------------|
| Anthropic (Claude) | API key paste | ✗ (영구) |
| DeepSeek | API key paste | ✗ (영구) |
| OpenAI GPT | API key paste | ✗ (영구, ChatGPT OAuth 와 별개) |
| Codex (ChatGPT) | OAuth 2.1 + PKCE | ✓ (8일 idle) |
| z.ai GLM | API key paste | ✗ (영구) |

API key paste 4종은 사용자 명시적 revoke 전까지 영구 유효. 만료 알림 cron 은 OUT OF SCOPE (별도 후속 SPEC).

---

## 4. Fallback File Schema (`~/.mink/auth/credentials.json`)

### 4.1 경로 + 권한

| OS | 경로 | 권한 |
|----|------|------|
| macOS / Linux | `$HOME/.mink/auth/credentials.json` | `0600` (POSIX `chmod 600`) |
| Windows | `%USERPROFILE%\.mink\auth\credentials.json` | NTFS ACL: 현재 사용자 only (FullControl), Administrators (FullControl), 다른 사용자 차단 |

부모 디렉터리 `~/.mink/auth/` 권한:
- POSIX: `0700` (drwx------)
- Windows: 동일 ACL 정책 (사용자 + Administrators only)

### 4.2 JSON Schema

```json
{
  "version": 1,
  "credentials": {
    "anthropic": {
      "kind": "api_key",
      "value": "sk-ant-...",
      "stored_at": "2026-05-16T20:00:00Z"
    },
    "deepseek": {
      "kind": "api_key",
      "value": "sk-...",
      "stored_at": "2026-05-16T20:00:00Z"
    },
    "openai_gpt": {
      "kind": "api_key",
      "value": "sk-proj-...",
      "stored_at": "2026-05-16T20:00:00Z"
    },
    "codex": {
      "kind": "oauth",
      "access_token": "sess-...",
      "refresh_token": "rt-...",
      "expires_at": "2026-05-16T21:00:00Z",
      "scope": "openid email profile offline_access",
      "stored_at": "2026-05-16T20:00:00Z"
    },
    "zai_glm": {
      "kind": "api_key",
      "value": "...",
      "stored_at": "2026-05-16T20:00:00Z"
    },
    "telegram_bot": {
      "kind": "bot_token",
      "value": "123:ABC...",
      "stored_at": "2026-05-16T20:00:00Z"
    },
    "slack": {
      "kind": "slack_combo",
      "signing_secret": "...",
      "bot_token": "xoxb-...",
      "stored_at": "2026-05-16T20:00:00Z"
    },
    "discord": {
      "kind": "discord_combo",
      "public_key": "0123...hex32",
      "bot_token": "Bot ...",
      "stored_at": "2026-05-16T20:00:00Z"
    }
  }
}
```

### 4.3 atomic write

`os.WriteFile(tempPath, data, 0600)` → `os.Rename(tempPath, finalPath)` 패턴. 동일 디렉터리 내 rename 은 POSIX atomic, Windows 는 `MoveFileEx` 사용 (Go runtime).

### 4.4 평문 저장 결정 근거

`~/.mink/auth/credentials.json` 은 **암호화하지 않는다**:

- 사용자가 명시적으로 keyring fallback 옵트인 (=keyring 사용 불가 환경) → 보안 환경 자체가 약함
- chacha20poly1305 / AES-GCM 적용 시 key derivation 필요 → OS-derived key 는 결국 keyring 의존 (재귀 문제) 또는 passphrase prompt 매 실행 (사용자 마찰)
- mode 0600 + 부모 디렉터리 0700 은 동일 사용자 프로세스 외 접근 차단 → POSIX/NTFS 가 보장
- iCloud / OneDrive / Dropbox 동기화 폴더 제외는 사용자 책임 (문서화로 안내)

대안 검토 (도입 보류): age (CLI tool) 통합으로 사용자 passphrase 기반 암호화 → 후속 SPEC 에서 OP 항목으로 평가.

---

## 5. 위협 모델 + 경쟁 프로젝트 비교

| 위험 | 평문 `~/.mink/auth/credentials.json` mode 0600 | OS keyring (default) |
|------|-----------------------------------------------|---------------------|
| 동일 사용자 다른 프로세스 (예: malicious npm package) | 노출 (file read 가능) | 격리 (process-level 접근 제한 — macOS Keychain access prompt / Wincred DPAPI master key) |
| 잠금 노트북 cold-boot 메모리 덤프 | 평문 file 직접 읽힘 | 디스크상 암호화 (macOS FileVault 가정 / Windows DPAPI) — 평문화 불가 |
| iCloud / OneDrive / Dropbox 백업 폴더 | 클라우드 동기화 가능 | OS keyring 은 자동 동기화 제외 |
| OAuth refresh_token long-lived (~30일) | 위험 큼 | 위험 작음 |
| 가족 공용 계정 | 동일 사용자 — file 노출 | 동일 — keyring 동일 사용자 접근 가능 |

### 5.1 경쟁 프로젝트 비교

| 프로젝트 | default 저장 | keyring 옵션 | refresh token 처리 |
|---------|-------------|--------------|------------------|
| Hermes (구 OpenClaw) | `~/.hermes/auth.json` 평문 | 옵션 (별도 설정) | 수동 갱신 |
| Codex CLI (OpenAI) | `~/.codex/auth.json` 평문 | ✗ | 자동 갱신 |
| 1Password CLI | 1Password vault | n/a (자체 vault) | 자체 처리 |
| Hashicorp Vault Agent | encrypted file / Vault server | n/a | 자체 처리 |
| **MINK (본 SPEC)** | **OS keyring** | n/a (default) + file fallback | **자동 갱신** |

**차별점**: MINK 는 keyring default + 자동 갱신 조합. 사용자가 별도 설정 없이 보안 베이스라인 확보. Hermes/Codex CLI 와 비교 시 보안 우위.

---

## 6. AGPL-3.0 헌장 정합 검증 (ADR-002 + §2)

### 6.1 헌장 §2 — 가중치 학습 사용 0

본 SPEC 은 credential 저장 코드만 다룸. 모델 가중치 / 학습 데이터 흐름 없음 (LLM-ROUTING-V2 가 호출 받음 → 별도 SPEC). 헌장 §2 정합.

### 6.2 ADR-001 — 사용자 데이터 zero-weight retention

Codex OAuth refresh_token 도 **사용자 자격증명** (= 사용자 데이터). MINK 가 자동 갱신을 위해 refresh_token 을 로컬 저장하지만:

- 갱신 트래픽은 OpenAI 와 MINK 사용자 머신 사이만 (MINK 클라우드 무경유)
- MINK telemetry / 학습 데이터로 refresh_token 전송 금지 (UN-5)
- 사용자 mink uninstall 시 keyring 항목 + fallback file 둘 다 삭제 (USERDATA-MIGRATE-001 정합)

ADR-001 정합.

### 6.3 라이선스 의존성 검증

§1.3 참조. 의존 패키지 모두 AGPL-3.0 호환 permissive license (MIT/BSD-2/BSD-3). copyleft 충돌 없음.

---

## 7. CROSSPLAT-001 §5.1 가드 정합

CROSSPLAT-001 §5.1 (PR #198 머지) 가 install 단계에서 OS detection + 의존 라이브러리 probe 를 정의. 본 SPEC 의 keyring 사용 가능 여부 detection 통합:

| Phase | 동작 |
|-------|------|
| install.sh / install.ps1 | Linux: `pkg-config --exists libsecret-1` 또는 `ldconfig -p \| grep libsecret` probe → 미설치 시 사용자에게 안내 (`sudo apt install libsecret-1-0` 등) |
| install (모든 OS) | `mink doctor auth-keyring` subcommand 즉시 실행 → keyring backend 가용성 표시 |
| ONBOARDING-001 Phase 5 (Web Step 2 / CLI wizard) | 동일 doctor 결과 표시 + 사용자 선택 prompt (keyring / file) |

WSL2 + headless Linux 환경에서는 `mink doctor` 가 "keyring unavailable → file fallback 권장" 결과 출력. 사용자가 `mink config set auth.store file` 로 명시 선택.

---

## 8. 결론 — Plan EARS 입력 기반

본 research 의 7 항목이 spec.md EARS 30 REQ + acceptance.md 32 AC 의 normative input:

1. 라이브러리 `zalando/go-keyring` 채택 (§1) → UB-7, UB-9
2. 플랫폼 매트릭스 (§2) → SD-1, UB-9
3. OAuth refresh flow (§3) → ED-4, ED-5, SD-4, UN-4
4. fallback file schema (§4) → UB-6, SD-2, UN-6
5. 위협 모델 비교 (§5) → UB-2, UB-5, UN-1
6. AGPL 헌장 정합 (§6) → UB-3, UB-5, UN-5
7. CROSSPLAT 정합 (§7) → UB-9, SD-1

각 항목은 plan.md 의 마일스톤 (M1~M4) 매핑 및 tasks.md 의 task ID 와 cross-reference 된다.

---

## 9. 미해결 / 후속 평가 항목

| 항목 | 처리 |
|------|------|
| age (CLI) passphrase-encrypted fallback | OP, 별도 SPEC |
| Touch ID / Apple Watch biometric unlock | OP-4 보류, 별도 SPEC |
| 1Password CLI integration (`op` command) | OP-2 보류 |
| HSM / Secure Enclave 직접 통합 | OP-1 보류 |
| 만료 알림 cron / notification | OUT OF SCOPE, 별도 후속 |
| 다중 계정 per provider (multi-account) | OUT OF SCOPE, 별도 SPEC |
| keyring 항목 chunking (Windows 2560 bytes 초과 시) | 본 SPEC 데이터 크기 추정 모두 제한 내 — M3 monitor only |
| credential rotation API (강제 갱신) | M3 task T-013 일부 (Store overwrite 으로 충분) |

---

Version: 0.2.0
Last Updated: 2026-05-16
References:
- ADR-001 (사용자 데이터 zero-weight retention)
- ADR-002 (AGPL-3.0 전환)
- SPEC-MINK-CROSSPLAT-001 §5.1
- SPEC-MINK-USERDATA-MIGRATE-001
- SPEC-MINK-LLM-ROUTING-V2-AMEND-001
- zalando/go-keyring (https://github.com/zalando/go-keyring)
- freedesktop.org Secret Service (https://specifications.freedesktop.org/secret-service-spec/)
- OpenAI Codex CLI Auth (https://developers.openai.com/codex/auth)
