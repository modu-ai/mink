---
id: SPEC-GOOSE-WIDGET-001
version: 0.1.0
status: Planned (stub)
created: 2026-04-22
updated: 2026-04-22
author: session-decision
priority: P1
issue_number: null
phase: 6
size: 중(M)
lifecycle: spec-first
---

# SPEC-GOOSE-WIDGET-001 — Home Screen Widget + Live Activity ★ v6.0 신규

> **상태**: 스켈레톤. M6 Week 6 진입 시 manager-spec이 완전 작성.

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-22 | App-First 피벗. iMessage 직접 통합 불가 대응으로 "Apple Native" 채널 강화. 위젯을 통한 zero-tap 브리핑 소비. | 세션 결정 |

---

## 1. 개요

사용자가 앱 실행 없이 GOOSE 정보·조작 가능:
- **iOS**: WidgetKit (Home Screen/Lock Screen/Smart Stack/Standby), Interactive Widget (iOS 17+)
- **Android**: Glance (Jetpack Compose for Widgets), App Widget
- **Desktop**: macOS Notification Center Widget, Windows Start Menu Tile, Linux GNOME Extension (옵션)

## 2. 범위

- 포함: Morning Briefing 위젯, 오늘 일정 위젯, 빠른 일기 입력 위젯, Live Activity
- 제외: 위젯 내부 LLM 응답 (백그라운드 fetch만, 실시간 질의 불가)

## 3. EARS 요구사항 (초안)

### 3.1 Ubiquitous
- **REQ-WIDGET-001**: The iOS app shall provide a Small/Medium/Large Home Screen widget showing today's briefing.
- **REQ-WIDGET-002**: The Android app shall provide a 4x2 App Widget with equivalent content.

### 3.2 Event-driven
- **REQ-WIDGET-010**: When 07:00 Morning Ritual fires, the Widget content shall refresh automatically via background task.
- **REQ-WIDGET-011**: When user taps Interactive Widget button (iOS 17+), the action shall execute without launching the app.

### 3.3 State-driven
- **REQ-WIDGET-020**: While Live Activity is active (Morning Ritual window), Dynamic Island shall show compact view.
- **REQ-WIDGET-021**: While app is in background, Widget timeline shall update every 30 minutes.

### 3.4 Unwanted
- **REQ-WIDGET-030**: If user data is sensitive (Journal/Health), Widget shall show placeholder icon only, not content.

## 4. 인수 기준 (초안)

- **AC-WIDGET-01**: iOS Small/Medium/Large 3종 + Lock Screen Rectangular
- **AC-WIDGET-02**: Android 1x1/2x2/4x2 3종
- **AC-WIDGET-03**: Live Activity Compact/Expanded/Minimal 3상태
- **AC-WIDGET-04**: 위젯 렌더링 < 50ms (Timeline Provider)
- **AC-WIDGET-05**: iOS 17 Action Button 등록 (iPhone 15 Pro+)

## 5. 의존성

- 상위: MOBILE-001, DESKTOP-001, NOTIFY-001
- 플랫폼: iOS 16.1+ (ActivityKit), Android 12+ (Glance)

## 6. 완료 정의 (DoD)

- App Store 위젯 스크린샷 확보
- Widget Preview Gallery 등록
- Channel HARD rule 준수 (Journal 본문 위젯 표시 금지)
