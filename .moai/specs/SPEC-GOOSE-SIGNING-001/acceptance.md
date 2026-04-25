---
id: SPEC-GOOSE-SIGNING-001
artifact: acceptance
version: 0.1.0
status: planned
created_at: 2026-04-25
updated_at: 2026-04-25
author: manager-spec
---

# SPEC-GOOSE-SIGNING-001 — Acceptance Criteria (BDD Scenarios)

## HISTORY

| Version | Date       | Change Summary                                                                   | Author       |
|---------|------------|----------------------------------------------------------------------------------|--------------|
| 0.1.0   | 2026-04-25 | Initial acceptance scenarios. Given-When-Then 형식, 9개 primary + edge cases. | manager-spec |

---

본 문서는 spec.md §5의 AC-SIGN-001~009에 대응하는 BDD 형식 시나리오와 edge case를 정의한다. 각 시나리오는 REQ 추적을 포함하며, Definition of Done 및 Quality Gate 기준을 §4~§6에 명시한다.

---

## §1. Primary Scenarios (AC-SIGN-001 ~ AC-SIGN-009)

### Scenario S1 — macOS notarization happy path (AC-SIGN-001 / REQ-SIGN-001, REQ-SIGN-002)

```gherkin
Given 유효한 Developer ID Application 인증서가 CI 키체인에 로드되어 있고
  And notarytool 자격증명(Apple ID, app-specific password, Team ID)이 환경변수로 주입되어 있고
  And goose-agent.app 번들이 CI runner에 빌드되어 있다
When scripts/sign/macos.sh 가 해당 번들에 대해 실행된다
Then codesign --verify --deep --strict <app> 는 exit 0 을 반환해야 한다
  And stapler validate <app> 는 staple 존재를 확인해야 한다
  And 결과 .app 은 Gatekeeper 활성 macOS 시스템에서 경고 없이 실행되어야 한다
  And signing 로그에 signer identity 와 SHA-256 이 기록되어야 한다
```

### Scenario S2 — Windows EV signing with RFC3161 timestamp (AC-SIGN-002 / REQ-SIGN-001, REQ-SIGN-002)

```gherkin
Given 유효한 EV code-signing 인증서가 Windows runner 인증서 저장소에 등록되어 있고
  And RFC3161 timestamp 서버 URL 이 signing policy 에 명시되어 있고
  And goose-agent_setup.msi 가 runner 에 빌드되어 있다
When scripts/sign/windows.ps1 이 해당 installer 에 대해 실행된다
Then signtool verify /pa /v <msi> 는 exit 0 을 반환해야 한다
  And verify 출력에 RFC3161 timestamp 가 포함되어야 한다
  And Windows SmartScreen 에서 publisher 이름이 조직명으로 표시되어야 한다
```

### Scenario S3 — Linux minisign detached signature (AC-SIGN-003 / REQ-SIGN-001)

```gherkin
Given 활성 minisign private key 가 CI secret 에서 로드되었고
  And goose-agent-0.1.0.AppImage 및 goosed 정적 바이너리가 runner 에 빌드되어 있다
When scripts/sign/linux.sh 가 두 아티팩트에 대해 실행된다
Then 각 아티팩트에 대해 .minisig 파일이 생성되어야 한다
  And minisign -Vm <artifact> -p <embedded public key> 는 exit 0 을 반환해야 한다
  And signing 로그에 signer identity 와 SHA-256 이 기록되어야 한다
```

### Scenario S4 — Linux cosign keyless 옵션 (AC-SIGN-003 / REQ-SIGN-005)

```gherkin
Given 환경변수 GOOSE_SIGN_COSIGN=1 이 설정되어 있고
  And OIDC 자격증명(GitHub Actions OIDC 토큰)이 runner 에 사용 가능하다
When scripts/sign/linux.sh 가 AppImage 에 대해 실행된다
Then cosign sign-blob --yes <artifact> 는 Rekor 트랜스페어런시 로그 엔트리를 생성해야 한다
  And .sig 와 .pem 이 아티팩트와 함께 출력되어야 한다
  And cosign verify-blob --certificate <pem> --signature <sig> <artifact> 는 exit 0 을 반환해야 한다
```

### Scenario S5 — Invalid signature rejection (AC-SIGN-004 / REQ-SIGN-003)

