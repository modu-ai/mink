---
id: SPEC-GOOSE-SIGNING-001
version: 0.1.0
status: planned
created_at: 2026-04-25
updated_at: 2026-04-25
author: manager-spec
priority: P0
issue_number: null
labels: [security, signing, code-sign, notarization, auto-update, phase-7]
---

# SPEC-GOOSE-SIGNING-001 — Binary Signing & Update Key Distribution

## HISTORY

| Version | Date       | Change Summary                                                                                                                                                                                                                                                                                                                     | Author       |
|---------|------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|--------------|
| 0.1.0   | 2026-04-25 | Initial draft. SPEC-GOOSE-DESKTOP-001의 REQ-DK-013(ed25519 signature verification contract)에서 분리된 키 분배 메커니즘 및 전체 플랫폼 코드 서명 정책을 독립 SPEC으로 정식화. macOS notarization, Windows EV code-sign, Linux cosign/minisign, Tauri updater public key 분배, key rotation/revocation, Signing CI workflow 포함. | manager-spec |

---

## §1. Overview

본 SPEC은 `goose-agent` 제품군(CLI `goose`, daemon `goosed`, Desktop App Tauri 번들, auto-update payload)에 대해 **플랫폼별 코드 서명**과 **auto-update 서명 키 분배**의 일관된 정책 및 검증 체인을 정의한다. 수요자는 엔드 유저의 바이너리 설치 과정과 자동 업데이트 경로 전반에서 **tamper 방지**와 **출처 무결성**을 요구한다.

본 SPEC의 핵심은 세 가지 레이어로 분해된다:

- **L1 Platform code-sign**: macOS Developer ID + notary, Windows EV certificate (signtool), Linux cosign/minisign detached signature.
- **L2 Auto-update signature**: Tauri updater ed25519 키쌍을 통한 update payload(`tar.gz`, `.msi`, `.dmg`, `AppImage`) 서명 및 앱 내 공개키 검증.
- **L3 Key distribution**: 업데이터 공개키 및 신뢰 앵커의 임베드/회전/폐기 프로토콜 — SPEC-GOOSE-DESKTOP-001 §9 Exclusions(2026-04-25 v0.2.0)에서 본 SPEC으로 위임된 범위.

본 SPEC은 **키 생성 자체** 및 **secrets vault 구현**은 제외한다 (아래 §Exclusions 및 선행 SPEC 의존성 참조).

---

## §2. Background

### 2.1 DESKTOP-001 REQ-DK-013와의 관계

SPEC-GOOSE-DESKTOP-001 `REQ-DK-013`은 다음을 정의한다:

> If the `goosed` binary signature does not match the expected ed25519 verification key obtained through the project-approved key-distribution mechanism, then the Desktop App shall not launch the daemon and shall display a tamper-warning dialog.

이 요구사항은 **검증 계약(verification contract)**만 명시하며, "project-approved key-distribution mechanism"이 무엇인지는 DESKTOP-001 §9 Exclusions에 따라 본 SPEC으로 위임되었다. 본 SPEC은 해당 "mechanism"을 구체화하고, 동일한 원칙을 전체 바이너리 산출물로 확장한다.

### 2.2 플랫폼별 공식 요구사항

- **macOS (Apple)**: Gatekeeper + notary 통과가 사실상 필수. Developer ID Application 인증서로 코드 서명 후 `notarytool`로 공증(staple)해야 사용자 경고 없이 실행 가능. 참조: Apple Developer Notarizing macOS software before distribution.
- **Windows (Microsoft)**: SmartScreen 신뢰 확보를 위해 EV(Extended Validation) code-signing 인증서가 실질 표준. `signtool.exe` + timestamp server 사용. 참조: Microsoft Authenticode.
- **Linux**: OS-level 강제 서명은 없으나, 공급망 무결성을 위해 `cosign`(Sigstore) 또는 `minisign`으로 detached `.sig` 배포가 관행. Tauri 번들(AppImage/deb/rpm)에도 동일 적용.
- **Tauri auto-update**: Tauri 공식 updater는 ed25519 키쌍을 요구하며, private key는 서명용, public key는 앱에 임베드. Tauri updater 공식 문서의 `tauri signer` CLI 기반.

