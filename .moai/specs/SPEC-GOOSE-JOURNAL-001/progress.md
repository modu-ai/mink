# SPEC-GOOSE-JOURNAL-001 Progress

- Started: 2026-05-12 (Plan Phase entry)
- Resume marker: Plan Phase audit-ready
- Development mode: TDD (RED-GREEN-REFACTOR)
- Coverage target: 85% (per quality.yaml)
- LSP gates baseline: 0 errors / 0 type errors / 0 lint warnings (Plan 시점 기준 — Run Phase 진입 직전 재측정)
- Lifecycle: spec-anchored
- Priority: P0
- Phase: 7 (Daily Companion, ritual/evening)
- Size: 중(M)

## 2026-05-12 Plan Phase Session

### Phase 1 — Planning

기존 산출물 확인:
- `spec.md` v0.2.0 (2026-04-25, status: planned, 36KB)
  - 0.1.0 → 0.2.0 변경: 감사 리포트(JOURNAL-001-audit.md, 0.55/FAIL) 결함 교정. EARS 라벨 정합성 + AC-013~026 신설.
  - 23 REQ (Ubiquitous 8 / Event-Driven 8 / State-Driven 3 / Unwanted 1 / Optional 3 — 0.2.0 audit 통과)
  - 26 AC (M1=19, M2=6, M3=1 + 보강)
- `research.md` (9.4KB): VAD 모델, 한국어 감정 사전, crisis 키워드, anniversary logic, terminal chart, prompt 디자인, weekly summary, export 스키마.
- `status.txt`: planned

Sprint 1+2 인프라 가용 자산 검토:
- `internal/audit` AuditWriter — 본 SPEC 의 audit log 어댑터 그대로 재사용. EventType 1개 신규 추가만 (`EventTypeRitualJournalInvoke = "ritual.journal.invoke"`).
- `internal/permission` Manager — network grant 불필요 (JOURNAL 은 e2e local), fs grant 만 사용. CI / non-interactive 환경은 FS-ACCESS-001 default seed 등록.
- MEMORY-001 SQLite driver — 재사용. 단 journal 전용 DB 파일 (`~/.goose/journal/journal.db`) 로 facts DB 와 격리.
- HOOK-001 callback registry — `EveningCheckInTime` 이벤트 consumer 등록.
- IDENTITY-001 — `important_dates` 노출 API 활용 (M2 anniversary).
- INSIGHTS-001 — `OnJournalEntry` consumer callback 호출 (M1 wiring).

WEATHER-001 의 plan 산출물 (plan.md / acceptance.md / tasks.md / spec-compact.md / progress.md) 패턴 학습 — milestone 분할 + atomic task + planned_files Drift Guard + Test plan 매핑 + DoD 정확히 정렬.

### 핵심 Architectural 결정 4건

1. **패키지 위치**: `internal/ritual/journal/` 그대로 유지 (spec.md §6.1 원안). WEATHER 가 `internal/tools/web/weather*.go` 로 이전된 것과 다르게 — JOURNAL 은 LLM Tool 이 아니라 별도 ritual 도메인 (HOOK consumer + 사용자 입력 storage + recall) 이므로 web tool 패키지에 합쳐선 안 됨. `internal/ritual/` prefix 가 향후 morning/midday/anniversary 등 다른 ritual SPEC 의 자연스러운 부모 디렉토리.

2. **Storage backend**: SQLite (MEMORY-001 reuse). bbolt / flat JSON / LevelDB 모두 평가 후 SQLite 채택 — (a) MEMORY-001 의존성 이미 존재, 신규 외부 의존성 0, (b) FTS5 native (M2 Search), (c) 날짜 범위/user 필터 query 가 indexed access, (d) backup/export 가 단일 파일 copy. 단 facts DB 와 격리하여 별도 파일 (`~/.goose/journal/journal.db`, 0600).