```gherkin
Given Desktop App 이 trust-anchors.json 에 pubkey K_A 를 임베드한 채 실행 중이고
  And auto-updater 가 pubkey K_B (K_A와 미일치) 로 서명된 update payload 를 수신했다
When 업데이터가 서명 검증을 수행한다
Then 업데이트는 500ms 이내에 거부되어야 한다
  And 다운로드된 payload 는 앱 설치 디렉토리에 persist 되지 않아야 한다
  And 로그 이벤트에 { reason: "signature_invalid" | "unknown_signer", artifact_sha256: <hex>, attempted_signer: <id> } 가 기록되어야 한다
  And 유저에게 거부 사유와 canonical recovery URL 을 포함한 다이얼로그가 표시되어야 한다
```

### Scenario S6 — Expired certificate handling (AC-SIGN-005 / REQ-SIGN-003)

```gherkin
Given Windows MSI 가 유효한 RFC3161 timestamp 가 부착된 상태로 서명되어 있고
  And 서명에 사용된 인증서가 이후 만료되었다
When 엔드유저가 현재 날짜 (cert 만료 이후) 에 installer 를 실행한다
Then installer 는 정상적으로 실행되어야 한다 (timestamp 체인이 유효하므로)
  And signtool verify /pa /v <msi> 는 exit 0 을 반환해야 한다

Given 동일 상황에서 timestamp 가 부착되지 않은 MSI 라면
When 엔드유저가 cert 만료 이후 실행한다
Then installer 는 REQ-SIGN-003 에 따라 거부되어야 한다
  And 로그 이벤트에 { reason: "cert_expired" } 가 기록되어야 한다
```

### Scenario S7 — Revoked key rejection during auto-update (AC-SIGN-006 / REQ-SIGN-003, REQ-SIGN-004)

```gherkin
Given Desktop App 현재 버전 N+1 이 trust-anchors.json 에 revoked_keys: [{key_id: "K_old", ...}] 를 포함하고 있고
  And 공격자가 K_old 로 서명된 악성 update payload 를 준비했다
When 업데이터가 해당 payload 에 대해 서명 검증을 수행한다
Then 서명 수학적 검증이 성공하더라도 업데이트는 거부되어야 한다
  And 로그 이벤트에 { reason: "key_revoked", revoked_key_id: "K_old" } 가 기록되어야 한다
  And 유저에게 "canonical source 에서 재설치" 권고 다이얼로그가 표시되어야 한다
```

### Scenario S8 — Key rotation grace period (AC-SIGN-007 / REQ-SIGN-004)

```gherkin
Given Desktop App 버전 N 이 trust-anchors = { active: [K_current, K_next] } 로 배포되어 있고
  And 로테이션 T-0 일 이후 버전 N+1 이 K_next 로 서명되어 릴리스되었다
When 버전 N 을 실행 중인 엔드유저의 auto-updater 가 N+1 을 감지한다
Then auto-update 는 수동 재설치 없이 성공해야 한다
  And 버전 N+1 의 trust-anchors 는 { active: [K_next, K_next2], revoked: [K_current] } 형태여야 한다
  And rotation 이벤트가 release notes 상단에 고정 표시되어야 한다
```

### Scenario S9 — CI signing pipeline integrity (AC-SIGN-008 / REQ-SIGN-002)

```gherkin
Given GitHub Actions sign-release.yml 워크플로가 설정되어 있고
  And 태그 v0.1.0 이 repository 에 push 되었다
When sign-release.yml 이 트리거되어 실행된다
Then signing material 은 오직 GitHub Actions secrets 에서만 retrieve 되어야 한다
  And 어떤 step 의 stdout/stderr/log/artifact 에도 private key material 이 노출되지 않아야 한다
  And signing step 하나라도 non-zero exit 이면 release publish job 이 실행되지 않아야 한다
  And 성공 시 SIGNATURES.json 이 릴리스 아티팩트와 함께 published 되어야 한다
  And SIGNATURES.json 은 각 아티팩트의 { sha256, signer_identity, signature_file, timestamp } 를 리스트로 포함해야 한다
```

### Scenario S10 — DESKTOP-001 REQ-DK-013 interoperability (AC-SIGN-009)

```gherkin
Given goosed 바이너리가 본 SPEC REQ-SIGN-001/002 에 따라 서명되어 릴리스되었고
  And Desktop App 이 본 SPEC 의 trust-anchors.json 에 정의된 공개키를 임베드하여 빌드되었다
When Desktop App 이 실행되어 DESKTOP-001 REQ-DK-013 검증 로직을 수행한다
Then 공개키 일치 여부 검증이 성공해야 한다
  And DESKTOP-001 AC-DK-010 은 본 SPEC 의 키 분배 메커니즘을 사용해 수정 없이 통과해야 한다
  And 두 SPEC 의 trust anchor 소스가 동일 파일(.moai/signing/trust-anchors.json) 임이 문서에 명시되어야 한다
```

---

## §2. Edge Cases

### EC1 — Notarization 지연 타임아웃

