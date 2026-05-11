---
id: SPEC-GOOSE-JOURNAL-001
artifact: acceptance
version: 0.1.0
created_at: 2026-05-12
updated_at: 2026-05-12
author: manager-spec
---

# SPEC-GOOSE-JOURNAL-001 — 수용 기준 (Acceptance)

본 문서는 spec.md §5 의 26 AC 를 Given-When-Then 형식으로 상세화하고, 각 AC 를 Go test 파일/함수와 매핑한다.

milestone 표기: 각 AC 의 헤더에 (M1) / (M2) / (M3) 를 명시.
- **M1 (P0)**: AC-001 / 002 / 003 / 004 / 005 / 008 / 009 / 010 / 011 / 012 / 013 / 014 / 015 / 016 / 017 / 018 / 019 / 022 / 023 (19 AC)
- **M2 (P0)**: AC-006 / 007 / 021 / 024 / 025 / 026 (6 AC)
- **M3 (P1)**: AC-020 (1 AC) + AC-002/012/023 의 LLM 분기 보강

---

## AC-JOURNAL-001 — Opt-in default off (M1)

**Given**
- `~/.goose/config/journal.yaml` 미존재 (또는 빈 파일).
- `LoadJournalConfig` 가 default `enabled: false` 반환.
- `JournalWriter` 가 default config 로 초기화.

**When**
- `writer.Write(ctx, JournalEntry{UserID:"u1", Date:today, Text:"오늘 좋았다"})` 호출.

**Then**
- 응답 error == `ErrJournalDisabled`.
- storage 에 entry 저장되지 않음 (`storage.ListByDate(u1, today, today)` 길이 == 0).
- audit log 0 line (config disabled 시 audit 도 emit 안 함).

**Edge — config 존재하지만 enabled 누락**
- yaml 파싱 결과 `enabled` zero-value (false) → 동일하게 `ErrJournalDisabled`.

**Test**
- File: `internal/ritual/journal/writer_test.go`
- Function: `TestWriter_OptInDefaultOff`

---

## AC-JOURNAL-002 — LLM 분석 opt-out 기본 (M1, 보강 M3)

**Given (M1)**
- `config.journal.enabled=true, emotion_llm_assisted=false` (default).
- LLM mock client 가 등록되었으나 `config.emotion_llm_assisted=false` 라서 호출되어선 안됨.

**When (M1)**
- `writer.Write(ctx, JournalEntry{UserID:"u1", Date:today, Text:"좋은 하루였다"})` 호출.

**Then (M1)**
- LLM mock counter == 0.
- entry 의 `vad`/`emotion_tags` 가 LocalDictAnalyzer 결과 (M1 분기 unconditional skip 검증).
- 응답 error nil + `*StoredEntry` 반환.

**Then (M3 보강)**
- `config.emotion_llm_assisted=true` 설정 후 동일 호출 시 LLM mock counter == 1.
- entry 의 `vad` 가 LLM 응답 기반 (또는 JSON parse 실패 시 local fallback).

**Test**
- File: `internal/ritual/journal/writer_test.go`
- Function: `TestWriter_LLMOptOutDefault` (M1) + `TestLLMAnalyzer_OptInCalled` (M3, `analyzer_llm_test.go`)

---

## AC-JOURNAL-003 — VAD 로컬 분석 (행복) (M1)

**Given**
- `LocalDictAnalyzer` 인스턴스 + 기본 한국어 감정 사전 로드 (research.md §2.2).
- text = `"오늘 정말 행복했어, 오랜만에 웃었어"`.
- emojiMood = `""`.

**When**
- `analyzer.Analyze(ctx, text, "")` 호출.

**Then**
- `vad.Valence >= 0.7` (happy 의 base valence 0.9 가중 + 강조어 "정말" intensity bonus).
- `emotion_tags` 슬라이스에 `"happy"` 또는 `"joy"` 또는 `"grateful"` 중 1개 이상 포함.
- `error == nil`.

**Edge — 빈 text**
- text = `""` → `vad = {0.5, 0.5, 0.5}` (neutral fallback), `emotion_tags = []`.

**Test**
- File: `internal/ritual/journal/analyzer_test.go`
- Function: `TestVAD_LocalAnalysis_Happy`

---

## AC-JOURNAL-004 — 이모지 기반 분석 (피곤) (M1)

