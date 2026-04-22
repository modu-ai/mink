---
id: SPEC-GOOSE-DISCOVERY-001
version: 0.1.0
status: Planned (stub)
created: 2026-04-22
updated: 2026-04-22
author: session-decision
priority: P0
issue_number: null
phase: 6
size: 소(S)
lifecycle: spec-first
---

# SPEC-GOOSE-DISCOVERY-001 — mDNS LAN P2P Discovery ★ v6.1 신규

> **상태**: 스켈레톤. M6 Week 2 진입 시 manager-spec이 완전 작성.

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-22 | Tier 0 Local 기본 동작 지원. 같은 WiFi 내에서 Mobile↔PC 클라우드 0으로 연결. | 세션 결정 |

---

## 1. 개요

Bonjour/mDNS `_goose._tcp.local.` 서비스 광고·발견 + QR 페어링 fallback. Tier 0 Local Only 모드의 핵심 인프라.

## 2. 범위

- 포함: mDNS 광고/발견 (hashicorp/mdns Go), QR 페어링 (EC-KEM + Noise XX)
- 제외: 외부망 NAT 통과 (CLOUD-001/BRIDGE-001 담당)

## 3. EARS 요구사항 (초안)

### 3.1 Ubiquitous
- **REQ-DISC-001**: The Desktop app shall advertise `_goose._tcp.local.` service on LAN while running.
- **REQ-DISC-002**: The Mobile app shall browse `_goose._tcp.local.` to discover peer Desktops.

### 3.2 Event-driven
- **REQ-DISC-010**: When Mobile discovers an unpaired Desktop, it shall prompt QR scan.
- **REQ-DISC-011**: When QR is scanned, Mobile and Desktop shall complete Noise XX handshake and exchange pinned public keys.

### 3.3 Unwanted
- **REQ-DISC-020**: If mDNS is blocked by router, the app shall fallback to QR-only pairing (manual IP input).

## 4. 인수 기준 (초안)

- **AC-DISC-01**: 같은 WiFi 내 30초 이내 발견
- **AC-DISC-02**: Noise XX handshake 1초 이내 완료
- **AC-DISC-03**: MITM 방어: 공격자 개입 시 handshake 실패

## 5. 의존성

- 상위: BRIDGE-001, AUTH-001
- 외부: `github.com/hashicorp/mdns`, `github.com/flynn/noise`

## 6. 완료 정의 (DoD)

- macOS/Linux/Windows에서 Bonjour/Avahi/mDNSResponder 호환 확인
- iOS/Android NSD 연동 (React Native bridge)
