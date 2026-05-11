---
id: SPEC-GOOSE-JOURNAL-001
artifact: tasks
scope: M1 only (Journal Core — Writer + Storage + Local emotion + Crisis + Orchestrator + Sample API)
version: 0.1.0
created_at: 2026-05-12
author: manager-spec
---

# SPEC-GOOSE-JOURNAL-001 — Task Decomposition (M1)

본 문서는 Phase 1.5 산출물. plan.md §2.2 의 22 atomic tasks 를 git-tracked artifact 로 보존하고
planned_files 컬럼을 통해 Phase 2 / 2.5 의 Drift Guard 가 사용한다.

각 task 는 단일 TDD cycle (RED-GREEN-REFACTOR) 내 완결. 의존 관계는 plan.md §2.2 와 정렬.

본 SPEC 은 다음 Sprint 1+2 인프라를 그대로 재사용한다:
- `internal/audit` AuditWriter (T-019 에서 EventType 1개 추가만 함)
- `internal/permission` Manager (network grant 불필요, fs grant 만 사용)
- MEMORY-001 SQLite driver (별도 DB 파일로 격리)
- HOOK-001 callback registry (T-020 wiring)
- IDENTITY-001 user 격리 (M2 anniversary 활성, M1 은 user_id 격리만)
- INSIGHTS-001 OnJournalEntry consumer (M1 callback wiring + mock 검증)

따라서 infrastructure task 는 최소화 (T-019 EventType 추가 + T-020 HOOK wiring 만) 하고
journal 도메인 task 에 집중한다.

---

## M1 Task Decomposition