**Given**
- `LocalDictAnalyzer` 인스턴스.
- text = `"😔 힘들다"`.
- emojiMood 는 text 에 inline 으로 포함.

**When**
- `analyzer.Analyze(ctx, text, "")` 호출.

**Then**
- `vad.Valence < 0.4` (sad/tired 의 낮은 valence + 이모지 😔 -0.2 bonus).
- `emotion_tags` 에 `"sad"` 또는 `"tired"` 중 1개 이상 포함.

**Edge — 이모지 없이 text 만 "힘들다"**
- valence 가 약간 더 높아도 무방 (이모지 bonus 부재).

**Test**
- File: `internal/ritual/journal/analyzer_test.go`
- Function: `TestEmoji_SadDetection_Tired`

---

## AC-JOURNAL-005 — 자해 키워드 crisis response (M1)

**Given**
- `config.journal.enabled=true`.
- `CrisisDetector` 등록.
- text = `"죽고 싶다"`.

**When**
- `writer.Write(ctx, JournalEntry{UserID:"u1", Date:today, Text:"죽고 싶다"})` 호출.

**Then**
- 응답 wrapper 의 user-facing message 가 `crisisResponse` literal (§6.4) 와 완전 일치.
- message 에 `"1577-0199"`, `"1393"`, `"1388"` 모두 포함 (생명의전화/자살예방/청소년).
- entry 가 storage 에 저장됨 (`crisis_flag = true`).
- audit log 1 line, `operation="write"`, `crisis_flag=true`, **entry text 미포함**.
- LLM mock counter == 0 (crisis entry 는 LLM 으로 전송 금지, AC-023 정렬).

**Edge — 대소문자 / 공백 변형**
- `"죽 고 싶 다"` (공백 포함) → 매칭 안 됨 (substring 정확 매치). v0.1 의 한계.
- `"죽고싶어"` (변형) → 매칭 안 됨. v0.2+ 에서 stem 매칭 검토.

**Test**
- File: `internal/ritual/journal/crisis_test.go` + `writer_test.go`
- Function: `TestCrisis_CannedResponseHasHotline` + `TestWriter_CrisisFlag_Set`

---

## AC-JOURNAL-006 — 작년 오늘 회상 (M2)

**Given (M2)**
- u1 이 2025-04-22 에 entry 저장 (`CreatedAt = 2025-04-22T21:00:00Z`).
- 오늘 = 2026-04-22.
- `MemoryRecall` + IDENTITY-001 client 등록.

**When (M2)**
- `recall.FindAnniversaryEvents(ctx, "u1", time.Date(2026, 4, 22, 0, 0, 0, 0, time.UTC))` 호출.

**Then (M2)**
- 반환 슬라이스 길이 ≥ 1.
- 반환 entry 중 적어도 한 entry 의 `CreatedAt.Year() == 2025` AND `CreatedAt.Month() == time.April` AND `|CreatedAt.Day() - 22| <= 1`.
- 반환 entry 의 스키마는 `StoredEntry` 정의 (§6.2) 만 (확장 필드 미요구).

**Edge — 작년 매칭 entry 가 valence < 0.3 (트라우마 회상 방지, R6)**
- valence < 0.3 entry 는 자동 필터링 (`research.md §5.3`) → 반환 슬라이스 길이 0 또는 다른 entry 만.
- user 가 명시적 opt-in (config.recall_low_valence=true) 시 포함.

**Edge — 10년 이상 과거**
- 11년 전 entry 는 반환되지 않음 (max 10년).

**Test**
- File: `internal/ritual/journal/recall_test.go`
- Function: `TestRecall_AnniversaryEvents_LastYear`

---

## AC-JOURNAL-007 — 기념일 프롬프트 (M2)

**Given (M2)**
- IDENTITY-001 의 u1 `important_dates = [{type:"wedding", name:"결혼기념일", date:"2020-04-22"}]`.
- 오늘 = 2026-04-22.
- 오늘 entry 미존재.

**When (M2)**
- `orchestrator.Prompt(ctx, "u1")` 호출.

**Then (M2)**
- emit 된 prompt 문자열에 `"결혼기념일"` 또는 `"특별한 날"` 중 1개 이상 포함.
- prompt 가 `prompts.PickAnniversary("결혼기념일")` 결과.
- audit log `operation="evening_prompt_emit"`.

**Edge — important_date date 가 ±1일 윈도우 밖**
- date = "2020-04-25" → 매칭 안 됨, neutral prompt 사용.

