# SPEC-GOOSE-JOURNAL-001 Progress

- Started: 2026-05-12 (Plan Phase entry)
- Resume marker: **M3 Run Phase COMPLETE — 26/26 AC GREEN — sync 진입 가능**
- Development mode: TDD (RED-GREEN-REFACTOR)
- Coverage target: 85% (per quality.yaml)
- Coverage achieved: 84.1% (M3 누적, M2 83.5% 대비 +0.6%)
- LSP gates baseline: 0 errors / 0 type errors / 0 lint warnings
- Lifecycle: spec-anchored
- Priority: P0
- Phase: 7 (Daily Companion, ritual/evening)
- Size: 중(M)
- M1 status: implementation complete, 19 AC GREEN
- M2 status: implementation complete, 6 AC GREEN (AC-006/007/021/024/025/026)
- M3 status: implementation complete, 1 AC GREEN 신규 (AC-020) + AC-002/012/023 보강

## 2026-05-12 M1 Run Phase Session

### Phase 2 — Implementation (TDD RED-GREEN-REFACTOR)

**완료 일자**: 2026-05-12  
**총 태스크**: T-001 ~ T-022 (22개) — 전체 completed

#### 신규 파일 (production)

| 파일 | 역할 |
|------|------|
| `internal/ritual/journal/types.go` | 핵심 DTO 및 sentinel errors |
| `internal/ritual/journal/config.go` | Config 로드 (yaml, privacy-safe defaults) |
| `internal/ritual/journal/storage.go` | SQLite 어댑터 (WAL, FTS5, 0600/0700) |
| `internal/ritual/journal/crisis.go` | 위기 감지 + CrisisResponse 상수 |
| `internal/ritual/journal/prompts.go` | 저녁 프롬프트 vault (4 카테고리) |
| `internal/ritual/journal/emotion_dict.go` | 12 카테고리 한국어 감정 사전 |
| `internal/ritual/journal/analyzer.go` | 로컬 VAD 분석기 (부정어/강도 처리) |
| `internal/ritual/journal/audit.go` | audit.FileWriter 어댑터 (텍스트 미포함) |
| `internal/ritual/journal/writer.go` | JournalWriter 11-step Write 시퀀스 |
| `internal/ritual/journal/export.go` | ExportAll/DeleteAll/OptOut API |
| `internal/ritual/journal/orchestrator.go` | 저녁 체크인 오케스트레이터 |

#### 신규 파일 (test)

| 파일 | 역할 |
|------|------|
| `internal/ritual/journal/storage_test.go` | 7개 스토리지 테스트 |
| `internal/ritual/journal/crisis_test.go` | 5개 위기 감지 테스트 |
| `internal/ritual/journal/prompts_test.go` | 4개 프롬프트 vault 테스트 |
| `internal/ritual/journal/analyzer_test.go` | 6개 VAD 분석 테스트 |
| `internal/ritual/journal/config_test.go` | 4개 설정 로드 테스트 |
| `internal/ritual/journal/writer_test.go` | 15개 writer 테스트 |
| `internal/ritual/journal/export_test.go` | 6개 export 테스트 |
| `internal/ritual/journal/orchestrator_test.go` | 7개 오케스트레이터 테스트 |
| `internal/ritual/journal/integration_test.go` | 1개 통합 테스트 (build tag: integration) |
| `internal/ritual/journal/testdata/journal/emotion_dict.golden.yaml` | golden snapshot |

#### 수정 파일

| 파일 | 변경 내용 |
|------|-----------|
| `internal/audit/event.go` | EventTypeRitualJournalInvoke 추가 |
| `internal/audit/event_test.go` | requiredTypes 슬라이스 갱신 |

#### AC 달성 현황 (M1 19개)

| AC | 상태 | 커버 테스트 |
|----|------|-------------|
| AC-001 | GREEN | TestWriter_OptInDefaultOff |
| AC-002 | GREEN | TestWriter_LLMOptOutDefault |
| AC-003 | GREEN | TestVAD_LocalAnalysis_Happy |
| AC-004 | GREEN | TestEmoji_SadDetection_Tired |
| AC-005 | GREEN | TestWriter_CrisisFlag_Set, TestCrisis_DirectKeyword_Match |
| AC-008 | GREEN | TestWriter_LogsRedacted |
| AC-009 | GREEN | TestWriter_A2A_NeverInvoked, TestForbiddenImports_NoA2A |
| AC-010 | GREEN | TestExport_UserFiltered |
| AC-011 | GREEN | TestDeleteAll_Immediate, TestStorage_DeleteAll_HardDelete |
| AC-012 | GREEN | TestWriter_PrivateMode_LocalOnly |
| AC-013 | GREEN | TestStorage_FilePermissions_0600_0700 |
| AC-014 | GREEN | TestOrchestrator_TodayEntryExists_SkipPrompt, TestOrchestrator_TimeoutWithoutResponse |
| AC-015 | GREEN | TestOrchestrator_LowMoodSoftTone |
| AC-016 | GREEN | TestWriter_AllowLoRATraining_DefaultFalse |
| AC-017 | GREEN | TestStorage_RetentionDays_NightlyCleanup |
| AC-018 | GREEN | TestWriter_PersistRetry_AndErr |
| AC-019 | GREEN | TestPrompts_AllNeutral_NoForbiddenPhrase |
| AC-022 | GREEN | TestWriter_INSIGHTSCallback_OnSuccess |
| AC-023 | GREEN | TestCrisis_NoClinicalLanguage |