3. **LLM integration 정도**: M1/M2 시점 LLM 호출 0 (`config.emotion_llm_assisted` 무관 unconditional skip). M3 에서만 분기 활성. 이유: (a) plan.md §1 의 milestone 분할 명확화, (b) M1 GREEN 게이트가 LLM mock counter==0 검증 (회귀 차단 강화), (c) crisis entry 는 영구 LLM 금지 (AC-023 + REQ-020), (d) PrivateMode entry 도 영구 LLM 금지 (AC-012). 사전 기반 LocalDictAnalyzer (research.md §2 알고리즘) 가 M1 baseline.

4. **Milestone 분할**: M1 = Journal Core (Writer + Storage + Local emotion + Crisis + Orchestrator + Export, 19 AC), M2 = Long-term Memory Recall (Anniversary + Trend + Search + Weekly summary cadence, 6 AC), M3 = LLM-assisted (LLM analyzer + Summary 향상, 1 AC + 보강). M1 단독 완료 시 일기 입력 + 로컬 감정 + crisis canned response 가 동작. M2 가 spec.md 의 "감성적 성장" 컨셉 (recall) 을 완성. M3 는 정확도 보강.

### Phase 1 산출물 신규 / 갱신

**spec.md** v0.2.0 → v0.2.1 갱신:
- HISTORY append v0.2.1 entry (Sprint 2 진입 + milestone 분할 + plan 산출물 신규 작성 명시)
- frontmatter: status `planned` → `audit-ready`, version 0.2.0 → 0.2.1, updated_at 2026-04-25 → 2026-05-12, labels 추가 5개 (sprint-2, milestone/m1-m3, tdd-mode, infra-reuse/audit, infra-reuse/permission)
- §3.1 IN SCOPE: milestone 분할 표 추가 (M1/M2/M3)
- 본문 §4 EARS / §5 AC / §6 기술적 접근 / §7 의존성 / §8 리스크 / §9 참고 / §10 Exclusions: **무변경** (0.2.0 audit 통과 내용 보존)

**plan.md** 신규 (10 §, ~620 lines):
- §1 Milestone 표 (M1/M2/M3 priority + 산출물 + 의존)
- §2 M1 — Journal Core (산출 파일 + 22 atomic tasks T-001~T-022 + 입력/출력 schema + SQLite schema + storage backend 선택 근거 + LLM integration boundary M3 + Privacy invariants)
- §3 M2 — Long-term Memory Recall (산출 파일 + 8 task T-023~T-030 high-level)
- §4 M3 — LLM-assisted Emotion + Summary (산출 파일 + 5 task T-031~T-035 high-level)
- §5 공통 인프라 (Sprint 1+2 재사용: Audit/Permission/Storage/HOOK/INSIGHTS/IDENTITY/Logging)
- §6 외부 의존성 표 (신규 0 목표)
- §7 테스트 전략 (단위/통합/goldenfile/보안/동시성)
- §8 Test plan 매핑 (26 AC ↔ Test file/function)
- §9 Risk mitigation (R1~R12 plan-level 작업)
- §10 종료 조건 (M1/M2/M3 DoD)

**acceptance.md** 신규 (~770 lines):
- 26 AC Given-When-Then + Test file/function 매핑
- milestone 표기 (M1/M2/M3) 모든 AC 헤더에
- edge case (boundary, error path, falsey 시나리오) 분리
- 종합 DoD (M1/M2/M3)
- 품질 게이트 TRUST 5 매핑

**tasks.md** 신규 (M1 only, 22 atomic tasks, ~180 lines):
- planned_files 컬럼 (Drift Guard 용)
- TDD 사이클 운영 규칙
- privacy-critical 추가 규칙 (AC-008/009/013/019/023 회귀 검증)
- M1 범위 외 (deferred) 명시
- M2/M3 task 는 진입 시 append 패턴 안내 (WEATHER-001 정렬)

**spec-compact.md** 신규 (~140 lines):
- 한 페이지 요약 (LLM 시스템 프롬프트용)
- 핵심 계약 11개 invariants
- 주요 타입 / API 표
- EARS 26 REQ 카테고리별 요약
- 26 AC milestone별 요약
- OUT 13개
- 핵심 invariants (LLM 컨텍스트용) 6개

