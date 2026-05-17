# MINK Credential 관리 가이드

SPEC: SPEC-MINK-AUTH-CREDENTIAL-001 | 버전: 0.3.0 | 최종 업데이트: 2026-05-17

---

## 1. 개요

MINK는 8종 제공자(Anthropic, DeepSeek, OpenAI GPT, z.ai GLM, OpenAI Codex, Telegram, Slack, Discord)의 credential을 **OS keyring**(기본값) 또는 **평문 파일**(fallback)에 저장합니다.

- **기본값(keyring)**: macOS Keychain, Linux Secret Service(libsecret), Windows Credential Manager에 암호화 저장
- **fallback(file)**: `~/.mink/auth/credentials.json` (권한 0600, 사용자만 읽기 가능)
- **자동 fallback(keyring,file)**: keyring을 먼저 시도하고, 사용 불가 시 file로 전환

설정 변경: `mink config set auth.store {keyring|file|keyring,file}`

---

## 2. 보안 트레이드오프

| 항목 | keyring | file (0600) |
|------|---------|-------------|
| 암호화 수준 | OS 네이티브 (AES-256, TPM 연동 가능) | 없음 (권한 비트만) |
| 타 프로세스 접근 | OS가 차단 | root 사용자 접근 가능 |
| 클라우드 동기화 위험 | 없음 | 높음 (아래 §4 참조) |
| 백업 포함 여부 | macOS 백업 제외 가능 | 포함됨 |
| 헤드리스/서버 환경 | 불가(GUI 잠금 해제 필요) | 가능 |

**권장**: 개인 로컬 환경은 `keyring`, CI/서버 환경은 환경 변수 또는 `file` 후 접근 제어 강화.

---

## 3. 8종 Credential 등록 방법

### 3.1 API Key 방식 (Anthropic, DeepSeek, OpenAI GPT, z.ai GLM)

입력값이 화면에 표시되지 않습니다.

```
mink login anthropic
mink login deepseek
mink login openai_gpt
mink login zai_glm
```

프롬프트 예시:
```
[anthropic] API Key를 입력하세요 (입력값은 화면에 표시되지 않습니다):
API Key: (입력)
anthropic credential이 저장되었습니다 (***4321).
```

### 3.2 OpenAI Codex — OAuth 2.1 + PKCE 브라우저 인증

Codex는 API Key가 없으며 OAuth 2.1 + PKCE 방식을 사용합니다.

```
mink login codex
```

실행 흐름:
1. 브라우저가 자동으로 열립니다 (또는 URL을 수동으로 열 수 있습니다).
2. OpenAI 계정으로 로그인하고 권한을 승인합니다.
3. `mink`가 로컬 콜백(127.0.0.1 임의 포트)을 수신하고 토큰을 저장합니다.

토큰 만료 시 `mink login codex`를 다시 실행하면 됩니다 (§7 참조).

### 3.3 Telegram Bot — Bot Token

```
mink login telegram_bot
```

프롬프트 예시:
```
[telegram_bot] Bot Token을 입력하세요 (입력값은 화면에 표시되지 않습니다):
Bot Token: (입력, 형식: 123456789:AABBcc...)
telegram_bot credential이 저장되었습니다 (***cc...).
```

Telegram Bot Token은 @BotFather에서 발급받을 수 있습니다.

### 3.4 Slack — Signing Secret + Bot Token

```
mink login slack
```

프롬프트 예시:
```
[slack] Slack credential을 입력하세요 (입력값은 화면에 표시되지 않습니다):
Signing Secret: (입력)
Bot Token: (입력, xoxb-... 형식)
slack credential이 저장되었습니다 (***xxxx).
```

- **Signing Secret**: Slack 앱 설정 → Basic Information → App Credentials
- **Bot Token**: OAuth & Permissions 페이지 → Bot User OAuth Token

### 3.5 Discord — Public Key + Bot Token

```
mink login discord
```

프롬프트 예시:
```
[discord] Discord credential을 입력하세요 (입력값은 화면에 표시되지 않습니다):
Public Key (64자 hex): (입력)
Bot Token: (입력)
discord credential이 저장되었습니다 (***xxxx).
```