```gherkin
Given notarytool submit 후 Apple 서버 응답이 지연된다
When --wait 모드에서 30 분이 경과한다
Then CI job 은 timeout 으로 실패해야 한다
  And 자동 재시도가 2 회까지 수행되어야 한다
  And 3 회 실패 시 릴리스 publish 가 차단되고 알림이 발생해야 한다
```

### EC2 — Dual-sign 요구 (Windows 레거시)

```gherkin
Given 본 SPEC 은 SHA-256 단일 서명만 요구한다 (Windows 10+ 전제)
When SHA-1 signing 요청이 입력된다
Then scripts/sign/windows.ps1 은 exit 1 (invalid argument) 을 반환해야 한다
  And 로그에 "SHA-1 signing is out of scope" 가 기록되어야 한다
```

### EC3 — Trust anchor embed 값 불일치

```gherkin
Given M3 빌드 스텝에서 tauri.conf.json 의 updater.pubkey 가 .moai/signing/trust-anchors.json 의 active anchor 와 다르다
When CI workflow 가 embed verification 스텝을 실행한다
Then 빌드가 중단되어야 한다 (AC-SIGN-008 (c) 조항 충족)
  And 로그에 expected SHA 와 actual SHA 가 함께 기록되어야 한다
```

### EC4 — minisign private key 형식 불일치

```gherkin
Given scripts/sign/linux.sh 에 주입된 minisign key 가 암호화되지 않은 형식이다
When 스크립트가 실행된다
Then 스크립트는 exit 2 (signing material 누락/오류) 를 반환해야 한다
  And 암호화되지 않은 키 사용 거부 로그가 기록되어야 한다
```

### EC5 — Revocation list 빈 상태 vs 누락 구분

```gherkin
Given trust-anchors.json 에 revoked_keys: [] (빈 배열) 이 존재한다
When Desktop App 이 이를 로드한다
Then 유효한 revocation 정책으로 인식되어야 한다 (revoke 대상 없음)

Given 대신 trust-anchors.json 에 revoked_keys 키 자체가 누락되었다
When Desktop App 이 이를 로드한다
Then 스키마 위반으로 취급하여 업데이트 차단 + tamper-warning 을 띄워야 한다
```

### EC6 — Downgrade attack 시도

```gherkin
Given Desktop App 현재 버전이 0.2.0 이고 모든 서명 검증에 성공한 update payload 의 버전이 0.1.5 이다
When 업데이터가 버전 비교를 수행한다
Then 업데이트는 monotonic version 정책에 따라 거부되어야 한다
  And 로그 이벤트에 { reason: "downgrade_rejected", current: "0.2.0", attempted: "0.1.5" } 가 기록되어야 한다
(Note: monotonic version 구현 자체는 DESKTOP-001 updater 로직에서 담당; 본 SPEC 은 서명된 manifest 에 version 필드가 존재함만 보장)
```

### EC7 — cosign Rekor 서비스 장애

```gherkin
Given GOOSE_SIGN_COSIGN=1 인 상태에서 Rekor 엔드포인트가 응답 불가하다
When scripts/sign/linux.sh 가 cosign 브랜치를 실행한다
Then primary 서명 (minisign) 은 성공해야 한다
  And cosign 실패는 warning 으로 로그되지만 빌드 전체를 중단시키지 않아야 한다 (Optional 요구사항이므로)
```

---

## §3. Quality Gate Criteria

Quality gate 통과 기준은 다음을 모두 충족해야 한다:

- **QG-1 TRUST 5 Tested**: §1 Primary Scenarios 중 fixture 기반 자동화 가능한 항목 (S1, S2, S3, S5, S7, S8) 은 CI 에 통합되어 자동 실행. S4, S6, S9, S10 은 integration 단계에서 최소 1회 수동 검증.
- **QG-2 TRUST 5 Readable**: 모든 signing 스크립트 및 정책 파일은 `shellcheck` (shell) 또는 PSScriptAnalyzer (PowerShell) 통과.
- **QG-3 TRUST 5 Unified**: 로그 이벤트 포맷은 spec.md §6.5 구조를 준수. 모든 SIGNATURES.json 필드명은 snake_case 통일.
- **QG-4 TRUST 5 Secured**:
  - Private key material 이 git 또는 CI log 에 노출된 흔적이 없음 (trufflehog 또는 동등 도구 검증).
  - `signtool /a` 대신 명시적 `/n` 또는 `/sha1` 인증서 지정.
  - macOS entitlements 가 최소 권한 원칙 준수.
- **QG-5 TRUST 5 Trackable**: 본 SPEC 관련 모든 commit 은 `SPEC: SPEC-GOOSE-SIGNING-001` trailer 포함. 각 AC 는 최소 1개 이상의 commit 또는 PR 로 traceability 확보.