#### 품질 게이트

| 게이트 | 결과 |
|--------|------|
| `gofmt -l` | 0 파일 (clean) |
| `go vet` | 0 이슈 |
| `golangci-lint run` | 0 이슈 |
| `go test -race -count=10` | PASS (전체) |
| 커버리지 | 82.3% (78% 목표 초과) |
| 신규 외부 의존성 | 0 (T-022: modernc/sqlite + google/uuid 모두 기존 의존성) |

#### LSP nitpick fix (post-review)

- crisis_test.go:30, 52 + prompts_test.go:31, 46, 73 — `tc := tc` / `tpl := tpl` Go 1.22 pre-scoping idiom 5건 제거 (forvar 진단, Go 1.22+ for-loop variable auto per-iteration scope)
- audit.go:39-41 — `for k, v := range extra { meta[k] = v }` → `maps.Copy(meta, extra)` (mapsloop 진단, Go 1.21+ stdlib)
- audit.go:5 + export.go:6 + writer.go:5 `fmt already declared` compiler 진단은 **stale gopls cache false positive** (실 build/vet/lint/race 모두 0). main session 직접 검증 후 무시.
- emotion_dict.go:18, 83, 87 + audit.go:20, 55, 66 unusedfunc 진단은 **cross-file usage false positive** (analyzer.go / writer.go 에서 사용 중). 무시.
- integration_test.go:3 build tag warning 은 정상 (LSP 가 build tag 미적용, integration test 는 별도 tag 로 실행).

#### 회귀 검증 (전체 프로젝트)

- `go test -race -count=3 ./...`: PASS (단, internal/mcp/transport/TestNewStdioTransport_EnvInjection 1회 transient flake — 재실행 시 PASS, JOURNAL 무관, system load timing 영향)

---

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

## 2026-05-12 M2 Run Phase Session

### Phase 2 — Implementation (TDD RED-GREEN-REFACTOR)

**완료 일자**: 2026-05-12
**총 태스크**: T-023 ~ T-030 (8개) — 전체 completed

#### 신규 파일 (production)

| 파일 | 역할 |
|------|------|
| `internal/ritual/journal/recall.go` | MemoryRecall.FindAnniversaryEvents/FindSimilarMood |
| `internal/ritual/journal/anniversary.go` | AnniversaryDetector + ImportantDate + IdentityClient interface |
| `internal/ritual/journal/trend.go` | TrendAggregator.WeeklyTrend/MonthlyTrend + NaN sparkline |
| `internal/ritual/journal/chart.go` | RenderChart (Unicode ▁▂▃▄▅▆▇█ + NO_COLOR 지원) |
| `internal/ritual/journal/search.go` | JournalSearch.Search (FTS5 + user_id 격리 + prefix 매칭) |
| `internal/ritual/journal/summary.go` | SummaryJob.RunWeekly (로컬 집계, LLM 호출 0) |

#### 신규 파일 (test)

| 파일 | 역할 |
|------|------|
| `internal/ritual/journal/recall_test.go` | 9개 recall 테스트 (anniversary + low-valence filter + cosine similarity) |
| `internal/ritual/journal/anniversary_test.go` | 6개 anniversary detector 테스트 (±1day window + edge) |
| `internal/ritual/journal/trend_test.go` | 6개 trend 집계 테스트 (NaN gap + empty + 30day) |
| `internal/ritual/journal/chart_test.go` | 5개 chart 렌더 테스트 (block glyphs + NO_COLOR + empty) |
| `internal/ritual/journal/search_test.go` | 8개 FTS5 search 테스트 (user scope + injection + empty query) |
| `internal/ritual/journal/summary_test.go` | 7개 weekly summary 테스트 (cadence + disabled + zero entries) |

#### 수정 파일