- **Public Key**: Discord Developer Portal → Application → General Information (64자 hex)
- **Bot Token**: Bot 탭 → Token 복사

---

## 4. 클라우드 폴더 위험 안내

파일 backend(`auth.store: file`)를 사용할 때, `~/.mink/` 폴더가 클라우드 동기화 경로 아래에 있으면 **credential이 외부 서버에 업로드**될 수 있습니다.

감지되는 경로 예시:
- macOS: `~/Library/Mobile Documents/com~apple~CloudDocs/` (iCloud Drive)
- `~/OneDrive/`
- `~/Dropbox/`
- `~/Google Drive/`
- `~/Box/`

위 경로에 `~/.mink/`가 위치할 경우 MINK는 경고를 출력합니다:

```
경고: ~/.mink/auth/credentials.json 이 클라우드 동기화 폴더 아래에 있습니다.
      credential이 외부 서버에 업로드될 수 있습니다. 폴더를 이동하거나
      auth.store: keyring 으로 전환하는 것을 권장합니다.
```

**권장 조치**: `mink config set auth.store keyring`으로 전환하거나, `~/.mink/`를 동기화 대상 외 경로로 이동하세요.

---

## 5. `mink config set auth.store` — 저장소 모드 변경

| 값 | 동작 |
|----|------|
| `keyring` | OS keyring만 사용. 불가 시 오류 반환 |
| `file` | 평문 파일만 사용. keyring 미접촉 |
| `keyring,file` | keyring 먼저 시도, 불가 시 file로 자동 전환 |

```
mink config set auth.store keyring
mink config set auth.store file
mink config set auth.store keyring,file
```

변경 즉시 반영됩니다. 기존에 저장된 credential은 이동하지 않습니다.
새 backend에 다시 `mink login {provider}`로 등록하거나, `mink logout → mink login` 순서로 이동하세요.

---

## 6. `mink doctor auth-keyring` — 상태 확인

모든 제공자의 등록 여부와 활성 backend를 확인합니다.

```
mink doctor auth-keyring
```

출력 예시:
```
Backend: keyring

PROVIDER      STATUS
--------      ------
anthropic     present (***4321)
deepseek      missing
openai_gpt    missing
codex         present (***xyz)
zai_glm       missing
telegram_bot  present (***abc)
slack         missing
discord       missing
```

- `present (***LAST4)`: 저장됨, 마지막 4자만 표시 (평문 비노출 보장)
- `missing`: 미등록
- `error: keyring unavailable`: OS keyring 접근 불가 (file fallback 권장)

---

## 7. Codex OAuth 재인증

Codex access token은 8일 idle 또는 `invalid_grant` 오류 시 자동 만료됩니다.
만료 시 MINK는 다음 메시지를 표시합니다:

```
Codex 인증이 만료되었습니다. `mink login codex` 를 다시 실행해 주세요.
```

재인증:
```
mink login codex
```

유효 기간 내 만료 임박(60초 전)에는 자동으로 갱신됩니다(silent refresh).
갱신이 실패하면 위 메시지가 표시됩니다.

---

## 8. Credential 삭제

```
mink logout {provider}
```

keyring과 file backend 양쪽에서 모두 삭제합니다(idempotent). 미등록 상태에서 실행해도 오류가 발생하지 않습니다.

```
mink logout anthropic
mink logout codex
mink logout slack
```

---

## 9. OP / HSM placeholder (미구현)

`auth.store: hsm` 및 `auth.store: op-cli` 식별자는 예약되어 있으며, 현재 구현되지 않았습니다.
설정 시 "NotImplemented" 안내가 표시됩니다. 향후 amendment SPEC에서 구현 예정입니다.

---

## 10. 참고

- [SPEC-MINK-AUTH-CREDENTIAL-001](.moai/specs/SPEC-MINK-AUTH-CREDENTIAL-001/spec.md)
- [ADR-001: AGPL-3.0-only 라이선스 헌장](.moai/specs/SPEC-ADR-001/spec.md)
- [ADR-002: Go 단일 런타임 선택](.moai/specs/SPEC-ADR-002/spec.md)