| Task ID | Description | Requirement | Dependencies | Planned Files | Status |
|---------|-------------|-------------|--------------|---------------|--------|
| T-001 | types.go: JournalEntry / StoredEntry / Vad / Anniversary / Trend DTO 정의 (M2 의 Trend 도 미리 선언; 본 task 는 단순 struct + JSON tag) | REQ-002, REQ-006 | — | internal/ritual/journal/types.go | pending |
| T-002 | config.go: Config struct + LoadJournalConfig(path) (yaml.v3, 누락 시 default: enabled=false / emotion_llm_assisted=false / allow_lora_training=false / cloud_backup=false / retention_days=-1 / prompt_timeout_min=60 / weekly_summary=false) | REQ-001, REQ-003, REQ-010, REQ-011, REQ-018 | T-001 | internal/ritual/journal/config.go | pending |
| T-003 | storage.go: MEMORY-001 SQLite 어댑터. journal_entries 테이블 + FTS5 mirror + 트리거 3개 (idempotent CREATE IF NOT EXISTS). 0600 파일 / 0700 디렉토리 권한 enforce. WAL 모드 활성. | REQ-002, AC-013 | T-001 | internal/ritual/journal/storage.go | pending |
| T-004 | storage_test.go RED: TestStorage_InsertAndReadByID / TestStorage_FilePermissions_0600_0700 (AC-013) / TestStorage_ListByDateRange / TestStorage_DeleteAll_HardDelete (AC-011) / TestStorage_DeleteByDateRange / TestStorage_RetentionDays_NightlyCleanup (AC-017) / TestStorage_UserScopedQuery (M1 부분, M2 Search 보강) | AC-011, AC-013, AC-017 | T-003 (signatures) | internal/ritual/journal/storage_test.go | pending |
| T-005 | crisis.go: crisisKeywords []string (research.md §4.1 의 8 직접 표현) + crisisResponse string const (§6.4 literal, 1577-0199/1393/1388 모두 포함) + CrisisDetector.Check(text) bool (case-insensitive substring) | REQ-015, REQ-020 | — | internal/ritual/journal/crisis.go | pending |
| T-006 | crisis_test.go RED: TestCrisis_DirectKeyword_Match (table-driven 8+ keywords, AC-005) + TestCrisis_NoFalsePositive_HappyText + TestCrisis_CaseInsensitive + TestCrisis_CannedResponseHasHotline (1577-0199/1393/1388 검증) + TestCrisis_NoClinicalLanguage (AC-023, "진단"/"우울"/"치료" 부재) | AC-005, AC-023, REQ-015, REQ-020 | T-005 (signatures) | internal/ritual/journal/crisis_test.go | pending |
| T-007 | prompts.go: 중립 프롬프트 vault (research.md §7.3 의 neutral / low_mood_sequence / anniversary_happy / anniversary_sensitive 카테고리 4종, 각 3+ variant). prompts.PickNeutral(seed) / prompts.PickLowMood() / prompts.PickAnniversary(date_name) / prompts.All() API. 모든 템플릿이 금지 구문 미포함 + 물음표로 끝남. | REQ-013 | — | internal/ritual/journal/prompts.go | pending |
| T-008 | prompts_test.go RED: TestPrompts_AllNeutral_NoForbiddenPhrase (AC-019 — 가장 큰 비밀/서운한 점/숨기고 싶은/부끄러운/가장 후회 부재) + TestPrompts_AllOpenQuestion (모든 템플릿이 ? 또는 ？ 로 끝남) + TestPrompts_PickAnniversary_IncludesDateName + TestPrompts_LengthBound (≤ 100 char) | AC-019, REQ-013 | T-007 (signatures) | internal/ritual/journal/prompts_test.go | pending |
| T-009 | analyzer.go: EmotionAnalyzer interface (Analyze(ctx, text, emojiMood) (*Vad, []string, error)) + LocalDictAnalyzer 구현 (토큰화 → tag 매칭 → Top-3 → VAD 가중평균 + 이모지 bonus + 부정어/강조어 처리, research.md §2 알고리즘) | REQ-006 | T-001 | internal/ritual/journal/analyzer.go | pending |
| T-010 | emotion_dict.go: 한국어 감정 사전 hardcoded (research.md §2.2 8 카테고리 + lonely/regret/bored/proud 4 추가 = 12 카테고리). 각 카테고리는 keywords []string + emoji []string + vad Vad. testdata/journal/emotion_dict.golden.yaml 와 동기화. | REQ-006 | T-001 | internal/ritual/journal/emotion_dict.go, testdata/journal/emotion_dict.golden.yaml | pending |
| T-011 | analyzer_test.go RED: TestVAD_LocalAnalysis_Happy (AC-003) + TestEmoji_SadDetection_Tired (AC-004) + TestVAD_NegationFlip (research.md §2.3, "행복하지 않아" valence 반전) + TestVAD_IntensityModifier (research.md §2.4, "너무 행복해" arousal *= 1.2) + TestVAD_NoMatch_NeutralFallback (vad = {0.5, 0.5, 0.5}) + TestVAD_TopThreeTags_OrderedByCount | AC-003, AC-004, REQ-006 | T-009, T-010 | internal/ritual/journal/analyzer_test.go | pending |
| T-012 | audit.go: internal/audit 어댑터. EventTypeRitualJournalInvoke 사용 (T-019 에서 추가). meta keys: user_id_hash (sha256[:8]) / operation (write/read/delete_all/delete_range/export/opt_out/evening_prompt_emit/evening_prompt_skip/evening_prompt_timeout/queue_evict/retention_cleanup/insights_callback_panic) / entry_length_bucket (<100/100-500/500+) / emotion_tags_count / has_attachment / crisis_flag / outcome (ok/err). entry text 절대 미포함. | REQ-004 | T-001, T-019 | internal/ritual/journal/audit.go | pending |
| T-013 | writer.go: JournalWriter interface (Write/Read/ListByDate/Search M2 stub returning empty + user filter) + sqliteJournalWriter 구현. Write() 11-step 시퀀스: (1) config gate ErrJournalDisabled (2) user 격리 검증 ErrInvalidUserID (3) CrisisDetector.Check → CrisisFlag set + crisisResponse prepend (4) LocalEmotionAnalyzer.Analyze (5) M3 LLM 분기 unconditional skip (6) anniversary nil (M1) (7) word count (8) allow_lora_training = config (9) storage.Insert retry max 3, queue max 10 (10) 실패 시 ErrPersistFailed + 사용자 메시지 + queue 폐기 (11) 성공 시 audit + INSIGHTS callback (mock 가능) | REQ-001, REQ-002, REQ-003, REQ-005, REQ-010, REQ-012, REQ-015, REQ-016, REQ-019 | T-001, T-002, T-003, T-005, T-009, T-010, T-012 | internal/ritual/journal/writer.go | pending |
| T-014 | writer_test.go RED: 9 시나리오: TestWriter_OptInDefaultOff (AC-001) + TestWriter_LLMOptOutDefault (AC-002, M1 LLM mock counter==0) + TestWriter_PrivateMode_LocalOnly (AC-012, M1 동일) + TestWriter_CrisisFlag_Set (AC-005 부분) + TestWriter_LogsRedacted (AC-008, zaptest/observer 로 entry text 부재 검증) + TestWriter_A2A_NeverInvoked (AC-009) + TestForbiddenImports_NoA2A (정적 검증) + TestWriter_AllowLoRATraining_DefaultFalse (AC-016) + TestWriter_PersistRetry_AndErr (AC-018, 3회 fail → ErrPersistFailed + queue evict) + TestWriter_INSIGHTSCallback_OnSuccess (AC-022, mock counter==1) | AC-001, AC-002, AC-005, AC-008, AC-009, AC-012, AC-016, AC-018, AC-022 | T-013 (signatures) | internal/ritual/journal/writer_test.go | pending |
| T-015 | export.go: ExportAll(ctx, userID) ([]byte, error) (storage layer 에서 WHERE user_id = ? strict filter, REQ-016 / AC-010) + DeleteAll(ctx, userID) (hard delete, AC-011) + DeleteByDateRange(ctx, userID, from, to) + OptOut(ctx, userID, deleteData bool) | REQ-016 | T-001, T-003, T-013 | internal/ritual/journal/export.go | pending |
| T-016 | export_test.go RED: TestExport_UserFiltered (AC-010 — u1 export 에 u2 entry 0건) + TestDeleteAll_Immediate (AC-011) + TestDeleteByDateRange_PartialDelete + TestOptOut_PreservesData_WhenFlagFalse + TestOptOut_DeletesData_WhenFlagTrue + TestExport_EmptyUserID_ErrInvalidUserID | AC-010, AC-011, REQ-016 | T-015 (signatures) | internal/ritual/journal/export_test.go | pending |
| T-017 | orchestrator.go: JournalOrchestrator struct + Prompt(ctx, userID) (저녁 프롬프트 flow). 흐름: (1) config.enabled 체크 (2) 오늘 entry 존재 여부 → 있으면 silent skip + INFO log evening_prompt_skip — AC-014 (3) 없으면 최근 3 entry valence 조회 → 모두 < 0.3 이면 PickLowMood, 그 외 PickNeutral (REQ-009, AC-015) (4) anniversary check (M2 활성, M1 always false) (5) prompt emit + prompt_timeout_min 대기 (timeout 시 INFO log evening_prompt_timeout) (6) 사용자 응답 수신 시 writer.Write 호출 | REQ-005, REQ-008, REQ-009, REQ-013 | T-002, T-007, T-013 | internal/ritual/journal/orchestrator.go | pending |
| T-018 | orchestrator_test.go RED: TestOrchestrator_TodayEntryExists_SkipPrompt (AC-014) + TestOrchestrator_TimeoutWithoutResponse (AC-014 보강 — operation=evening_prompt_timeout INFO) + TestOrchestrator_LowMoodSoftTone (AC-015 — "언제든 이야기해주세요" + "전문가 상담" 포함, 진단성 어휘 부재) + TestOrchestrator_NoLowMoodNeutralPrompt + TestOrchestrator_DisabledConfig_NoOp | AC-014, AC-015, REQ-005, REQ-009 | T-017 (signatures) | internal/ritual/journal/orchestrator_test.go | pending |
| T-019 | internal/audit/eventtypes.go (modify): EventTypeRitualJournalInvoke = "ritual.journal.invoke" 상수 추가. internal/audit/eventtypes_test.go (modify): expected EventType list 에 신규 entry 추가 + count +1 갱신. 기존 EventType 회귀 0. | REQ-004 | — | internal/audit/eventtypes.go (modify), internal/audit/eventtypes_test.go (modify) | pending |
| T-020 | HOOK-001 wiring 통합 테스트: cmd/goose-runtime (또는 동등 bootstrap) 에서 HookManager.Subscribe("EveningCheckInTime", orchestrator.OnEveningCheckIn) 호출. 별도 wiring 변경 불필요한 경우 integration test (TestEveningHookDispatch_TriggersOrchestratorPrompt) 만 작성. | REQ-005 | T-017 | internal/ritual/journal/integration_test.go (신규, build tag integration), cmd/goose-runtime/wire_journal.go (modify if needed) | pending |
| T-021 | .moai/docs/journal-quickstart.md (신규): opt-in 절차 (config.journal.enabled=true) + 프라이버시 약관 (로컬 only / LLM opt-in / LoRA opt-in) + 사용자 제어 API 사용법 (Export/Delete/OptOut) + 위기 시 전문 상담 안내 (1577-0199/1393/1388). 사용자 문서, 한국어. | docs | none | .moai/docs/journal-quickstart.md | pending |
| T-022 | go.mod / go.sum: SQLite driver (mattn/go-sqlite3 또는 modernc.org/sqlite) 가 MEMORY-001 의존성으로 이미 존재 검증 (`go list -m all | grep sqlite`). uuid (`google/uuid`) 도 동일. 미존재 시 plan.md §6 갱신 + go get + AskUserQuestion 으로 user 승인. **목표: 신규 외부 의존성 0**. | enabling, REQ-002 | none | go.mod, go.sum (modify if needed), .moai/specs/SPEC-GOOSE-JOURNAL-001/plan.md (modify §6) | pending |