### 2.3 위협 모델 (요약)

- **T1**: 유저 머신 바이너리 swap (로컬 권한 상승 후 `goosed` 교체) → L1/L2 검증으로 차단
- **T2**: update 채널 MITM (네트워크 경로 하이재킹) → L2 서명 + TLS 이중 방어
- **T3**: 서명 키 유출 (CI 환경 침해) → §4 REQ-SIGN-004 rotation + revocation
- **T4**: downgrade attack (구 버전 강제 설치) → update manifest에 monotonic version 요구
- **T5**: 인증서 만료 → §5 AC에서 expired cert 명시적 처리

---

## §3. Scope

### 3.1 IN-SCOPE

- [IN] macOS: Developer ID Application 인증서 기반 `codesign` + `notarytool` 통합
- [IN] Windows: EV 인증서 기반 `signtool` 서명 + RFC3161 timestamp
- [IN] Linux: Tauri 번들(AppImage/deb/rpm) + `goose`/`goosed` 정적 바이너리 detached signature (cosign 또는 minisign — §4 REQ-SIGN-005 선택 옵션)
- [IN] Tauri updater ed25519 키쌍 운영 정책 및 앱 내 공개키 임베드 메커니즘
- [IN] Update payload (`latest.json` 또는 동등한 manifest) 서명 및 앱 측 검증 로직
- [IN] Key rotation 정책 (대체 키 사전 임베드, grace period)
- [IN] Revocation 정책 (기존 키 거부, revocation list 배포 경로)
- [IN] Signing CI workflow (GitHub Actions 기준): secrets 주입 경로, reproducibility, artifact 아카이빙
- [IN] Verification 계약 (DESKTOP-001 REQ-DK-013과의 일관성)

### 3.2 OUT-OF-SCOPE

- [OUT] **Secrets vault 구현 자체**: GitHub Actions `secrets.` 또는 외부 vault(HashiCorp Vault, 1Password CLI) 도입 의사결정은 SPEC-GOOSE-CREDPOOL-001 범위. 본 SPEC은 "signing material이 CI 환경에서 안전히 주입된다"는 전제만 사용.
- [OUT] **키 생성 자체의 프로토콜**: ed25519 keygen, CSR 생성, HSM 프로비저닝 등은 운영 문서(RUNBOOK)로 분리.
- [OUT] **Apple Developer 계정 발급 / EV 인증서 구매 프로세스**: 조직 차원 조달 업무로 분리.
- [OUT] **Supply-chain attestation** (SLSA, in-toto): 후속 SPEC에서 다룸 (SPEC-GOOSE-SLSA-001 예정).
- [OUT] **Sigstore keyless(OIDC) 모드**: §4 REQ-SIGN-005에서 cosign 옵션으로 언급하나 상세 프로토콜은 후속 논의.
- [OUT] **End-user 수동 signature 검증 도구 배포**: 엔드유저는 OS-level 검증에 의존.

### 3.3 DESKTOP-001과의 경계

- DESKTOP-001: `goosed` 바이너리의 ed25519 서명 검증 **계약** 및 Desktop App의 tamper dialog UI (REQ-DK-013, AC-DK-010)
- SIGNING-001 (본 SPEC): 그 서명을 **누가 만들고 어떻게 공개키를 유통하며 어떻게 회전/폐기하는가**

---

## §4. EARS Requirements

### REQ-SIGN-001 [Ubiquitous] — 전체 바이너리 서명 강제

The goose-agent release pipeline **shall** produce, for every user-facing binary artifact (CLI `goose`, daemon `goosed`, Desktop App bundles, auto-update payloads), at least one platform-appropriate signature (macOS codesign+notary, Windows Authenticode EV, Linux cosign/minisign) before the artifact is published to any distribution channel.

**Rationale**: 미서명 바이너리는 SmartScreen/Gatekeeper 경고를 유발하고 T1/T2 위협에 노출된다.

---