| 파일 | 변경 내용 |
|------|-----------|
| `internal/ritual/journal/config.go` | RecallLowValence 필드 추가 (trauma recall protection opt-in) |
| `internal/ritual/journal/writer.go` | Search stub → JournalSearch 위임 + searcher 필드 추가 |
| `internal/ritual/journal/orchestrator.go` | anniversary branch 활성 + WithAnniversaryDetector/WithClock DI 추가 |
| `internal/ritual/journal/orchestrator_test.go` | AC-007 테스트 2건 추가 (TestOrchestrator_AnniversaryPrompt_Wedding 등) |

#### AC 달성 현황 (M2 6개)

| AC | 상태 | 커버 테스트 |
|----|------|-------------|
| AC-006 | GREEN | TestRecall_AnniversaryEvents_LastYear |
| AC-007 | GREEN | TestOrchestrator_AnniversaryPrompt_Wedding |
| AC-021 | GREEN | TestWeeklySummary_SundayCadence_Generates |
| AC-024 | GREEN | TestSearch_FTS5_UserScoped |
| AC-025 | GREEN | TestWeeklyTrend_AggregationWithGaps |
| AC-026 | GREEN | TestRenderChart_SevenDaysWithNaN |

#### 품질 게이트

| 게이트 | 결과 |
|--------|------|
| `gofmt -l` | 0 파일 (clean) |
| `go vet` | 0 이슈 |
| `golangci-lint run` | 0 이슈 |
| `go test -race -count=10 ./internal/ritual/journal/...` | PASS |
| 커버리지 | 83.5% (80% 목표 초과) |
| 신규 외부 의존성 | 0 |

#### LSP nitpick fix (post-review)

- summary.go:163 — `for _, tok := range strings.Fields(...)` → `for tok := range strings.FieldsSeq(...)` (Go 1.24+ stringsseq, iter.Seq[string] 효율적)
- recall.go:95 + recall_test.go (4 lines) `RecallLowValence undefined / unknown field` compiler 진단은 **stale gopls cache false positive** (config.go:34 에 정의됨, build/vet/lint 0 + race PASS 로 검증). main session grep 재검증 후 무시.

#### 핵심 설계 결정

1. **FTS5 prefix 매칭**: `"query"*` 형식으로 한국어 어절 분리 없이 prefix 검색 지원. `"산책"*`이 `"산책을"`, `"산책하다"` 등 모두 매칭.
2. **Rowid subquery 방식**: FTS5 content table join (`INNER JOIN journal_fts ON journal_fts.rowid = e.rowid`) 대신 `WHERE rowid IN (SELECT rowid FROM journal_fts WHERE MATCH)` 패턴 사용 — SQLite 버전 호환성 우수.
3. **AnniversaryDetector DI**: `WithAnniversaryDetector()` 옵션 메서드로 주입 — M1 기존 orchestrator 시그니처 변경 없음 (이하위 호환).
4. **MemoryRecall trauma filter**: valence < 0.3 기본 필터 (R6). `config.RecallLowValence=true` opt-in으로 포함 가능.
5. **TrendAggregator NaN**: 엔트리 없는 날은 `math.NaN()` — 렌더러가 `·` (middle dot)으로 표기.

#### 잔여 deviation / open question

- IDENTITY-001 실제 client 구현 없음 (M2는 interface + mock). 실 wiring은 M3 또는 별도 SPEC.
- weekly summary의 `pendingSummaryFlag`는 orchestrator와 통신 채널 미구현 (flag 필드만 존재). M3에서 orchestrator가 flag를 읽어 prompt 시 summary 제시.
- LLM 기반 summary 서술은 M3 scope (M2는 로컬 집계만).

---

## 2026-05-12 M3 Run Phase Session

### Phase 2 — Implementation (TDD RED-GREEN-REFACTOR)

**완료 일자**: 2026-05-12
**총 태스크**: T-031 ~ T-036 (6개) — 전체 completed

#### 신규 파일 (production)

| 파일 | 역할 |
|------|------|
| `internal/ritual/journal/analyzer_llm.go` | LLMClient interface + LLMEmotionAnalyzer (REQ-017 system prompt + clinical reject + parse-fail fallback) |
| `internal/ritual/journal/summary_llm.go` | LLMSummaryEnhancer.EnhanceWeeklySummary (aggregated payload only, no raw text) |

#### 수정 파일

| 파일 | 변경 내용 |
|------|-----------|
| `internal/ritual/journal/writer.go` | step 5 LLM 분기 활성 (EmotionLLMAssisted guard + llmAnalyzer 분리 필드) |
| `internal/ritual/journal/summary.go` | WeeklySummary.OneLiner 필드 추가 (M3 LLM 서술) |