**Test**
- File: `internal/ritual/journal/orchestrator_test.go`
- Function: `TestOrchestrator_AnniversaryPrompt_Wedding`

---

## AC-JOURNAL-008 — 일기 텍스트 로그 미노출 (M1)

**Given**
- zap logger 에 `zaptest/observer.New(zap.DebugLevel)` 적용 (모든 log entry 캡처).
- text = `"비밀 이야기입니다 - 오늘 회사에서 X 매니저와 다툼"`.
- `config.enabled=true`.

**When**
- `writer.Write(ctx, JournalEntry{UserID:"u1", Date:today, Text:text})` 호출.

**Then**
- 캡처된 모든 log entry 의 message + field 값 + serialized JSON 에 `"비밀 이야기"`, `"X 매니저"`, `"다툼"` 문자열 부재.
- audit log entry meta 에 `entry_length_bucket` (`"<100"` / `"100-500"` / `"500+"`), `emotion_tags_count`, `has_attachment`, `crisis_flag` 만 노출.
- `user_id` 는 sha256(user_id)[:8] 로 hash.

**Edge — error path**
- storage Insert fail 시 error log 도 entry text 부재.

**Test**
- File: `internal/ritual/journal/writer_test.go`
- Function: `TestWriter_LogsRedacted`

---

## AC-JOURNAL-009 — A2A 전송 금지 (M1)

**Given**
- A2A mock connection 등록 (모든 outgoing message capture).
- writer 의 dependency injection 에 A2A client 가 **포함되지 않음** (정적 검증).
- `config.enabled=true`.

**When**
- `writer.Write(ctx, JournalEntry{UserID:"u1", Date:today, Text:"테스트"})` 호출.

**Then**
- A2A mock outgoing message counter == 0.
- writer 의 dependency tree 에 A2A client 부재 (`go list -deps` grep `a2a` → 0).

**정적 검증**
- `internal/ritual/journal/` 패키지 import list 에 A2A 패키지 부재.
- forbidden import 테스트 (`TestForbiddenImports_NoA2A` in `writer_test.go`).

**Test**
- File: `internal/ritual/journal/writer_test.go`
- Function: `TestWriter_A2A_NeverInvoked` + `TestForbiddenImports_NoA2A`

---

## AC-JOURNAL-010 — Export 필터링 (M1)

**Given**
- storage 에 u1 entry 5건, u2 entry 3건 저장.
- `config.enabled=true`.

**When**
- `export.ExportAll(ctx, "u1")` 호출.

**Then**
- 반환 JSON 의 `entry_count == 5`.
- `entries` 슬라이스 모든 element 의 `user_id == "u1"`.
- u2 의 entry 0건 포함.
- storage layer 의 SQL 이 `WHERE user_id = ?` 로 필터 (post-processing 아님 — code review 또는 EXPLAIN QUERY PLAN 검증).

**Edge — userID 빈 string**
- `ExportAll(ctx, "")` → `ErrInvalidUserID` 반환, 어떤 entry 도 export 안 됨.

**Test**
- File: `internal/ritual/journal/export_test.go`
- Function: `TestExport_UserFiltered`

---

## AC-JOURNAL-011 — DeleteAll 즉시 반영 (M1)

**Given**
- u1 entry 100건 저장.
- `config.enabled=true`.

**When**
- `export.DeleteAll(ctx, "u1")` 호출.

**Then**
- `writer.ListByDate(ctx, "u1", time.Time{}, time.Now())` 길이 == 0.
- SQLite raw query (`SELECT COUNT(*) FROM journal_entries WHERE user_id = 'u1'`) 결과 == 0 (soft delete 아님).
- FTS5 mirror table 도 동기 삭제 (CASCADE 트리거 검증).
- audit log 1 line, `operation="delete_all"`.

**Edge — 다른 user 영향 없음**
- u2 entry 5건 동시 보유 시 DeleteAll(u1) 후 u2 entry 5건 그대로.

**Test**
- File: `internal/ritual/journal/export_test.go`
- Function: `TestDeleteAll_Immediate`

---

## AC-JOURNAL-012 — Private mode LLM 미호출 (M1, 보강 M3)

**Given (M1)**
- `config.emotion_llm_assisted=true` (M3 시점에는 의미 있음, M1 시점에는 무관).
- entry 의 `PrivateMode=true`.
- LLM mock 등록.