### REQ-SIGN-002 [Event-Driven] — 릴리스 빌드 시 자동 서명

**When** the release build pipeline produces a new artifact for a tagged version (git tag matching `v[0-9]+\.[0-9]+\.[0-9]+(-[a-z0-9.]+)?`), the signing stage **shall** sign the artifact using the platform-appropriate signing material retrieved from the CI secrets store, and **shall** attach both the signature and a signature manifest (SHA-256 digest + signer identity + timestamp) to the release artifact bundle.

**Rationale**: 수동 서명은 인적 오류 및 누락을 유발한다. 릴리스 이벤트와 서명이 원자적으로 묶여야 한다.

---

### REQ-SIGN-003 [Unwanted] — 유효하지 않은 서명 차단

**If** a signature verification step during installation or auto-update fails (invalid signature, expired certificate, revoked key, or unknown signer), **then** the installer or updater **shall** reject the artifact, **shall not** apply the installation/update, and **shall** surface a user-visible tamper warning that identifies (a) the reason for rejection and (b) the recommended recovery path (reinstall from canonical source).

**Rationale**: T1/T2/T5 위협 차단. Fail-open(경고만 하고 진행)은 보안 실패.

---

### REQ-SIGN-004 [State-Driven] — 키 회전 운영 상태

**While** a signing key is within `scheduled_rotation_window` (default: 30 days prior to planned rotation) as defined in the signing-policy manifest, the release pipeline **shall** (a) continue signing with the active key, (b) include the next-generation public key as a trust anchor in all released Desktop App binaries so that future updates signed with the new key will verify post-rotation, and (c) after rotation completes, revoke the old key via the revocation list distributed with the next release.

**Rationale**: Tauri updater는 private key 단일 지점 실패를 갖는다. Pre-announced rotation으로 grace period 확보.

---

### REQ-SIGN-005 [Optional] — Linux cosign 옵션

**Where** a Linux distribution channel supports Sigstore tooling (cosign), the release pipeline **shall** produce, in addition to the primary detached signature, a cosign `.sig` + `.pem` (certificate) pair enabling keyless verification via Rekor transparency log lookup.

**Rationale**: Sigstore keyless는 키 관리 부담을 줄이지만 인프라 요구가 있다. minisign을 기본 옵션으로, cosign을 고급 옵션으로 제공.

---

## §5. Acceptance Criteria

### AC-SIGN-001 — macOS notarization happy path (REQ-SIGN-001, REQ-SIGN-002)

**Given** a macOS build of `goose-agent.app` with embedded `goosed` has been produced on the CI runner,
**When** the signing stage executes with a valid Developer ID Application certificate and notarytool credentials,
**Then** the resulting `.app` **shall** pass `codesign --verify --deep --strict` with exit code 0, **shall** have a notarization staple attached verifiable via `stapler validate`, and **shall** launch on a fresh macOS system without Gatekeeper warnings.

---

### AC-SIGN-002 — Windows EV signing with timestamp (REQ-SIGN-001, REQ-SIGN-002)

**Given** a Windows `.msi` installer has been produced on the CI runner,
**When** the signing stage executes `signtool sign /tr <RFC3161 timestamp URL> /td sha256 /fd sha256 /a <artifact>` with a valid EV code-signing certificate,
**Then** the resulting installer **shall** pass `signtool verify /pa /v` with exit code 0, **shall** include an RFC3161 timestamp preserving validity past certificate expiry, and **shall** show the organization name in the Windows SmartScreen publisher field.

---

### AC-SIGN-003 — Linux detached signature (REQ-SIGN-001, REQ-SIGN-005)

**Given** a Linux AppImage and a static `goosed` binary have been produced on the CI runner,
**When** the signing stage produces a minisign detached signature `.minisig` (and optionally a cosign `.sig` + `.pem`) using the active signing key,
**Then** `minisign -Vm <artifact> -p <embedded public key>` **shall** exit 0, and **where** cosign is used, `cosign verify-blob --certificate <pem> --signature <sig> <artifact>` **shall** confirm Rekor transparency inclusion.

---