**합계**: 22 atomic tasks, ~14 production files (신규) + ~7 test files (신규) + ~3 modifications (audit eventtypes / runtime wiring / go.mod).

---

## Drift Guard Reference

이 표의 `Planned Files` 컬럼은 Phase 2.5 Drift Guard 가 사용한다.
- drift = (unplanned_new_files / total_planned_files) * 100
- ≤ 20%: informational
- 20% < drift ≤ 30%: warning
- > 30%: Phase 2.7 re-planning gate

Total planned files (M1):
- production 신규 (12): types.go, config.go, storage.go, crisis.go, prompts.go, analyzer.go, emotion_dict.go, audit.go, writer.go, export.go, orchestrator.go, .moai/docs/journal-quickstart.md
- test 신규 (8): storage_test.go, crisis_test.go, prompts_test.go, analyzer_test.go, writer_test.go, export_test.go, orchestrator_test.go, integration_test.go (build tag)
- testdata 신규 (1): testdata/journal/emotion_dict.golden.yaml (T-010 산출)
- modifications (3): internal/audit/eventtypes.go, internal/audit/eventtypes_test.go, cmd/goose-runtime/wire_journal.go (필요 시), go.mod/go.sum (T-022 결과에 따라)

unplanned files 가 5건 이하 → drift ≤ 20% (informational), 그 이상 발생 시 progress.md 에 deviation 기록 + plan-auditor 재평가.