**progress.md** 신규 (본 파일):
- Plan Phase 결정 기록
- Phase 산출물 트래킹

### Phase 1.5 — Tasks Decomposition

- 22 atomic task 정의 완료 (tasks.md §"M1 Task Decomposition").
- Test pair 패턴 enforce (T-005 crisis.go ↔ T-006 crisis_test.go, T-009/T-010 analyzer.go+emotion_dict.go ↔ T-011 analyzer_test.go 등).
- T-013 큰 task (writer.go) 는 sub-AC 단위로 RED-GREEN 분할 (sub 1~6).
- T-019 (audit EventType 추가) 를 T-012 작업 진입 전 처리 명시.
- T-022 (의존성 검증) 을 T-003 시작 전 처리 명시.
- T-020 integration test 는 모든 unit test 통과 후 마지막에 RED → GREEN.

### Phase 2 — Annotation Cycle (1차 self-audit)

**EARS 형식 검증**: ✓
- Ubiquitous: REQ-001~004, 013, 014, 016, 020 (8개) — `shall` / `shall not` 형식 엄수.
- Event-Driven: REQ-005~009, 021, 022, 023 (8개) — `When ... shall` 형식 엄수.
- State-Driven: REQ-010~012 (3개) — `While ... shall` 형식 엄수.
- Unwanted: REQ-015 (1개) — `If ... then ... shall` 형식 엄수.
- Optional: REQ-017, 018, 019 (3개) — `Where ... shall` 형식 엄수.
- 총 23 REQ — 0.2.0 audit 통과 형식 보존.

**AC Given-When-Then 형식**: ✓ — 26 AC 모두 Given/When/Then + Test file/function 매핑.

**의존성 Reference-only 검증**: ✓ — SCHEDULER-001 (Sprint 1 v0.2.2 completed) / MEMORY-001 (completed) / HOOK-001 (M1 dispatch wired) / AUDIT-001 (completed) / PERMISSION-001 (completed) / IDENTITY-001 (M2 important_dates 활용) / INSIGHTS-001 (consumer) / RITUAL-001 / LORA-001 명시.

**Privacy/Crisis 핵심 invariants 명시**: ✓
- spec-compact.md 의 "핵심 invariants" 6개 + acceptance.md 의 "Privacy Invariants" 표 (M1 부터 enforce, AC ↔ enforcement layer 매핑).
- crisis canned response literal-only enforce (REQ-020 + AC-023).
- multi-user 격리 storage layer (AC-010 + AC-024).
- A2A 전송 영구 금지 (REQ-014 + AC-009 정적 import 검사).

**Negative path AC 포함**: ✓
- AC-001 (config disabled → ErrJournalDisabled)
- AC-005 + AC-023 (crisis 응답 literal + LLM 호출 0회 + 진단 어휘 부재)
- AC-008 (entry text log 부재 negative 검증)
- AC-009 (A2A counter==0 + 정적 import 검사)
- AC-013 (file permission 잘못된 디렉토리 → error)
- AC-018 (storage 3회 실패 → ErrPersistFailed + queue evict + SIGTERM 후 재시작 시 큐 empty)
- AC-019 (forbidden phrase 부재 + open question 강제)
- AC-020 (LLM payload 에 user_id/date/attachment 부재 검증)
- AC-024 (SQL injection attempt → 0건 + table 손상 없음)

**Behavioral 표현 일관**: ✓ — 모든 REQ 가 `shall` / `shall not` 사용. `should` / `might` / `usually` 부재 (0.2.0 audit 통과 보존).

**OUT 명시 충분**: ✓ — spec.md §3.2 의 13개 항목 + spec.md §10 Exclusions 의 13개 명시. plan.md §2.7 Privacy Invariants 표 가 enforce layer 명시.

**Risks**: ✓ — R1~R12 (12개), 모두 가능성/영향/완화 컬럼 보유. spec.md 의 R1~R9 + plan.md 의 R10~R12 신규 (외부 의존성 / audit text 누출 / prompt vault drift).