### AC-SIGN-004 — Invalid signature rejection (REQ-SIGN-003)

**Given** the Desktop App or updater receives an artifact whose signature does not match the embedded trust anchor,
**When** the verification step runs,
**Then** the installer/updater **shall** reject the artifact within ≤500ms, **shall not** persist any downloaded payload to the application directory, **shall** log a structured error event with fields `{reason: "signature_invalid" | "cert_expired" | "key_revoked" | "unknown_signer", artifact_sha256: <hex>, attempted_signer: <identity>}`, and **shall** display a user-visible dialog carrying the reason and the canonical-source recovery URL.

---

### AC-SIGN-005 — Expired certificate handling (REQ-SIGN-003)

**Given** a Windows `.msi` whose code-signing certificate has expired,
**When** the installer runs on an end-user machine,
**Then** verification **shall** succeed if and only if the artifact carries a valid RFC3161 timestamp from before the certificate's expiry and the timestamping authority's chain is still trusted; otherwise the installer **shall** follow REQ-SIGN-003 rejection behavior.

---

### AC-SIGN-006 — Revoked key rejection during auto-update (REQ-SIGN-003, REQ-SIGN-004)

**Given** a key `K_old` has been revoked via a revocation list embedded in the current Desktop App release,
**When** the auto-updater receives an update payload signed with `K_old`,
**Then** the updater **shall** reject the update even if the signature would otherwise verify mathematically, **shall** log `{reason: "key_revoked", revoked_key_id: <id>}`, and **shall** prompt the user to reinstall from the canonical source.

---

### AC-SIGN-007 — Key rotation grace period (REQ-SIGN-004)

**Given** a Desktop App version `N` shipped with trust anchors `{K_current, K_next}`,
**When** version `N+1` is released signed with `K_next` after rotation completes,
**Then** an end user running version `N` **shall** successfully auto-update to `N+1` without manual reinstall, and version `N+1` **shall** ship with a revocation entry for `K_current` and a new anchor `{K_next, K_next2}`.

---

### AC-SIGN-008 — CI signing pipeline integrity (REQ-SIGN-002)

**Given** a release tag is pushed to the repository,
**When** the signing CI workflow executes,
**Then** the workflow **shall** (a) retrieve signing material only from the approved secrets store, (b) never write private key material to stdout/stderr/logs/artifacts, (c) fail the build if any signing step exits non-zero, and (d) publish a signature manifest file (`SIGNATURES.json`) alongside the release artifacts listing each artifact's SHA-256 + signer identity + signature file name + timestamp.

---

### AC-SIGN-009 — DESKTOP-001 REQ-DK-013 interoperability (Consistency)

**Given** a `goosed` binary signed per REQ-SIGN-001/002 and a Desktop App containing a public key anchor distributed per this SPEC,
**When** the Desktop App launches the daemon,
**Then** the verification described in DESKTOP-001 REQ-DK-013 **shall** succeed using the public key material defined by this SPEC's distribution mechanism, and DESKTOP-001 AC-DK-010 **shall** pass without modification.

---

## §6. Technical Notes

### 6.1 macOS notarization

- **Command flow**: `codesign --sign "Developer ID Application: ..." --options runtime --timestamp --deep goose-agent.app` → `ditto -c -k --keepParent goose-agent.app goose-agent.zip` → `xcrun notarytool submit goose-agent.zip --apple-id ... --team-id ... --password ... --wait` → `xcrun stapler staple goose-agent.app`.
- **Credentials**: Apple ID + App-specific password + Team ID, stored in CI secrets. Hardened Runtime는 `--options runtime` 플래그로 활성화.
- **Entitlements**: `goosed`가 네트워크/파일 접근을 요구하면 `entitlements.plist`에 명시하고 `codesign --entitlements` 전달.

### 6.2 Windows signtool

