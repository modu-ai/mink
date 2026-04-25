---
id: SPEC-GOOSE-SIGNING-001
artifact: plan
version: 0.1.0
status: planned
created_at: 2026-04-25
updated_at: 2026-04-25
author: manager-spec
---

# SPEC-GOOSE-SIGNING-001 — Implementation Plan

## HISTORY

| Version | Date       | Change Summary                                                    | Author       |
|---------|------------|-------------------------------------------------------------------|--------------|
| 0.1.0   | 2026-04-25 | Initial plan. Milestones M1-M5 priority-based, no time estimates. | manager-spec |

---

## §1. Approach Summary

본 구현 계획은 **"검증 계약 → 서명 생성 → 키 분배 → 회전/폐기 → 통합 검증"** 순서로 단계적 달성한다. 각 마일스톤은 선행 마일스톤 산출물에 의존하며, 우선순위(Priority High/Medium/Low)로 표기한다. 시간 추정은 사용하지 않는다 (CLAUDE.md agent-common-protocol.md `Time Estimation` HARD rule).

전체 접근의 핵심 의사결정:

- **Tauri updater ed25519 서명은 전체 플랫폼 공통 채널**로 사용하여, L2(auto-update) 레이어에서 플랫폼 중립적 검증을 확보.
- **L1(OS-level code-sign)**는 각 플랫폼의 네이티브 체인(macOS Developer ID, Windows Authenticode EV, Linux minisign/cosign)을 그대로 사용. 서명 방식을 통일하지 않고 플랫폼 관행을 존중.
- **키 분배는 번들 임베드 전용**. 별도 CRL/KMS 서버를 두지 않아 가용성 의존성을 최소화.
- **Rotation은 pre-announced dual-anchor** 모델. 신규 키를 이전 릴리스에 미리 포함하여 grace period를 보장.
- **CI secrets 의존성은 CREDPOOL-001과 분리 가능하도록** 인터페이스만 정의. 본 SPEC은 GitHub Actions `secrets.` 네임스페이스를 초기 기본값으로 사용.

---

## §2. Milestones

### M1 [Priority: High] — Signing Policy Manifest 확정

**Deliverables:**
- `.moai/signing/policy.yaml` 초안 작성 (active keys, revoked keys, rotation schedule, platform matrix)
- `.moai/signing/trust-anchors.json` 스키마 정의 (v1)
- Policy 검증 스크립트 (schema validation, Go 또는 shell)

**Exit Criteria:**
- Policy 스키마가 §4 REQ-SIGN-004 rotation workflow 및 §6.7 revocation list 형식을 표현 가능
- 스키마 validation이 CI 워크플로에 wiring 가능한 단위로 분리됨

**Blocks:** M2, M3, M4

---

### M2 [Priority: High] — Platform Code-Sign 스크립트 구현

**Deliverables:**
- `scripts/sign/macos.sh`: codesign + notarytool + stapler 파이프라인 (REQ-SIGN-001 / AC-SIGN-001)
- `scripts/sign/windows.ps1`: signtool + RFC3161 timestamp (REQ-SIGN-001 / AC-SIGN-002, AC-SIGN-005)
- `scripts/sign/linux.sh`: minisign 서명 + optional cosign 브랜치 (REQ-SIGN-001, REQ-SIGN-005 / AC-SIGN-003)
- 각 스크립트는 `--dry-run` 플래그 지원 (secrets 미주입 환경에서 구조 검증)

**Exit Criteria:**
- 로컬 환경에서 `--dry-run` 으로 명령 시퀀스 전체가 검증되어야 함
- 실제 signing 실행은 M5 CI 통합 시점까지 유예 가능
- 각 스크립트 exit code 규약: 0=성공, 1=잘못된 인자, 2=signing material 누락, 3=tool 미설치, 10=검증 실패

**Depends On:** M1
**Blocks:** M5

---

### M3 [Priority: High] — Tauri Updater Integration (L2)

**Deliverables:**
- `tauri.conf.json` `updater` 섹션 설정: public key 임베드 경로, endpoints
- `scripts/sign/updater.sh`: `tauri signer sign` 래핑 + `latest.json` manifest 생성 로직
- Desktop App 측 updater 검증 코드 (REQ-SIGN-003 / AC-SIGN-004, AC-SIGN-006 적용 포인트 식별)
- Trust anchor embed 자동화: 빌드 시점에 `.moai/signing/trust-anchors.json` → `tauri.conf.json` sync

**Exit Criteria:**
- Tauri 번들이 updater 서명 검증을 활성 상태로 빌드 가능
- Invalid signature 주입 테스트에서 REQ-SIGN-003 동작 관측 가능 (단위 테스트 수준)
- DESKTOP-001 REQ-DK-013 검증 경로와 공용 trust anchor 사용 확인 (AC-SIGN-009 foundation)

**Depends On:** M1, M2
**Blocks:** M4, M5

---

### M4 [Priority: Medium] — Key Rotation & Revocation 운영 로직

**Deliverables:**
- `scripts/sign/rotate.sh`: dual-anchor 단계 (T-30d, T-0, T+grace) 자동화
- `scripts/sign/revoke.sh`: revocation entry 생성 + trust-anchors.json 갱신
- Desktop App 측 revocation list 검증 로직 (REQ-SIGN-003, REQ-SIGN-004 / AC-SIGN-006, AC-SIGN-007)
- Rotation / revocation 이벤트 로깅 (structured log event)