**When (M1)**
- `writer.Write(ctx, JournalEntry{UserID:"u1", Date:today, Text:"private text", PrivateMode:true})` 호출.

**Then (M1)**
- LLM mock counter == 0 (M1 은 `config` 무관 unconditional skip).
- `vad`/`emotion_tags` 가 LocalDictAnalyzer 결과.

**Then (M3 보강)**
- `config.emotion_llm_assisted=true` + `PrivateMode=false` → LLM mock counter == 1.
- `config.emotion_llm_assisted=true` + `PrivateMode=true` → LLM mock counter == 0 (M3 분기 enforce).

**Test**
- File: `internal/ritual/journal/writer_test.go` (M1) + `internal/ritual/journal/analyzer_llm_test.go` (M3)
- Function: `TestWriter_PrivateMode_LocalOnly` (M1) + `TestLLMAnalyzer_PrivateMode_NeverCalled` (M3)

---

## AC-JOURNAL-013 — 파일 권한 0600/0700 (M1)

**Given**
- 임시 디렉토리 (`t.TempDir()`).
- `config.journal.dataDir = tmpDir/journal`.

**When**
- `writer.Write(ctx, JournalEntry{...})` 첫 호출 (storage 가 lazy init).

**Then**
- `os.Stat(tmpDir+"/journal")` mode == `0700` (다른 bit 0).
- `os.Stat(tmpDir+"/journal/journal.db")` mode == `0600`.
- WAL 파일 (`journal.db-wal`, `journal.db-shm`) 존재 시 mode == `0600`.

**Edge — 디렉토리 이미 존재 (잘못된 권한)**
- 기존 mode `0755` 디렉토리 → writer init 시 `os.Chmod(0700)` 강제 (또는 error 반환). 결정: error 반환 후 user 에게 수동 수정 안내 (silent chmod 는 보안 risk).

**Test**
- File: `internal/ritual/journal/storage_test.go`
- Function: `TestStorage_FilePermissions_0600_0700`

---

## AC-JOURNAL-014 — 저녁 프롬프트 이미 기록된 날 skip (M1)

**Given**
- 오늘 날짜에 대한 `StoredEntry` 가 이미 storage 에 존재 (`u1`, `created_at = today 21:00`).
- `config.prompt_timeout_min=60`.
- HOOK-001 dispatcher 등록.

**When**
- HOOK-001 이 `EveningCheckInTime` 을 dispatch (today 22:00).

**Then**
- 사용자에게 prompt 미노출 (UI mock 의 prompt counter == 0).
- INFO log 1건, `operation=evening_prompt_skip`.
- 프롬프트 대기 타이머 미가동 (orchestrator 의 wait goroutine 부재).

**And Given (timeout 시나리오)**
- 오늘 entry 미존재 + prompt emit 후 60분 동안 사용자 응답 없음.

**Then (timeout)**
- 60분 후 INFO log `operation=evening_prompt_timeout`.
- storage 에 새 entry 저장 안 됨.

**Test**
- File: `internal/ritual/journal/orchestrator_test.go`
- Function: `TestOrchestrator_TodayEntryExists_SkipPrompt` + `TestOrchestrator_TimeoutWithoutResponse`

---

## AC-JOURNAL-015 — 연속 저가 Valence 시 소프트 톤 (M1)

**Given**
- u1 의 최근 저장 3건 (`ORDER BY created_at DESC`) 의 `Vad.Valence` 가 각각 `0.20`, `0.25`, `0.28`.
- 오늘 entry 미존재.
- `config.enabled=true`.

**When**
- `orchestrator.Prompt(ctx, "u1")` 호출.

**Then**
- 렌더된 prompt 문자열에 `"언제든 이야기해주세요"` AND `"전문가 상담"` 모두 포함.
- prompt 가 `prompts.PickLowMood()` 결과.
- 진단성 어휘 `"진단"`, `"우울증"`, `"PHQ"`, `"장애"`, `"치료"` 부재.

**Edge — 최근 3건 중 1건만 valence < 0.3**
- `[0.20, 0.50, 0.60]` → low mood 분기 비활성, neutral prompt 사용.

**Edge — entry 가 3건 미만**
- 1건만 저장 → low mood 분기 평가 skip, neutral prompt 사용.

**Test**
- File: `internal/ritual/journal/orchestrator_test.go`
- Function: `TestOrchestrator_LowMoodSoftTone`

---