- **Command**: `signtool sign /tr http://timestamp.digicert.com /td sha256 /fd sha256 /a /n "<Subject Name>" <artifact>`.
- **Timestamp**: RFC3161 필수. Timestamp 없이 서명하면 인증서 만료 시점에 모든 설치가 실패 (AC-SIGN-005).
- **Dual-sign**: SHA-1 레거시 체인 미지원 (Windows 10+ 전제).
- **SmartScreen**: EV 인증서 사용 시 초기 신뢰 자동 획득. OV 인증서는 신뢰 빌드업 기간 필요.

### 6.3 Tauri updater ed25519

- **Keygen**: `tauri signer generate` → private key (aead-encrypted) + public key. Private key는 CI secret, public key는 `tauri.conf.json` `updater.pubkey`에 임베드.
- **Sign**: `tauri signer sign --private-key <encrypted> --password <secret> <artifact>` → `.sig` 생성.
- **Manifest**: `latest.json` (또는 프로젝트 관행 manifest)에 `{version, notes, pub_date, platforms: {"darwin-x86_64": {"signature": "<base64>", "url": "..."}}}` 형식.
- **App-side verify**: Tauri 런타임이 manifest 다운로드 → signature 검증 → payload 다운로드 → payload signature 검증 → apply.

### 6.4 cosign (Sigstore)

- **Keyless mode**: `cosign sign-blob --yes <artifact>` + OIDC authentication → Rekor transparency log entry. 증명서는 Fulcio가 단기 발급.
- **Keyed mode**: `cosign sign-blob --key <private> <artifact>` → `.sig` + `.pem`.
- **Verify**: `cosign verify-blob --certificate <pem> --certificate-identity <email> --certificate-oidc-issuer <url> <artifact>`.

### 6.5 Signing CI workflow (GitHub Actions 기준)

```yaml
# .github/workflows/sign-release.yml (형식 예시, 실제 파일은 Run phase에서 작성)
name: sign-release
on:
  push:
    tags: ['v*']
jobs:
  sign-macos:
    runs-on: macos-latest
    steps:
      - uses: actions/checkout@v4
      - name: Import Developer ID cert
        run: |
          echo "$APPLE_CERT_P12" | base64 --decode > cert.p12
          security import cert.p12 -P "$APPLE_CERT_PASSWORD" -k build.keychain
        env:
          APPLE_CERT_P12: ${{ secrets.APPLE_CERT_P12 }}
          APPLE_CERT_PASSWORD: ${{ secrets.APPLE_CERT_PASSWORD }}
      - name: Codesign + Notarize
        # (명령 예시, 구체 파일은 Run phase)
  sign-windows:
    runs-on: windows-latest
    # signtool + RFC3161 timestamp
  sign-linux:
    runs-on: ubuntu-latest
    # minisign + optional cosign
  publish-manifest:
    needs: [sign-macos, sign-windows, sign-linux]
    # SIGNATURES.json 생성 및 릴리스 첨부
```

Secrets 주입 경로는 GitHub Actions `secrets.` 네임스페이스 사용. Private key material은 환경변수로만 전달, 파일시스템 영속화 시 즉시 `shred`로 소거.

### 6.6 Key rotation 프로토콜 (요약)

1. T-30일: 차기 키 `K_next` 생성, CI secrets에 등록
2. T-30일 ~ T-0: 신규 릴리스에 `{K_current, K_next}` 쌍 anchor 포함 (기존 키로 서명)
3. T-0 (rotation day): `K_next`로 서명 시작, 다음 릴리스에 `K_current` revocation entry + 새 anchor `{K_next, K_next2}` 포함
4. T+∞: `K_current` private key 폐기 (HSM zeroize 또는 CI secret 삭제)

### 6.7 Revocation 배포 경로

Revocation list는 각 릴리스 Desktop App 번들 내 `trust-anchors.json`에 임베드:

```json
{
  "active_anchors": ["<pubkey_base64>"],
  "revoked_keys": [{"key_id": "<id>", "revoked_at": "2026-05-01T00:00:00Z", "reason": "scheduled_rotation"}],
  "manifest_version": 3
}
```

별도 CRL 서버를 두지 않는 이유: (a) 추가 가용성 의존성 회피, (b) offline 환경 대응, (c) MITM 차단 (서명된 Desktop App 번들 내에서만 신뢰).