### Self-audit 결론

**PASS** — Plan Phase 산출물이 EARS 컴플라이언스 + 완전성 + 일관성 + Privacy invariants 명시 기준 충족. status `planned` → `audit-ready` 전환 가능.

미흡 사항 (deferred to plan-auditor 검증):
- M1 시점에는 LLM 분기가 unconditional skip 으로 구현되므로 AC-002 / AC-012 의 "LLM mock counter==0" 검증이 trivial 통과. M3 진입 시 실제 LLM 분기 enforce 가 회귀 검증의 핵심이 됨. plan-auditor 가 M1/M3 분리 검증 전략을 합리적 분할로 인정하는지 확인 필요.
- T-022 의 SQLite driver 실제 의존성 확인은 main session 진입 후 `go list -m all | grep sqlite` 실행 결과에 따라 결정 — 미존재 시 plan.md §6 갱신 + AskUserQuestion.
- AC-013 의 Windows 권한 테스트는 OS 별 분기 또는 skip on Windows. 본 SPEC 은 Linux/macOS 우선 (goose 사용자 분포 가정).

### 잔여 deviation / open question

1. **HOOK-001 wiring 변경 필요 여부**: T-020 의 `cmd/goose-runtime/wire_journal.go` 수정 필요 여부는 HOOK-001 의 callback registry 패턴 확인 후 결정. callback subscribe API 가 깔끔하게 노출되어 있으면 wiring 변경 없이 integration test 만 추가.
2. **FS-ACCESS-001 default seed**: `~/.goose/journal/**` 가 default seed 에 미포함 시 첫 disk write 가 사용자 grant 대기. 본 SPEC 은 첫 write 가 user 첫 일기 입력 직후이므로 사용자 가시 환경이면 정상. CI / non-interactive 환경에서는 별도 grant 사전 등록 필요. 후속 SPEC 또는 T-022 후속 검토.
3. **SQLite driver 선택**: `mattn/go-sqlite3` (CGo) vs `modernc.org/sqlite` (pure Go). MEMORY-001 의 선택과 정렬. CGo 빌드 환경 (cross-compile, Docker scratch) 영향 확인.
4. **WAL 모드 적용**: SQLite WAL 모드는 multi-reader / single-writer 성능 + crash safety 개선. 단 WAL 파일 (`-wal`, `-shm`) 의 0600 권한 동기 enforce 필요 (T-003 + AC-013).
5. **emotion_dict.golden.yaml 의 사전 범위**: research.md §2.2 의 8 카테고리 + 추가 4 카테고리 (lonely/regret/bored/proud) 로 충분한지 사용자 검증 필요. M1 진입 시 sample 일기 코퍼스 (50+ entry) 로 회귀 검증 후 사전 확장 가능.
6. **Anniversary trauma protection (R6)**: research.md §5.3 의 valence < 0.3 자동 필터는 default ON. 사용자 명시 opt-in (`config.recall_low_valence=true`) 추가 여부는 M2 진입 시 결정.

---

## Status Transitions

- 2026-04-22: created (v0.1.0, status: planned, manager-spec)
- 2026-04-25: v0.2.0 — 감사 리포트 결함 교정 (EARS 라벨 정합성 + AC 신설), status: planned 유지
- 2026-05-12: Plan Phase 산출물 (plan.md / acceptance.md / tasks.md / spec-compact.md / progress.md) 작성 완료, spec.md v0.2.1 갱신 (HISTORY entry 추가, status: planned → audit-ready)
- (next) plan-auditor 1라운드 검증 → audit-ready → ready (run 진입 가능)

---

## 잠재 deviation 추적

본 섹션은 Run Phase 진입 후 발견되는 deviation 을 기록한다 (현재 빈 상태).

| Date | Phase | Description | Resolution |
|------|-------|-------------|------------|
| — | — | — | — |

---

Version: 0.1.0
Last Updated: 2026-05-12