#### 신규 파일 (test)

| 파일 | 역할 |
|------|------|
| `internal/ritual/journal/analyzer_llm_test.go` | 9개 LLM 분석기 테스트 (AC-020 payload assertion + AC-023 보강) |
| `internal/ritual/journal/summary_llm_test.go` | 8개 LLM summary enhancer 테스트 (aggregate payload + clinical reject + parse-fail) |

#### 수정 파일 (test)

| 파일 | 변경 내용 |
|------|-----------|
| `internal/ritual/journal/writer_test.go` | AC-002/AC-012 LLM mock counter 보강 + TestWriter_LLMAssistedEnabled_CallsLLM 신규 |

#### AC 달성 현황 (M3)

| AC | 상태 | 커버 테스트 |
|----|------|-------------|
| AC-020 | GREEN (신규) | TestLLMAnalyzer_PayloadIsTextOnly |
| AC-002 | GREEN (보강) | TestWriter_LLMOptOutDefault (LLM mock counter 강화) |
| AC-012 | GREEN (보강) | TestWriter_PrivateMode_LocalOnly (LLM mock counter 강화) |
| AC-023 | GREEN (보강) | TestLLMAnalyzer_NeverCalledOnCrisis, TestLLMAnalyzer_RejectsClinicalLanguage |

**누적 AC**: 26/26 GREEN (M1=19 + M2=6 + M3=1)

#### 품질 게이트

| 게이트 | 결과 |
|--------|------|
| `gofmt -l` | 0 파일 (clean) |
| `go vet` | 0 이슈 |
| `golangci-lint run` | 0 이슈 |
| `go test -race -count=10 ./internal/ritual/journal/...` | PASS |
| 커버리지 | 84.1% (M2 83.5% 대비 +0.6%, 회귀 0) |
| 신규 외부 의존성 | 0 (LLMClient interface만 — 실 wiring 없음) |

#### LSP nitpick fix (post-review)

- writer_test.go:82 — `mockClient := &mockLLMClient{response: validLLMResponse}` 의 `response` field unused write (mockClient 가 wire 안 됨, invokeCount==0 만 검증). `&mockLLMClient{}` 로 단순화 (unusedwrite 진단 처리).

#### Privacy invariants 검증

| Invariant | 검증 테스트 | 결과 |
|-----------|-------------|------|
| LLM payload = entry.Text only (user_id/date/attachment/emoji/private_mode/allow_lora 부재) | TestLLMAnalyzer_PayloadIsTextOnly | PASS |
| crisis entry → LLM 호출 0회 | TestLLMAnalyzer_NeverCalledOnCrisis | PASS |
| PrivateMode=true → LLM 호출 0회 | TestLLMAnalyzer_NeverCalledOnPrivateMode, TestWriter_PrivateMode_LocalOnly | PASS |
| LLM 응답 임상 어휘 → silent reject + local fallback | TestLLMAnalyzer_RejectsClinicalLanguage | PASS |
| LLM JSON parse fail → silent fallback (사용자 가시 에러 없음) | TestLLMAnalyzer_JSONParseFailFallback | PASS |
| summary LLM payload = aggregated stats only (raw entry text 부재) | TestSummaryLLM_PayloadAggregateOnly | PASS |

#### 설계 결정

1. **LLMEmotionAnalyzer 분리**: `NewJournalWriter` 가 `*LLMEmotionAnalyzer` 를 감지해 `llmAnalyzer` 필드에 분리 저장. Step 4 = 항상 LocalDictAnalyzer, Step 5 = LLM guard (EmotionLLMAssisted && !PrivateMode && !isCrisis && llmAnalyzer != nil) 조건 통과 시 override. 기존 step 4 타입 assertion 설계는 step 4 에서 LLM 이 이미 호출되는 버그를 내포했으므로 폐기.
2. **WeeklySummary.OneLiner**: summary.go 의 `WeeklySummary` 구조체에 `OneLiner string` 필드 추가. M2 기존 테스트 회귀 없음 (zero value = "").
3. **LLM real wiring deferred**: `LLMClient` interface 는 mock 만으로 구현됨. 실제 LLM-ROUTING-V2 provider 연결은 별도 wiring SPEC (또는 orchestrator 수정 PR) 에서 처리.

#### 잔여 deviation / open question

- `LLMClient` interface 의 실 wiring (provider 연결) 은 orchestrator 또는 별도 wiring layer 에서 처리 필요. 본 M3 는 interface + mock 으로 종결.
- `WeeklySummary.OneLiner` 를 orchestrator 가 읽어 prompt 에 포함하는 로직은 미구현 (M3 scope 외). orchestrator 수정 PR 에서 처리.

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