## AC-JOURNAL-016 — AllowLoRATraining 기본 false (M1)

**Given**
- `config.journal.allow_lora_training` 미지정 (default false).
- `config.enabled=true`.

**When**
- `writer.Write(ctx, JournalEntry{...})` 호출.

**Then**
- 반환 `*StoredEntry` 의 `AllowLoRATraining == false`.
- LORA-001 의 training dataset exporter mock 호출 시 본 entry 미포함 (M1 시점에는 mock 검증만; LORA-001 미구현 시 정적 unit test 만).

**Edge — config 명시적 true**
- `config.allow_lora_training=true` → `StoredEntry.AllowLoRATraining=true`.

**Test**
- File: `internal/ritual/journal/writer_test.go`
- Function: `TestWriter_AllowLoRATraining_DefaultFalse`

---

## AC-JOURNAL-017 — Retention 기반 자동 삭제 (M1)

**Given**
- `config.journal.retention_days=30`.
- storage 에 `created_at = now-40d` entry 1건, `created_at = now-10d` entry 1건.
- nightly cleanup job (03:00 local) trigger 가능.

**When**
- cleanup job 수동 trigger (`storage.RunCleanup(ctx)`).

**Then**
- 40일 전 entry 가 SQLite 에서 hard delete (`writer.ListByDate(ctx, u1, ...)` 에 미포함).
- 10일 전 entry 는 유지.
- audit log 1 line per deletion, `operation="retention_cleanup"`.

**Edge — `retention_days=-1` (default)**
- 동일 시나리오에서 두 entry 모두 유지, cleanup job no-op.

**Edge — `retention_days=0`**
- 모든 entry 삭제 (boundary case). 의도된 동작 (사용자가 0 설정 시 즉시 삭제 의도).

**Test**
- File: `internal/ritual/journal/storage_test.go`
- Function: `TestStorage_RetentionDays_NightlyCleanup`

---

## AC-JOURNAL-018 — MEMORY 불가 시 버퍼링 + ErrPersistFailed (M1)

**Given**
- MEMORY-001 / SQLite mock 이 `Insert` 호출 시 error 반환하도록 설정 (e.g., disk I/O error).
- `config.enabled=true`.
- in-memory queue max 10.

**When**
- `writer.Write(ctx, entry)` 를 연속 3회 호출 (동일 entry, 또는 서로 다른 3 entry).

**Then**
- (a) 1~2회차는 내부 큐 (max 10) 에 buffer + retry (writer 의 retry max 3회).
- (b) 3회차 최종 실패 후 `ErrPersistFailed` 반환.
- (c) 사용자에게 `"일기 저장 실패, 다시 시도해주세요"` 메시지 전달.
- (d) 프로세스 종료 (SIGTERM) 후 재시작 시 큐가 비어 있음 (디스크 영속화 없음 — silent leakage 방지).

**Edge — 큐 만석 (10건 buffered)**
- 11번째 entry 호출 시 oldest entry evict + 새 entry buffer (LRU). audit log 1 line `operation="queue_evict"`.

**Test**
- File: `internal/ritual/journal/writer_test.go`
- Function: `TestWriter_PersistRetry_AndErr`

---

## AC-JOURNAL-019 — 중립 프롬프트 강제 (M1)

**Given**
- `prompts.All()` 가 모든 등록된 프롬프트 템플릿 (`neutral` + `low_mood_sequence` + `anniversary_*`) 슬라이스 반환.

**When**
- 슬라이스 각 element 를 검사.

**Then**
- 어떤 템플릿도 다음 금지 구문 미포함: `"가장 큰 비밀"`, `"서운한 점"`, `"숨기고 싶은"`, `"부끄러운"`, `"가장 후회"`.
- 모든 템플릿이 open question (`?` 또는 `？` 로 끝남).
- 모든 템플릿 길이 ≤ 100 char.

**Edge — 빈 vault**
- `prompts.All()` 가 빈 슬라이스 반환 → test fail (vault 가 적어도 1개 카테고리 보유 enforce).

**Test**
- File: `internal/ritual/journal/prompts_test.go`
- Function: `TestPrompts_AllNeutral_NoForbiddenPhrase` + `TestPrompts_AllOpenQuestion`

---

## AC-JOURNAL-020 — LLM 프롬프트 payload 제약 (M3)

