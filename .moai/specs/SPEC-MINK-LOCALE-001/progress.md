# SPEC-MINK-LOCALE-001 — Progress Log

이 문서는 SPEC 의 amendment 흐름과 Phase 진척을 짧게 기록한다. 본 SPEC 은 단일 spec.md 패턴(별도 acceptance.md / plan.md / tasks.md 분리 없음)을 사용하므로, 본 progress.md 는 amendment 사이클의 hand-off 노트만 남긴다.

---

## 2026-05-16 — amendment-v0.2 작성 (manager-spec)

### 변경 요약

- frontmatter: `version 0.1.1 → 0.3.0`, `status planned → in-progress`, `updated_at 2026-04-25 → 2026-05-16`
- HISTORY: v0.3.0 amendment-v0.2 entry 추가 (v0.2.0 Phase 1 종결물 직후)
- §3.1 IN SCOPE: 항목 10/11/12 추가 (자동 감지 entry / 정확도 등급 / 프라이버시 고지)
- §4.6 EARS Requirements 신규 6 개 추가 (REQ-LC-040 ~ REQ-LC-045)
- §5 Acceptance Criteria 신규 6 개 추가 (AC-LC-020 ~ AC-LC-025, AC-LC-019 는 예약 슬롯)
- §6.11 자동 감지 흐름 + Web/CLI 분기 신설
- §6.12 프라이버시 고지 정책 (GDPR/PIPA/CCPA 매핑) 신설
- §7 Dependencies: 자동 감지 의존성 3 행 추가 (navigator.geolocation, ipapi.co, Nominatim) — 새 모듈 도입 0
- §8 Risks: R8/R9/R10/R11/R12 신규 (VPN 부정확 / 외부 HTTP outage / GPS 프라이버시 / accuracy backward compat / Web 고지 시각 분리)
- §10 영향 범위 (Impact Analysis) 신설 — 백엔드 9 항목 / 프런트엔드 5 항목 / 통합 5 항목 / 보존 6 항목 / 마이그레이션 노트

### 영향 통계

- spec.md: 589 라인 → ~ 870 라인 (대략 280 라인 증가)
- 신규 REQ: 6 (REQ-LC-040~045)
- 신규 AC: 6 (AC-LC-020~025)
- 신규 § (Technical Approach): 2 (§6.11, §6.12)
- 신규 § (top-level): 1 (§10 영향 범위)
- 영향 받는 파일 (Phase 2 구현 PR 예상): 백엔드 9 / 프런트엔드 5 / 통합 5

### 트레이서빌리티

| 신규 AC | Covers REQ |
|--------|-----------|
| AC-LC-020 | REQ-LC-040, REQ-LC-041 |
| AC-LC-021 | REQ-LC-041, REQ-LC-043 |
| AC-LC-022 | REQ-LC-042, REQ-LC-045 |
| AC-LC-023 | REQ-LC-044 |
| AC-LC-024 | REQ-LC-040, REQ-LC-041 |
| AC-LC-025 | REQ-LC-045 |

모든 신규 REQ 는 최소 1 개의 AC 가 커버하며, 모든 신규 AC 는 binary verifiable (test fixture / DOM assertion / subprocess capture / strings.Contains 단언 등).

### 보존 사항 (회귀 차단 대상)

- REQ-LC-001 ~ REQ-LC-016: 변경 없음 (Phase 1 종결물)
- AC-LC-001 ~ AC-LC-018: 변경 없음 (Phase 1 종결물 18 AC 모두 PASS 유지 의무)
- §6.1 ~ §6.10: 변경 없음
- `internal/locale/cultural.go` / `os_*.go` / `geo.go` / `prompts.go` 의 기존 export 시그니처 보존

### 후속 작업 (별도 PR 권장)

1. **Phase 2 구현 PR** — §10 영향 범위 체크리스트에 따라 백엔드 + 프런트엔드 구현. AC-LC-020~025 6 개를 GREEN 으로 만든다. expert-backend + expert-frontend 병렬 spawn 권장 (isolation 미사용 foreground, 누적 21회 검증된 패턴).
2. **SPEC-MINK-ONBOARDING-001 amendment** — Step 1 acceptance criteria 갱신 (자동 감지 결과 소비).
3. **SPEC-MINK-I18N-001 catalog 등록** — 6.12.2 의 `locale.privacy.notice.web` 키 추가 (별도 hotfix PR).
4. **Web E2E (Playwright)** — 자동 감지 6 시나리오 (성공 / 권한 거부 → IP fallback / timeout / 모두 실패 → manual / 정확도 배지 / privacy notice DOM). Phase 4 hotfix PR 에 포함 권장.

### Known Limitations (amendment-v0.2 범위 외)

- Web reverse geocoding 의 정확도 검증 (성/시 단위) — vendor 응답 신뢰. amendment-v0.3 후보.
- 다국적 사용자(primary+secondary language) 의 자동 감지 동시 해석 — primary 만 자동 감지, secondary 는 manual override 로 위임. REQ-LC-009 보존.
- China GFW 환경에서의 동작 검증 — REQ-LC-015 (ipapi.co skip) 와 자동 감지 entry 통합 시 edge case (network probe 자체가 OS env 만으로 fallback) 가 발생. unit test 는 mock 으로 검증, 실 device 검증은 별도 manual QA.
- `iplookup.go` 의 RFC 1918 / loopback / link-local 차단 정책의 정확한 CIDR 리스트 — Phase 2 구현 시 결정 (감사 검토 대상).

---
