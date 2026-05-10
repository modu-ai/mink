# SPEC-GOOSE-WEATHER-001 Progress

- Started: 2026-05-10 (Plan Phase entry)
- Resume marker: Plan Phase audit-ready
- Development mode: TDD (RED-GREEN-REFACTOR)
- Coverage target: 85% (per quality.yaml)
- LSP gates baseline: 0 errors / 0 type errors / 0 lint warnings (Plan 시점 기준 — Run Phase 진입 직전 재측정)

## 2026-05-10 Plan Phase Session

### Phase 1 — Planning
- 기존 산출물 확인: spec.md v0.1.0 (planned, 2026-04-22) + research.md (Provider 비교, DFS_XY_CONV, 한국 PM2.5 기준).
- Sprint 1 의 TOOLS-WEB-001 산출물 (plan.md / tasks.md / acceptance.md / spec-compact.md / progress.md) 패턴 학습 — `webWikipedia` Tool 인터페이스 + `RegisterWebTool` + `common.Deps` + `common.Response` 구조 정확히 정렬.
- Architectural 결정 5건:
  1. **패키지 위치**: `internal/ritual/weather/` (원안) → `internal/tools/web/weather*.go` (Sprint 2 이전). TOOLS-WEB-001 인프라 재사용 극대화. 결정 근거: TOOLS-WEB-001 의 common.Cache + Blocklist + Permission + Audit + RateLimit 가 weather 의 요구사항을 100% 커버.
  2. **Tool naming**: `Weather` (PascalCase, 원안) → `weather_current` / `weather_forecast` / `weather_air_quality` (snake_case, TOOLS-WEB-001 컨벤션 정렬). 단일 도구 + operation enum 패턴 회피 (web_maps 의 operation enum 의 schema allOf if/then 복잡도 학습).
  3. **M1 milestone 범위**: `weather_current` (OpenWeatherMap + cache + offline + IP geolocation + config) 만. KMA 는 M2 로 이연 (R1 KMA API 키 수동 승인 진입 장벽).
  4. **KMA API 키 정책**: M1 사용자 의존성 0 (OWM 만), M2 opt-in. `.moai/docs/weather-quickstart.md` 작성.
  5. **새 외부 의존성**: M1 시점 0 또는 1 (singleflight 기존 여부, T-023 검증). 모두 stdlib + 기존 의존성으로 구현 가능.

### Phase 1 산출물 갱신/신규
- **spec.md** v0.1.0 → v0.1.1 갱신:
  - HISTORY append v0.1.1 entry
  - frontmatter: status `planned` → `audit-ready`, version 0.1.0 → 0.1.1, updated_at 2026-05-10, labels 추가 8개
  - §1 개요: 패키지 위치 + naming + M1~M3 분할 반영
  - §3.1 IN SCOPE: 16개 항목 재작성 (TOOLS-WEB-001 재사용 명시)
  - §3.2 OUT OF SCOPE: 5개 항목 추가 (TOOLS-WEB-001 8 도구 보호, weather.yaml hot reload 등)
  - §4 EARS: REQ-WEATHER-016 (표준 응답 shape) + REQ-WEATHER-017 (RegisterWebTool 통합) 신규 (15 → 17 REQ)
  - §5 AC: AC-WEATHER-009 (registry inventory) + AC-WEATHER-010 (응답 shape) 신규 + AC-WEATHER-008 응답 코드 명시 강화 (8 → 10 AC)
  - §6.1 패키지 레이아웃: weather*.go 파일군 재정리 (M1~M3 표기)
  - §6.6 라이브러리: TOOLS-WEB-001 재사용 명시
  - §6.7 TDD 진입 순서: M1 우선 + M2/M3 누적 갱신
  - §7 의존성: TOOLS-WEB-001 + PERMISSION/AUDIT/RATELIMIT 명시
  - §8 리스크: R7~R10 신규 (디스크 fallback corrupt, bbolt 락, register count 변동, weather.yaml 충돌)
- **plan.md** 신규 (10 §, ~620 lines):
  - M1~M3 milestone 표
  - M1 23 atomic task (T-001~T-023)
  - 입력 schema 상세 3건 + 출력 DTO 명세
  - Provider 비교 + Cache key 정규화 + Offline 정책 + Singleflight 정책
  - M2/M3 high-level breakdown
  - 외부 의존성 표 (신규 0~1)
  - Test 전략 + AC ↔ Test 매핑 (10 AC)
  - Risk mitigation 작업
  - M1/M2/M3 DoD
- **acceptance.md** 신규 (10 AC + DoD + TRUST 5 매핑, ~340 lines):
  - 10 AC Given-When-Then + Test file/function 매핑
  - milestone 표기 (M1/M2/M3)
  - edge case (boundary, error path, NPS 시나리오) 분리
- **tasks.md** 신규 (M1 only, 23 atomic tasks, ~150 lines):
  - planned_files 컬럼 (Drift Guard 용)
  - TDD 사이클 운영 규칙
  - M2/M3 분할 정책 (예비)