---

## TDD 사이클 운영 규칙

1. Pair (예: T-005 crisis.go ↔ T-006 crisis_test.go) 에서 항상 test 부터 작성 (RED).
2. `go test ./internal/ritual/journal/...` compile fail / test fail 확인 후 production 코드 작성 (GREEN).
3. T-013 ~ T-014 의 큰 task (writer.go) 는 sub-AC 단위로 RED-GREEN 분할:
   - sub 1: AC-001 (OptInDefaultOff) → T-013 step 1 + T-014 부분
   - sub 2: AC-005 (CrisisFlag) → T-013 step 3 + T-014 부분
   - sub 3: AC-008 (LogsRedacted) → T-012 + T-013 step 11 + T-014 부분
   - sub 4: AC-016 (AllowLoRATraining default) → T-013 step 8 + T-014 부분
   - sub 5: AC-018 (Persist retry) → T-013 step 9-10 + T-014 부분
   - sub 6: AC-022 (INSIGHTS callback) → T-013 step 11 + T-014 부분
4. 각 GREEN 직후 `go vet ./internal/ritual/journal/...` + `golangci-lint run ./internal/ritual/journal/...` 0 warning 유지.
5. T-019 (EventType 추가) 는 T-012 작업 진입 전 처리 (의존성 미해결 상태로 작업 진입 금지).
6. T-022 (의존성 검증) 은 T-003 시작 전에 처리 (SQLite driver 미존재 시 user 승인 필수).
7. T-020 integration test 는 모든 unit test 통과 후 마지막에 RED → GREEN.

