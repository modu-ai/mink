---
id: SPEC-GOOSE-NOTIFY-001
version: 0.1.0
status: Planned (stub)
created: 2026-04-22
updated: 2026-04-22
author: session-decision
priority: P0
issue_number: null
phase: 6
size: 중(M)
lifecycle: spec-first
---

# SPEC-GOOSE-NOTIFY-001 — Messenger Gateway Notifications

> **v0.2 Amendment (2026-04-24)**: SPEC-GOOSE-ARCH-REDESIGN-v0.2 에 따라 **스코프 축소**.
> 제거: APNs (Apple Push Notification) 통합, FCM (Firebase Cloud Messaging) 통합, 네이티브 모바일 OS 푸시.
> 유지: **Messenger gateway push** (Telegram Bot API · Web UI SSE · CLI stderr 알림).
> 원격 알림은 각 messenger provider의 네이티브 채널을 활용.
> 기존 본문은 v6.0 맥락 참조용으로만 유지.

---

## 원본 타이틀 (v6.0): Unified Push Notifications ★ 스코프 축소됨

> **상태**: 스켈레톤. M6 Week 2 진입 시 manager-spec이 완전 작성.

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-22 | App-First 피벗으로 Hermes Gateway 푸시 대체. APNs + FCM + Desktop native 통합. Live Activity, Rich Notification 포함. | 세션 결정 |

---

## 1. 개요

모든 채널·플랫폼의 푸시 알림 통합 관리:
- **iOS**: APNs (p8 Key), Rich Notification, Live Activity (ActivityKit), Lock Screen Quick Reply
- **Android**: FCM, Big Text Style, Interactive Notification
- **Desktop**: macOS UserNotifications, Windows Toast, Linux libnotify

## 2. 범위

- 포함: 플랫폼 추상 인터페이스, payload 스키마, 스케줄링, Quick Reply action 처리
- 제외: 푸시 **발송 인프라** (CLOUD-001 담당, Tier 1+)
- 제외: 메신저 푸시 (GATEWAY-* SPECs 담당, 플랫폼이 자체 처리)

## 3. EARS 요구사항 (초안)

### 3.1 Ubiquitous
- **REQ-NOTIFY-001**: The app shall support rich notifications with inline quick reply on iOS 15+ and Android 7+.
- **REQ-NOTIFY-002**: The iOS app shall display Live Activity for Morning Briefing on Lock Screen.

### 3.2 Event-driven
- **REQ-NOTIFY-010**: When user taps Quick Reply on notification, the app shall capture input without launching.
- **REQ-NOTIFY-011**: When Ritual event fires at scheduled time, the app shall deliver notification through the highest-priority channel available (Mobile Push > Desktop Tray > Messenger).

### 3.3 State-driven
- **REQ-NOTIFY-020**: While Quiet Hours (23:00-06:00 user-local), the app shall NOT deliver non-urgent notifications.
- **REQ-NOTIFY-021**: While user is in "Do Not Disturb" mode, crisis-detected messages shall still deliver.

### 3.4 Unwanted
- **REQ-NOTIFY-030**: If APNs/FCM token is invalid, the app shall rotate token and retry once, then log for user re-auth.

## 4. 인수 기준 (초안)

- **AC-NOTIFY-01**: Morning Push 발송 성공률 99%+ (앱 실행 상태)
- **AC-NOTIFY-02**: Morning Push 지연 < 10초 (예약 시간 기준)
- **AC-NOTIFY-03**: Lock Screen Quick Reply 입력 → 저장 성공률 99%+
- **AC-NOTIFY-04**: iOS 17 Live Activity Dynamic Island 표시
- **AC-NOTIFY-05**: Android 13+ POST_NOTIFICATIONS permission 요청 흐름

## 5. 의존성

- 상위: SCHEDULER-001 (스케줄 트리거)
- 하위: CLOUD-001 (Tier 1+ 푸시 발송), WIDGET-001 (Live Activity 공유 타겟)
- 외부: APNs p8, FCM v1 API

## 6. 완료 정의 (DoD)

- iOS/Android 심사 통과 (푸시 권한 요청 흐름 검수)
- Channel HARD rule 통합: Journal/Health 본문 포함 푸시는 E2EE 채널만 허용
- 사용자 대시보드: 이번 주 푸시 수신 로그 (개인정보 제외)