- **spec-compact.md** 신규 (~75 lines):
  - 한 페이지 요약 (LLM 시스템 프롬프트용)
  - 3 도구 시그니처 + 17 REQ + 10 AC + Milestone + OUT + 의존
- **progress.md** 신규 (본 파일):
  - Plan Phase 결정 기록
  - Phase 산출물 트래킹

### Phase 1.5 — Tasks Decomposition
- 23 atomic task 정의 완료 (tasks.md §"M1 Task Decomposition").
- Test pair 패턴 enforce (T-004 weather_offline.go ↔ T-005 weather_offline_test.go 등).
- T-010~T-014 weather_current.go 의 큰 task 는 sub-AC 단위로 RED-GREEN 분할 (sub 1~7).
- T-023 (singleflight 의존성 확인) 을 T-012 작업 진입 전 처리로 명시.

### Phase 2 — Annotation Cycle (1차 self-audit)
- EARS 형식 검증: ✓
  - Ubiquitous: REQ-001~004, 016~017 (6개) — `shall` 형식 엄수.
  - Event-Driven: REQ-005~009 (5개) — `When ... shall` 형식 엄수.
  - State-Driven: REQ-010~011 (2개) — `While ... shall` 형식 엄수.
  - Unwanted: REQ-012~013 (2개) — `shall not` 형식 엄수.
  - Optional: REQ-014~015 (2개) — `Where ... shall` 형식 엄수.
- AC Given-When-Then 형식: ✓ — 10 AC 모두 Given/When/Then + Test file/function 매핑.
- 의존성 Reference-only: ✓ — TOOLS-001 v0.1.2 / TOOLS-WEB-001 (M1 implemented) / PERMISSION-001 v0.2.0 / AUDIT-001 v0.1.0 / RATELIMIT-001 v0.2.0 / FS-ACCESS-001 (planned) / CONFIG-001 (planned) 명시.
- Negative path AC 포함: ✓ — AC-WEATHER-008 (rate limit 차단), AC-WEATHER-003 edge (디스크 corrupt), AC-WEATHER-006 (API key redaction 음성 검증), AC-WEATHER-009 edge (duplicate registration), AC-WEATHER-010 negative (cache_hit && stale 동시 발생 불가 assert).
- Behavioral 표현 일관: ✓ — 모든 REQ 가 `shall` / `shall not` 사용. `should` / `might` / `usually` 부재.
- OUT 명시 충분: ✓ — §3.2 의 13개 항목 (원본 6 + Sprint 2 신규 5 + 추가 2). TOOLS-WEB-001 8 도구 변경 금지 명시.
- Risks: ✓ — R1~R10 (10개), 모두 가능성/영향/완화 컬럼 보유.

### Self-audit 결론

PASS — Plan Phase 산출물이 EARS 컴플라이언스 + 완전성 + 일관성 기준 충족. status `planned` → `audit-ready` 전환 가능.

미흡 사항 (deferred to plan-auditor 검증):
- AC-WEATHER-009 의 milestone 별 누적 카운트 (M1=15, M2=16, M3=17) 는 M1 단독 구현 시점에서는 +1 만 검증 가능 (R9). plan-auditor 가 이를 합리적 분할로 인정하는지 확인 필요.
- AC-WEATHER-010 의 `cache_hit && stale` 동시 발생 불가 assert 는 구현 단계에서 정확히 enforce 해야 함 (T-013 시나리오에 명시).

### 잔여 deviation / open question

1. **CONFIG-001 의 status**: planned 로 가정. 실제로 spec.md / 다른 SPEC 의 status 확인 필요. 본 SPEC 은 TOOLS-WEB-001 의 LoadWebConfig 패턴을 모방하므로 CONFIG-001 미완료 상태에서도 진행 가능.
2. **FS-ACCESS-001 default seed**: `~/.goose/cache/weather/**` 가 default seed 에 미포함 시 첫 disk write 가 사용자 grant 대기. 본 SPEC 은 첫 write 가 OWM 호출 직후이므로 사용자 가시 (interactive) 환경이면 정상. CI / non-interactive 환경에서는 별도 grant 사전 등록 필요. T-022 지점 또는 별도 후속 SPEC.
3. **singleflight 의존성** (T-023): `go list -m all | grep singleflight` 결과에 따라 외부 의존성 신규 0 또는 1. plan.md §6 의 "신규 외부 의존성: 0 또는 1" 기록.
4. **OpenWeatherMap v3.0 onecall**: M2 weather_forecast 에서 v2.5 (forecast endpoint) 와 v3.0 (onecall) 중 선택. v3.0 은 별도 subscription 필요할 수 있음 (사용자 비용). M2 진입 시 재검토.

---

## Status Transitions

- 2026-04-22: created (v0.1.0, status: planned, manager-spec)
- 2026-05-10: Plan Phase 산출물 (plan.md / acceptance.md / tasks.md / spec-compact.md / progress.md) 작성 완료, spec.md v0.1.1 갱신 (HISTORY entry 추가, status: planned → audit-ready)
- (next) plan-auditor 1라운드 검증 → audit-ready → ready (run 진입 가능)

---

Version: 0.1.0
Last Updated: 2026-05-10