## RED-GREEN-REFACTOR 추가 규칙 (privacy-critical)

- AC-008 (LogsRedacted) 은 매 PR 마다 회귀 검증 필수. zap observer 가 audit log + zap log 모두 capture 해야 함.
- AC-009 (A2A NeverInvoked) 의 정적 import 검사 (`TestForbiddenImports_NoA2A`) 는 패키지 import list 변경 시 자동 fail.
- AC-013 (FilePermissions) 는 OS 별 (Linux/macOS/Windows) 동작 차이 주의. Windows 는 `os.Chmod` 가 read-only flag 만 처리하므로 별도 분기 또는 skip on Windows.
- AC-019 (PromptVault) 는 prompts.go 변경 시 자동 회귀. 새 카테고리 추가 시 forbidden phrase 검사 자동 적용.
- AC-023 (NoClinicalLanguage) 는 crisis.go + (M3) analyzer_llm.go 변경 시 자동 회귀.

---

## M1 범위 외 (deferred)

- AC-JOURNAL-006 (작년 오늘 회상): M2 recall.go + anniversary.go
- AC-JOURNAL-007 (기념일 프롬프트): M2 anniversary.go + orchestrator.go 수정
- AC-JOURNAL-021 (weekly summary cadence): M2 summary.go
- AC-JOURNAL-024 (Search FTS5 user scope): M2 search.go (M1 은 stub 만)
- AC-JOURNAL-025 (WeeklyTrend/MonthlyTrend 집계): M2 trend.go
- AC-JOURNAL-026 (RenderChart): M2 chart.go
- AC-JOURNAL-020 (LLM payload 제약): M3 analyzer_llm.go
- REQ-JOURNAL-007 (FindAnniversaryEvents): M2
- REQ-JOURNAL-008 (AnniversaryDetector): M2
- REQ-JOURNAL-017 (LLM-assisted): M3
- REQ-JOURNAL-018 (weekly summary): M2 (cadence) + M3 (LLM 향상)
- REQ-JOURNAL-021 (Search): M2
- REQ-JOURNAL-022 (TrendAggregator): M2
- REQ-JOURNAL-023 (RenderChart): M2

---

## M2 / M3 Task Decomposition (예비 — Sprint 3+ 진입 시 상세화)

M2 / M3 는 본 plan.md 의 §3 / §4 에서 high-level breakdown 으로 정의되었으며, 별도 atomic task 는 M1 audit-ready → ready → run 진입 후 본 tasks.md 에 append 한다 (WEATHER-001 의 T-024~T-043 추가 패턴 정렬).

---

Version: 0.1.0
Last Updated: 2026-05-12