---

## §7. Dependencies

### 7.1 선행 SPEC

- **SPEC-GOOSE-DESKTOP-001** (required, status: in-progress):
  본 SPEC은 DESKTOP-001 REQ-DK-013 및 §9 Exclusions에서 명시적으로 분리됨. DESKTOP-001이 검증 계약(verification contract)을 정의하고 본 SPEC이 서명 생성/키 분배(signing & distribution)를 정의한다. 양 SPEC의 통합 검증은 AC-SIGN-009에서 명시.

- **SPEC-GOOSE-CREDPOOL-001** (related, non-blocking):
  CI secrets 관리 인프라. 본 SPEC은 "signing material이 안전히 CI에 주입된다"는 전제만 사용하며, vault 구현 자체는 CREDPOOL-001 범위. 양 SPEC은 병렬 진행 가능.

### 7.2 후행 SPEC (본 SPEC이 선행 조건)

- **SPEC-GOOSE-DESKTOP-001 v0.3.0**: 본 SPEC 확정 후 DESKTOP-001 §9 Exclusions에서 "키 분배 unresolved" 항목 제거 및 REQ-DK-013 cross-reference 업데이트 예정.
- **SPEC-GOOSE-SLSA-001** (future): supply-chain attestation (SLSA level 3+), in-toto 레이어. 본 SPEC의 서명 체인 위에 구축.

### 7.3 외부 의존성

- Apple Developer Program 등록 (조직 계정)
- Windows EV code-signing certificate (DigiCert/Sectigo 등 CA로부터 조달)
- GitHub Actions (또는 동등 CI) secrets 저장소
- Tauri CLI v2.x (`tauri signer`)
- `minisign` 또는 `cosign` CLI

---

## §8. Risks

| ID      | Risk                                                                   | Likelihood | Impact     | Mitigation                                                                                                                                 |
|---------|------------------------------------------------------------------------|------------|------------|--------------------------------------------------------------------------------------------------------------------------------------------|
| RISK-01 | Signing key leak (CI 환경 침해)                                        | Medium     | Critical   | REQ-SIGN-004 rotation; CI secrets read-only scope; audit log; HSM 사용 권장. 유출 감지 시 즉시 revocation 발행.                            |
| RISK-02 | Apple Developer 인증서 만료 / 갱신 누락                                | Low        | High       | 만료 D-60일 알림 자동화 (CI 작업으로 구현). 만료 후 서명 시 AC-SIGN-005에 따라 timestamp로 부분 완화.                                      |
| RISK-03 | Windows EV cert revocation by CA                                       | Low        | Critical   | CA 다원화 (primary + backup CA 검토). Revocation 시 본 SPEC REQ-SIGN-004 rotation 프로토콜 재사용.                                          |
| RISK-04 | CI 무결성 침해 (Actions runner compromise)                             | Low        | Critical   | Self-hosted runner 대신 managed runner 사용; pin runner images; required reviewers for workflow changes; dependency pinning.              |
| RISK-05 | Key rotation grace period 내 user가 update 하지 않음                   | Medium     | Medium     | Grace period 30일 + release 내 sticky notification; rotation 통지를 release notes 상단에 고정.                                             |
| RISK-06 | Downgrade attack (older signed version 강제)                           | Medium     | High       | Update manifest에 monotonic `version` + `min_version` 필드; 앱이 현재 버전 미만 거부 (본 SPEC 범위 외 구현, DESKTOP-001 updater 로직 보강). |
| RISK-07 | Linux minisign vs cosign 선택 혼란                                     | Low        | Low        | REQ-SIGN-005 Optional로 명시; 기본은 minisign, cosign은 advanced opt-in. 문서에 명시.                                                     |
| RISK-08 | Tauri updater 공개키 임베드 실수 (잘못된 키 포함)                      | Low        | Critical   | CI workflow에서 임베드 직전 expected SHA 비교; 실패 시 빌드 중단. AC-SIGN-008 (c) 조항이 차단.                                              |
| RISK-09 | macOS notarization 지연 (Apple 서버 측 queue)                          | Medium     | Low        | `notarytool submit --wait` 사용으로 동기 처리; timeout 30분 설정; 실패 시 재시도 2회.                                                     |
| RISK-10 | Release pipeline에서 SIGNATURES.json 누락 (AC-SIGN-008 위반)           | Low        | High       | CI workflow 마지막 단계에서 manifest 존재 확인; 없으면 릴리스 published 상태 전환 차단.                                                     |