**Given (M3)**
- `config.emotion_llm_assisted=true`.
- entry `{UserID:"u1", Date:today, Text:"오늘 좋았다", PrivateMode:false, AttachmentPaths:["/path/a.jpg"], Anniversary:nil}`.
- LLM adapter mock 가 invoke payload capture.

**When (M3)**
- `writer.Write(ctx, entry)` 호출 시 LLM 호출 캡처.

**Then (M3)**
- LLM 호출 1회.
- system prompt string 에 다음 두 문구 모두 포함:
  - `"VAD 모델로 분석하고 Top-3 감정 태그를 JSON으로 반환"`
  - `"분석 결과 외에 어떤 조언이나 해석도 포함하지 마세요"`
- user payload (또는 messages 배열의 user role content) 에 `entry.Text` 만 포함, 다음 필드는 **부재**:
  - `user_id` 또는 `UserID` 또는 `"u1"`
  - `date` 또는 `today.Format(...)` 결과 string
  - `attachment_paths` 또는 `"/path/a.jpg"`
  - `anniversary` 또는 `null` (단순 부재 검증)
  - `emoji_mood`, `private_mode`, `allow_lora_training` 모두 부재

**Edge — JSON 응답 parse 실패**
- LLM 응답이 invalid JSON → silent fallback to LocalDictAnalyzer (사용자 가시 에러 없음). audit log 1 line `outcome="llm_parse_fail"`.

**Test**
- File: `internal/ritual/journal/analyzer_llm_test.go`
- Function: `TestLLMAnalyzer_PayloadIsTextOnly`

---

## AC-JOURNAL-021 — 주간 요약 생성 cadence (M2)

**Given (M2)**
- `config.weekly_summary=true`.
- 가상 시계 (`clock.Mock`) 를 일요일 22:00 으로 설정.
- 지난 7일 간 entry 5건 저장.

**When (M2)**
- weekly summary job trigger (`summary.RunWeekly(ctx, u1)`).

**Then (M2)**
- 생성된 summary 객체에 다음 필드 포함:
  - 지난 7일 평균 `Valence` (실제 5건의 산술 평균).
  - 빈도 상위 3 emotion tag.
  - wordcloud 상위 토큰 리스트 (10+ tokens).
- 다음 저녁 prompt 렌더 시 summary 제시 flag (`pendingSummaryFlag=true`) 활성.
- audit log 1 line `operation="weekly_summary_generated"`.

**Edge — `weekly_summary=false`**
- 동일 시나리오에서 summary 미생성, `pendingSummaryFlag=false`, audit log 0.

**Edge — 지난 7일 entry 0건**
- summary 객체에 `entry_count=0`, summary 미렌더 (사용자에게 노출 안 함).

**Test**
- File: `internal/ritual/journal/summary_test.go`
- Function: `TestWeeklySummary_SundayCadence_Generates`

---

## AC-JOURNAL-022 — INSIGHTS 연동 (M1)

**Given**
- INSIGHTS-001 mock 등록 (`OnJournalEntry(entry *StoredEntry)` callback counter + 인자 capture).
- `config.enabled=true`.

**When**
- `writer.Write(ctx, JournalEntry{UserID:"u1", ...})` 성공.

**Then**
- `insights.OnJournalEntry` mock 정확히 1회 호출.
- 전달된 `entry` 의 `ID`, `Vad`, `EmotionTags` 가 storage 에 저장된 `StoredEntry` 와 동일.

**Edge — INSIGHTS 미등록**
- INSIGHTS mock 없이 호출 시 `Write` 정상 성공, callback 호출 0회.

**Edge — INSIGHTS 콜백이 panic**
- `OnJournalEntry` 가 panic → writer 가 recover + audit log `outcome="insights_callback_panic"` + Write 정상 반환 (downstream consumer panic 이 user-facing 에러로 전파 안 됨).

**Test**
- File: `internal/ritual/journal/writer_test.go`
- Function: `TestWriter_INSIGHTSCallback_OnSuccess`

---

## AC-JOURNAL-023 — 진단/조언 금지 (M1, 보강 M3)

**Given (M1)**
- 두 시나리오:
  - (a) crisis 키워드 포함 input (`"죽고 싶다"`).
  - (b) 저가 valence 입력 (`"너무 우울해서 아무것도 못하겠다"`).

**When (M1)**
- 시스템 응답 (writer.Write 의 user-facing message + orchestrator 의 prompt 응답) 수집.