**Exit Criteria:**
- Rotation happy-path 시뮬레이션 테스트 통과 (AC-SIGN-007)
- Revocation 시 기존 키로 서명된 update 거부 동작 검증 (AC-SIGN-006)
- 시뮬레이션은 실제 키 교체 없이 fixture 기반으로 수행

**Depends On:** M1, M3
**Blocks:** M5

---

### M5 [Priority: Medium] — CI Signing Workflow 통합

**Deliverables:**
- `.github/workflows/sign-release.yml`: M2 스크립트를 매트릭스 job으로 호출
- Secrets 주입 경로 확정 (Apple certs, Windows EV cert, Tauri updater key, minisign key)
- `SIGNATURES.json` manifest 생성 job (AC-SIGN-008 (d))
- 실패 시 릴리스 publish 차단 gate

**Exit Criteria:**
- Tag push 이벤트로 전체 워크플로 E2E 실행 (테스트 태그 기준)
- Secrets 로그 노출 없음 (AC-SIGN-008 (b))
- SIGNATURES.json이 모든 아티팩트를 커버 (AC-SIGN-008 (d))
- CREDPOOL-001과의 secrets 인터페이스 경계 문서화

**Depends On:** M1, M2, M3, M4
**Blocks:** (SPEC 완료)

---

## §3. Technical Approach

### 3.1 구현 순서 근거

M1 → M2 병렬 가능 평가: M2 스크립트의 입력 스키마는 M1 policy manifest에 의존. 순차 진행이 안전. M3과 M2는 partial 병렬 가능 (updater 레이어는 platform codesign과 독립된 서명 경로).

M4는 M3에 의존 (revocation은 trust-anchors.json 포맷 확정 후). M5는 M1~M4 모두 의존 (CI는 최종 통합 지점).

### 3.2 플랫폼별 구현 선택

- **macOS**: Apple 공식 `notarytool` 사용. `altool`은 2023년 deprecated로 미사용. 병렬 submit은 Apple API rate limit (분당 ~5건) 준수.
- **Windows**: `signtool` (Windows SDK) 사용. Azure Code Signing 같은 매니지드 옵션은 후속 검토 (현 SPEC 범위 외).
- **Linux**: `minisign` 기본. `cosign` keyless는 REQ-SIGN-005 Optional로 처리 — 기본 빌드에서 비활성, 환경변수 `GOOSE_SIGN_COSIGN=1` 시 활성.
- **Tauri updater**: v2.x `tauri signer` CLI. v1.x 형식은 사용 안 함.

### 3.3 Trust anchor 데이터 흐름

```
.moai/signing/policy.yaml           (소스)
    ↓ (M1 build script)
.moai/signing/trust-anchors.json    (정규화)
    ↓ (M3 embed step)
tauri.conf.json updater.pubkey      (Tauri 번들 임베드)
    ↓ (빌드)
Desktop App binary 내부              (런타임 검증 소스)
```

### 3.4 테스트 전략

- **Unit**: signing 스크립트 `--dry-run` 모드 검증, trust-anchors.json 스키마 validation
- **Integration (fixture)**: rotation/revocation 시뮬레이션 (실제 키 없이 test fixture 사용)
- **E2E (tag-driven)**: 테스트 태그 (`v0.0.0-sign-test`)로 CI workflow 전체 실행 → staging 아티팩트 검증
- **Manual**: 프로덕션 키로 실제 서명/공증은 첫 공식 릴리스 시 1회 E2E 수행

### 3.5 관찰성

모든 signing/verify 이벤트는 구조화 로그 emit:

```json
{
  "ts": "2026-05-01T10:30:00Z",
  "event": "signing.sign_complete",
  "artifact": "goose-agent_0.1.0_x64.dmg",
  "platform": "darwin",
  "signer_identity": "Developer ID Application: <org>",
  "sha256": "<hex>",
  "duration_ms": 1234
}
```

---

## §4. Risks & Mitigations (Plan-Level)

spec.md §8 Risks를 참조. Plan-level 특화 리스크:

- **Plan Risk A**: M2 스크립트가 플랫폼 런너 환경 차이 (Xcode 버전, Windows SDK 버전)에 민감 → CI 러너 이미지 버전 pin.
- **Plan Risk B**: M5 E2E가 실제 secrets 없이는 부분 검증만 가능 → 첫 공식 릴리스 전 1회 staging 태그로 전체 경로 검증 필수.
- **Plan Risk C**: Tauri v2 updater 스펙 변동 → 외부 의존성 버전 pin (Cargo.lock + package.json).

---

## §5. Exit Conditions for SPEC Closure

본 SPEC이 `status: completed`로 전환되기 위한 조건:

1. M1-M5 모두 Exit Criteria 충족
2. spec.md §5 AC-SIGN-001 ~ AC-SIGN-009 전체 통과 (최소 fixture 기반)
3. DESKTOP-001 REQ-DK-013 integration test(AC-SIGN-009) 통과 — DESKTOP-001 v0.3.0 업데이트로 cross-reference 확정
4. CI workflow E2E 1회 이상 성공 실행 (staging 태그 기준)
5. Operations RUNBOOK 작성 (키 생성, 회전, 폐기 절차 문서) — 본 SPEC 외부 아티팩트이나 SPEC closure의 trailing indicator

---

End of plan.md.