---

## §Exclusions (What NOT to Build)

본 SPEC은 다음을 **명시적으로 제외**한다. 각 항목은 별도 SPEC 또는 운영 프로세스로 분리된다.

- **EXCL-01 Secrets vault 구현**: GitHub Actions `secrets.` 또는 외부 vault 도입 자체의 아키텍처 결정은 **SPEC-GOOSE-CREDPOOL-001** 범위. 본 SPEC은 "secrets이 안전히 CI에 주입된다"는 인터페이스 전제만 사용.
- **EXCL-02 키 생성 자체의 암호학적 프로토콜**: ed25519 keygen 도구 선택, HSM 프로비저닝, CSR 생성 절차 등은 운영 RUNBOOK으로 분리. 본 SPEC은 "적절히 생성된 키가 CI에 존재한다"를 전제.
- **EXCL-03 Apple/Microsoft 인증서 조달 프로세스**: 조직 차원의 법인 인증, 연간 갱신, 결제 등은 조달 업무로 분리.
- **EXCL-04 Supply-chain attestation (SLSA, in-toto)**: 서명 자체는 본 SPEC, attestation 레이어는 후속 SPEC-GOOSE-SLSA-001. 두 레이어는 독립적으로 가치를 가진다.
- **EXCL-05 End-user 수동 검증 도구 배포**: 엔드유저는 OS-level 검증(Gatekeeper, SmartScreen) 및 앱 내장 검증에 의존한다. 별도 verification CLI 배포는 범위 외.
- **EXCL-06 Sigstore keyless 모드 상세 설계**: REQ-SIGN-005에서 옵션으로만 언급. 상세 OIDC 플로우, Fulcio 정책, Rekor 쿼리 최적화는 후속 SPEC 또는 운영 RUNBOOK.
- **EXCL-07 CRL 서버 운영**: §6.7에서 설명한 이유로 별도 CRL 서버를 두지 않음. 모든 revocation은 릴리스 번들 내 trust-anchors.json으로 전달.
- **EXCL-08 코드 난독화 / anti-tamper (RASP)**: 서명은 tamper 감지이지 tamper 방지가 아니다. Anti-tamper 런타임 방어는 범위 외.
- **EXCL-09 엔드유저 모바일 배포 (iOS/Android)**: 본 SPEC은 데스크톱 플랫폼(macOS/Windows/Linux) 한정. 모바일 앱 서명(App Store/Play Store)은 해당 플랫폼 표준 워크플로 별도 적용.

---

## §Traceability Summary

| Requirement   | Acceptance Criteria                       | Dependencies                   | Out-of-Scope Link |
|---------------|-------------------------------------------|--------------------------------|-------------------|
| REQ-SIGN-001  | AC-SIGN-001, AC-SIGN-002, AC-SIGN-003     | DESKTOP-001 REQ-DK-013         | EXCL-01, EXCL-02  |
| REQ-SIGN-002  | AC-SIGN-001, AC-SIGN-002, AC-SIGN-008     | CREDPOOL-001 (secrets)         | EXCL-01, EXCL-03  |
| REQ-SIGN-003  | AC-SIGN-004, AC-SIGN-005, AC-SIGN-006     | DESKTOP-001 AC-DK-010          | EXCL-08           |
| REQ-SIGN-004  | AC-SIGN-006, AC-SIGN-007                  | (internal)                     | EXCL-07           |
| REQ-SIGN-005  | AC-SIGN-003                               | cosign infra (optional)        | EXCL-06           |
| (consistency) | AC-SIGN-009                               | DESKTOP-001 REQ-DK-013, AC-DK-010 | —              |

---

End of spec.md.