---

## §4. Definition of Done

본 SPEC 이 `status: completed` 로 전환되기 위해 다음 조건을 모두 충족해야 한다:

### 4.1 산출물 완결성

- [ ] `.moai/signing/policy.yaml` 및 `.moai/signing/trust-anchors.json` (초기 버전) 커밋됨
- [ ] `scripts/sign/{macos.sh, windows.ps1, linux.sh, updater.sh, rotate.sh, revoke.sh}` 작성 및 `shellcheck`/PSScriptAnalyzer 통과
- [ ] `.github/workflows/sign-release.yml` 머지됨
- [ ] Tauri `tauri.conf.json` updater 섹션 구성 및 trust-anchor embed 자동화 완료

### 4.2 테스트 완결성

- [ ] §1 Primary Scenarios S1~S10 모두 최소 1회 검증됨 (자동 또는 수동)
- [ ] §2 Edge Cases EC1~EC7 최소 명시적 검증 기록 존재
- [ ] Fixture 기반 rotation/revocation 시뮬레이션 성공
- [ ] Staging 태그 (`v0.0.0-sign-test`) 로 CI workflow E2E 1회 성공

### 4.3 통합 완결성

- [ ] DESKTOP-001 v0.3.0 에서 §9 Exclusions "key distribution unresolved" 항목 제거됨
- [ ] DESKTOP-001 REQ-DK-013 의 "project-approved key-distribution mechanism" 이 본 SPEC 을 cross-reference 하도록 갱신됨
- [ ] AC-SIGN-009 (DESKTOP-001 interoperability) 통과

### 4.4 문서화 완결성

- [ ] Operations RUNBOOK (키 생성/회전/폐기 절차) 작성됨 (본 SPEC 외부 아티팩트)
- [ ] Release notes 템플릿에 rotation 이벤트 상단 고정 섹션 추가됨
- [ ] EXCL-01~09 항목이 후속 SPEC/RUNBOOK 로 올바르게 연결되어 있음 (CREDPOOL-001, SLSA-001 placeholder 포함)

### 4.5 Quality Gate

- [ ] §3 QG-1 ~ QG-5 모두 통과
- [ ] 보안 리뷰 1회 완료 (내부 expert-security 또는 외부 레뷰어)

---

## §5. Traceability Matrix

| Scenario / EC | REQ                         | AC           | Plan Milestone |
|---------------|-----------------------------|--------------|----------------|
| S1            | REQ-SIGN-001, REQ-SIGN-002  | AC-SIGN-001  | M2, M5         |
| S2            | REQ-SIGN-001, REQ-SIGN-002  | AC-SIGN-002  | M2, M5         |
| S3            | REQ-SIGN-001                | AC-SIGN-003  | M2, M5         |
| S4            | REQ-SIGN-005                | AC-SIGN-003  | M2, M5         |
| S5            | REQ-SIGN-003                | AC-SIGN-004  | M3             |
| S6            | REQ-SIGN-003                | AC-SIGN-005  | M2             |
| S7            | REQ-SIGN-003, REQ-SIGN-004  | AC-SIGN-006  | M4             |
| S8            | REQ-SIGN-004                | AC-SIGN-007  | M4             |
| S9            | REQ-SIGN-002                | AC-SIGN-008  | M5             |
| S10           | (consistency)               | AC-SIGN-009  | M3             |
| EC1           | REQ-SIGN-002                | AC-SIGN-001  | M5             |
| EC2           | REQ-SIGN-001                | AC-SIGN-002  | M2             |
| EC3           | REQ-SIGN-002                | AC-SIGN-008  | M3, M5         |
| EC4           | REQ-SIGN-001                | AC-SIGN-003  | M2             |
| EC5           | REQ-SIGN-003, REQ-SIGN-004  | AC-SIGN-006  | M4             |
| EC6           | (external — DESKTOP-001)    | AC-SIGN-006  | M3             |
| EC7           | REQ-SIGN-005                | AC-SIGN-003  | M2             |

---

## §6. Out of Acceptance Scope

다음 항목은 본 SPEC 의 acceptance 범위 밖이며 별도 SPEC/RUNBOOK 에서 검증한다:

- Secrets vault (CREDPOOL-001) 자체의 기능 검증
- SLSA attestation 검증 (SPEC-GOOSE-SLSA-001 예정)
- End-user 수동 검증 도구 UX
- 모바일 플랫폼 (iOS/Android) 서명
- Sigstore Fulcio 정책 튜닝 (keyless 옵션 기본 구성 이상)

---

End of acceptance.md.