**Then (M1)**
- 두 경우 모두 응답 텍스트가 `crisisResponse` (§6.4) literal 와 **완전 일치** 또는 빈 문자열 (기본 prompt).
- 응답에 다음 임상/지시적 어휘 부재: `"진단"`, `"우울증"`, `"우울"`, `"장애"`, `"처방"`, `"상담받으세요"`, `"치료"`, `"증상"`, `"평가"`.
- LLM adapter mock 호출 0회.

**Then (M3 보강)**
- `config.emotion_llm_assisted=true` 시점에도 crisis entry 는 LLM 호출 0회 (`TestLLMAnalyzer_NeverCalledOnCrisis`).
- LLM 응답이 위 임상 어휘 포함 시 silent reject + LocalDictAnalyzer fallback (사용자에게 임상 어휘 노출 0).

**Test**
- File: `internal/ritual/journal/crisis_test.go` (M1) + `internal/ritual/journal/analyzer_llm_test.go` (M3)
- Function: `TestCrisis_NoClinicalLanguage` + `TestLLMAnalyzer_NeverCalledOnCrisis`

---

## AC-JOURNAL-024 — Search FTS5 사용자 스코프 (M2)

**Given (M2)**
- u1 entry 20건 (그 중 5건 text 에 `"산책"` 포함).
- u2 entry 10건 (그 중 3건 `"산책"` 포함).
- FTS5 mirror table populated.

**When (M2)**
- `writer.Search(ctx, "u1", "산책")` 호출.

**Then (M2)**
- 반환 entry 수 == 5.
- 모든 반환 entry 의 `UserID == "u1"`.
- u2 entry 0건 포함.
- 결과 순서가 FTS5 `rank` 기준 내림차순 (동률 시 `created_at DESC` tiebreak).

**Edge — query 가 SQL injection attempt**
- `writer.Search(ctx, "u1", "'; DROP TABLE journal_entries; --")` → FTS5 가 quoted string 처리, 0건 반환, table 손상 없음.

**Edge — query 가 빈 string**
- `writer.Search(ctx, "u1", "")` → `ErrInvalidQuery` 반환.

**Test**
- File: `internal/ritual/journal/search_test.go`
- Function: `TestSearch_FTS5_UserScoped`

---

## AC-JOURNAL-025 — WeeklyTrend/MonthlyTrend 집계 (M2)

**Given (M2)**
- u1, 지난 7일 중 6일에 entry 1건씩 저장 (`Valence` = 0.2, 0.4, 0.6, 0.5, 0.7, 0.8).
- 중간 1일 (e.g., day 4) 은 entry 없음.

**When (M2)**
- `trend.WeeklyTrend(ctx, "u1", today)` 호출.

**Then (M2)**
- 반환 `*Trend` 의:
  - `Period == "week"`.
  - `From` ~ `To` = 7일 윈도우.
  - `EntryCount == 6`.
  - `AvgValence` 가 실제 저장 6건의 산술 평균 == `(0.2+0.4+0.6+0.5+0.7+0.8)/6 == 0.5333...`.
  - `SparklinePoints` 길이 == 7.
  - entry 없는 day 의 해당 index `math.IsNaN(SparklinePoints[i]) == true`.
  - `MoodDistribution` map 합계 == `EntryCount` (즉 6).

**Edge — 7일 모두 entry 없음**
- `EntryCount=0`, `AvgValence=NaN` (또는 0 — 결정: NaN 반환), `SparklinePoints` 모두 NaN.

**Test**
- File: `internal/ritual/journal/trend_test.go`
- Function: `TestWeeklyTrend_AggregationWithGaps`

---

## AC-JOURNAL-026 — RenderChart 출력 (M2)

**Given (M2)**
- `Trend.SparklinePoints = [0.2, 0.5, 0.9, NaN, 0.3, 0.7, 0.6]`.
- `From..To` = 7일 (월~일).
- 환경변수 `NO_COLOR=""` (color 허용).

**When (M2)**
- `chart.RenderChart(trend)` 호출 후 출력 string capture.

**Then (M2)**
- 출력은 7줄 (요일 label 포함).
- 각 줄의 막대 부분은 `{▁▂▃▄▅▆▇█}` 집합에 속하는 정확히 1개 글리프 (NaN 제외).
- NaN 인 day 는 공백 또는 `·` 로 표기.
- 환경변수 `NO_COLOR=""` 일 때 ANSI color escape sequence (`\x1b[`) 포함 허용.

