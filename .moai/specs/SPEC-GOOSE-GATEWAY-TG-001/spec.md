---
id: SPEC-GOOSE-GATEWAY-TG-001
version: 0.1.0
status: Planned (stub)
created: 2026-04-22
updated: 2026-04-22
author: session-decision
priority: P1
issue_number: null
phase: 9
size: 중(M)
lifecycle: spec-first
---

# SPEC-GOOSE-GATEWAY-TG-001 — Telegram Bot Gateway (Self-hosted) ★ v1.0 포함

> **상태**: 스켈레톤. M7 Week 4 진입 시 manager-spec이 완전 작성.

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-22 | GATEWAY-001 재정의 하위 구현. Telegram Bot API long-polling. v1.0 범위 포함(Hermes 복원 지시 반영). | 세션 결정 |

---

## 1. 개요

사용자 PC에서 Telegram Bot API long-polling으로 작동하는 원격 리모컨. **클라우드 0, 계정 0**. @BotFather에서 Bot Token 발급 후 GOOSE Desktop 설정 화면에 붙여넣기만 하면 활성.

## 2. 범위

- 포함: long-polling, sendMessage, inline keyboard, MarkdownV2, Trusted User 인증, Crisis keyword 이중 검사
- 제외: Telegram Payments, Voice, Video (v2.0+)
- **HARD**: Journal/Health/Identity Graph 본문 송출 금지 (SAFETY-001 Channel HARD rule 준수)

## 3. EARS 요구사항 (초안)

### 3.1 Ubiquitous
- **REQ-TG-001**: The Desktop app shall implement Telegram Bot API v6.0+ long-polling.
- **REQ-TG-002**: Bot Token shall be stored in OS keychain (macOS Keychain / Windows Credential Manager / Linux Secret Service), never in plaintext config file.

### 3.2 Event-driven
- **REQ-TG-010**: When user messages bot for the first time, bot shall require 6-digit auth code from Desktop app to establish Trusted User.
- **REQ-TG-011**: When Trusted User sends query, bot shall forward to local GOOSE QueryEngine and reply with response.
- **REQ-TG-012**: When query response contains Journal/Health category content, bot shall reply with placeholder and prompt user to open Desktop/Mobile app.

### 3.3 State-driven
- **REQ-TG-020**: While Desktop is offline, bot shall reply with "GOOSE PC is offline, try later" and queue message.
- **REQ-TG-021**: While Crisis keyword is detected in incoming message, bot shall respond with 1577-0199 (Korean) / local hotline + pause LLM execution.

### 3.4 Unwanted
- **REQ-TG-030**: If Bot Token is revoked on Telegram side, the app shall detect within 60s and prompt user re-input.
- **REQ-TG-031**: If untrusted user messages bot, bot shall respond with help message only (no LLM execution).

## 4. 인수 기준 (초안)

- **AC-TG-01**: @BotFather token 입력 후 30초 내 활성화
- **AC-TG-02**: 응답 지연 중앙값 < 2초 (쿼리 → 답변)
- **AC-TG-03**: Trusted User 인증 흐름 (6자리 코드)
- **AC-TG-04**: 가족 공유 (Trusted User 5명까지)
- **AC-TG-05**: Journal/Health 송출 차단 테스트 통과
- **AC-TG-06**: Crisis keyword 이중 감지 정확도 95%+

## 5. 의존성

- 상위: GATEWAY-001 (umbrella), QUERY-001, SAFETY-001
- 외부: Telegram Bot API (go-telegram-bot-api/telegram-bot-api v5)

## 6. 완료 정의 (DoD)

- macOS/Linux/Windows에서 동일 동작
- BotFather 가이드 사용자 문서 (스크린샷 포함)
- Trusted User 권한 세분화 UI (Calendar-read / Tool-exec / Admin)
- Telegram 메시지 30일 보관 정책 고지 (설정 화면)
