---
id: SPEC-GOOSE-AUTH-001
version: 0.1.0
status: planned
created_at: 2026-04-22
updated_at: 2026-04-22
author: session-decision
priority: P0
issue_number: null
phase: 6
size: 중(M)
lifecycle: spec-first
labels: []
---

# SPEC-GOOSE-AUTH-001 — Local Token-Based Authentication

> **v0.2 Amendment (2026-04-24)**: SPEC-GOOSE-ARCH-REDESIGN-v0.2 에 따라 **스코프 축소**.
> 제거: 이메일 가입 플로우, QR 페어링, 디바이스 키 교환 (multi-device sync 전제 폐기).
> 유지: Web UI · CLI 용 **로컬 token 기반 auth** (localhost 접근 제어, API bearer token).
> 관련 키 보관은 SPEC-GOOSE-CREDENTIAL-PROXY-001 + OS keyring 으로 이관.
> 기존 본문은 v6.1 맥락 참조용으로만 유지; 실제 구현은 축소된 스코프만 반영한다.

---

## 원본 타이틀 (v6.1): Zero-Knowledge Email + Device Key Management ★ 스코프 축소됨

> **상태**: 스켈레톤. M6 Week 2 진입 시 manager-spec이 완전 작성.

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.1.0 | 2026-04-22 | Tier 1/2 Cloud 이메일 가입 + 장치 공개키 관리. Zero-Knowledge 원칙. | 세션 결정 |

---

## 1. 개요

Tier 1 Cloud Free 진입 시 이메일만으로 가입. 개인키는 장치 외부 유출 불가.

## 2. 범위

- 포함: 이메일 verification, 장치 키페어 생성, 공개키 등록, 비밀번호 복구
- 제외: 결제·구독 (별도 SPEC v1.3 Tier 2 Plus)

## 3. EARS 요구사항 (초안)

### 3.1 Ubiquitous
- **REQ-AUTH-001**: The system shall store only sha256+salt(email) on Cloud, never plaintext email.
- **REQ-AUTH-002**: Device private keys shall never leave the device.

### 3.2 Event-driven
- **REQ-AUTH-010**: When user opts into Tier 1, the app shall generate Ed25519 keypair and register public key.
- **REQ-AUTH-011**: When user loses access, the app shall offer recovery via email-signed challenge.

### 3.3 State-driven
- **REQ-AUTH-020**: While paired, each Trusted Device shall maintain an individual JWT (24h expiry) + refresh token.
- **REQ-AUTH-021**: While user is not verified, Cloud features shall remain disabled.

### 3.4 Unwanted
- **REQ-AUTH-030**: If JWT is revoked, the device shall clear local cache within 1 minute.
- **REQ-AUTH-031**: If email is compromised, user shall be able to rotate all device keys via master recovery.

## 4. 인수 기준 (초안)

- **AC-AUTH-01**: 이메일 검증 흐름 30초 내 완료
- **AC-AUTH-02**: 5개 기기 동시 페어링 지원
- **AC-AUTH-03**: 복구 플로우 테스트 (이메일 접근만으로 모든 장치 복구)
- **AC-AUTH-04**: GDPR 7조 동의 철회 흐름 (30초 내 완전 삭제)

## 5. 의존성

- 상위: CLOUD-001
- 하위: SYNC-001 (Tier 2 키 파생)
- 기술: Ed25519, Argon2id (KDF), PASETO (JWT 대안 고려)

## 6. 완료 정의 (DoD)

- OWASP ASVS Level 2 준수
- 사용자 대시보드: 연결된 장치 목록 + 즉시 revoke
- 이메일 주소 변경 플로우