**Edge — `NO_COLOR=1`**
- 동일 input 에서 ANSI color escape sequence (`\x1b[`) occurrence count == 0.

**Edge — 빈 SparklinePoints**
- 빈 슬라이스 → 빈 string 반환 (또는 `"(데이터 없음)"`). 결정: 빈 string + audit log warn.

**Test**
- File: `internal/ritual/journal/chart_test.go`
- Function: `TestRenderChart_SevenDaysWithNaN`

---

## 종합 Definition of Done

### M1 DoD

- [ ] AC-JOURNAL-001 / 002 / 003 / 004 / 005 / 008 / 009 / 010 / 011 / 012 / 013 / 014 / 015 / 016 / 017 / 018 / 019 / 022 / 023 (19 AC) 모두 GREEN.
- [ ] 각 AC 의 edge case 별도 test 분리되어 GREEN.
- [ ] `internal/ritual/journal/*.go` coverage ≥ 85%.
- [ ] `golangci-lint run ./internal/ritual/journal/...` 0 warning.
- [ ] `go vet ./internal/ritual/journal/...` 0 issue.
- [ ] integration test (`integration_test.go` build tag): HOOK dispatch → orchestrator prompt → writer → audit 1회 GREEN.
- [ ] e2e: AUDIT-001 + PERMISSION-001 + HOOK-001 통합 시나리오 1회 GREEN.
- [ ] AUDIT-001 의 EventType 카탈로그에 `EventTypeRitualJournalInvoke` 추가 후 회귀 0.
- [ ] `.moai/docs/journal-quickstart.md` 작성.
- [ ] 외부 의존성 신규 0 (T-022 검증 후).
- [ ] `crisis_keywords.golden.txt` 의 모든 키워드가 `TestCrisis_DirectKeyword_Match` table-driven 으로 검증.
- [ ] `prompts.golden.yaml` 의 모든 템플릿이 `TestPrompts_AllNeutral_NoForbiddenPhrase` + `TestPrompts_AllOpenQuestion` 통과.

### M2 DoD

- [ ] AC-JOURNAL-006 / 007 / 021 / 024 / 025 / 026 (6 AC) 모두 GREEN.
- [ ] M2 신규 production 파일 6개 (recall/anniversary/trend/chart/search/summary) coverage ≥ 85%.
- [ ] FTS5 query 가 100 entry 미만에서 100ms 이내 응답 (벤치마크).
- [ ] Anniversary 자동 회상이 valence < 0.3 entry 자동 제외 (R6).
- [ ] M1 19 AC 회귀 0.

### M3 DoD

- [ ] AC-JOURNAL-020 (LLM payload 제약) GREEN.
- [ ] AC-JOURNAL-002 / 012 / 023 의 LLM 분기 보강 GREEN.
- [ ] LLM-ROUTING-V2 client 통합 + journal 전용 model 선택 정상 동작.
- [ ] M1+M2 25 AC 회귀 0.

---

## 품질 게이트 (TRUST 5 매핑)

- **Tested**: 26 AC + edge case + integration + e2e (커버리지 ≥ 85%). M1 단계는 19 AC + edge case 우선.
- **Readable**: journal 도메인 godoc 영문 (code comments 영문 정책 §2.5 정렬), 명확한 에러 코드 (`ErrJournalDisabled`, `ErrPersistFailed`, `ErrInvalidUserID`, `ErrInvalidQuery`), 표준 응답 wrapper.
- **Unified**: 모든 storage access 가 `WHERE user_id = ?` strict filter, 모든 audit emit 가 동일 EventType + meta key 스키마, 모든 LLM 호출 (M3) 이 동일 payload 제약.
- **Secured**: opt-in default off (REQ-001), 로컬 only (REQ-002), A2A 금지 (REQ-014), 로그 redaction (REQ-004 / AC-008), crisis canned response 진단 금지 (REQ-020 / AC-023), file permission 0600/0700 (REQ-002 / AC-013), hard delete (REQ-016 / AC-011), LLM payload 제약 M3 (REQ-017 / AC-020).
- **Trackable**: 모든 변경 구조화 로그 (entry text 제외), export/delete 가 audit 1 line 보장, REQ ↔ AC ↔ Test function 매핑 §8 (plan.md) 명시, REQ 번호 0.2.0 부터 불변.

---

Version: 0.1.0
Last Updated: 2026-05-12
