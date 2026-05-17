---
id: SPEC-MINK-ONBOARDING-001
version: 0.4.0
amendment_of: v0.3.2
status: draft
created_at: 2026-05-17
author: manager-spec
priority: high
---

# Amendment v0.4 — Step 1 자동 감지 결과 소비 명시화

> **본 문서는 SPEC-MINK-ONBOARDING-001 v0.3.2 에 대한 Amendment 이다.**
> v0.3.2 본문은 그대로 유지되며, 본 Amendment 는 v0.4.0 으로 승격 시 본문에 병합된다.
> v0.4 Amendment 의 핵심 변경: Step 1 (Welcome + Locale) AC 를 SPEC-MINK-LOCALE-001 v0.4.0 의 자동 감지 결과 (`accuracy=high/medium/manual`) 소비 흐름으로 정렬한다. 기존 REQ/AC 번호 재배치 없음.

---

## HISTORY

| 버전 | 날짜 | 변경 사유 | 담당 |
|-----|------|---------|------|
| 0.4.0 | 2026-05-17 | Amendment v0.4 작성. SPEC-MINK-LOCALE-001 v0.4.0 (Phase 2 implemented, amendment-v0.2 6 AC GREEN) 가 Web 측 `navigator.geolocation` + reverse geocoding + IP geolocation HTTP 프로브 + CLI 측 IP geolocation + `--no-auto-detect` flag 를 구현 완료함. 본 amendment 는 ONBOARDING-001 Step 1 의 AC 표현을 LOCALE-001 자동 감지 결과 소비 흐름으로 정렬한다. 기존 AC-OB-003 / AC-OB-004 의 의미를 보존하면서 "LOCALE-001 Detect() 결과를 어떻게 Step 1 UI 에 pre-select 하고 사용자가 어떤 시점에 override 할 수 있는지" 를 명문화한다. 신규 AC-OB-021~023 (자동 감지 high / medium fallback / manual fallback) 추가. 기존 AC 변경 0. | manager-spec |

---

## 1. Amendment 개요

### 1.1 변경 배경

