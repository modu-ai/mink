---
id: SPEC-GOOSE-SIGNING-001
artifact: compact
version: 0.1.0
status: planned
created_at: 2026-04-25
updated_at: 2026-04-25
author: manager-spec
---

# SPEC-GOOSE-SIGNING-001 — Compact Summary

> Agent 컨텍스트 로딩용 요약본. 상세 내용은 `spec.md` / `plan.md` / `acceptance.md` 참조.

## HISTORY

| Version | Date       | Change Summary | Author       |
|---------|------------|-----------------|--------------|
| 0.1.0   | 2026-04-25 | Initial compact. | manager-spec |

---

## 1. What

`goose-agent` 제품군 전 플랫폼(macOS/Windows/Linux + Tauri 데스크톱)에 대한 **코드 서명 + auto-update 서명 키 분배** 통합 SPEC. SPEC-GOOSE-DESKTOP-001 REQ-DK-013 (검증 계약)에서 OUT-OF-SCOPE 로 분리된 키 분배 메커니즘을 정식화.

## 2. Why

- DESKTOP-001은 "서명 일치하지 않으면 거부" 검증 계약만 정의, 공개키 분배 전략 미정.
- 플랫폼별 코드 서명 요건 (Apple notary, Windows EV, Linux cosign/minisign) 통합 필요.
- 키 회전/폐기 운영 프로토콜 부재 시 단일 지점 실패.

## 3. Scope

**IN**: macOS Developer ID + notarytool, Windows EV signtool + RFC3161 timestamp, Linux minisign (default) / cosign (optional), Tauri updater ed25519, trust anchor 임베드 (번들 내장), dual-anchor rotation, revocation list, CI workflow (sign-release.yml), SIGNATURES.json manifest.

**OUT**: secrets vault 구현 (→ CREDPOOL-001), 키 생성 프로토콜 (→ RUNBOOK), 인증서 조달, SLSA attestation (→ SLSA-001 future), 모바일 플랫폼, CRL 서버, anti-tamper 런타임.

## 4. Requirements (EARS)

- **REQ-SIGN-001** [Ubiquitous]: 모든 user-facing 바이너리에 platform-appropriate 서명 필수.
- **REQ-SIGN-002** [Event-Driven]: tag push → 자동 서명 + signature manifest 첨부.
- **REQ-SIGN-003** [Unwanted]: invalid/expired/revoked signature → 설치/업데이트 거부 + tamper dialog + recovery URL.
- **REQ-SIGN-004** [State-Driven]: rotation window 동안 dual-anchor (current + next) 배포; rotation 후 이전 키 revoke.
- **REQ-SIGN-005** [Optional]: Linux 에서 cosign keyless 모드 지원 (Rekor transparency log).

## 5. Acceptance (요약)

AC-SIGN-001~009: macOS notary / Windows EV / Linux minisign happy path, invalid/expired/revoked 거부, rotation grace period, CI integrity (SIGNATURES.json, no key leak), DESKTOP-001 AC-DK-010 interoperability.

## 6. Milestones

- **M1 (High)**: policy.yaml + trust-anchors.json 스키마 확정
- **M2 (High)**: platform signing scripts (macOS/Windows/Linux) — dry-run 지원
- **M3 (High)**: Tauri updater integration + trust-anchor embed 자동화
- **M4 (Medium)**: key rotation + revocation 운영 로직 + fixture 테스트
- **M5 (Medium)**: CI workflow E2E (.github/workflows/sign-release.yml) + SIGNATURES.json

Priority 기반 (시간 추정 없음). 의존 관계: M1 → M2 → M3 → M4 → M5.

## 7. Dependencies

- **선행**: DESKTOP-001 (REQ-DK-013 검증 계약), CREDPOOL-001 (secrets 인프라; 병렬 가능)
- **후행**: DESKTOP-001 v0.3.0 cross-reference 갱신, SLSA-001 (future)
- **외부**: Apple Developer Program, Windows EV cert, GitHub Actions secrets, Tauri CLI v2.x, minisign/cosign CLI

## 8. Key Risks

- RISK-01 Signing key leak → rotation + HSM + audit log
- RISK-02 Cert 만료 → D-60 알림 + timestamp 부착 의무
- RISK-04 CI 침해 → managed runner + workflow review
- RISK-06 Downgrade attack → monotonic version (DESKTOP-001 updater 로직 보강 필요)
- RISK-08 Trust anchor embed 실수 → CI 빌드 전 SHA 비교 게이트

## 9. Exclusions 요약

EXCL-01 secrets vault / EXCL-02 keygen 프로토콜 / EXCL-03 인증서 조달 / EXCL-04 SLSA / EXCL-05 end-user verification CLI / EXCL-06 cosign keyless 상세 / EXCL-07 CRL 서버 / EXCL-08 anti-tamper / EXCL-09 모바일 플랫폼.

## 10. Key Files (예정)

- `.moai/signing/policy.yaml`
- `.moai/signing/trust-anchors.json`
- `scripts/sign/{macos.sh,windows.ps1,linux.sh,updater.sh,rotate.sh,revoke.sh}`
- `.github/workflows/sign-release.yml`
- `tauri.conf.json` (updater 섹션 갱신)

## 11. DESKTOP-001 연결

- DESKTOP-001 REQ-DK-013 §9 Exclusions (2026-04-25 v0.2.0) 에서 본 SPEC 으로 분배 메커니즘 위임됨.
- 본 SPEC closure 시 DESKTOP-001 v0.3.0 에서 Exclusions 항목 제거 + REQ-DK-013 cross-ref 업데이트.
- 양 SPEC 은 `.moai/signing/trust-anchors.json` 단일 소스 공유.

---

End of spec-compact.md.
