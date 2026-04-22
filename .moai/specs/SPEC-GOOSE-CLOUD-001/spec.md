---
id: SPEC-GOOSE-CLOUD-001
version: 0.1.0
status: Planned (stub)
created: 2026-04-22
updated: 2026-04-22
author: session-decision
priority: P0
issue_number: null
phase: 6
size: 대(L)
lifecycle: spec-first
---

# SPEC-GOOSE-CLOUD-001 — Zero-Knowledge Thin Cloud ★ v6.1 신규

> **상태**: 스켈레톤. M6 Week 2 진입 시 manager-spec이 완전 작성.

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-22 | ROADMAP v6.1 3-Tier 아키텍처 도입에 따른 신규 SPEC. Tier 1 Cloud Free + Tier 2 Cloud Plus 인프라 정의. | 세션 결정 |

---

## 1. 개요

Tier 1/2 클라우드 인프라. **Zero-Knowledge 원칙** 준수:
- STUN/TURN 릴레이 (NAT 통과)
- APNs/FCM Push Relay (암호화 payload blob만 보관)
- Device Registry (공개키만)
- 이메일 인증 (sha256+salt 해시만)

클라우드는 **Journal/Health/Identity Graph/LLM Prompt 절대 저장·접근 불가**.

## 2. 범위

- 포함: STUN (Pion Go), TURN, Push Relay, Device Registry, Auth API
- 제외: LLM 추론 (Tier 2 Cloud Plus는 별도 SPEC)
- 배포: `goose-cloud/` 별도 리포 (OSS), Hetzner/Cloudflare Workers/Fly.io 중 선택

## 3. EARS 요구사항 (초안)

### 3.1 Ubiquitous
- **REQ-CLOUD-001**: The Cloud service shall expose only STUN, TURN, Push Relay, Device Registry, and Auth endpoints.
- **REQ-CLOUD-002**: The Cloud service shall never store plaintext payload beyond routing metadata.

### 3.2 Event-driven
- **REQ-CLOUD-010**: When a PC pushes a scheduled notification payload, the Cloud shall store it as an opaque encrypted blob indexed by device token hash.
- **REQ-CLOUD-011**: When the scheduled delivery time arrives, the Cloud shall forward the blob to APNs/FCM without decryption.

### 3.3 State-driven
- **REQ-CLOUD-020**: While a device is paired, the Cloud shall maintain the device public key in Registry.
- **REQ-CLOUD-021**: While a user account exists, the Cloud shall store only sha256+salt of the email.

### 3.4 Unwanted
- **REQ-CLOUD-030**: If any attempt is made to write Journal/Health/Identity data to Cloud storage, the operation shall fail with error `ERR_CLOUD_FORBIDDEN_CATEGORY`.
- **REQ-CLOUD-031**: If LLM prompts are routed through Cloud (Tier 2 Plus only), user identifier shall be permanently separated via pepper-based one-way token.

## 4. 인수 기준 (초안)

- **AC-CLOUD-01**: 3rd-party 감사 통과 (Trail of Bits 급)
- **AC-CLOUD-02**: Reproducible build + Sigstore 서명
- **AC-CLOUD-03**: STUN/TURN NAT 통과율 95%+ (대칭 NAT 제외 조건)
- **AC-CLOUD-04**: Push 지연 < 5초 (APNs/FCM 평균)
- **AC-CLOUD-05**: 1,000 DAU 기준 월 운영비 < $400

## 5. 의존성

- 상위: BRIDGE-001, NOTIFY-001, AUTH-001
- 내부: DISCOVERY-001과 상호보완 (LAN fallback)
- 외부: Pion (Go STUN/TURN), APNs (Apple), FCM (Google)

## 6. 완료 정의 (DoD)

- `goose-cloud/` 리포 OSS 배포
- KR/JP/US/EU 4개 리전 edge 배치
- 사용자 대시보드: 내 데이터 어디 있나요? (실시간 확인)
- 탈퇴 시 모든 연결 데이터 30초 내 삭제