선행 SPEC-MINK-LOCALE-001 v0.4.0 implementation 종결 (2026-05-16, 3 PR 머지: #223 백엔드 probe + IP geolocation, #224 frontend + reverse geocoding, #225 CLI auto-detect + `--no-auto-detect`) 로 다음 동작이 production-ready 가 되었다:

- **Web 측**: `Step1Locale.tsx` 컴포넌트 mount 시 (1) `navigator.geolocation.getCurrentPosition` → reverse geocoding 백엔드 hop, (2) 권한 거부 / 5s timeout 시 `InstallApi.probeLocale` (ipapi.co) fallback, (3) 모두 실패 시 4-preset radio 가 manual fallback 으로 노출.
- **CLI 측**: `mink init` 진입 시 stderr 1-line 고지 (`CLINoticeText`) → `DetectOptions.AutoDetect=true` 경로로 IP geolocation + OS timezone 시도. `--no-auto-detect` flag 가 set 이면 기존 OS env 경로만 사용.
- **정확도 등급**: `LocaleContext.Accuracy ∈ {high, medium, manual}` 가 LLM prompt + UI 배지에 노출됨.

ONBOARDING-001 본문 (v0.3.2) 의 AC-OB-003 / AC-OB-004 는 LOCALE-001 `Detect()` 결과를 참조하지만 amendment-v0.2 이전의 단순 OS detect 흐름을 가정하고 있다. 본 amendment 는 그 표현을 자동 감지 흐름으로 갱신하고, 자동 감지 결과 소비 의무를 ONBOARDING-001 의 명시적 acceptance criteria 로 승격한다.

### 1.2 변경 요약

| 항목 | v0.3.2 | v0.4.0 (본 Amendment) |
|------|--------|----------------------|
| Step 1 데이터 소스 | LOCALE-001 `Detect()` (Phase 1 OS-only) | LOCALE-001 `DetectWithOptions(AutoDetect: true)` (Phase 2 자동 감지) |
| AC-OB-003 의미 | OS 감지 → dropdown pre-select | LOCALE-001 자동 감지 (`accuracy=high|medium|manual`) → 4-preset / dropdown pre-select |
| AC-OB-004 의미 | OS vs IP 충돌 시 사용자 선택 강제 | (보존) 충돌 발생 시 `LocaleConflict` 표시 + 선택 강제 |
| 신규 AC | — | **AC-OB-021** (Web high) / **AC-OB-022** (Web medium fallback) / **AC-OB-023** (CLI `--no-auto-detect` 우회) |
| Privacy notice 의무 | 암묵 | **AC-OB-024** — `locale.privacy.notice.web` / `locale.privacy.notice.cli` i18n 키 사용 (SPEC-MINK-I18N-001 v0.4 catalog) |

---

## 2. AC 갱신 본문 (v0.4.0 으로 승격 시 §5 Test Scenarios 에 병합)

### 2.1 갱신 — AC-OB-003

> **변경 의도**: 기존 표현은 LOCALE-001 Phase 1 (OS-only) 가정. 본 amendment 는 Phase 2 자동 감지 결과 소비를 명문화한다.

**AC-OB-003 — Locale 감지 + 수정 (verifies REQ-OB-006, REQ-OB-003 — v0.4 갱신)**

- **Given** LOCALE-001 `DetectWithOptions(AutoDetect: true)` 가 `country="KR"`, `primary_language="ko-KR"`, `timezone="Asia/Seoul"`, `accuracy="high"` 반환
- **When** Step 1 (Welcome + Locale) 표시
- **Then** "거주 국가: 대한민국" (Web UI 4-preset radio 중 KR 가 pre-selected, `accuracy="high"` 배지 표시) 또는 selection list (CLI 첫 항목 pre-selected) 로 표시되며 사용자가 "일본" 으로 변경 후 Next → CONFIG-001 에 `country="JP"`, `accuracy="manual"` 저장 (override 시 accuracy 는 manual 로 강등), UI 언어는 한국어 → 일본어로 즉시 전환

### 2.2 보존 — AC-OB-004

**AC-OB-004 — OS vs IP 충돌 해결 (verifies REQ-OB-012 — v0.4 변경 없음)**

본 AC 는 LOCALE-001 `LocaleConflict` 반환 시 동작을 정의한다. amendment-v0.2 이후에도 동일 의미로 유효하며 본 amendment 에서 표현 변경 없음. `LocaleConflict.OS` 와 `LocaleConflict.IP` 값이 자동 감지 결과 (`accuracy="medium"` 이상) 에서 추출됨을 §6.11 참조 주석으로만 명시한다.

### 2.3 신규 — AC-OB-021

**AC-OB-021 — Web Step 1 자동 감지 high 경로 (verifies REQ-OB-006, LOCALE-001 REQ-LC-041)**

- **Given** Web UI Step 1 진입, 브라우저 Geolocation 권한 허용, `navigator.geolocation.getCurrentPosition` 가 5s 내에 좌표 반환, 백엔드 reverse geocoding (`internal/locale/reverse.go` Nominatim) 가 `country="JP"` 응답
- **When** `Step1Locale` 컴포넌트가 mount 됨
- **Then** Step 1 의 4-preset radio 중 JP 가 pre-selected, `accuracy="high"` 배지 (e.g. "정확도: 높음 (GPS)") 표시, "지역 자동 감지 중..." indicator 가 사라지고 Next 버튼 활성. 사용자가 그대로 Next 시 CONFIG-001 에 `country="JP"`, `accuracy="high"` 저장

### 2.4 신규 — AC-OB-022

**AC-OB-022 — Web Step 1 권한 거부 → IP fallback medium (verifies LOCALE-001 REQ-LC-041, REQ-LC-045)**

- **Given** Web UI Step 1 진입, 브라우저 Geolocation 권한 거부 (또는 5s timeout), `InstallApi.probeLocale(sessionId)` 가 `country="US"`, `accuracy="medium"` 반환
- **When** `Step1Locale` 컴포넌트가 mount 됨
- **Then** GPS 실패 후 자동으로 IP fallback 으로 전환, US 가 pre-selected, `accuracy="medium"` 배지 (e.g. "정확도: 보통 (IP)") 표시. 사용자에게 별도 에러 메시지 노출 없이 진행 가능 (non-blocking, REQ-LC-041)

### 2.5 신규 — AC-OB-023

**AC-OB-023 — CLI `--no-auto-detect` flag 자동 감지 우회 (verifies LOCALE-001 REQ-LC-042, AC-LC-022)**

- **Given** 사용자가 `mink init --no-auto-detect` 실행
- **When** CLI Step 1 진입
- **Then** stderr 에 `locale.privacy.notice.cli` 텍스트 (= `CLINoticeText`) 가 출력되지 **않음**, IP geolocation HTTP 프로브 호출 0, 기존 OS env 경로 (Phase 1 `Detect()`) 만 실행됨, 결과는 `accuracy="manual"` 로 기록. (대조군: `mink init` 또는 `mink init --auto-detect` 실행 시 stderr 에 1-line 고지 출력 + IP 프로브 1회 호출)

### 2.6 신규 — AC-OB-024

**AC-OB-024 — Privacy notice 텍스트 i18n 키 사용 (verifies LOCALE-001 REQ-LC-045)**

- **Given** SPEC-MINK-I18N-001 v0.4 catalog (`internal/i18n/catalog/{ko,en}.yaml`) 에 `locale.privacy.notice.web` + `locale.privacy.notice.cli` 키가 정의됨
- **When** Web UI Step 1 진입 또는 `mink init` 실행
- **Then** Web 측은 `data-testid="privacy-notice"` DOM 노드가 GPS prompt 전에 렌더링 (현 구현은 bilingual 하드코딩 — 후속 hygiene PR 에서 i18n 키 lookup 으로 전환), CLI 측은 stderr 1-line 출력이 catalog `locale.privacy.notice.cli` 텍스트와 1:1 일치. 두 채널 모두 PIPA / GDPR / CCPA 준수 표기 포함 (catalog 본문 또는 후속 sub-key)

---

## 3. 비기능 영향

### 3.1 회귀 위험

- 본 amendment 는 acceptance criteria 만 갱신하며 신규 코드 작성 의무를 만들지 않는다. 기존 LOCALE-001 v0.4.0 구현 + 기존 ONBOARDING 본문 구현으로 이미 AC-OB-021~023 은 GREEN 으로 판정 가능 (Phase 4 hotfix #226, LOCALE Phase 2 #223/#224/#225 의 합산 결과).
- AC-OB-024 는 i18n 키 정의 의무만 부과하며, Web 컴포넌트의 catalog lookup 전환은 별도 hygiene PR 로 후속.

### 3.2 측정 가능 종결물

- Step 1 자동 감지 6 시나리오 Playwright E2E (`e2e/locale-autodetect.spec.ts`) 통과 — 본 amendment 와 동시 PR 로 작성됨.
- `internal/i18n/catalog/{ko,en}.yaml` 에 `locale.privacy.notice.web` / `locale.privacy.notice.cli` 키 존재 검증 (기존 `internal/i18n` 테스트 suite 가 자동 검증).

### 3.3 사용자 가시 변경

- 명시적 사용자 가시 변경 없음. 본 amendment 는 기존 동작에 대한 acceptance criteria 표현 강화이며, 행동 변경은 LOCALE-001 v0.4.0 이미 production 에 반영된 자동 감지 흐름 그대로다.

---

## 4. Known Limitations 이월

본 amendment 는 다음 항목을 v0.4.0 본문에 병합하지 않고 amendment-v0.5 후보로 남겨둔다:

1. Web 측 `privacy-notice` DOM 의 하드코딩 bilingual 텍스트 → catalog i18n 키 lookup 으로 전환 (현재 단방향: catalog 만 존재).
2. `accuracy` 배지의 i18n 키화 (현재 `Step1Locale.tsx` 하드코딩 — `accuracy.high`, `accuracy.medium`, `accuracy.manual` 키 후보).
3. AC-OB-022 의 "non-blocking" 정확한 의미 — 5s timeout 동안 사용자가 수동 4-preset 선택을 우선시할 수 있는지에 대한 UX 결정 (현재 구현은 detection 중 4-preset disable). Playwright 6 시나리오 수동 dogfooding 결과 기반 결정 예정.

---

## 5. 승격 절차

본 amendment 가 v0.4.0 으로 승격될 때:

1. `.moai/specs/SPEC-MINK-ONBOARDING-001/spec.md` frontmatter `version: 0.3.2 → 0.4.0`, `updated_at` 갱신.
2. §5 Test Scenarios 의 AC-OB-003 본문을 본 문서 §2.1 표현으로 교체 (의미 보존, 표현만 갱신).
3. §5 Test Scenarios 끝에 AC-OB-021~024 추가. AC-OB-013 은 deprecated 유지 (번호 재배치 금지).
4. `status: draft → implemented` (Web E2E `e2e/locale-autodetect.spec.ts` 통과 후).
5. 본 amendment 파일은 `archived/` 로 이동 (관례 준수, 현재 본 SPEC 디렉터리에 `amendment-v0.3.md` 가 같은 패턴).

---

REQ coverage: REQ-OB-003, REQ-OB-006, REQ-OB-012 (기존 — 표현 갱신 only), LOCALE-001 REQ-LC-040~045 (consumer 명시)
AC coverage: AC-OB-003 (갱신), AC-OB-004 (보존), AC-OB-021/022/023/024 (신규)
관련 SPEC: SPEC-MINK-LOCALE-001 v0.4.0, SPEC-MINK-I18N-001 v0.4 (catalog `locale.privacy.notice.*`)
