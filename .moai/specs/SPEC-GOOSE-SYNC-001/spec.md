---
id: SPEC-GOOSE-SYNC-001
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

# SPEC-GOOSE-SYNC-001 — Multi-Device Encrypted Sync (Tier 2) ★ v6.1 신규

> **상태**: 스켈레톤. v1.3 Tier 2 Cloud Plus 진입 시 manager-spec이 완전 작성.

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-22 | Tier 2 Cloud Plus 전용. Mac + Windows + iPhone + iPad 동시 사용자 멀티 디바이스 상태 일관성 보장. 암호화 전제. | 세션 결정 |

---

## 1. 개요

멀티 디바이스 상태 싱크. 사용자 파생 키로 암호화된 CRDT 또는 op-log 기반. 클라우드는 **암호화 blob만 전달**.

## 2. 범위

- 포함: 설정·대화 히스토리·Ritual 참여 기록·Calendar OAuth token
- 제외: Journal 본문·Health 데이터·Identity Graph (Tier 0/1에서도 로컬 전용 HARD)
- 대안 허용: Tier 2 사용자 명시적 opt-in 시 Journal 암호화 백업만 지원 (싱크는 불가)

## 3. EARS 요구사항 (초안)

### 3.1 Ubiquitous
- **REQ-SYNC-001**: The Sync service shall use end-to-end encryption with user-derived keys.
- **REQ-SYNC-002**: The Cloud shall store only encrypted blobs indexed by user sha256+salt.

### 3.2 Event-driven
- **REQ-SYNC-010**: When a device modifies synced state, it shall emit an encrypted op-log entry.
- **REQ-SYNC-011**: When a device comes online, it shall fetch new op-log entries and apply in order.

### 3.3 State-driven
- **REQ-SYNC-020**: While offline, the device shall queue op-log entries locally.

### 3.4 Unwanted
- **REQ-SYNC-030**: If decryption fails on a device, the device shall surface recovery flow via email.
- **REQ-SYNC-031**: If Journal/Health data is attempted to sync, operation shall fail with `ERR_SYNC_CATEGORY_FORBIDDEN`.

## 4. 인수 기준 (초안)

- **AC-SYNC-01**: 4개 기기(Mac/Windows/iPhone/iPad) 동시 사용 일관성
- **AC-SYNC-02**: 오프라인 → 온라인 전환 시 30초 내 동기화
- **AC-SYNC-03**: 암호화 키 복구 플로우 (백업 시드 12단어)

## 5. 의존성

- 상위: CLOUD-001, AUTH-001
- 기술: Automerge-2 (CRDT) 또는 자체 op-log

## 6. 완료 정의 (DoD)

- Tier 2 구독자 전용 활성
- 기기 추가/제거 UX
- Journal/Health HARD rule 명문화 및 자동 테스트
